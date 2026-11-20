package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/yldm-ai/neo-switch/internal/daemon"
	"github.com/yldm-ai/neo-switch/internal/provider"
	"github.com/yldm-ai/neo-switch/internal/server"
)

const (
	backendAntigravity = "antigravity"
	backendGemini      = "gemini"
	// backendOpenAI is the experimental OpenAI Responses API backend. It is opt-in only: automatic selection never picks it.
	backendOpenAI = "openai"

	// backendEnv selects the backend for one process over the stored choice. The SessionStart hook sets it for the daemon it spawns, so the daemon serves the backend the hook checked.
	backendEnv = "NEOSWITCH_BACKEND"
	// legacyACPEnv is the earlier switch for the ACP route, still honoured.
	legacyACPEnv = "NEOSWITCH_ANTIGRAVITY_ACP"
	// Earlier plugin versions declared userConfig options, which Claude Code passes to hooks as these variables. The plugin no longer declares them, because Claude Code asks for every declared option when the plugin is enabled and the first question was a Gemini API key even for people signing in with OAuth. Values saved by those versions are still honoured if Claude Code passes them.
	pluginKeyEnv         = "CLAUDE_PLUGIN_OPTION_GEMINI_API_KEY"
	pluginAntigravityEnv = "CLAUDE_PLUGIN_OPTION_ANTIGRAVITY_OAUTH"
)

var errNoBackend = errors.New("NeoSwitch is not set up yet: run /neoswitch:key to use a Gemini API key, or /neoswitch:login to sign in with your Google account")

// resolveBackend decides which upstream to use and why. An explicit choice wins: NEOSWITCH_BACKEND, then the choice saved by whichever of /neoswitch:login (antigravity), /neoswitch:key (gemini) or `neoswitch backend` ran last. Without one, Gemini credentials in the environment or a key saved by /neoswitch:key select the Gemini API, and otherwise an installed agy selects Antigravity OAuth. OpenAI is only used when chosen explicitly, even if OPENAI_API_KEY is set.
func resolveBackend() (kind, reason string, err error) {
	switch value := strings.ToLower(strings.TrimSpace(os.Getenv(backendEnv))); value {
	case backendAntigravity, backendGemini, backendOpenAI:
		return value, "set by " + backendEnv, nil
	case "", "auto":
	default:
		return "", "", fmt.Errorf("%s=%q is not a backend; use antigravity, gemini, openai or auto", backendEnv, value)
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
		return state.Backend, "chosen with /neoswitch:login, /neoswitch:key or `neoswitch backend`", nil
	}
	if geminiCredentialConfigured() {
		return backendGemini, "Gemini credentials found in the environment or saved with /neoswitch:key", nil
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
		if errors.Is(err, provider.ErrAntigravityCLIMissing) {
			return nil, "", "", fmt.Errorf("%w: install the Antigravity CLI from Google, then start a new Claude Code session", err)
		}
		if err != nil {
			return nil, "", "", err
		}
		return backend, backend.Model(), backendLabel(kind, ""), nil
	}
	if kind == backendOpenAI {
		backend, err := provider.NewOpenAIFromEnv()
		if err != nil {
			return nil, "", "", err
		}
		return backend, backend.Model, backendLabel(kind, ""), nil
	}
	// A key in the environment wins over one saved with /neoswitch:key, and Vertex AI settings win over both. genai reads the key from the environment, so the saved one is passed on that way, also to the daemon spawned from this process.
	if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("GOOGLE_API_KEY") == "" {
		if os.Getenv(pluginKeyEnv) != "" {
			_ = os.Setenv("GEMINI_API_KEY", os.Getenv(pluginKeyEnv))
		} else if !vertexConfigured() {
			saved, err := savedGeminiKey()
			if err != nil {
				return nil, "", "", fmt.Errorf("NeoSwitch cannot read the saved Gemini API key (%w): run /neoswitch:key to save it again", err)
			}
			if saved != "" {
				_ = os.Setenv("GEMINI_API_KEY", saved)
			}
		}
	}
	if !geminiCredentialConfigured() {
		if antigravityCLIAvailable() {
			return nil, "", "", errors.New("NeoSwitch is set to use a Gemini API key, but none is saved: run /neoswitch:key to add one, or /neoswitch:login to sign in with your Google account instead")
		}
		return nil, "", "", errors.New("NeoSwitch is set to use a Gemini API key, but none is saved: run /neoswitch:key to add one")
	}
	backend, err := provider.NewGeminiFromEnv(ctx)
	if err != nil {
		return nil, "", "", err
	}
	return backend, backend.Model, backendLabel(kind, backend.BackendName()), nil
}

func backendLabel(kind, geminiName string) string {
	switch kind {
	case backendAntigravity:
		return "Antigravity OAuth (agy ACP)"
	case backendOpenAI:
		return "OpenAI Responses API (experimental)"
	}
	return geminiName
}

// plainLabel names a backend label from backendLabel the way the user set it up, for messages shown in Claude Code.
func plainLabel(label string) string {
	switch label {
	case backendLabel(backendAntigravity, ""):
		return "your Google account (Antigravity sign-in)"
	case backendLabel(backendOpenAI, ""):
		return "an OpenAI API key (experimental)"
	case "Vertex AI":
		return "Vertex AI"
	}
	return "a Gemini API key"
}

// modelName names a model in messages: Gemini 4 Neon by its product name, any other model by its ID.
func modelName(model string) string {
	if model == provider.DefaultModel {
		return "Gemini 4 Neon"
	}
	return model
}

// backendCommand shows or changes the saved backend choice.
func backendCommand(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: neoswitch backend [auto|antigravity|gemini|openai]")
	}
	if len(args) == 1 {
		choice := strings.ToLower(strings.TrimSpace(args[0]))
		switch choice {
		case "auto":
			choice = ""
		case backendAntigravity, backendGemini, backendOpenAI:
		default:
			return fmt.Errorf("usage: neoswitch backend [auto|antigravity|gemini|openai]")
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
	info, running := daemon.Status(state)
	// Gemini labels vary (Developer API or Vertex AI), so the switch is detected through the fixed labels of the other backends.
	differs := func(other string) bool { return (kind == other) != (info.Backend == backendLabel(other, "")) }
	if running && info.Backend != "" && (differs(backendAntigravity) || differs(backendOpenAI)) {
		fmt.Printf("The running NeoSwitch proxy still uses %s for its %d open session(s); new sessions switch once those exit.\n", info.Backend, info.Sessions)
	}
}

func geminiCredentialConfigured() bool {
	if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) != "" || strings.TrimSpace(os.Getenv("GOOGLE_API_KEY")) != "" || strings.TrimSpace(os.Getenv(pluginKeyEnv)) != "" || vertexConfigured() {
		return true
	}
	// An unreadable key file counts as no key here; configuredBackend reports the read error.
	key, _ := savedGeminiKey()
	return key != ""
}

func vertexConfigured() bool {
	return truthy(os.Getenv("GOOGLE_GENAI_USE_ENTERPRISE")) || truthy(os.Getenv("GOOGLE_GENAI_USE_VERTEXAI"))
}

// savedGeminiKey returns the key saved by /neoswitch:key, or "" when there is none.
func savedGeminiKey() (string, error) {
	dir, err := daemon.Dir()
	if err != nil {
		return "", err
	}
	return daemon.LoadGeminiAPIKey(dir)
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
