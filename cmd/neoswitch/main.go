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
	if len(args) != 1 || args[0] != "probe" {
		return fmt.Errorf("usage: neoswitch antigravity probe")
	}
	path := os.Getenv("ANTIGRAVITY_CLI_PATH")
	if path == "" {
		path = "agy"
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

func serve() error {
	backend, err := provider.NewGeminiFromEnv()
	if err != nil {
		return err
	}
	listen := os.Getenv("NEOSWITCH_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:8080"
	}
	return serveWithListener(backend, listen, os.Getenv("NEOSWITCH_AUTH_TOKEN"))
}

func claude(args []string) error {
	backend, err := provider.NewGeminiFromEnv()
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
	model := backend.Model
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

func serveWithListener(backend *provider.Gemini, listen, token string) error {
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", listen, err)
	}
	defer listener.Close()
	httpServer := &http.Server{Handler: server.NewHandler(server.Config{Backend: backend, Model: backend.Model, Token: token}), ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintf(os.Stderr, "NeoSwitch listening on http://%s (model %s)\n", listener.Addr(), backend.Model)
	err = httpServer.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func doctor() error {
	if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) == "" {
		return fmt.Errorf("GEMINI_API_KEY is not set")
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-4-neon"
	}
	claudePath := os.Getenv("CLAUDE_CLI_PATH")
	if claudePath == "" {
		claudePath = "claude"
	}
	if _, err := exec.LookPath(claudePath); err != nil {
		return fmt.Errorf("Claude CLI %q was not found: %w", claudePath, err)
	}
	fmt.Printf("ok: Gemini credentials present\nok: model %s\nok: Claude CLI %s\n", model, claudePath)
	return nil
}

func models() error {
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-4-neon"
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
  neoswitch antigravity probe                 verify the official agy session without reading its token
  neoswitch doctor                            check local prerequisites
  neoswitch models                            print the configured Gemini model

Environment:
  GEMINI_API_KEY       official Gemini API key (required)
  GEMINI_MODEL         backend model (default: gemini-4-neon)
  GEMINI_BASE_URL      Gemini API base URL
  CLAUDE_CLI_PATH      Claude executable (default: claude)
  ANTIGRAVITY_CLI_PATH  Antigravity executable (default: agy)
  NEOSWITCH_LISTEN     serve listen address (default: 127.0.0.1:8080)
  NEOSWITCH_AUTH_TOKEN optional token for standalone serve`)
}
