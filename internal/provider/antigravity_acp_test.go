package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"github.com/yldm-ai/NeoSwitch/internal/translate"
)

// fakeACPEnv makes the test binary act as a scripted `agy acp` server when it is started as the ACP subprocess, so the JSON-RPC client is exercised over real pipes without the Antigravity CLI.
const fakeACPEnv = "NEOSWITCH_FAKE_ACP"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeACPEnv); mode != "" {
		runFakeACP(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runFakeACP serves ACP over stdin/stdout. Modes: "ok" answers every prompt; "no-image" also answers but does not advertise image prompts; "exit" dies on session/prompt; "hang" never answers session/prompt. An answer is the comma-separated list of prompt block types it received, as {"text": ...}.
func runFakeACP(mode string) {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64<<10), 32<<20)
	out := json.NewEncoder(os.Stdout)
	for in.Scan() {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(in.Bytes(), &msg) != nil {
			continue
		}
		reply := func(result any) {
			_ = out.Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": result})
		}
		switch msg.Method {
		case "initialize":
			reply(map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{
				"promptCapabilities": map[string]bool{"image": mode != "no-image"},
			}})
		case "session/new":
			reply(map[string]string{"sessionId": "s1"})
		case "session/set_config_option", "session/close":
			reply(map[string]any{})
		case "session/prompt":
			switch mode {
			case "exit":
				os.Exit(1)
			case "hang":
				continue
			}
			var params struct {
				Prompt []struct {
					Type     string `json:"type"`
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"prompt"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			var types []string
			for _, block := range params.Prompt {
				if block.Type == "image" {
					types = append(types, "image:"+block.MimeType+":"+block.Data)
					continue
				}
				types = append(types, block.Type)
			}
			// A real agent may call client methods mid-prompt; NeoSwitch must answer them with an error rather than leave the agent waiting.
			_ = out.Encode(map[string]any{"jsonrpc": "2.0", "id": "perm-1", "method": "session/request_permission", "params": map[string]any{}})
			if !in.Scan() || !strings.Contains(in.Text(), `"perm-1"`) || !strings.Contains(in.Text(), "-32601") {
				_ = out.Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "error": map[string]any{"code": -1, "message": "client did not reject request_permission"}})
				continue
			}
			answer, _ := json.Marshal(map[string]string{"text": strings.Join(types, ",")})
			half := len(answer) / 2
			for _, chunk := range []string{string(answer[:half]), string(answer[half:])} {
				_ = out.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
					"sessionId": "s1",
					"update":    map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": chunk}},
				}})
			}
			reply(map[string]string{"stopReason": "end_turn"})
		}
	}
}

func newFakeACP(t *testing.T, mode string) *AntigravityACP {
	t.Setenv(fakeACPEnv, mode)
	a := &AntigravityACP{path: os.Args[0], agent: "bare-llm", model: DefaultModel, cwd: t.TempDir()}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func textRequest(text string) translate.Request {
	return translate.Request{Contents: []*genai.Content{genai.NewContentFromText(text, genai.RoleUser)}}
}

func generatedText(t *testing.T, resp *genai.GenerateContentResponse) string {
	t.Helper()
	if len(resp.Candidates) != 1 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) != 1 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	return resp.Candidates[0].Content.Parts[0].Text
}

func TestACPGenerateCollectsChunksAndRejectsClientMethods(t *testing.T) {
	a := newFakeACP(t, "ok")
	for i := 0; i < 2; i++ {
		resp, err := a.Generate(context.Background(), textRequest("hi"))
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if got := generatedText(t, resp); got != "text" {
			t.Fatalf("request %d: got %q", i, got)
		}
	}
}

func TestACPGenerateRecoversAfterAgentExit(t *testing.T) {
	a := newFakeACP(t, "exit")
	if _, err := a.Generate(context.Background(), textRequest("hi")); err == nil {
		t.Fatal("expected an error when agy exits mid-prompt")
	}
	// The next request must start a fresh agy instead of writing to the dead one.
	t.Setenv(fakeACPEnv, "ok")
	resp, err := a.Generate(context.Background(), textRequest("hi"))
	if err != nil {
		t.Fatalf("request after agy exit: %v", err)
	}
	if got := generatedText(t, resp); got != "text" {
		t.Fatalf("got %q", got)
	}
}

func TestACPGenerateHonorsContextDeadline(t *testing.T) {
	a := newFakeACP(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := a.Generate(ctx, textRequest("hi"))
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Generate ignored the context deadline while agy hung")
	}
	// A hung request must not keep the shared ACP process locked for later sessions.
	t.Setenv(fakeACPEnv, "ok")
	if _, err := a.Generate(context.Background(), textRequest("hi")); err != nil {
		t.Fatalf("request after a timed-out one: %v", err)
	}
}

func imageRequest() translate.Request {
	return translate.Request{Contents: []*genai.Content{
		{Role: genai.RoleUser, Parts: []*genai.Part{
			{Text: "describe"},
			{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("png")}},
			{InlineData: &genai.Blob{MIMEType: "application/pdf", Data: []byte("pdf")}},
		}},
		{Role: genai.RoleModel, Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: "c1", Name: "Read", Args: map[string]any{"file_path": "a.jpg"}}}}},
		{Role: genai.RoleUser, Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{
			ID: "c1", Name: "Read", Response: map[string]any{"output": ""},
			Parts: []*genai.FunctionResponsePart{{InlineData: &genai.FunctionResponseBlob{MIMEType: "image/jpeg", Data: []byte("jpg")}}},
		}}}},
	}}
}

func TestACPGenerateSendsImagesWhenAdvertised(t *testing.T) {
	a := newFakeACP(t, "ok")
	resp, err := a.Generate(context.Background(), imageRequest())
	if err != nil {
		t.Fatal(err)
	}
	// "cG5n" and "anBn" are base64 for "png" and "jpg"; the PDF is not an image and stays out of the prompt blocks.
	if got, want := generatedText(t, resp), "text,image:image/png:cG5n,image:image/jpeg:anBn"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestACPGenerateOmitsImagesWhenNotAdvertised(t *testing.T) {
	a := newFakeACP(t, "no-image")
	resp, err := a.Generate(context.Background(), imageRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got := generatedText(t, resp); got != "text" {
		t.Fatalf("got %q", got)
	}
}

func TestACPPromptMarksAttachments(t *testing.T) {
	text, images := acpPrompt(imageRequest(), true)
	if len(images) != 2 {
		t.Fatalf("expected 2 image blocks, got %d", len(images))
	}
	for _, want := range []string{"[image 1 (image/png) attached]", "[image 2 (image/jpeg) attached]", "[application/pdf attachment omitted"} {
		if !strings.Contains(text, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, text)
		}
	}
	text, images = acpPrompt(imageRequest(), false)
	if len(images) != 0 || !strings.Contains(text, "[image/png attachment omitted") || !strings.Contains(text, "[image/jpeg attachment omitted") {
		t.Fatalf("images must be marked as omitted when the agent does not accept them:\n%s", text)
	}
}

func TestACPResponsePartsText(t *testing.T) {
	parts, err := acpResponseParts(`{"text":"hello"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || parts[0].Text != "hello" {
		t.Fatalf("unexpected parts: %#v", parts)
	}
}

func TestACPResponsePartsToolUse(t *testing.T) {
	parts, err := acpResponseParts(`{"tool_use":{"name":"Read","input":{"file_path":"/tmp/a"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || parts[0].FunctionCall == nil {
		t.Fatalf("unexpected parts: %#v", parts)
	}
	if parts[0].FunctionCall.Name != "Read" || parts[0].FunctionCall.Args["file_path"] != "/tmp/a" {
		t.Fatalf("unexpected function call: %#v", parts[0].FunctionCall)
	}
}
