package provider

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"google.golang.org/genai"

	"github.com/yldm-ai/NeoSwitch/internal/translate"
)

// AntigravityACP uses the official agy ACP server as the upstream model
// session. It never reads Antigravity credentials. agy resolves its own OAuth
// profile from the platform keyring.
type AntigravityACP struct {
	path  string
	agent string
	model string
	cwd   string

	mu          sync.Mutex
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      *bufio.Scanner
	nextID      int64
	initialized bool
}

func (a *AntigravityACP) Model() string { return a.model }

func NewAntigravityACPFromEnv(ctx context.Context) (*AntigravityACP, error) {
	path := strings.TrimSpace(os.Getenv("ANTIGRAVITY_CLI_PATH"))
	if path == "" {
		path = "agy"
	}
	if _, err := exec.LookPath(path); err != nil {
		return nil, fmt.Errorf("Antigravity CLI %q was not found: %w", path, err)
	}
	agent := strings.TrimSpace(os.Getenv("ANTIGRAVITY_ACP_AGENT"))
	if agent == "" {
		agent = "bare-llm"
	}
	cwd := strings.TrimSpace(os.Getenv("CLAUDE_PROJECT_DIR"))
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("locate Antigravity ACP workspace: %w", err)
		}
	}
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("locate Antigravity ACP workspace: %w", err)
	}
	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		model = DefaultModel
	}
	return &AntigravityACP{path: path, agent: agent, model: model, cwd: cwd}, nil
}

func (a *AntigravityACP) Generate(ctx context.Context, req translate.Request) (*genai.GenerateContentResponse, error) {
	text, err := a.prompt(ctx, req)
	if err != nil {
		return nil, err
	}
	parts, err := acpResponseParts(text)
	if err != nil {
		return nil, err
	}
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content:      &genai.Content{Role: genai.RoleModel, Parts: parts},
			FinishReason: genai.FinishReasonStop,
		}},
	}, nil
}

func (a *AntigravityACP) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cmd == nil || a.cmd.Process == nil {
		return nil
	}
	_ = a.stdin.Close()
	err := a.cmd.Process.Kill()
	_ = a.cmd.Wait()
	a.cmd = nil
	a.stdin = nil
	a.stdout = nil
	a.initialized = false
	return err
}

func (a *AntigravityACP) Stream(ctx context.Context, req translate.Request) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		result, err := a.Generate(ctx, req)
		yield(result, err)
	}
}

func (a *AntigravityACP) CountTokens(ctx context.Context, req translate.Request) (int, error) {
	// ACP does not expose a token-count method. This estimate keeps Claude Code's
	// count_tokens endpoint usable without claiming provider billing precision.
	return len([]rune(acpPrompt(req))) / 4, nil
}

func (a *AntigravityACP) prompt(ctx context.Context, req translate.Request) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.startLocked(); err != nil {
		return "", err
	}
	if !a.initialized {
		if err := a.callLocked(ctx, "initialize", map[string]any{
			"protocolVersion":    1,
			"clientCapabilities": map[string]any{},
			"clientInfo":         map[string]string{"name": "neoswitch", "version": "1"},
		}, nil); err != nil {
			return "", err
		}
		a.initialized = true
	}
	var session struct {
		SessionID string `json:"sessionId"`
	}
	if err := a.callLocked(ctx, "session/new", map[string]any{"cwd": a.cwd, "mcpServers": []any{}}, &session); err != nil {
		return "", err
	}
	if session.SessionID == "" {
		return "", errors.New("Antigravity ACP returned no session ID")
	}
	var options struct {
		ConfigOptions []struct {
			ID string `json:"id"`
		} `json:"configOptions"`
	}
	if err := a.callLocked(ctx, "session/set_config_option", map[string]any{
		"sessionId": session.SessionID,
		"configId":  "model",
		"value":     a.model,
	}, &options); err != nil {
		return "", err
	}
	var promptResult struct {
		StopReason string `json:"stopReason"`
	}
	var response strings.Builder
	err := a.callLocked(ctx, "session/prompt", map[string]any{
		"sessionId": session.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": acpPrompt(req)}},
	}, &promptResult, func(method string, params json.RawMessage) {
		if method != "session/update" {
			return
		}
		var update struct {
			Update struct {
				SessionUpdate string `json:"sessionUpdate"`
				Content       struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"update"`
		}
		if json.Unmarshal(params, &update) == nil && update.Update.SessionUpdate == "agent_message_chunk" {
			response.WriteString(update.Update.Content.Text)
		}
	})
	_ = a.callLocked(context.Background(), "session/close", map[string]any{"sessionId": session.SessionID}, nil)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(response.String()) == "" {
		return "", errors.New("Antigravity ACP returned an empty response")
	}
	return response.String(), nil
}

func (a *AntigravityACP) startLocked() error {
	if a.cmd != nil {
		return nil
	}
	cmd := exec.Command(a.path, "acp", "--agent", a.agent, "--dangerously-skip-permissions")
	cmd.Dir = a.cwd
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("start Antigravity ACP: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("start Antigravity ACP: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("start Antigravity ACP: %w", err)
	}
	a.cmd = cmd
	a.stdin = stdin
	a.stdout = bufio.NewScanner(stdout)
	a.stdout.Buffer(make([]byte, 64<<10), 32<<20)
	a.nextID = 1
	return nil
}

func (a *AntigravityACP) callLocked(ctx context.Context, method string, params any, result any, notify ...func(string, json.RawMessage)) error {
	_ = ctx
	id := a.nextID
	a.nextID++
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return err
	}
	if _, err := a.stdin.Write(append(payload, '\n')); err != nil {
		a.resetLocked()
		return fmt.Errorf("write Antigravity ACP request: %w", err)
	}
	for a.stdout.Scan() {
		line := a.stdout.Bytes()
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(line, &envelope) != nil {
			continue
		}
		if envelope.Method != "" {
			if len(notify) > 0 {
				notify[0](envelope.Method, envelope.Params)
			}
			if len(envelope.ID) > 0 {
				response, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(envelope.ID), "error": map[string]any{"code": -32601, "message": "NeoSwitch does not provide ACP client methods"}})
				_, _ = a.stdin.Write(append(response, '\n'))
			}
			continue
		}
		if string(envelope.ID) != fmt.Sprint(id) {
			continue
		}
		if envelope.Error != nil {
			return fmt.Errorf("Antigravity ACP error %d: %s", envelope.Error.Code, envelope.Error.Message)
		}
		if result != nil && len(envelope.Result) > 0 {
			if err := json.Unmarshal(envelope.Result, result); err != nil {
				return fmt.Errorf("decode Antigravity ACP response: %w", err)
			}
		}
		return nil
	}
	if err := a.stdout.Err(); err != nil {
		a.resetLocked()
		return fmt.Errorf("read Antigravity ACP response: %w", err)
	}
	a.resetLocked()
	return errors.New("Antigravity ACP exited unexpectedly")
}

func (a *AntigravityACP) resetLocked() {
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_ = a.cmd.Wait()
	}
	a.cmd = nil
	a.stdin = nil
	a.stdout = nil
	a.initialized = false
}

func acpPrompt(req translate.Request) string {
	var out strings.Builder
	out.WriteString("Act as the language model for Claude Code. Return exactly one JSON object and no markdown. For a normal answer use {\"text\":\"...\"}. To invoke one of the supplied Claude Code tools use {\"tool_use\":{\"name\":\"tool name\",\"input\":{...}}}. Do not use ACP tools, edit files, or run commands yourself.\n\n")
	if req.Config != nil && req.Config.SystemInstruction != nil {
		out.WriteString("System instructions:\n")
		writeParts(&out, req.Config.SystemInstruction.Parts)
		out.WriteString("\n")
	}
	for _, content := range req.Contents {
		out.WriteString(strings.ToUpper(string(content.Role)))
		out.WriteString(":\n")
		writeParts(&out, content.Parts)
		out.WriteString("\n\n")
	}
	if req.Config != nil && len(req.Config.Tools) > 0 {
		declarations, _ := json.Marshal(req.Config.Tools)
		out.WriteString("Claude Code tool definitions for context only:\n")
		out.Write(declarations)
		out.WriteString("\n")
	}
	return out.String()
}

func acpResponseParts(raw string) ([]*genai.Part, error) {
	clean := strings.TrimSpace(raw)
	if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```")
		if newline := strings.IndexByte(clean, '\n'); newline >= 0 {
			clean = clean[newline+1:]
		}
		clean = strings.TrimSuffix(strings.TrimSpace(clean), "```")
	}
	var response struct {
		Text    string `json:"text"`
		ToolUse *struct {
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"tool_use"`
	}
	if err := json.Unmarshal([]byte(clean), &response); err != nil {
		return []*genai.Part{{Text: raw}}, nil
	}
	if response.ToolUse != nil {
		if strings.TrimSpace(response.ToolUse.Name) == "" {
			return nil, errors.New("Antigravity ACP returned a tool call without a name")
		}
		var input map[string]any
		if len(response.ToolUse.Input) > 0 && string(response.ToolUse.Input) != "null" {
			if err := json.Unmarshal(response.ToolUse.Input, &input); err != nil {
				return nil, fmt.Errorf("decode Antigravity ACP tool call: %w", err)
			}
		}
		id := "acp_tool_" + rand.Text()
		return []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: id, Name: response.ToolUse.Name, Args: input}}}, nil
	}
	if response.Text == "" {
		return nil, errors.New("Antigravity ACP returned neither text nor a tool call")
	}
	return []*genai.Part{{Text: response.Text}}, nil
}

func writeParts(out *strings.Builder, parts []*genai.Part) {
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.Text != "" {
			out.WriteString(part.Text)
			out.WriteString("\n")
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			fmt.Fprintf(out, "[tool call %s %s]\n", part.FunctionCall.Name, args)
		}
		if part.FunctionResponse != nil {
			response, _ := json.Marshal(part.FunctionResponse.Response)
			fmt.Fprintf(out, "[tool result %s %s]\n", part.FunctionResponse.Name, response)
		}
	}
}
