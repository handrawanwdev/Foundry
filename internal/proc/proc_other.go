//go:build !unix && !windows

package proc

import "os/exec"

func group(cmd *exec.Cmd) {}

func killRest(cmd *exec.Cmd) {}

// alive cannot tell a dead pid here without more access; see Alive.
func alive(pid int) bool { return true }
