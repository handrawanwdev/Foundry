//go:build unix

package pipeline_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

func exited(pid int) bool {
	if syscall.Kill(pid, 0) == syscall.ESRCH {
		return true
	}
	out, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	st := strings.TrimSpace(string(out))
	return st == "" || strings.HasPrefix(st, "Z")
}

// Shutdown cancels a running agent build: the agent and the server it
// started are killed, and the job and task are left in a state a restart
// can continue from.
func TestShutdownStopsRunningJobs(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep tidak ada")
	}
	s := newService(t, fakeAI{}, "hang")
	pids := filepath.Join(t.TempDir(), "pids")
	t.Setenv("FAKE_PIDS", pids)
	resetTask(t, s)
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	var agentPid, childPid int
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if b, err := os.ReadFile(pids); err == nil {
			fmt.Sscan(string(b), &agentPid, &childPid)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("agent did not start")
		}
	}
	t.Cleanup(func() { syscall.Kill(childPid, syscall.SIGKILL) })

	if err := s.Shutdown(15 * time.Second); err != nil {
		t.Fatal(err)
	}
	for _, pid := range []int{agentPid, childPid} {
		for deadline := time.Now().Add(10 * time.Second); !exited(pid); time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("process %d still running after shutdown", pid)
			}
		}
	}
	p := load(t, s)
	mustJob(t, p, "build", core.JobDone)
	if p.Job.Message != "Build dihentikan" {
		t.Fatalf("job = %+v", p.Job)
	}
	if tk := p.Task("T1"); tk.Status != core.TaskTodo {
		t.Fatalf("task = %s, want TODO", tk.Status)
	}
}

// Shutdown returns at once when nothing runs.
func TestShutdownIdle(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if err := s.Shutdown(time.Second); err != nil {
		t.Fatal(err)
	}
}
