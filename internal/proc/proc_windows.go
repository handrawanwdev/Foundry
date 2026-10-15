//go:build windows

package proc

import (
	"os/exec"
	"strconv"
)

// group makes cancelling cmd stop its whole process tree. Windows has no
// process groups to signal; taskkill /T walks the children of cmd's pid,
// which cannot be reused while Go still holds cmd's process handle.
func group(cmd *exec.Cmd) {
	if cmd.Cancel == nil { // not made by exec.CommandContext
		return
	}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run() == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
}

// killRest does nothing on Windows: once cmd exited its tree can no longer
// be found by pid, and guessing would risk killing unrelated processes.
func killRest(cmd *exec.Cmd) {}
