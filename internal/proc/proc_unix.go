//go:build unix

package proc

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// group puts cmd in a new process group led by cmd itself; its children
// inherit it. The group id is cmd's pid, which stays reserved while any
// member lives, so the kill below cannot hit an unrelated group.
func group(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if cmd.Cancel != nil { // set by exec.CommandContext
		cmd.Cancel = func() error { return kill(cmd) }
	}
}

func kill(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func killRest(cmd *exec.Cmd) {
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
		kill(cmd)
	}
}
