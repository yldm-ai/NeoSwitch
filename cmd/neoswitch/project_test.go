package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Tests run from inside Claude Code (its Bash tool) inherit the session's CLAUDECODE and CLAUDE_PID. Left set, they would make enableProject treat every test as a live session and start a daemon from the test binary for it. NEOSWITCH_PLUGIN would likewise make attach start the test binary as its background update check.
func TestMain(m *testing.M) {
	for _, key := range []string{"CLAUDECODE", "CLAUDE_PID", "CLAUDE_CODE_USE_GATEWAY", pluginEnv} {
		_ = os.Unsetenv(key)
	}
	os.Exit(m.Run())
}

func settingsExist(t *testing.T, project string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(project, ".claude", "settings.local.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// The session running /neoswitch:on applies the project settings on its next request. If NeoSwitch cannot serve it, writing them would send that request to a port nothing listens on, so the settings must stay untouched.
func TestOnInsideSessionWithoutBackendLeavesSettingsUntouched(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_PID", strconv.Itoa(os.Getpid()))
	t.Setenv("CLAUDE_CODE_USE_GATEWAY", "")
	project := t.TempDir()
	if err := on([]string{project}); !errors.Is(err, errNoBackend) {
		t.Fatalf("on without a backend = %v, want errNoBackend", err)
	}
	if settingsExist(t, project) {
		t.Fatal("settings were written although the session could not be served")
	}
}

// A gateway session ignores project env until it restarts, so on only writes the settings and starts nothing; no daemon could start here, since the test binary is not NeoSwitch.
func TestOnInsideGatewaySessionOnlyWritesSettings(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_PID", strconv.Itoa(os.Getpid()))
	t.Setenv("CLAUDE_CODE_USE_GATEWAY", "1")
	t.Setenv(backendEnv, backendGemini)
	project := t.TempDir()
	if err := on([]string{project}); err != nil {
		t.Fatal(err)
	}
	if !settingsExist(t, project) {
		t.Fatal("settings were not written")
	}
}

func TestOnOutsideClaudeCodeOnlyWritesSettings(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CLAUDE_PID", "")
	t.Setenv(backendEnv, backendGemini)
	project := t.TempDir()
	if err := on([]string{project}); err != nil {
		t.Fatal(err)
	}
	if !settingsExist(t, project) {
		t.Fatal("settings were not written")
	}
}

func TestCallingSession(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_PID", "4242")
	if pid, inside := callingSession(); pid != 4242 || !inside {
		t.Fatalf("callingSession() = %d, %v", pid, inside)
	}
	t.Setenv("CLAUDE_PID", "")
	if pid, inside := callingSession(); pid != 0 || !inside {
		t.Fatalf("without CLAUDE_PID = %d, %v", pid, inside)
	}
	t.Setenv("CLAUDECODE", "")
	if _, inside := callingSession(); inside {
		t.Fatal("outside Claude Code must not count as a session")
	}
}
