//go:build !(darwin || linux)

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// Google's shell installer covers macOS and Linux only; elsewhere the user installs agy from Google's download page.
const installSupported = false

func isolateInstaller(*exec.Cmd) {}

func lockFile(context.Context, string) (*os.File, error) {
	return nil, errors.New("installing agy is not supported on this system")
}
