//go:build unix

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// privateDir refuses a folder that another user owns or can enter.
func privateDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !ok || int(st.Uid) != os.Getuid() || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("the folder %s must belong to you and be closed to other users (chmod 700) before signing in by code", dir)
	}
	return nil
}

// killGroup kills a process started with detach together with the processes it started.
func killGroup(cmd *exec.Cmd) {
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	// EPERM means the process exists but belongs to someone else, which still counts as alive.
	return err == nil || errors.Is(err, syscall.EPERM)
}
