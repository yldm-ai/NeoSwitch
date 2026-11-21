//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// installSupported reports whether Google's shell installer covers this system.
const installSupported = true

// isolateInstaller runs cmd in a new session, without a controlling terminal, so nothing in it can stop to read a prompt, and, when its context ends, kills the whole session's process group so the installer's curl and tar stop with it.
func isolateInstaller(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// lockFile takes an exclusive lock on path, waiting while another process holds it, until ctx ends. Closing the returned file releases the lock; so does the death of every process holding the file, so a crashed install never blocks the next one.
func lockFile(ctx context.Context, path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			file.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			file.Close()
			return nil, errors.New("stopped while another install of agy was running")
		case <-time.After(200 * time.Millisecond):
		}
	}
}
