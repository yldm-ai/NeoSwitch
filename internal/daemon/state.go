// Package daemon runs one shared NeoSwitch proxy for every project that opted in through `neoswitch on`, and stops it once the last Claude Code session using it has exited.
package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// State is the proxy address and token shared by every enabled project. It is created once and reused so that settings written into projects stay valid across daemon restarts.
type State struct {
	Listen string `json:"listen"`
	Token  string `json:"token"`
	// Backend is the upstream the user chose, "antigravity" or "gemini"; empty means choose from the environment. /neoswitch:login sets it so a Gemini key exported for other tools does not override the account the user signed in with.
	Backend string `json:"backend,omitempty"`
}

func (s State) BaseURL() string { return "http://" + s.Listen }

// Dir is where NeoSwitch keeps its state file and daemon log.
func Dir() (string, error) {
	if dir := os.Getenv("NEOSWITCH_STATE_DIR"); dir != "" {
		return dir, nil
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate config directory: %w", err)
	}
	return filepath.Join(config, "neoswitch"), nil
}

func statePath(dir string) string { return filepath.Join(dir, "proxy.json") }

// LoadState reads the shared state; ok is false when `neoswitch on` has never run.
func LoadState(dir string) (State, bool, error) {
	data, err := os.ReadFile(statePath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil || s.Listen == "" || s.Token == "" {
		return State{}, false, fmt.Errorf("%s is corrupt; delete it and run `neoswitch on` again", statePath(dir))
	}
	return s, true, nil
}

// EnsureState returns the existing state or creates one with a free loopback port and a random token. The file is private to the user because the token authorizes use of their Gemini quota.
func EnsureState(dir string) (State, error) {
	if s, ok, err := LoadState(dir); err != nil || ok {
		return s, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return State{}, fmt.Errorf("pick a loopback port: %w", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	var token [24]byte
	_, _ = rand.Read(token[:])
	s := State{Listen: addr, Token: hex.EncodeToString(token[:])}
	if err := writeState(dir, s); err != nil {
		return State{}, err
	}
	return s, nil
}

// SaveBackend records the user's backend choice ("" for automatic), creating the state when needed.
func SaveBackend(dir, backend string) (State, error) {
	s, err := EnsureState(dir)
	if err != nil {
		return State{}, err
	}
	s.Backend = backend
	return s, writeState(dir, s)
}

// writeState replaces the state file through a temp file so a crash never leaves it truncated. It is private to the user because the token authorizes use of their quota.
func writeState(dir string, s State) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	tmp := statePath(dir) + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, statePath(dir))
}
