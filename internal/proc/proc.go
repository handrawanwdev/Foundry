// Package proc keeps the processes a command starts (a dev server, a test
// server, an agent's helpers) tied to that command, so stopping it stops
// them too and none outlives Foundry's own run.
package proc

import "os/exec"

// Group makes cmd and every process it starts one unit: cancelling cmd's
// context (exec.CommandContext) kills all of them, not only cmd. Call it
// before cmd starts.
func Group(cmd *exec.Cmd) { group(cmd) }

// KillGroup kills what cmd left running after it returned: background
// processes it started. It only reaches cmd's own group, never a process
// Foundry did not start. Call it after cmd.Run or cmd.Wait.
func KillGroup(cmd *exec.Cmd) { killRest(cmd) }

// Alive reports whether a process with this pid exists. Where that cannot be
// told it answers true, so a caller never treats a live process as gone.
func Alive(pid int) bool { return pid > 0 && alive(pid) }
