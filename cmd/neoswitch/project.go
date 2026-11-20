package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yldm-ai/neon-switch/internal/daemon"
	"github.com/yldm-ai/neon-switch/internal/provider"
	"github.com/yldm-ai/neon-switch/internal/server"
)

// pluginKeyEnv is how Claude Code hands the Gemini key saved in the plugin's sensitive userConfig (kept in the macOS Keychain) to hook processes.
const pluginKeyEnv = "CLAUDE_PLUGIN_OPTION_GEMINI_API_KEY"
const pluginAntigravityEnv = "CLAUDE_PLUGIN_OPTION_ANTIGRAVITY_OAUTH"

func projectDir(args []string) (string, error) {
	dir := os.Getenv("CLAUDE_PROJECT_DIR")
	if len(args) > 0 {
		dir = args[0]
	}
	if dir == "" {
		var err error
		if dir, err = os.Getwd(); err != nil {
			return "", err
		}
	}
	return filepath.Abs(dir)
}

// on routes Claude Code sessions in one project to the shared NeoSwitch daemon. Only the project's settings.local.json is touched, so other projects keep using Anthropic.
func on(args []string) error {
	project, err := projectDir(args)
	if err != nil {
		return err
	}
	dir, err := daemon.Dir()
	if err != nil {
		return err
	}
	state, err := daemon.EnsureState(dir)
	if err != nil {
		return err
	}
	if err := daemon.Enable(project, state); err != nil {
		return err
	}
	fmt.Printf("NeoSwitch is on for %s.\n", project)
	fmt.Println("Start a new Claude Code session in this project to use Gemini; the current session keeps its connection.")
	if ignored, known := gitIgnored(project, filepath.Join(".claude", "settings.local.json")); known && !ignored {
		fmt.Println("Warning: .claude/settings.local.json is not git-ignored. It holds the local proxy token; add it to .gitignore.")
	}
	return nil
}

func off(args []string) error {
	project, err := projectDir(args)
	if err != nil {
		return err
	}
	dir, err := daemon.Dir()
	if err != nil {
		return err
	}
	state, ok, err := daemon.LoadState(dir)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Printf("NeoSwitch was never turned on; %s is unchanged.\n", project)
		return nil
	}
	if err := daemon.Disable(project, state); err != nil {
		return err
	}
	fmt.Printf("NeoSwitch is off for %s. New Claude Code sessions here use Anthropic again.\n", project)
	return nil
}

func status(args []string) error {
	project, err := projectDir(args)
	if err != nil {
		return err
	}
	dir, err := daemon.Dir()
	if err != nil {
		return err
	}
	state, ok, err := daemon.LoadState(dir)
	if err != nil {
		return err
	}
	enabled := false
	if ok {
		if enabled, err = daemon.Enabled(project, state); err != nil {
			return err
		}
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = provider.DefaultModel
	}
	fmt.Printf("project:  %s\n", project)
	if !enabled {
		fmt.Println("routing:  Anthropic (NeoSwitch off)")
		return nil
	}
	fmt.Printf("routing:  NeoSwitch %s -> %s\n", state.BaseURL(), model)
	if sessions, running := daemon.Status(state); running {
		fmt.Printf("daemon:   running, %d Claude Code session(s) attached\n", sessions)
	} else {
		fmt.Println("daemon:   not running; it starts with the next Claude Code session in this project")
	}
	return nil
}

// attach runs from the plugin's SessionStart hook. Claude Code waits for that hook before its first model request, so the daemon is up by the time the session needs it. Problems are reported to the user through the hook's systemMessage instead of failing silently.
func attach(args []string) error {
	flags := flag.NewFlagSet("attach", flag.ContinueOnError)
	pid := flags.Int("pid", 0, "Claude Code process to serve")
	if err := flags.Parse(args); err != nil {
		return err
	}
	dir, err := daemon.Dir()
	if err != nil {
		return hookMessage(err.Error())
	}
	state, ok, err := daemon.LoadState(dir)
	if err != nil {
		return hookMessage(err.Error())
	}
	// The hook inherits the session's settings env, so a matching base URL means this project is routed through NeoSwitch. It is the URL the session uses unless gateway mode overrides it, which is handled below.
	if !ok || os.Getenv("ANTHROPIC_BASE_URL") != state.BaseURL() {
		return nil
	}
	if *pid <= 1 {
		return hookMessage("NeoSwitch: the SessionStart hook did not pass the Claude Code process ID")
	}
	// Projects enabled before `neoswitch on` turned gateway mode off still let a gateway launcher's ANTHROPIC_BASE_URL win. The running session cannot be changed any more, so fix the project for the next one and say so instead of letting status claim a route the session may not use.
	var notice string
	if gatewayModeOn() {
		project, err := projectDir(nil)
		if err == nil {
			err = daemon.Enable(project, state)
		}
		if err != nil {
			return hookMessage("NeoSwitch: Claude Code was started in gateway mode (" + daemon.EnvUseGateway + "), which can send this session's requests to the gateway instead of NeoSwitch, and turning it off for this project failed: " + err.Error())
		}
		notice = "NeoSwitch: Claude Code was started in gateway mode (" + daemon.EnvUseGateway + "), which can send this session's requests to the gateway instead of Gemini. This project's .claude/settings.local.json now turns gateway mode off; start a new session to use NeoSwitch."
	}
	if pluginAntigravityEnabled() {
		_ = os.Setenv("NEOSWITCH_ANTIGRAVITY_ACP", "true")
	} else if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("GOOGLE_API_KEY") == "" && os.Getenv(pluginKeyEnv) != "" {
		_ = os.Setenv("GEMINI_API_KEY", os.Getenv(pluginKeyEnv))
	} else if noGeminiCredentialConfigured() && antigravityCLIAvailable() {
		// A successful /neoswitch:login leaves the OAuth profile in agy's keyring.
		// Select the official ACP bridge automatically so the user does not need
		// to repeat the provider choice in the plugin configuration.
		_ = os.Setenv("NEOSWITCH_ANTIGRAVITY_ACP", "true")
	}
	if _, _, _, err := configuredBackend(context.Background()); err != nil {
		if pluginAntigravityEnabled() {
			return hookMessage("NeoSwitch: " + err.Error() + ". Run /neoswitch:login first to authenticate Antigravity OAuth.")
		}
		return hookMessage("NeoSwitch: " + err.Error() + ". Set the Gemini API key in the plugin's settings (/plugin) or export it before starting Claude Code.")
	}
	if err := daemon.Attach(state, *pid, func() error { return spawnDaemon(dir) }); err != nil {
		return hookMessage("NeoSwitch: " + err.Error() + "; see " + filepath.Join(dir, "daemon.log"))
	}
	if notice != "" {
		return hookMessage(notice)
	}
	return nil
}

// gatewayModeOn reports whether the session runs in Claude Code's gateway mode. Project settings that turn it off are already applied to the hook's environment.
func gatewayModeOn() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(daemon.EnvUseGateway))) {
	case "", "0", "false", "no", "off":
		return false
	}
	return true
}

func pluginAntigravityEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(pluginAntigravityEnv)))
	return value == "1" || value == "true" || value == "yes"
}

func noGeminiCredentialConfigured() bool {
	if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) != "" || strings.TrimSpace(os.Getenv("GOOGLE_API_KEY")) != "" || strings.TrimSpace(os.Getenv(pluginKeyEnv)) != "" {
		return false
	}
	if value := strings.ToLower(strings.TrimSpace(os.Getenv("GOOGLE_GENAI_USE_ENTERPRISE"))); value == "1" || value == "true" {
		return false
	}
	if value := strings.ToLower(strings.TrimSpace(os.Getenv("GOOGLE_GENAI_USE_VERTEXAI"))); value == "1" || value == "true" {
		return false
	}
	return true
}

func antigravityCLIAvailable() bool {
	_, err := exec.LookPath(antigravityCLIPath())
	return err == nil
}

func hookMessage(message string) error {
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"systemMessage": message})
}

// spawnDaemon starts `neoswitch daemon` in its own session so it survives the hook process, with a scrubbed environment: Claude Code's API settings and plugin options other than the Gemini key stay behind.
func spawnDaemon(dir string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(dir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd := exec.Command(self, "daemon")
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ANTHROPIC_") || strings.HasPrefix(kv, "CLAUDE_") {
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func runDaemon() error {
	dir, err := daemon.Dir()
	if err != nil {
		return err
	}
	state, ok, err := daemon.LoadState(dir)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no NeoSwitch state; run `neoswitch on` first")
	}
	backend, model, backendName, err := configuredBackend(context.Background())
	if err != nil {
		return err
	}
	if closer, ok := backend.(interface{ Close() error }); ok {
		defer func() { _ = closer.Close() }()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(os.Stderr, "NeoSwitch daemon on %s (%s, model %s)\n", state.BaseURL(), backendName, model)
	handler := server.NewHandler(server.Config{Backend: backend, Model: model, Token: state.Token})
	err = daemon.Run(ctx, state, handler, daemon.Options{})
	fmt.Fprintln(os.Stderr, "NeoSwitch daemon stopped: no Claude Code session attached")
	return err
}

// gitIgnored reports whether path is ignored in project's git repository; known is false outside a repository or without git.
func gitIgnored(project, path string) (ignored, known bool) {
	cmd := exec.Command("git", "-C", project, "check-ignore", "-q", path)
	err := cmd.Run()
	if err == nil {
		return true, true
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false, true
	}
	return false, false
}
