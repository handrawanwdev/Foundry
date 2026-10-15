//go:build !unix && !windows

package proc

import "os/exec"

func group(cmd *exec.Cmd) {}

func killRest(cmd *exec.Cmd) {}
