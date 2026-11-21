//go:build !unix

package main

import "os/exec"

// privateDir has nothing to check where sign-in by code does not run.
func privateDir(string) error { return nil }

func killGroup(cmd *exec.Cmd) { _ = cmd.Process.Kill() }

func processAlive(int) bool { return false }
