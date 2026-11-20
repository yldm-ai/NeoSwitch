package translate

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"

	"google.golang.org/genai"
)

// toolIDPrefix marks tool_use IDs minted by NeoSwitch. Claude Code stores tool_use blocks verbatim and sends them back on the next turn, so the ID is the only place a stateless proxy can carry Gemini state across turns without a session store that would break on restart or `claude --resume`.
const toolIDPrefix = "toolu_ns_"

// toolCallMeta is the Gemini state a function call needs when it is replayed: Gemini 3+ rejects a functionCall part in history whose thoughtSignature is missing (finish reason MISSING_THOUGHT_SIGNATURE), and functionResponse should echo the call ID when Gemini provided one.
type toolCallMeta struct {
	Nonce     string `json:"n"`
	Signature []byte `json:"s,omitempty"`
	CallID    string `json:"c,omitempty"`
}

// newToolID returns an Anthropic-compatible ID (only [A-Za-z0-9_-]) that embeds the call's thought signature and Gemini call ID.
func newToolID(signature []byte, callID string) string {
	payload, _ := json.Marshal(toolCallMeta{Nonce: rand.Text(), Signature: signature, CallID: callID})
	return toolIDPrefix + base64.RawURLEncoding.EncodeToString(payload)
}

// skipSignatureValidator is the placeholder thought signature Gemini documents for function calls it did not generate, such as history transferred from another model (https://ai.google.dev/gemini-api/docs/thought-signatures, FAQ).
var skipSignatureValidator = []byte("skip_thought_signature_validator")

// signUnsignedStep gives the first function call of a model step the placeholder signature when it carries none. Gemini 3+ validates exactly that part and rejects the request with a 400 when it is unsigned, which happens whenever the history holds tool calls Claude made: a session switched to NeoSwitch mid-conversation, or one started against Anthropic and resumed through NeoSwitch. Later parallel calls are not validated and stay unsigned, and a call Gemini signed keeps its real signature.
func signUnsignedStep(parts []*genai.Part) {
	for _, part := range parts {
		if part.FunctionCall == nil {
			continue
		}
		if len(part.ThoughtSignature) == 0 {
			part.ThoughtSignature = skipSignatureValidator
		}
		return
	}
}

// parseToolID recovers the metadata from an ID minted by newToolID. IDs minted elsewhere (for example a session started against Anthropic and resumed through NeoSwitch) carry no Gemini state and yield the zero value.
func parseToolID(id string) toolCallMeta {
	encoded, ok := strings.CutPrefix(id, toolIDPrefix)
	if !ok {
		return toolCallMeta{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return toolCallMeta{}
	}
	var meta toolCallMeta
	if json.Unmarshal(payload, &meta) != nil {
		return toolCallMeta{}
	}
	return meta
}
