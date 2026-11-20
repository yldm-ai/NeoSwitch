package translate

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
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
