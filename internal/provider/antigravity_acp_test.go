package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"google.golang.org/genai"

	"github.com/yldm-ai/NeoSwitch/internal/translate"
)

// fakeACPEnv makes the test binary act as a scripted `agy acp` server when it is started as the ACP subprocess, so the client is exercised over real pipes without the Antigravity CLI.
const fakeACPEnv = "NEOSWITCH_FAKE_ACP"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeACPEnv); mode != "" {
		runFakeACP(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// streamEnvelope exercises escapes, a \u escape and a UTF-16 surrogate pair; the fake sends it one character per chunk so every escape is split across chunks.
const (
	streamEnvelope = `{"text":"say \"hi\"\n\u003c😀 \ud83d\ude00 done"}`
	streamText     = "say \"hi\"\n<😀 😀 done"
)

// runFakeACP serves ACP over stdin/stdout. Every answering mode first sends session/request_permission and only answers once the client has rejected it. Modes:
//   - ok: answers with the comma-separated prompt block types as {"text": ...}
//   - no-image: like ok, without advertising image prompts
//   - pair: holds prompts until two have arrived, then answers both
//   - hang-first: the first prompt streams a thought and then hangs until session/cancel, which it answers with stopReason cancelled; later prompts answer "after-cancel" once that happened
//   - quiet-first: the first prompt hangs without any output; later prompts answer normally
//   - hang: prompts never answer and produce no output
//   - effort: sessions offer thought_level low and high; answers with the level the client selected, or "unset"
//   - exit: the process dies on session/prompt
//   - stream, tool: answer with streamEnvelope or a tool call, one character per chunk
//   - refusal: answers nothing with stopReason refusal
func runFakeACP(mode string) {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64<<10), 32<<20)
	out := json.NewEncoder(os.Stdout)
	send := func(msg map[string]any) {
		msg["jsonrpc"] = "2.0"
		_ = out.Encode(msg)
	}
	type prompt struct {
		id     json.RawMessage
		sid    string
		answer string
	}
	var (
		sessions   int
		prompts    int
		waiting    []prompt
		permission = map[string]prompt{}
		hung       *prompt
		cancelled  bool
		levels     = map[string]string{}
	)
	update := func(sid, kind, text string) {
		send(map[string]any{"method": "session/update", "params": map[string]any{
			"sessionId": sid,
			"update":    map[string]any{"sessionUpdate": kind, "content": map[string]string{"type": "text", "text": text}},
		}})
	}
	usage := map[string]any{"inputTokens": 20000, "outputTokens": 20, "cachedReadTokens": 30, "totalTokens": 20020}
	askPermission := func(p prompt) {
		permission["perm-"+p.sid] = p
		send(map[string]any{"id": "perm-" + p.sid, "method": "session/request_permission", "params": map[string]any{
			"sessionId": p.sid, "toolCall": map[string]any{"toolCallId": "t1"}, "options": []any{
				map[string]string{"optionId": "allow", "name": "Allow", "kind": "allow_once"},
				map[string]string{"optionId": "reject", "name": "Reject", "kind": "reject_once"},
			},
		}})
	}
	answer := func(p prompt) {
		stop := "end_turn"
		if mode == "refusal" {
			stop = "refusal"
		}
		for _, r := range p.answer {
			update(p.sid, "agent_message_chunk", string(r))
		}
		send(map[string]any{"id": p.id, "result": map[string]any{"stopReason": stop, "usage": usage}})
	}
	for in.Scan() {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Error  *struct {
				Code int `json:"code"`
			} `json:"error"`
			Result struct {
				Outcome struct {
					Outcome  string `json:"outcome"`
					OptionID string `json:"optionId"`
				} `json:"outcome"`
			} `json:"result"`
		}
		if json.Unmarshal(in.Bytes(), &msg) != nil {
			continue
		}
		reply := func(result any) { send(map[string]any{"id": msg.ID, "result": result}) }
		var params struct {
			SessionID string `json:"sessionId"`
			MethodID  string `json:"methodId"`
			Prompt    []struct {
				Type     string `json:"type"`
				MimeType string `json:"mimeType"`
				Data     string `json:"data"`
			} `json:"prompt"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		switch msg.Method {
		case "":
			// A response from the client: only the permission requests expect one, and it must select the reject option.
			var id string
			_ = json.Unmarshal(msg.ID, &id)
			p, ok := permission[id]
			if !ok {
				continue
			}
			delete(permission, id)
			if msg.Error != nil || msg.Result.Outcome.Outcome != "selected" || msg.Result.Outcome.OptionID != "reject" {
				send(map[string]any{"id": p.id, "error": map[string]any{"code": -1, "message": "client did not reject request_permission"}})
				continue
			}
			answer(p)
		case "initialize":
			reply(map[string]any{
				"protocolVersion":   1,
				"agentCapabilities": map[string]any{"promptCapabilities": map[string]bool{"image": mode != "no-image"}},
				"authMethods":       []any{map[string]string{"id": "oauth-personal", "name": "Log in with Google"}},
			})
		case "authenticate":
			if params.MethodID != "oauth-personal" {
				send(map[string]any{"id": msg.ID, "error": map[string]any{"code": -1, "message": "unexpected auth method " + params.MethodID}})
				continue
			}
			reply(map[string]any{})
		case "session/new":
			sessions++
			result := map[string]any{"sessionId": "s" + strings.Repeat("x", sessions)}
			if mode == "effort" {
				result["configOptions"] = []any{map[string]any{
					"type": "select", "id": "thought_level", "name": "Thinking", "currentValue": "high",
					"options": []any{map[string]string{"value": "low", "name": "Low"}, map[string]string{"value": "high", "name": "High"}},
				}}
			}
			reply(result)
		case "session/set_config_option":
			var option struct {
				SessionID string `json:"sessionId"`
				ConfigID  string `json:"configId"`
				Value     string `json:"value"`
			}
			_ = json.Unmarshal(msg.Params, &option)
			if option.ConfigID == "thought_level" {
				levels[option.SessionID] = option.Value
			}
			reply(map[string]any{})
		case "session/close":
			reply(map[string]any{})
		case "session/cancel":
			if hung != nil && hung.sid == params.SessionID {
				cancelled = true
				send(map[string]any{"id": hung.id, "result": map[string]any{"stopReason": "cancelled"}})
				hung = nil
			}
		case "session/prompt":
			var types []string
			for _, block := range params.Prompt {
				if block.Type == "image" {
					types = append(types, "image:"+block.MimeType+":"+block.Data)
					continue
				}
				types = append(types, block.Type)
			}
			summary, _ := json.Marshal(map[string]string{"text": strings.Join(types, ",")})
			p := prompt{id: msg.ID, sid: params.SessionID, answer: string(summary)}
			prompts++
			switch mode {
			case "exit":
				os.Exit(1)
			case "hang":
				continue
			case "quiet-first":
				if prompts == 1 {
					continue
				}
			case "hang-first":
				if prompts == 1 {
					update(p.sid, "agent_thought_chunk", "thinking")
					hung = &p
					continue
				}
				if cancelled {
					p.answer = `{"text":"after-cancel"}`
				} else {
					p.answer = `{"text":"no-cancel"}`
				}
			case "pair":
				waiting = append(waiting, p)
				if len(waiting) < 2 {
					continue
				}
				for _, w := range waiting {
					askPermission(w)
				}
				waiting = nil
				continue
			case "effort":
				level := levels[p.sid]
				if level == "" {
					level = "unset"
				}
				p.answer = `{"text":"` + level + `"}`
			case "stream":
				p.answer = streamEnvelope
			case "tool":
				p.answer = `{"tool_use":{"name":"Read","input":{"file_path":"a.go"}}}`
			case "refusal":
				p.answer = ""
			}
			askPermission(p)
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
		if resp.Candidates[0].FinishReason != genai.FinishReasonStop {
			t.Fatalf("request %d: finish reason %q", i, resp.Candidates[0].FinishReason)
		}
		u := resp.UsageMetadata
		if u == nil || u.PromptTokenCount != 20000 || u.CachedContentTokenCount != 30 || u.CandidatesTokenCount != 20 || u.ThoughtsTokenCount != 0 {
			t.Fatalf("request %d: usage %+v", i, u)
		}
	}
}

func TestACPRunsConcurrentRequestsInOneProcess(t *testing.T) {
	a := newFakeACP(t, "pair")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// The fake answers only once both prompts are in flight, so a client that serializes requests times out here.
			_, errs[i] = a.Generate(ctx, textRequest("hi"))
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
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

func generateWithDeadline(t *testing.T, a *AntigravityACP) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := a.Generate(ctx, textRequest("hi"))
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("Generate ignored the context deadline")
		return nil
	}
}

func TestACPDeadlineCancelsOnlyItsSession(t *testing.T) {
	a := newFakeACP(t, "hang-first")
	if err := generateWithDeadline(t, a); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	// The hung prompt produced output, so agy is busy rather than wedged: it must get session/cancel and keep serving. A restarted fake would hang on its first prompt again.
	resp, err := a.Generate(context.Background(), textRequest("hi"))
	if err != nil {
		t.Fatalf("request after a timed-out one: %v", err)
	}
	if got := generatedText(t, resp); got != "after-cancel" {
		t.Fatalf("got %q, want the same agy to have received session/cancel", got)
	}
}

func TestACPDeadlineRestartsSilentAgent(t *testing.T) {
	a := newFakeACP(t, "hang")
	if err := generateWithDeadline(t, a); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	// agy produced nothing before the deadline: it is restarted so later requests do not inherit a wedged process.
	t.Setenv(fakeACPEnv, "ok")
	if _, err := a.Generate(context.Background(), textRequest("hi")); err != nil {
		t.Fatalf("request after a wedged agy: %v", err)
	}
}

func TestACPDeadlineKeepsAgentThatServesOtherSessions(t *testing.T) {
	a := newFakeACP(t, "quiet-first")
	timedOut := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := a.Generate(ctx, textRequest("hi"))
		timedOut <- err
	}()
	// Let the silent prompt reach agy first, then keep agy visibly busy with another session until the deadline passes.
	time.Sleep(200 * time.Millisecond)
	if _, err := a.Generate(context.Background(), textRequest("hi")); err != nil {
		t.Fatalf("concurrent request: %v", err)
	}
	if err := <-timedOut; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	// agy answered another session moments ago, so it is not stuck and must keep running; a restarted fake would hang on this prompt.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := a.Generate(ctx, textRequest("hi")); err != nil {
		t.Fatalf("request after the timeout: %v", err)
	}
}

func collectStream(t *testing.T, a *AntigravityACP, req translate.Request) []*genai.GenerateContentResponse {
	t.Helper()
	var chunks []*genai.GenerateContentResponse
	for resp, err := range a.Stream(context.Background(), req) {
		if err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, resp)
	}
	if len(chunks) == 0 {
		t.Fatal("stream yielded nothing")
	}
	return chunks
}

func TestACPStreamDecodesTextIncrementally(t *testing.T) {
	a := newFakeACP(t, "stream")
	chunks := collectStream(t, a, textRequest("hi"))
	var text strings.Builder
	for _, chunk := range chunks {
		for _, part := range chunk.Candidates[0].Content.Parts {
			text.WriteString(part.Text)
		}
	}
	if text.String() != streamText {
		t.Fatalf("streamed %q, want %q", text.String(), streamText)
	}
	if len(chunks) < 5 {
		t.Fatalf("expected the answer in many chunks, got %d", len(chunks))
	}
	if first := chunks[0].UsageMetadata; first == nil || first.PromptTokenCount <= 0 {
		t.Fatalf("first chunk must carry an input estimate for message_start, got %+v", first)
	}
	last := chunks[len(chunks)-1]
	if last.Candidates[0].FinishReason != genai.FinishReasonStop || last.UsageMetadata == nil || last.UsageMetadata.PromptTokenCount != 20000 {
		t.Fatalf("unexpected final chunk: %+v / %+v", last.Candidates[0], last.UsageMetadata)
	}
}

func TestACPEstimateLearnsAgentOverhead(t *testing.T) {
	a := newFakeACP(t, "stream")
	before, _ := a.CountTokens(context.Background(), textRequest("hi"))
	_ = collectStream(t, a, textRequest("hi"))
	// The fake bills 20000 input tokens; once seen, the estimate for the same prompt matches it, and so does the next message_start.
	if after, _ := a.CountTokens(context.Background(), textRequest("hi")); after != 20000 || before >= after {
		t.Fatalf("estimate before %d, after %d; want 20000 after", before, after)
	}
	if first := collectStream(t, a, textRequest("hi"))[0].UsageMetadata; first == nil || first.PromptTokenCount != 20000 {
		t.Fatalf("first chunk usage %+v", first)
	}
}

func TestACPStreamDeliversToolCallWhole(t *testing.T) {
	a := newFakeACP(t, "tool")
	chunks := collectStream(t, a, textRequest("hi"))
	if len(chunks) != 1 {
		t.Fatalf("a tool call must not be streamed in pieces, got %d chunks", len(chunks))
	}
	parts := chunks[0].Candidates[0].Content.Parts
	if len(parts) != 1 || parts[0].FunctionCall == nil || parts[0].FunctionCall.Name != "Read" || parts[0].FunctionCall.Args["file_path"] != "a.go" {
		t.Fatalf("unexpected parts: %#v", parts)
	}
}

func TestACPRefusalWithoutText(t *testing.T) {
	a := newFakeACP(t, "refusal")
	resp, err := a.Generate(context.Background(), textRequest("hi"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Candidates[0].FinishReason != genai.FinishReasonSafety || len(resp.Candidates[0].Content.Parts) != 0 {
		t.Fatalf("unexpected refusal: %+v", resp.Candidates[0])
	}
}

func TestACPLoginUsesAdvertisedMethod(t *testing.T) {
	a := newFakeACP(t, "ok")
	if err := a.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestACPEffortSelectsOfferedThoughtLevel(t *testing.T) {
	a := newFakeACP(t, "effort")
	for _, tc := range []struct {
		level genai.ThinkingLevel
		want  string
	}{
		{genai.ThinkingLevelLow, "low"},
		{genai.ThinkingLevelHigh, "high"},
		// Not offered by this agy: the request still runs, at agy's default.
		{genai.ThinkingLevelXhigh, "unset"},
		{"", "unset"},
	} {
		req := textRequest("hi")
		req.ThinkingLevel = tc.level
		resp, err := a.Generate(context.Background(), req)
		if err != nil {
			t.Fatalf("%q: %v", tc.level, err)
		}
		if got := generatedText(t, resp); got != tc.want {
			t.Fatalf("%q: agy thought level %q, want %q", tc.level, got, tc.want)
		}
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

func TestACPResponsePartsKeepsTextBeforeToolUse(t *testing.T) {
	parts, err := acpResponseParts(`{"text":"Reading it.","tool_use":{"name":"Read","input":{}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 || parts[0].Text != "Reading it." || parts[1].FunctionCall == nil {
		t.Fatalf("unexpected parts: %#v", parts)
	}
}

func TestACPResponsePartsAcceptsRawNewlinesInStrings(t *testing.T) {
	parts, err := acpResponseParts("{\n  \"text\": \"line one\nline two\"\n}")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || parts[0].Text != "line one\nline two" {
		t.Fatalf("unexpected parts: %#v", parts)
	}
}

func TestEnvelopeDecoder(t *testing.T) {
	cases := []struct {
		name, answer, streamed string
		tool                   bool
	}{
		{"text", streamEnvelope, streamText, false},
		{"fenced", "```json\n" + `{"text": "fenced"}` + "\n```", "fenced", false},
		{"plain", "Just an answer.", "Just an answer.", false},
		{"raw newline", "{\"text\":\"a\nb\"}", "a\nb", false},
		{"text then tool", `{"text":"Reading.","tool_use":{"name":"Read","input":{}}}`, "Reading.", true},
		{"tool", `{"tool_use":{"name":"Read","input":{}}}`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d envelopeDecoder
			var streamed strings.Builder
			for _, r := range tc.answer {
				streamed.WriteString(d.feed(string(r)))
			}
			rest, err := d.finish(acp.PromptResponse{StopReason: acp.StopReasonEndTurn})
			if err != nil {
				t.Fatal(err)
			}
			for _, part := range rest {
				streamed.WriteString(part.Text)
			}
			if streamed.String() != tc.streamed {
				t.Fatalf("text %q, want %q", streamed.String(), tc.streamed)
			}
			hasTool := len(rest) > 0 && rest[len(rest)-1].FunctionCall != nil
			if hasTool != tc.tool {
				t.Fatalf("tool call delivered: %v, want %v (%#v)", hasTool, tc.tool, rest)
			}
		})
	}
}

func TestACPRejectsAgentToolPermissions(t *testing.T) {
	allow := acp.PermissionOption{OptionId: "allow", Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow"}
	allowAlways := acp.PermissionOption{OptionId: "allow-always", Kind: acp.PermissionOptionKindAllowAlways, Name: "Always allow"}
	rejectOnce := acp.PermissionOption{OptionId: "reject", Kind: acp.PermissionOptionKindRejectOnce, Name: "Reject"}
	rejectAlways := acp.PermissionOption{OptionId: "reject-always", Kind: acp.PermissionOptionKindRejectAlways, Name: "Always reject"}
	for _, tc := range []struct {
		name    string
		options []acp.PermissionOption
		want    acp.PermissionOptionId
	}{
		{"prefers one-time rejection", []acp.PermissionOption{allow, allowAlways, rejectAlways, rejectOnce}, "reject"},
		{"falls back to permanent rejection", []acp.PermissionOption{allow, rejectAlways}, "reject-always"},
		{"cancels without a reject option", []acp.PermissionOption{allow, allowAlways}, ""},
		{"cancels without options", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &acpConn{}
			resp, err := c.RequestPermission(context.Background(), acp.RequestPermissionRequest{SessionId: "s", Options: tc.options})
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if resp.Outcome.Cancelled == nil || resp.Outcome.Selected != nil {
					t.Fatalf("outcome = %+v, want cancelled", resp.Outcome)
				}
				return
			}
			if resp.Outcome.Selected == nil || resp.Outcome.Selected.OptionId != tc.want {
				t.Fatalf("outcome = %+v, want selected %q", resp.Outcome, tc.want)
			}
		})
	}
}
