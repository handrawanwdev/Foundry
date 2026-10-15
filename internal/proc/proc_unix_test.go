//go:build unix

package proc

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// gone reports whether pid has exited: no such process, or a zombie its
// new parent has not reaped yet.
func gone(pid int) bool {
	if syscall.Kill(pid, 0) == syscall.ESRCH {
		return true
	}
	out, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	st := strings.TrimSpace(string(out))
	return st == "" || strings.HasPrefix(st, "Z")
}

// waitGone fails unless pid exits soon; the deadline only bounds a broken
// run, it is not what the test waits for.
func waitGone(t *testing.T, pid int) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !gone(pid); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("process %d still running", pid)
		}
	}
}

// outsider starts a process Foundry did not start through Group.
func outsider(t *testing.T) *exec.Cmd {
	t.Helper()
	c := exec.Command("sleep", "300")
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Process.Kill(); c.Wait() })
	return c
}

func needSh(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
}

// Cancelling a grouped command kills the processes it started, and only those.
func TestCancelKillsWholeGroup(t *testing.T) {
	needSh(t)
	other := outsider(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", "sleep 300 & echo $!; wait")
	Group(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n') // the child is running
	if err != nil {
		t.Fatal(err)
	}
	child, _ := strconv.Atoi(strings.TrimSpace(line))
	cancel()
	cmd.Wait()
	waitGone(t, child)
	if gone(other.Process.Pid) {
		t.Fatal("a process outside the group was killed")
	}
}

// A command that exits leaves its background child; KillGroup removes it.
func TestKillGroupAfterExit(t *testing.T) {
	needSh(t)
	other := outsider(t)
	cmd := exec.Command("sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $!")
	Group(cmd)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	child, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if gone(child) {
		t.Fatal("background child did not outlive the command")
	}
	KillGroup(cmd)
	waitGone(t, child)
	if gone(other.Process.Pid) {
		t.Fatal("a process outside the group was killed")
	}
	KillGroup(cmd) // nothing left: a second call must not reach anything else
	if gone(other.Process.Pid) {
		t.Fatal("a process outside the group was killed")
	}
}

// Without Group, KillGroup does nothing: it never guesses a group.
func TestKillGroupNeedsGroup(t *testing.T) {
	needSh(t)
	cmd := exec.Command("sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $!")
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	child, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	t.Cleanup(func() { syscall.Kill(child, syscall.SIGKILL) })
	KillGroup(cmd)
	if gone(child) {
		t.Fatal("KillGroup killed a process of an ungrouped command")
	}
}
