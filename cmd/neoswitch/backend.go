package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/yldm-ai/NeoSwitch/internal/daemon"
	"github.com/yldm-ai/NeoSwitch/internal/provider"
	"github.com/yldm-ai/NeoSwitch/internal/server"
)

const (
	backendAntigravity = "antigravity"
	backendGemini      = "gemini"

	// backendEnv selects the backend for one process over the stored choice. The SessionStart hook sets it for the daemon it spawns, so the daemon serves the backend the hook checked.
	backendEnv = "NEOSWITCH_BACKEND"
	// legacyACPEnv is the earlier switch for the ACP route, still honoured.
	legacyACPEnv = "NEOSWITCH_ANTIGRAVITY_ACP"
	// Earlier plugin versions declared userConfig options, which Claude Code passes to hooks as these variables. The plugin no longer declares them, because Claude Code asks for every declared option when the plugin is enabled and the first question was a Gemini API key even for people signing in with OAuth. Values saved by those versions are still honoured if Claude Code passes them.
	pluginKeyEnv         = "CLAUDE_PLUGIN_OPTION_GEMINI_API_KEY"
	pluginAntigravityEnv = "CLAUDE_PLUGIN_OPTION_ANTIGRAVITY_OAUTH"
)

var errNoBackend = errors.New("no backend is configured: run /neoswitch:login to sign in with Antigravity OAuth (needs the official agy CLI), or export GEMINI_API_KEY before starting Claude Code")

// resolveBackend decides which upstream to use and why. An explicit choice wins: NEOSWITCH_BACKEND, then the choice saved by /neoswitch:login or `neoswitch backend`. Without one, Gemini credentials in the environment select the Gemini API, and otherwise an installed agy selects Antigravity OAuth.
func resolveBackend() (kind, reason string, err error) {
	switch value := strings.ToLower(strings.TrimSpace(os.Getenv(backendEnv))); value {
	case backendAntigravity, backendGemini:
		return value, "set by " + backendEnv, nil
	case "", "auto":
	default:
		return "", "", fmt.Errorf("%s=%q is not a backend; use antigravity, gemini or auto", backendEnv, value)
	}
	if truthy(os.Getenv(legacyACPEnv)) {
		return backendAntigravity, "set by " + legacyACPEnv, nil
	}
	if truthy(os.Getenv(pluginAntigravityEnv)) {
		return backendAntigravity, "set in an earlier plugin version's settings", nil
	}
	dir, err := daemon.Dir()
	if err != nil {
		return "", "", err
	}
	state, ok, err := daemon.LoadState(dir)
	if err != nil {
		return "", "", err
	}
	if ok && state.Backend != "" {
		return state.Backend, "chosen with /neoswitch:login or `neoswitch backend`", nil
	}
	if geminiCredentialConfigured() {
		return backendGemini, "Gemini credentials found in the environment", nil
	}
	if antigravityCLIAvailable() {
		return backendAntigravity, "agy is installed and no Gemini credentials are set", nil
	}
	return "", "", errNoBackend
}

// configuredBackend builds the resolved backend. It returns the backend, its model and a description for logs and status.
func configuredBackend(ctx context.Context) (server.Backend, string, string, error) {
	kind, _, err := resolveBackend()
	if err != nil {
		return nil, "", "", err
	}
	if kind == backendAntigravity {
		backend, err := provider.NewAntigravityACPFromEnv(ctx)
		if err != nil {
			return nil, "", "", fmt.Errorf("%w; install it, or run `neoswitch backend gemini` to use a Gemini API key instead", err)
		}
		return backend, backend.Model(), backendLabel(kind, ""), nil
	}
	if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("GOOGLE_API_KEY") == "" && os.Getenv(pluginKeyEnv) != "" {
		_ = os.Setenv("GEMINI_API_KEY", os.Getenv(pluginKeyEnv))
	}
	backend, err := provider.NewGeminiFromEnv(ctx)
	if err != nil {
		if antigravityCLIAvailable() {
			return nil, "", "", fmt.Errorf("%w; or run /neoswitch:login to use Antigravity OAuth", err)
		}
		return nil, "", "", err
	}
	return backend, backend.Model, backendLabel(kind, backend.BackendName()), nil
}

func backendLabel(kind, geminiName string) string {
	if kind == backendAntigravity {
		return "Antigravity OAuth (agy ACP)"
	}
	return geminiName
}

// backendCommand shows or changes the saved backend choice.
func backendCommand(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: neoswitch backend [auto|antigravity|gemini]")
	}
	if len(args) == 1 {
		choice := strings.ToLower(strings.TrimSpace(args[0]))
		switch choice {
		case "auto":
			choice = ""
		case backendAntigravity, backendGemini:
		default:
			return fmt.Errorf("usage: neoswitch backend [auto|antigravity|gemini]")
		}
		dir, err := daemon.Dir()
		if err != nil {
			return err
		}
		if _, err := daemon.SaveBackend(dir, choice); err != nil {
			return err
		}
	}
	kind, reason, err := resolveBackend()
	if err != nil {
		return err
	}
	fmt.Printf("backend: %s (%s)\n", kind, reason)
	if len(args) == 1 {
		printDaemonSwitchNote(kind)
	}
	return nil
}

// printDaemonSwitchNote explains when a running daemon still serves another backend: it keeps its backend until the sessions using it exit.
func printDaemonSwitchNote(kind string) {
	dir, err := daemon.Dir()
	if err != nil {
		return
	}
	state, ok, err := daemon.LoadState(dir)
	if err != nil || !ok {
		return
	}
	if info, running := daemon.Status(state); running && info.Backend != "" && (kind == backendAntigravity) != (info.Backend == backendLabel(backendAntigravity, "")) {
		fmt.Printf("The running NeoSwitch proxy still uses %s for its %d open session(s); new sessions switch once those exit.\n", info.Backend, info.Sessions)
	}
}

func geminiCredentialConfigured() bool {
	if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) != "" || strings.TrimSpace(os.Getenv("GOOGLE_API_KEY")) != "" || strings.TrimSpace(os.Getenv(pluginKeyEnv)) != "" {
		return true
	}
	return truthy(os.Getenv("GOOGLE_GENAI_USE_ENTERPRISE")) || truthy(os.Getenv("GOOGLE_GENAI_USE_VERTEXAI"))
}

func antigravityCLIAvailable() bool {
	_, err := exec.LookPath(antigravityCLIPath())
	return err == nil
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	}
	return false
}
