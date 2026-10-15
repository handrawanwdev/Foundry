//go:build unix

package checks

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// waitExited fails unless the process exits soon (gone, or a zombie not
// reaped yet). The deadline only bounds a broken run.
func waitExited(t *testing.T, pid string) {
	t.Helper()
	n, err := strconv.Atoi(pid)
	if err != nil {
		t.Fatalf("pid %q", pid)
	}
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		out, _ := exec.Command("ps", "-o", "stat=", "-p", pid).Output()
		if st := strings.TrimSpace(string(out)); syscall.Kill(n, 0) == syscall.ESRCH || st == "" || strings.HasPrefix(st, "Z") {
			return
		}
		if time.Now().After(deadline) {
			syscall.Kill(n, syscall.SIGKILL)
			t.Fatalf("process %s left running by the check", pid)
		}
	}
}
