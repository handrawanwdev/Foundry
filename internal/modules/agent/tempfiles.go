package agent

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/handrawanwdev/foundry/internal/proc"
)

// A run that hands its prompt to the CLI as a file (system prompt, prompt
// file) writes it into a folder of its own under TempRoot, named after the
// Foundry process that owns it: run-<pid>-<random>. The folder goes when the
// run returns, whatever the outcome (answer, error, timeout, cancel, panic).
// A process that is killed cannot clean up after itself, so the next Foundry
// process sweeps what a dead process left and anything past TempTTL. Only
// those names are ever removed: never a project, source or user file.

// TempTTL is the age after which a run folder is removed even when its
// process still lives. A run never lasts that long: Runner.Timeout is 30
// minutes.
const TempTTL = 6 * time.Hour

const runPrefix = "run-"

// Legacy file patterns: versions before run folders wrote straight into the
// system temp folder. They carry no owner, so only TempTTL removes them.
var legacyTemp = []string{"foundry-system-*.txt", "foundry-prompt-*.txt"}

// TempRoot is the folder that holds every run folder.
func TempRoot() string { return filepath.Join(os.TempDir(), "foundry-agent") }

// TempStats counts the cleanups since the process started.
type TempStats struct {
	Removed int64 `json:"removed"` // run folders and legacy files removed
	Failed  int64 `json:"failed"`  // removals that failed (logged)
}

var (
	tempRemoved, tempFailed atomic.Int64
	sweepOnce               sync.Once
)

// Stats returns the cleanup counters.
func Stats() TempStats {
	return TempStats{Removed: tempRemoved.Load(), Failed: tempFailed.Load()}
}

// newRunDir creates the private folder of one run. The first call of a
// process sweeps what earlier processes left behind.
func newRunDir() (string, error) {
	sweepOnce.Do(func() { SweepTemp(time.Now(), TempTTL) })
	root := TempRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return os.MkdirTemp(root, fmt.Sprintf("%s%d-", runPrefix, os.Getpid()))
}

// removeRunDir removes one run folder. It is idempotent: a folder already
// gone counts as removed.
func removeRunDir(dir string) {
	if dir == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		tempFailed.Add(1)
		log.Printf("agent: hapus folder sementara %s gagal: %v", dir, err)
		return
	}
	tempRemoved.Add(1)
}

// runOwner is the pid in a run folder name; 0 when the name is no run folder.
func runOwner(name string) int {
	rest, ok := strings.CutPrefix(name, runPrefix)
	if !ok {
		return 0
	}
	pid, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(pid)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// SweepTemp removes the run folders whose process is gone and those older
// than ttl, plus legacy temp files older than ttl. A folder of a live process
// younger than ttl is never touched. It returns how many entries it removed.
func SweepTemp(now time.Time, ttl time.Duration) int {
	removed := 0
	entries, err := os.ReadDir(TempRoot())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		tempFailed.Add(1)
		log.Printf("agent: baca %s gagal: %v", TempRoot(), err)
	}
	for _, e := range entries {
		pid := runOwner(e.Name())
		if pid == 0 || !e.IsDir() {
			continue // not ours
		}
		old := false
		if info, err := e.Info(); err == nil {
			old = now.Sub(info.ModTime()) > ttl
		}
		if !old && proc.Alive(pid) {
			continue
		}
		if removeEntry(filepath.Join(TempRoot(), e.Name())) {
			removed++
		}
	}
	for _, pattern := range legacyTemp {
		files, _ := filepath.Glob(filepath.Join(os.TempDir(), pattern))
		for _, f := range files {
			info, err := os.Lstat(f)
			if err != nil || !info.Mode().IsRegular() || now.Sub(info.ModTime()) <= ttl {
				continue
			}
			if removeEntry(f) {
				removed++
			}
		}
	}
	if removed > 0 {
		log.Printf("agent: %d file/folder sementara sisa run sebelumnya dihapus", removed)
	}
	return removed
}

func removeEntry(path string) bool {
	if err := os.RemoveAll(path); err != nil {
		tempFailed.Add(1)
		log.Printf("agent: hapus %s gagal: %v", path, err)
		return false
	}
	tempRemoved.Add(1)
	return true
}

// RemoveOwnTemp removes every run folder of this process. It is for
// shutdown, after the jobs were stopped: a run cut off by the shutdown
// timeout does not get to run its own cleanup.
func RemoveOwnTemp() int {
	entries, _ := os.ReadDir(TempRoot())
	removed := 0
	for _, e := range entries {
		if e.IsDir() && runOwner(e.Name()) == os.Getpid() && removeEntry(filepath.Join(TempRoot(), e.Name())) {
			removed++
		}
	}
	return removed
}
