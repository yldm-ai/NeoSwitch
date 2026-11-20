package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yldm-ai/neo-switch/internal/daemon"
)

// isolateBackendEnv clears every input resolveBackend reads, points the state at a temp dir, and makes agy available or not.
func isolateBackendEnv(t *testing.T, agyInstalled bool) string {
	t.Helper()
	for _, key := range []string{backendEnv, legacyACPEnv, pluginKeyEnv, pluginAntigravityEnv, "GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENAI_USE_VERTEXAI", "GOOGLE_GENAI_USE_ENTERPRISE", "OPENAI_API_KEY", "OPENAI_MODEL", "OPENAI_BASE_URL"} {
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
	if err := backendCommand([]string{"anthropic"}); err == nil {
		t.Fatal("an unknown backend must be rejected")
	}
	t.Setenv(backendEnv, "anthropic")
	if _, _, err := resolveBackend(); err == nil {
		t.Fatalf("an unknown %s must be rejected", backendEnv)
	}
}

func TestOpenAIIsOptInOnly(t *testing.T) {
	isolateBackendEnv(t, true)
	t.Setenv("OPENAI_API_KEY", "sk-test-not-a-real-key")
	t.Setenv("OPENAI_MODEL", "test-model")
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("an OpenAI key must not change automatic selection, got %q", got)
	}
	t.Setenv("GEMINI_API_KEY", "key")
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("an OpenAI key must not change automatic selection, got %q", got)
	}
	if err := backendCommand([]string{"openai"}); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendOpenAI {
		t.Fatalf("`neoswitch backend openai` must select openai, got %q", got)
	}
	if err := backendCommand([]string{"auto"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(backendEnv, "OpenAI")
	if got := mustResolve(t); got != backendOpenAI {
		t.Fatalf("%s=openai must select openai, got %q", backendEnv, got)
	}
}

func TestOpenAIBackendRequiresModel(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv(backendEnv, backendOpenAI)
	t.Setenv("OPENAI_API_KEY", "sk-test-not-a-real-key")
	_, _, _, err := configuredBackend(context.Background())
	if err == nil || !strings.Contains(err.Error(), "OPENAI_MODEL") {
		t.Fatalf("a missing OPENAI_MODEL must be reported, got %v", err)
	}
	if strings.Contains(err.Error(), "sk-test") {
		t.Fatal("the error must not echo the key")
	}
	t.Setenv("OPENAI_MODEL", "test-model")
	_, model, label, err := configuredBackend(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if model != "test-model" || label != backendLabel(backendOpenAI, "") {
		t.Fatalf("model %q label %q", model, label)
	}
}
