package provider

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"google.golang.org/genai"

	"github.com/yldm-ai/NeoSwitch/internal/translate"
)

// AntigravityACP uses the official agy ACP server as the upstream model session. It never reads Antigravity credentials: agy resolves its own OAuth profile from the platform keyring.
//
// One agy process serves every request. Each request gets its own ACP session, and agy runs sessions concurrently, so parallel Claude Code requests (subagents, background calls, several sessions sharing the daemon) do not queue behind each other.
type AntigravityACP struct {
	path  string
	agent string
	model string
	cwd   string

	mu   sync.Mutex
	conn *acpConn

	// promptOverhead is how many input tokens agy adds around every prompt (its own agent instructions, about 14k for bare-llm), learned from the usage of earlier answers. ACP reports usage only after an answer, so estimates made before one need it.
	promptOverhead atomic.Int64
}

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

func (a *AntigravityACP) Model() string { return a.model }

// Login performs ACP's protocol-driven OAuth flow. The agy ACP server owns the browser callback and keyring state, so this method never reads a token or waits for an authorization code on stdin. It uses a dedicated agy process that exits when the login finishes.
func (a *AntigravityACP) Login(ctx context.Context) error {
	c, err := startACP(a.path, a.agent, a.cwd)
	if err != nil {
		return err
	}
	defer c.close()
	init, err := c.conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{Auth: acp.AuthCapabilities{Terminal: false}},
		ClientInfo:         &acp.Implementation{Name: "neoswitch", Version: "1"},
	})
	if err != nil {
		return fmt.Errorf("initialize Antigravity ACP: %w", err)
	}
	methodID := acp.AuthMethodId("oauth-personal")
	if len(init.AuthMethods) > 0 {
		if init.AuthMethods[0].Agent == nil {
			return errors.New("Antigravity ACP advertised an authentication method NeoSwitch cannot run")
		}
		methodID = init.AuthMethods[0].Agent.Id
	}
	if methodID == "" {
		return errors.New("Antigravity ACP did not advertise an authentication method")
	}
	_, err = c.conn.Authenticate(ctx, acp.AuthenticateRequest{MethodId: methodID})
	return err
}

func (a *AntigravityACP) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn == nil {
		return nil
	}
	err := a.conn.close()
	a.conn = nil
	return err
}

// Generate runs one prompt and returns the complete answer.
func (a *AntigravityACP) Generate(ctx context.Context, req translate.Request) (*genai.GenerateContentResponse, error) {
	var raw strings.Builder
	result, err := a.run(ctx, req, func(text string) { raw.WriteString(text) })
	if err != nil {
		return nil, err
	}
	parts, err := acpAnswerParts(raw.String(), result)
	if err != nil {
		return nil, err
	}
	resp := acpFinalResponse(parts, result)
	if resp.UsageMetadata == nil {
		estimate, _ := a.CountTokens(ctx, req)
		resp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: int32(estimate)}
	}
	return resp, nil
}

// Stream relays the answer while agy generates it. Text answers are decoded out of the {"text": ...} envelope incrementally; tool calls are delivered whole in the final chunk, which also carries the finish reason and usage.
//
// ACP reports usage only when the prompt ends, but Claude Code sizes its context from the input tokens in message_start, which the first chunk becomes. That chunk therefore carries the prompt-size estimate; the final chunk carries agy's exact count.
func (a *AntigravityACP) Stream(ctx context.Context, req translate.Request) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		estimate, _ := a.CountTokens(ctx, req)
		firstUsage := &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: int32(estimate)}
		// Ending the iteration early (Claude Code disconnected) cancels the prompt in agy.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		queue := newChunkQueue()
		type outcome struct {
			result acp.PromptResponse
			err    error
		}
		done := make(chan outcome, 1)
		go func() {
			result, err := a.run(ctx, req, queue.push)
			done <- outcome{result, err}
		}()
		var decoder envelopeDecoder
		relay := func() bool {
			for _, chunk := range queue.drain() {
				delta := decoder.feed(chunk)
				if delta == "" {
					continue
				}
				if !yield(acpTextChunk(delta, firstUsage), nil) {
					return false
				}
				firstUsage = nil
			}
			return true
		}
		for {
			select {
			case <-queue.ready:
				if !relay() {
					return
				}
			case out := <-done:
				// Every chunk is queued before the prompt returns, so draining here loses nothing.
				if !relay() {
					return
				}
				if out.err != nil {
					yield(nil, out.err)
					return
				}
				parts, err := decoder.finish(out.result)
				if err != nil {
					yield(nil, err)
					return
				}
				final := acpFinalResponse(parts, out.result)
				if final.UsageMetadata == nil {
					// No streamed chunk and no usage from agy: keep the estimate rather than report zero.
					final.UsageMetadata = firstUsage
				}
				yield(final, nil)
				return
			}
		}
	}
}

// CountTokens estimates the input tokens agy will bill for the request. ACP has no token-count method, so this is the prompt size at about four characters per token plus the overhead agy reported for earlier prompts; images are not counted.
func (a *AntigravityACP) CountTokens(ctx context.Context, req translate.Request) (int, error) {
	return promptTextTokens(req) + int(a.promptOverhead.Load()), nil
}

func promptTextTokens(req translate.Request) int {
	text, _ := acpPrompt(req, false)
	return len([]rune(text)) / 4
}

// run sends the request as a prompt in a fresh ACP session and passes each answer chunk to onText as it arrives. The session is closed afterwards; when ctx ends first, the prompt is cancelled in agy.
func (a *AntigravityACP) run(ctx context.Context, req translate.Request, onText func(string)) (acp.PromptResponse, error) {
	c, err := a.connection(ctx)
	if err != nil {
		return acp.PromptResponse{}, err
	}
	session, err := c.conn.NewSession(ctx, acp.NewSessionRequest{Cwd: a.cwd, McpServers: []acp.McpServer{}})
	if err != nil {
		return acp.PromptResponse{}, a.requestError(ctx, c, "create Antigravity ACP session", err)
	}
	if session.SessionId == "" {
		return acp.PromptResponse{}, errors.New("Antigravity ACP returned no session ID")
	}
	sid := session.SessionId
	defer func() {
		// Close with a context of its own so a cancelled request still releases its session.
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), acpCloseTimeout)
		defer cancel()
		_, _ = c.conn.CloseSession(closeCtx, acp.CloseSessionRequest{SessionId: sid})
	}()
	if _, err := c.conn.SetSessionConfigOption(ctx, acp.SetSessionConfigOptionRequest{ValueId: &acp.SetSessionConfigOptionValueId{
		SessionId: sid, ConfigId: "model", Value: acp.SessionConfigValueId(a.model),
	}}); err != nil {
		return acp.PromptResponse{}, a.requestError(ctx, c, "select Antigravity model "+a.model, err)
	}
	c.subscribe(sid, func(update acp.SessionUpdate) {
		if chunk := update.AgentMessageChunk; chunk != nil && chunk.Content.Text != nil {
			onText(chunk.Content.Text.Text)
		}
	})
	defer c.unsubscribe(sid)
	text, images := acpPrompt(req, c.imagePrompts)
	blocks := append([]acp.ContentBlock{acp.TextBlock(text)}, images...)
	result, err := c.conn.Prompt(ctx, acp.PromptRequest{SessionId: sid, Prompt: blocks})
	if err != nil {
		if ctx.Err() != nil {
			cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), acpCloseTimeout)
			_ = c.conn.Cancel(cancelCtx, acp.CancelNotification{SessionId: sid})
			cancel()
		}
		return acp.PromptResponse{}, a.requestError(ctx, c, "Antigravity ACP prompt", err)
	}
	if result.StopReason == acp.StopReasonCancelled {
		return acp.PromptResponse{}, errors.New("Antigravity ACP cancelled the prompt")
	}
	// Learn only from text-only prompts: image tokens are not part of the text estimate and would inflate the overhead.
	if result.Usage != nil && len(images) == 0 {
		if overhead := result.Usage.InputTokens - int64(promptTextTokens(req)); overhead > 0 {
			a.promptOverhead.Store(overhead)
		}
	}
	return result, nil
}

// requestError turns a failed ACP call into the error the proxy reports. A context error is returned as such so the server maps a deadline to 504. When a deadline passes while agy has produced no output for any session in acpStuckAfter, agy is treated as stuck and restarted, so one wedged process cannot fail every later request. A slow request on a working agy only cancels its own session.
func (a *AntigravityACP) requestError(ctx context.Context, c *acpConn, action string, err error) error {
	select {
	case <-c.conn.Done():
		a.discard(c)
		return fmt.Errorf("%s: Antigravity ACP exited unexpectedly", action)
	default:
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) && c.quietFor(acpStuckAfter) {
			a.discard(c)
		}
		return fmt.Errorf("%s: %w", action, ctxErr)
	}
	return fmt.Errorf("%s: %w", action, err)
}

// connection returns the running agy, starting and initializing one when there is none or the previous one exited.
func (a *AntigravityACP) connection(ctx context.Context) (*acpConn, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn != nil {
		select {
		case <-a.conn.conn.Done():
			_ = a.conn.close()
			a.conn = nil
		default:
			return a.conn, nil
		}
	}
	c, err := startACP(a.path, a.agent, a.cwd)
	if err != nil {
		return nil, err
	}
	init, err := c.conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
		ClientInfo:         &acp.Implementation{Name: "neoswitch", Version: "1"},
	})
	if err != nil {
		_ = c.close()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("initialize Antigravity ACP: %w", ctxErr)
		}
		return nil, fmt.Errorf("initialize Antigravity ACP: %w", err)
	}
	c.imagePrompts = init.AgentCapabilities.PromptCapabilities.Image
	a.conn = c
	return c, nil
}

// discard stops c if it is still the current connection; requests already holding it fail and the next request starts a new agy.
func (a *AntigravityACP) discard(c *acpConn) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn == c {
		a.conn = nil
	}
	_ = c.close()
}

const (
	// acpCloseTimeout bounds the session/close and session/cancel messages sent after a request has ended.
	acpCloseTimeout = 5 * time.Second
	// acpStuckAfter is how long agy must have been silent on every session before a timed-out request restarts it.
	acpStuckAfter = time.Minute
)

// acpLogger receives the SDK's connection diagnostics, which end up in the daemon log. Some of them carry the raw JSON-RPC line, which can hold prompt or answer text; that attribute is dropped because NeoSwitch logs no conversation content.
var acpLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelWarn,
	ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		if attr.Key == "raw" {
			return slog.Attr{}
		}
		return attr
	},
}))

// acpConn is one running `agy acp` process and the client side of its ACP connection.
type acpConn struct {
	cmd          *exec.Cmd
	conn         *acp.ClientSideConnection
	imagePrompts bool

	mu    sync.Mutex
	sinks map[acp.SessionId]func(acp.SessionUpdate)
	// lastUpdate is when agy last sent a session update for any session, in Unix nanoseconds; zero means never.
	lastUpdate atomic.Int64

	closeOnce sync.Once
	closeErr  error
}

var _ acp.Client = (*acpConn)(nil)

func startACP(path, agent, cwd string) (*acpConn, error) {
	cmd := exec.Command(path, "acp", "--agent", agent, "--dangerously-skip-permissions")
	cmd.Dir = cwd
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("start Antigravity ACP: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("start Antigravity ACP: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start Antigravity ACP: %w", err)
	}
	c := &acpConn{cmd: cmd, sinks: map[acp.SessionId]func(acp.SessionUpdate){}}
	c.conn = acp.NewClientSideConnection(c, stdin, stdout)
	c.conn.SetLogger(acpLogger)
	return c, nil
}

func (c *acpConn) close() error {
	c.closeOnce.Do(func() {
		if err := c.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			c.closeErr = err
		}
		_ = c.cmd.Wait()
	})
	return c.closeErr
}

// quietFor reports whether agy has sent no session update for d.
func (c *acpConn) quietFor(d time.Duration) bool {
	last := c.lastUpdate.Load()
	return last == 0 || time.Since(time.Unix(0, last)) > d
}

func (c *acpConn) subscribe(sid acp.SessionId, sink func(acp.SessionUpdate)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sinks[sid] = sink
}

func (c *acpConn) unsubscribe(sid acp.SessionId) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sinks, sid)
}

// SessionUpdate routes an update to the request that owns the session. It runs on the SDK's single notification goroutine shared by all sessions, so sinks must not block.
func (c *acpConn) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	c.lastUpdate.Store(time.Now().UnixNano())
	c.mu.Lock()
	sink := c.sinks[n.SessionId]
	c.mu.Unlock()
	if sink != nil {
		sink(n.Update)
	}
	return nil
}

// The prompt tells agy not to use its own tools, and agy runs with --dangerously-skip-permissions, so it has no reason to call back into the client. Any such call is refused rather than left waiting.

func (c *acpConn) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{}, acp.NewMethodNotFound(acp.ClientMethodSessionRequestPermission)
}

func (c *acpConn) ReadTextFile(context.Context, acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsReadTextFile)
}

func (c *acpConn) WriteTextFile(context.Context, acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsWriteTextFile)
}

func (c *acpConn) CreateElicitation(context.Context, acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	return acp.CreateElicitationResponse{}, acp.NewMethodNotFound(acp.ClientMethodElicitationCreate)
}

func (c *acpConn) CompleteElicitation(context.Context, acp.CompleteElicitationNotification) error {
	return nil
}

func (c *acpConn) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalCreate)
}

func (c *acpConn) KillTerminal(context.Context, acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalKill)
}

func (c *acpConn) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalOutput)
}

func (c *acpConn) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalRelease)
}

func (c *acpConn) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalWaitForExit)
}

// chunkQueue hands answer chunks from the SDK's notification goroutine to the streaming request without ever blocking the former: a slow Claude Code connection must not stall updates for every other session.
type chunkQueue struct {
	mu     sync.Mutex
	chunks []string
	ready  chan struct{}
}

func newChunkQueue() *chunkQueue { return &chunkQueue{ready: make(chan struct{}, 1)} }

func (q *chunkQueue) push(chunk string) {
	q.mu.Lock()
	q.chunks = append(q.chunks, chunk)
	q.mu.Unlock()
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

func (q *chunkQueue) drain() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	chunks := q.chunks
	q.chunks = nil
	return chunks
}
