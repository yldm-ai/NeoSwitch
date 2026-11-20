package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/yldm-ai/NeoSwitch/internal/provider"
	"github.com/yldm-ai/NeoSwitch/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve()
	case "claude":
		err = claude(os.Args[2:])
	case "antigravity":
		err = antigravity(os.Args[2:])
	case "doctor":
		err = doctor()
	case "models":
		err = models()
	case "on":
		err = on(os.Args[2:])
	case "off":
		err = off(os.Args[2:])
	case "status":
		err = status(os.Args[2:])
	case "attach":
		err = attach(os.Args[2:])
	case "daemon":
		err = runDaemon()
	case "help", "--help", "-h":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "neoswitch:", err)
		os.Exit(1)
	}
}

func antigravity(args []string) error {
	if len(args) != 1 || (args[0] != "probe" && args[0] != "login" && args[0] != "oauth") {
		return fmt.Errorf("usage: neoswitch antigravity {login|probe}")
	}
	path := antigravityCLIPath()
	if args[0] == "login" || args[0] == "oauth" {
		return antigravityLogin(path)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path,
		"-p", "Reply with exactly: neoswitch-ok",
		"--output-format", "json",
		"--disable-slash-commands",
		"--print-timeout", "30s",
	)
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("Antigravity probe timed out; run %q interactively to re-authenticate", path)
		}
		// Do not echo stderr: it may contain account or local-path details.
		return fmt.Errorf("Antigravity CLI probe failed; run %q interactively to inspect authentication", path)
	}
	var result struct {
		Status   string `json:"status"`
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
		return fmt.Errorf("Antigravity returned an unexpected JSON envelope")
	}
	if result.Status != "SUCCESS" || result.Error != "" || strings.TrimSpace(result.Response) != "neoswitch-ok" {
		return fmt.Errorf("Antigravity session probe was not successful; run %q interactively to inspect authentication", path)
	}
	fmt.Println("ok: Antigravity CLI session is usable")
	return nil
}

func antigravityCLIPath() string {
	if path := strings.TrimSpace(os.Getenv("ANTIGRAVITY_CLI_PATH")); path != "" {
		return path
	}
	return "agy"
}

// antigravityLogin delegates authentication to the official CLI's headless
// mode. agy owns the browser OAuth flow and stores its resulting profile in the
// platform keyring without opening its interactive TUI.
func antigravityLogin(path string) error {
	command := exec.Command(path, "acp", "--login")
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		if execErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("Antigravity OAuth login failed (exit code %d)", execErr.ExitCode())
		}
		return fmt.Errorf("start Antigravity CLI %q for OAuth login: %w", path, err)
	}
	fmt.Println("ok: Antigravity OAuth login completed through agy ACP")
	return nil
}

func configuredBackend(ctx context.Context) (server.Backend, string, string, error) {
	if usesAntigravityACP() {
		backend, err := provider.NewAntigravityACPFromEnv(ctx)
		if err != nil {
			return nil, "", "", err
		}
		return backend, backend.Model(), "Antigravity ACP", nil
	}
	backend, err := provider.NewGeminiFromEnv(ctx)
	if err != nil {
		return nil, "", "", err
	}
	return backend, backend.Model, backend.BackendName(), nil
}

func usesAntigravityACP() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("NEOSWITCH_ANTIGRAVITY_ACP")))
	return value == "1" || value == "true" || value == "yes"
}

func serve() error {
	backend, model, _, err := configuredBackend(context.Background())
	if err != nil {
		return err
	}
	listen := os.Getenv("NEOSWITCH_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:8080"
	}
	return serveWithListener(backend, model, listen, os.Getenv("NEOSWITCH_AUTH_TOKEN"))
}

func claude(args []string) error {
	backend, model, _, err := configuredBackend(context.Background())
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("allocate local proxy port: %w", err)
	}
	token, err := randomToken()
	if err != nil {
		_ = listener.Close()
		return err
	}
	handler := server.NewHandler(server.Config{Backend: backend, Model: model, Token: token})
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = httpServer.Serve(listener) }()
	defer func() { _ = httpServer.Shutdown(context.Background()) }()

	claudePath := os.Getenv("CLAUDE_CLI_PATH")
	if claudePath == "" {
		claudePath = "claude"
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	child := exec.Command(claudePath, args...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = append(os.Environ(),
		"ANTHROPIC_BASE_URL=http://"+listener.Addr().String(),
		"ANTHROPIC_AUTH_TOKEN="+token,
		"ANTHROPIC_API_KEY=",
	)
	return child.Run()
}

func serveWithListener(backend server.Backend, model, listen, token string) error {
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", listen, err)
	}
	defer listener.Close()
	httpServer := &http.Server{Handler: server.NewHandler(server.Config{Backend: backend, Model: model, Token: token}), ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintf(os.Stderr, "NeoSwitch listening on http://%s (model %s)\n", listener.Addr(), model)
	err = httpServer.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func doctor() error {
	_, model, backendName, err := configuredBackend(context.Background())
	if err != nil {
		return err
	}
	claudePath := os.Getenv("CLAUDE_CLI_PATH")
	if claudePath == "" {
		claudePath = "claude"
	}
	if _, err := exec.LookPath(claudePath); err != nil {
		return fmt.Errorf("Claude CLI %q was not found: %w", claudePath, err)
	}
	fmt.Printf("ok: %s credentials configured\nok: model %s\nok: Claude CLI %s\n", backendName, model, claudePath)
	return nil
}

func models() error {
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = provider.DefaultModel
	}
	fmt.Println(model)
	return nil
}

func randomToken() (string, error) {
	var data [24]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("generate local auth token: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}

func usage() {
	fmt.Println(`NeoSwitch — local Claude Code to Gemini bridge

Usage:
  neoswitch claude [-- claude arguments...]   start Claude Code through a local proxy
  neoswitch serve                             run the Anthropic-compatible proxy
  neoswitch antigravity login                 sign in with the official agy browser OAuth flow
  neoswitch antigravity probe                 verify the official agy session without reading its token
  neoswitch on [project]                      route new Claude Code sessions in a project to Gemini
  neoswitch off [project]                     route them back to Anthropic
  neoswitch status [project]                  show routing and daemon state
  neoswitch doctor                            check local prerequisites
  neoswitch models                            print the configured Gemini model

Environment:
  GEMINI_API_KEY       Gemini Developer API key
  GOOGLE_GENAI_USE_VERTEXAI=true with GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION
                       use Vertex AI with Application Default Credentials instead
  GEMINI_MODEL         backend model (default: gemini-4-neon)
  GEMINI_BASE_URL      Gemini API base URL
  CLAUDE_CLI_PATH      Claude executable (default: claude)
  ANTIGRAVITY_CLI_PATH  Antigravity executable (default: agy)
  NEOSWITCH_LISTEN     serve listen address (default: 127.0.0.1:8080)
  NEOSWITCH_AUTH_TOKEN optional token for standalone serve`)
}
