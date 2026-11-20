package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yldm-ai/neo-switch/internal/daemon"
	"github.com/yldm-ai/neo-switch/internal/provider"
	"github.com/yldm-ai/neo-switch/internal/server"
)

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
	// Turning a project on with no way to reach a model would only make every message of its next session fail. The check is local: it starts no agy and makes no network call, so an installed but signed-out agy still passes.
	if _, _, err := resolveBackend(); err != nil {
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
	// The settings file holds the proxy token. Excluding it in the clone's own exclude file keeps it out of commits without touching the shared .gitignore; the note is only for when that does not work.
	settings := filepath.Join(".claude", "settings.local.json")
	if ignored, known := gitIgnored(project, settings); known && !ignored {
		if err := gitExclude(project, settings); err != nil || !isGitIgnored(project, settings) {
			fmt.Println("Note: Git may include .claude/settings.local.json in commits. It holds NeoSwitch's local access key, so add it to .gitignore.")
		}
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
	if kind, _, err := resolveBackend(); err == nil && kind == backendOpenAI {
		model = os.Getenv("OPENAI_MODEL")
	}
	fmt.Printf("project:  %s\n", project)
	if !enabled {
		fmt.Println("routing:  Anthropic (NeoSwitch off)")
		return nil
	}
	fmt.Printf("routing:  NeoSwitch %s -> %s\n", state.BaseURL(), model)
	if kind, reason, err := resolveBackend(); err != nil {
		fmt.Printf("backend:  none: %v\n", err)
	} else {
		fmt.Printf("backend:  %s (%s)\n", kind, reason)
	}
	if info, running := daemon.Status(state); running {
		fmt.Printf("daemon:   running on %s, %d Claude Code session(s) attached\n", info.Backend, info.Sessions)
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
	kind, _, err := resolveBackend()
	if err != nil {
		return hookMessage("NeoSwitch: " + err.Error())
	}
	// configuredBackend also moves a Gemini key saved by an earlier plugin version into GEMINI_API_KEY. Pinning the kind makes the daemon spawned below, whose environment drops Claude Code's variables, serve the backend checked here.
	_, _, label, err := configuredBackend(context.Background())
	if err != nil {
		return hookMessage("NeoSwitch: " + err.Error())
	}
	_ = os.Setenv(backendEnv, kind)
	if err := daemon.Attach(state, *pid, func() error { return spawnDaemon(dir) }); err != nil {
		var busy *daemon.PortInUseError
		if errors.As(err, &busy) {
			_, port, _ := net.SplitHostPort(busy.Listen)
			return hookMessage("NeoSwitch: another program on this computer is using port " + port + ", which NeoSwitch needs, so this session cannot answer. Close that program and start a new Claude Code session, or run /neoswitch:off to go back to Claude.")
		}
		return hookMessage("NeoSwitch: " + err.Error() + "; see " + filepath.Join(dir, "daemon.log"))
	}
	if info, running := daemon.Status(state); running && info.Backend != "" && info.Backend != label {
		running := "NeoSwitch: the shared proxy still uses " + info.Backend + " because other Claude Code sessions are using it; this session gets " + label + " once they have all exited."
		if notice != "" {
			notice += " " + running
		} else {
			notice = running
		}
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
	if err := daemon.Run(ctx, state, handler, daemon.Options{Backend: backendName}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "NeoSwitch daemon stopped: no Claude Code session attached")
	return nil
}

// isGitIgnored reports whether path is known to be ignored in project's git repository.
func isGitIgnored(project, path string) bool {
	ignored, _ := gitIgnored(project, path)
	return ignored
}

// gitExclude adds path, relative to project, to the repository's local exclude file (normally .git/info/exclude), which only affects this clone. git resolves the file, because in a worktree .git is a file rather than a directory.
func gitExclude(project, path string) error {
	revParse := func(args ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", project, "rev-parse"}, args...)...).Output()
		return strings.TrimSpace(string(out)), err
	}
	exclude, err := revParse("--git-path", "info/exclude")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(exclude) {
		exclude = filepath.Join(project, exclude)
	}
	// The project can be a subdirectory of the repository; exclude patterns are relative to its top level.
	prefix, err := revParse("--show-prefix")
	if err != nil {
		return err
	}
	pattern := "/" + filepath.ToSlash(filepath.Join(prefix, path))
	data, err := os.ReadFile(exclude)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		pattern = "\n" + pattern
	}
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(pattern + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
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
