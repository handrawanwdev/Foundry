package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// isolateTemp points the system temp folder at a fresh one, so a test sees
// only its own run folders, never those of a live Foundry or another test.
func isolateTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	if os.TempDir() != dir {
		t.Skip("TMPDIR tidak dipakai os.TempDir di platform ini")
	}
	return dir
}

// runDirs lists the run folders under TempRoot.
func runDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(TempRoot())
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// sysScript is a CLI that reads its system prompt file (--sys <file>) after
// an optional delay and exits with the given status.
func sysRunner(script string, timeout time.Duration) *Runner {
	return &Runner{argv: []string{"sh", "-c", script, "sh"}, lean: []string{"--lean"}, noLean: new(atomic.Bool),
		systemFile: "--sys", Timeout: timeout}
}

const readSys = `f=""; prev=""; for a in "$@"; do [ "$prev" = --sys ] && f="$a"; prev="$a"; done; cat >/dev/null; `

func needSh(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
}

// The run folder is removed whatever way the run ends.
func TestRunFolderRemovedOnEveryOutcome(t *testing.T) {
	needSh(t)
	cases := []struct {
		name    string
		script  string
		timeout time.Duration
		ctx     func() (context.Context, context.CancelFunc)
		wantErr bool
	}{
		{name: "success", script: readSys + `cat "$f"`, timeout: time.Minute},
		{name: "failure", script: readSys + `echo boom >&2; exit 3`, timeout: time.Minute, wantErr: true},
		{name: "timeout", script: readSys + `sleep 5`, timeout: 200 * time.Millisecond, wantErr: true},
		{name: "cancel", script: readSys + `sleep 5`, timeout: time.Minute, wantErr: true,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 200*time.Millisecond)
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isolateTemp(t)
			ctx, cancel := context.Background(), func() {}
			if c.ctx != nil {
				ctx, cancel = c.ctx()
			}
			defer cancel()
			before := Stats()
			res, err := sysRunner(c.script, c.timeout).Exec(ctx, t.TempDir(), Prompt{Static: "rules", Body: "task"})
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v", err)
			}
			if !c.wantErr && res.Text != "rules" {
				t.Fatalf("system file not read: %q", res.Text)
			}
			if left := runDirs(t); len(left) > 0 {
				t.Fatalf("left behind: %v", left)
			}
			if Stats().Removed <= before.Removed {
				t.Fatal("removal not counted")
			}
		})
	}
}

// A panic while the answer is read still removes the run folder.
func TestRunFolderRemovedOnPanic(t *testing.T) {
	needSh(t)
	isolateTemp(t)
	r := sysRunner(readSys+`echo out`, time.Minute)
	r.unwrap = func(string) (string, bool) { panic("unwrap") }
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("no panic")
			}
		}()
		r.Exec(context.Background(), t.TempDir(), Prompt{Static: "rules", Body: "task"})
	}()
	if left := runDirs(t); len(left) > 0 {
		t.Fatalf("left behind: %v", left)
	}
}

// Concurrent runs each get their own folder; a sweep running alongside
// never removes the folder of a live run.
func TestConcurrentRunsAndSweep(t *testing.T) {
	needSh(t)
	isolateTemp(t)
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				SweepTemp(time.Now(), TempTTL)
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			want := fmt.Sprintf("rules-%d", i)
			res, err := sysRunner(readSys+`sleep 0.3; cat "$f"`, time.Minute).Exec(context.Background(), t.TempDir(), Prompt{Static: want, Body: "task"})
			if err == nil && res.Text != want {
				err = fmt.Errorf("run %d read %q", i, res.Text)
			}
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(stop)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if left := runDirs(t); len(left) > 0 {
		t.Fatalf("left behind: %v", left)
	}
}

// deadPID is the pid of a process that already exited.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

// The sweep removes what a dead process and old runs left, and nothing
// else: not a live run, not a young legacy file, not a foreign name.
func TestSweepTemp(t *testing.T) {
	needSh(t)
	tmp := isolateTemp(t)
	root := TempRoot()
	mk := func(name string, age time.Duration) string {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(p, "system.txt"), []byte("x"), 0o600)
		ts := time.Now().Add(-age)
		os.Chtimes(p, ts, ts)
		return p
	}
	file := func(name string, age time.Duration) string {
		p := filepath.Join(tmp, name)
		os.WriteFile(p, []byte("x"), 0o600)
		ts := time.Now().Add(-age)
		os.Chtimes(p, ts, ts)
		return p
	}
	live := mk(fmt.Sprintf("run-%d-1", os.Getpid()), time.Minute)
	dead := mk(fmt.Sprintf("run-%d-2", deadPID(t)), time.Minute)
	stale := mk(fmt.Sprintf("run-%d-3", os.Getpid()), TempTTL+time.Hour)
	foreign := mk("notes", 48*time.Hour)
	bad := mk("run-abc-4", 48*time.Hour)
	legacyOld := file("foundry-system-1.txt", TempTTL+time.Hour)
	legacyNew := file("foundry-prompt-2.txt", time.Minute)
	other := file("other.txt", 48*time.Hour)

	if got := SweepTemp(time.Now(), TempTTL); got != 3 {
		t.Fatalf("removed %d, want 3", got)
	}
	for _, p := range []string{dead, stale, legacyOld} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still there", p)
		}
	}
	for _, p := range []string{live, foreign, bad, legacyNew, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s removed: %v", p, err)
		}
	}
	// Idempotent: a second sweep finds nothing more.
	if got := SweepTemp(time.Now(), TempTTL); got != 0 {
		t.Fatalf("second sweep removed %d", got)
	}
	if got := RemoveOwnTemp(); got != 1 {
		t.Fatalf("own removed %d, want 1", got)
	}
	if _, err := os.Stat(live); !os.IsNotExist(err) {
		t.Fatal("own run folder kept")
	}
}

// A Foundry process killed mid-run (SIGKILL: no defer runs) leaves its run
// folder; the next process sweeps it.
func TestSweepAfterKilledProcess(t *testing.T) {
	needSh(t)
	if os.Getenv("FOUNDRY_TEMP_HELPER") == "1" {
		r := sysRunner(readSys+`echo started; sleep 2`, time.Minute)
		r.Exec(context.Background(), os.TempDir(), Prompt{Static: "rules", Body: "task"})
		return
	}
	tmp := isolateTemp(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestSweepAfterKilledProcess$")
	cmd.Env = append(os.Environ(), "FOUNDRY_TEMP_HELPER=1", "TMPDIR="+tmp)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for len(runDirs(t)) == 0 {
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			t.Fatal("helper run never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := cmd.Process.Kill(); err != nil { // SIGKILL on unix
		t.Fatal(err)
	}
	err := cmd.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("helper not killed: %v", err)
	}
	left := runDirs(t)
	if len(left) != 1 || !strings.HasPrefix(left[0], "run-"+strconv.Itoa(cmd.Process.Pid)+"-") {
		t.Fatalf("before sweep: %v (pid %d)", left, cmd.Process.Pid)
	}
	if got := SweepTemp(time.Now(), TempTTL); got != 1 {
		t.Fatalf("sweep removed %d", got)
	}
	if left := runDirs(t); len(left) > 0 {
		t.Fatalf("after sweep: %v", left)
	}
}
