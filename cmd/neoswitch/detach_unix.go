//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// detach puts the daemon in a new session so it is not killed with the hook's process group.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
