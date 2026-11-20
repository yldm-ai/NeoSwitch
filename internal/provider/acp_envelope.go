package provider

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	acp "github.com/coder/acp-go-sdk"
	"google.golang.org/genai"

	"github.com/yldm-ai/neon-switch/internal/translate"
)

// ---- prompt ----

// acpPrompt renders the request as the text of one ACP prompt. Images become ACP image blocks sent after that text when the agent accepts them (acceptImages); the text marks where each one belongs. Attachments ACP cannot carry are marked as omitted so the model knows they existed.
func acpPrompt(req translate.Request, acceptImages bool) (string, []acp.ContentBlock) {
	p := &promptBuilder{acceptImages: acceptImages}
	p.out.WriteString("Act as the language model for Claude Code. Return exactly one JSON object and no markdown. For a normal answer use {\"text\":\"...\"}. To invoke one of the supplied Claude Code tools use {\"tool_use\":{\"name\":\"tool name\",\"input\":{...}}}. Do not use ACP tools, edit files, or run commands yourself.\n\n")
	if req.Config != nil && req.Config.SystemInstruction != nil {
		p.out.WriteString("System instructions:\n")
		p.writeParts(req.Config.SystemInstruction.Parts)
		p.out.WriteString("\n")
	}
	for _, content := range req.Contents {
		p.out.WriteString(strings.ToUpper(string(content.Role)))
		p.out.WriteString(":\n")
		p.writeParts(content.Parts)
		p.out.WriteString("\n\n")
	}
	if req.Config != nil && len(req.Config.Tools) > 0 {
		declarations, _ := json.Marshal(req.Config.Tools)
		p.out.WriteString("Claude Code tool definitions for context only:\n")
		p.out.Write(declarations)
		p.out.WriteString("\n")
	}
	return p.out.String(), p.images
}

type promptBuilder struct {
	out          strings.Builder
	acceptImages bool
	images       []acp.ContentBlock
}

func (p *promptBuilder) writeParts(parts []*genai.Part) {
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.Text != "" {
			p.out.WriteString(part.Text)
			p.out.WriteString("\n")
		}
		if part.InlineData != nil {
			p.attach(part.InlineData.MIMEType, part.InlineData.Data)
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			fmt.Fprintf(&p.out, "[tool call %s %s]\n", part.FunctionCall.Name, args)
		}
		if part.FunctionResponse != nil {
			response, _ := json.Marshal(part.FunctionResponse.Response)
			fmt.Fprintf(&p.out, "[tool result %s %s]\n", part.FunctionResponse.Name, response)
			for _, media := range part.FunctionResponse.Parts {
				if media != nil && media.InlineData != nil {
					p.attach(media.InlineData.MIMEType, media.InlineData.Data)
				}
			}
		}
	}
}

func (p *promptBuilder) attach(mimeType string, data []byte) {
	if !p.acceptImages || !strings.HasPrefix(mimeType, "image/") {
		fmt.Fprintf(&p.out, "[%s attachment omitted: the Antigravity ACP route cannot send it]\n", mimeType)
		return
	}
	p.images = append(p.images, acp.ImageBlock(base64.StdEncoding.EncodeToString(data), mimeType))
	fmt.Fprintf(&p.out, "[image %d (%s) attached]\n", len(p.images), mimeType)
}

// ---- answer ----

var errEmptyACPAnswer = errors.New("Antigravity ACP returned an empty response")

// acpResponseParts parses the JSON envelope the prompt asks for. An answer that is not JSON at all is passed through as text, since a readable answer beats an error.
func acpResponseParts(raw string) ([]*genai.Part, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errEmptyACPAnswer
	}
	clean := stripCodeFence(raw)
	var response struct {
		Text    string `json:"text"`
		ToolUse *struct {
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"tool_use"`
	}
	if err := json.Unmarshal([]byte(clean), &response); err != nil {
		// Models sometimes put literal newlines or tabs inside JSON strings; accept those before giving up on the envelope.
		if json.Unmarshal([]byte(escapeControlInStrings(clean)), &response) != nil {
			return []*genai.Part{{Text: raw}}, nil
		}
	}
	var parts []*genai.Part
	if response.Text != "" {
		parts = append(parts, &genai.Part{Text: response.Text})
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
		parts = append(parts, &genai.Part{FunctionCall: &genai.FunctionCall{ID: id, Name: response.ToolUse.Name, Args: input}})
	}
	if len(parts) == 0 {
		return nil, errors.New("Antigravity ACP returned neither text nor a tool call")
	}
	return parts, nil
}

// acpAnswerParts is acpResponseParts for a finished prompt: a refusal may legitimately come with no text.
func acpAnswerParts(raw string, result acp.PromptResponse) ([]*genai.Part, error) {
	parts, err := acpResponseParts(raw)
	if errors.Is(err, errEmptyACPAnswer) && result.StopReason == acp.StopReasonRefusal {
		return nil, nil
	}
	return parts, err
}

func stripCodeFence(raw string) string {
	clean := strings.TrimSpace(raw)
	if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```")
		if newline := strings.IndexByte(clean, '\n'); newline >= 0 {
			clean = clean[newline+1:]
		}
		clean = strings.TrimSuffix(strings.TrimSpace(clean), "```")
	}
	return clean
}

// escapeControlInStrings escapes raw control characters that appear inside JSON string literals, leaving whitespace between tokens alone.
func escapeControlInStrings(s string) string {
	var out strings.Builder
	inString, escaped := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case c == '"':
			inString = !inString
		case inString && c < 0x20:
			out.WriteString(controlEscape(c))
			continue
		}
		out.WriteByte(c)
	}
	return out.String()
}

func controlEscape(c byte) string {
	switch c {
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\t':
		return `\t`
	}
	return fmt.Sprintf(`\u%04x`, c)
}

// acpTextChunk is one streamed piece of answer text.
func acpTextChunk(text string, usage *genai.GenerateContentResponseUsageMetadata) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates:    []*genai.Candidate{{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: text}}}}},
		UsageMetadata: usage,
	}
}

// acpFinalResponse carries the remaining parts with agy's stop reason and token usage. ACP reports output tokens including reasoning (totalTokens = inputTokens + outputTokens), so they are passed as candidate tokens only.
func acpFinalResponse(parts []*genai.Part, result acp.PromptResponse) *genai.GenerateContentResponse {
	finish := genai.FinishReasonStop
	switch result.StopReason {
	case acp.StopReasonMaxTokens:
		finish = genai.FinishReasonMaxTokens
	case acp.StopReasonRefusal:
		finish = genai.FinishReasonSafety
	}
	resp := &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{
		Content:      &genai.Content{Role: genai.RoleModel, Parts: parts},
		FinishReason: finish,
	}}}
	if u := result.Usage; u != nil {
		meta := &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(u.InputTokens),
			CandidatesTokenCount: int32(u.OutputTokens),
		}
		if u.CachedReadTokens != nil {
			meta.CachedContentTokenCount = int32(min(*u.CachedReadTokens, u.InputTokens))
		}
		resp.UsageMetadata = meta
	}
	return resp
}

// ---- streaming ----

// textEnvelopePrefix matches the start of a {"text": "..."} answer, optionally inside a code fence. partialTextEnvelope matches input that is still a prefix of such a start, so the decision waits for more chunks. Both are anchored RE2 expressions and run in linear time.
var (
	textEnvelopePrefix  = regexp.MustCompile("^\\s*(?:```(?:json)?\\s*)?\\{\\s*\"text\"\\s*:\\s*\"")
	partialTextEnvelope = regexp.MustCompile("^\\s*(?:`{1,3}(?:j(?:s(?:o(?:n)?)?)?)?\\s*)?(?:\\{\\s*(?:\"(?:t(?:e(?:x(?:t(?:\"\\s*(?::\\s*)?)?)?)?)?)?)?)?$")
)

type envelopeMode int

const (
	envelopeUndecided envelopeMode = iota
	// envelopeText streams the decoded string value of a {"text": ...} answer.
	envelopeText
	// envelopePlain streams an answer that ignored the JSON format verbatim, exactly as acpResponseParts would return it.
	envelopePlain
	// envelopeBuffered holds the answer (a tool call, or a JSON shape that cannot be streamed safely) until the prompt ends.
	envelopeBuffered
	// envelopeDone means the text value has been streamed completely, or streaming stopped at malformed input.
	envelopeDone
)

// envelopeDecoder turns answer chunks into text deltas as early as the envelope allows. The complete answer is still parsed at the end, which stays authoritative for tool calls.
type envelopeDecoder struct {
	raw     strings.Builder
	mode    envelopeMode
	pos     int
	emitted strings.Builder
}

// feed adds a chunk and returns the text that can be shown now.
func (d *envelopeDecoder) feed(chunk string) string {
	d.raw.WriteString(chunk)
	s := d.raw.String()
	if d.mode == envelopeUndecided {
		trimmed := strings.TrimLeftFunc(s, unicode.IsSpace)
		switch {
		case trimmed == "":
			return ""
		case trimmed[0] != '{' && trimmed[0] != '`':
			d.mode = envelopePlain
		case textEnvelopePrefix.MatchString(s):
			d.mode = envelopeText
			d.pos = textEnvelopePrefix.FindStringIndex(s)[1]
		case partialTextEnvelope.MatchString(s):
			return ""
		default:
			// A tool call, another key first, or a fence around something else.
			d.mode = envelopeBuffered
		}
	}
	var delta string
	switch d.mode {
	case envelopePlain:
		delta = s[d.pos:]
		d.pos = len(s)
	case envelopeText:
		delta = d.decodeText(s)
	}
	d.emitted.WriteString(delta)
	return delta
}

// decodeText decodes the string value from d.pos up to the last point where no escape sequence is cut off, and stops at its closing quote.
func (d *envelopeDecoder) decodeText(s string) string {
	end, closed := d.pos, false
scan:
	for end < len(s) {
		switch s[end] {
		case '"':
			closed = true
			break scan
		case '\\':
			n := escapeLength(s[end:])
			if n == 0 {
				break scan
			}
			end += n
		default:
			end++
		}
	}
	text, err := decodeJSONStringBody(s[d.pos:end])
	if err != nil {
		d.mode = envelopeDone
		return ""
	}
	d.pos = end
	if closed {
		d.mode = envelopeDone
	}
	return text
}

// escapeLength returns the length of the escape sequence at the start of s, or 0 when it is not complete yet. A UTF-16 high surrogate is kept with the low surrogate that follows it so the pair decodes to one character.
func escapeLength(s string) int {
	if len(s) < 2 {
		return 0
	}
	if s[1] != 'u' {
		return 2
	}
	if len(s) < 6 {
		return 0
	}
	code, err := strconv.ParseUint(s[2:6], 16, 16)
	if err != nil || code < 0xD800 || code > 0xDBFF {
		return 6
	}
	if len(s) < 8 {
		return 0
	}
	if s[6:8] != `\u` {
		return 6
	}
	if len(s) < 12 {
		return 0
	}
	return 12
}

// decodeJSONStringBody decodes the inside of a JSON string literal, accepting raw control characters. body never holds an unescaped quote, because decodeText stops before the closing one.
func decodeJSONStringBody(body string) (string, error) {
	var text string
	err := json.Unmarshal([]byte(escapeControlInStrings(`"`+body+`"`)), &text)
	return text, err
}

// finish parses the complete answer and returns the parts not streamed yet: the rest of the text, if any, and a tool call.
func (d *envelopeDecoder) finish(result acp.PromptResponse) ([]*genai.Part, error) {
	parts, err := acpAnswerParts(d.raw.String(), result)
	if err != nil || d.emitted.Len() == 0 {
		return parts, err
	}
	emitted := d.emitted.String()
	var rest []*genai.Part
	for _, part := range parts {
		if part.FunctionCall != nil {
			rest = append(rest, part)
			continue
		}
		if tail, ok := strings.CutPrefix(part.Text, emitted); ok && tail != "" {
			rest = append(rest, &genai.Part{Text: tail})
		}
	}
	return rest, nil
}
