// Package checks runs the project's own build/test commands and records the
// result. PASSED comes only from exit codes; only commands produced by scan
// are ever executed (allowlist).
package checks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

const maxOutput = 6000

// StepTimeout bounds one command.
var StepTimeout = 10 * time.Minute

// Run executes cmds in order and stops at the first failure.
func Run(ctx context.Context, dir string, cmds []core.Command, taskID string) core.BuildRun {
	run := core.BuildRun{ID: fmt.Sprintf("BUILD-%d", time.Now().UnixNano()), TaskID: taskID, At: time.Now().UTC(), Passed: true}
	if len(cmds) == 0 {
		run.Passed = false
		run.Reason = "Tidak ada command build/test yang terdeteksi di project"
		return run
	}
	for _, c := range cmds {
		st := runOne(ctx, dir, c)
		run.Steps = append(run.Steps, st)
		if st.ExitCode != 0 {
			run.Passed = false
			run.Reason = fmt.Sprintf("%s gagal (%s)", c.Name, st.Command)
			break
		}
	}
	return run
}

func runOne(ctx context.Context, dir string, c core.Command) core.StepResult {
	ctx, cancel := context.WithTimeout(ctx, StepTimeout)
	defer cancel()
	st := core.StepResult{Name: c.Name, Command: strings.Join(c.Argv, " "), Dir: c.Dir}
	start := time.Now()
	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Dir = filepath.Join(dir, c.Dir)
	cmd.Env = append(os.Environ(), "CI=true") // one-shot test runners, no watch mode
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	st.Millis = time.Since(start).Milliseconds()
	st.Output = tail(out.String())
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		st.ExitCode = ee.ExitCode()
		if st.ExitCode == 0 {
			st.ExitCode = -1
		}
	default:
		st.ExitCode = -1
		st.Output = tail(st.Output + "\n" + err.Error())
	}
	if ctx.Err() == context.DeadlineExceeded {
		st.ExitCode = -1
		st.Output = tail(st.Output + "\ntimeout")
	}
	return st
}

func tail(s string) string {
	if len(s) > maxOutput {
		return "…" + s[len(s)-maxOutput:]
	}
	return s
}

// Snapshot records file modification times to detect what an agent changed.
type Snapshot map[string]time.Time

var skip = map[string]bool{"node_modules": true, ".git": true, core.StateDir: true, "dist": true, "vendor": true, ".next": true}

func Take(dir string) Snapshot {
	s := Snapshot{}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if info, err := d.Info(); err == nil {
			rel, _ := filepath.Rel(dir, path)
			s[rel] = info.ModTime()
		}
		return nil
	})
	return s
}

// Changed lists files created or modified since before.
func Changed(dir string, before Snapshot) []string {
	var out []string
	for rel, t := range Take(dir) {
		if bt, ok := before[rel]; !ok || !bt.Equal(t) {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}

// Excerpt renders changed files for a reviewer, bounded in size.
func Excerpt(dir string, files []string, budget int) string {
	var b strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		chunk := string(data)
		if len(chunk) > 4000 {
			chunk = chunk[:4000] + "\n…(dipotong)"
		}
		entry := fmt.Sprintf("--- %s\n%s\n", f, chunk)
		if b.Len()+len(entry) > budget {
			fmt.Fprintf(&b, "--- %s (tidak ditampilkan, batas ukuran)\n", f)
			continue
		}
		b.WriteString(entry)
	}
	if b.Len() == 0 {
		return "(tidak ada file yang berubah)"
	}
	return b.String()
}
