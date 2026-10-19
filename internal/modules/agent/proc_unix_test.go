//go:build unix

package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// exited reports whether pid is gone or a zombie not reaped yet.
func exited(pid int) bool {
	if syscall.Kill(pid, 0) == syscall.ESRCH {
		return true
	}
	out, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	st := strings.TrimSpace(string(out))
	return st == "" || strings.HasPrefix(st, "Z")
}

func waitExited(t *testing.T, pid int) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !exited(pid); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("agent child %d still running", pid)
		}
	}
}

func readPid(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return pid
}

// A dev server the agent left running dies with the run.
func TestRunKillsAgentLeftover(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	pidFile := filepath.Join(t.TempDir(), "pid")
	r, _ := NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; sleep 300 >/dev/null 2>&1 & echo $! > " + pidFile + "; echo done"}, Options{})
	if out, err := r.Run(context.Background(), t.TempDir(), "hi"); err != nil || out != "done\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	waitExited(t, readPid(t, pidFile))
}

// Stopping a running agent stops what it started too.
func TestCancelKillsAgentChildren(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	dir := t.TempDir()
	pidFile, fifo := filepath.Join(dir, "pid"), filepath.Join(dir, "started")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	r, _ := NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; sleep 300 & echo $! > " + pidFile + "; echo > " + fifo + "; wait"}, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := r.Run(ctx, t.TempDir(), "hi")
		done <- err
	}()
	f, err := os.Open(fifo) // returns once the agent wrote its child's pid
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	pid := readPid(t, pidFile)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled run reported success")
	}
	waitExited(t, pid)
}
