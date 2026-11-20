package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yldm-ai/neon-switch/internal/daemon"
)

// isolateBackendEnv clears every input resolveBackend reads, points the state at a temp dir, and makes agy available or not.
func isolateBackendEnv(t *testing.T, agyInstalled bool) string {
	t.Helper()
	for _, key := range []string{backendEnv, legacyACPEnv, pluginKeyEnv, pluginAntigravityEnv, "GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENAI_USE_VERTEXAI", "GOOGLE_GENAI_USE_ENTERPRISE"} {
		t.Setenv(key, "")
	}
	dir := t.TempDir()
	t.Setenv("NEOSWITCH_STATE_DIR", filepath.Join(dir, "state"))
	agy := filepath.Join(dir, "agy")
	if agyInstalled {
		if err := os.WriteFile(agy, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ANTIGRAVITY_CLI_PATH", agy)
	return filepath.Join(dir, "state")
}

func mustResolve(t *testing.T) string {
	t.Helper()
	kind, _, err := resolveBackend()
	if err != nil {
		t.Fatal(err)
	}
	return kind
}

func TestLoginChoiceWinsOverGeminiKeyInEnvironment(t *testing.T) {
	state := isolateBackendEnv(t, true)
	// A key exported for other tools, as many shells have.
	t.Setenv("GEMINI_API_KEY", "for-another-tool")
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("without a saved choice the key selects gemini, got %q", got)
	}
	if _, err := daemon.SaveBackend(state, backendAntigravity); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("after /neoswitch:login the saved choice must win, got %q", got)
	}
	t.Setenv(backendEnv, backendGemini)
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("%s must override the saved choice, got %q", backendEnv, got)
	}
}

func TestAutomaticBackend(t *testing.T) {
	isolateBackendEnv(t, true)
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("agy without Gemini credentials selects antigravity, got %q", got)
	}
	t.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "true")
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("Vertex AI credentials select gemini, got %q", got)
	}
}

func TestNoBackendExplainsBothPaths(t *testing.T) {
	isolateBackendEnv(t, false)
	if _, _, err := resolveBackend(); !errors.Is(err, errNoBackend) {
		t.Fatalf("expected errNoBackend, got %v", err)
	}
}

func TestLegacyPluginOptionsAreHonoured(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv(pluginKeyEnv, "saved-in-plugin-settings")
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("a key saved by an earlier plugin version selects gemini, got %q", got)
	}
	t.Setenv(pluginAntigravityEnv, "true")
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("the earlier OAuth toggle selects antigravity, got %q", got)
	}
}

func TestBackendCommandSavesAndClearsChoice(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv("GEMINI_API_KEY", "key")
	if err := backendCommand([]string{"antigravity"}); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("got %q", got)
	}
	if err := backendCommand([]string{"auto"}); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("auto must clear the saved choice, got %q", got)
	}
	if err := backendCommand([]string{"openai"}); err == nil {
		t.Fatal("an unknown backend must be rejected")
	}
	t.Setenv(backendEnv, "openai")
	if _, _, err := resolveBackend(); err == nil {
		t.Fatalf("an unknown %s must be rejected", backendEnv)
	}
}
