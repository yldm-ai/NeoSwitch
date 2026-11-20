package translate

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sen/neoswitch/internal/anthropic"
	"github.com/sen/neoswitch/internal/gemini"
)

type Result struct {
	Request   gemini.GenerateContentRequest
	ToolsByID map[string]string
}

func Request(in anthropic.MessagesRequest) (Result, error) {
	if len(in.Messages) == 0 {
		return Result{}, fmt.Errorf("messages must not be empty")
	}
	if in.MaxTokens <= 0 {
		return Result{}, fmt.Errorf("max_tokens must be greater than zero")
	}
	out := Result{ToolsByID: map[string]string{}}
	out.Request.GenerationConfig = gemini.GenerationConfig{
		MaxOutputTokens: in.MaxTokens,
		Temperature:     in.Temperature,
		TopP:            in.TopP,
		StopSequences:   in.Stop,
	}

	if len(in.System) > 0 && string(in.System) != "null" {
		blocks, err := decodeBlocks(in.System)
		if err != nil {
			return Result{}, fmt.Errorf("system: %w", err)
		}
		var parts []gemini.Part
		for _, b := range blocks {
			if b.Type != "text" {
				return Result{}, fmt.Errorf("system block %q is not supported", b.Type)
			}
			parts = append(parts, gemini.Part{Text: b.Text})
		}
		if len(parts) > 0 {
			out.Request.SystemInstruction = &gemini.Content{Parts: parts}
		}
	}

	for _, tool := range in.Tools {
		if strings.TrimSpace(tool.Name) == "" {
			return Result{}, fmt.Errorf("tool name must not be empty")
		}
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			return Result{}, fmt.Errorf("tool %q input_schema: %w", tool.Name, err)
		}
		out.Request.Tools = append(out.Request.Tools, gemini.ToolContainer{FunctionDeclarations: []gemini.FunctionDeclaration{{
			Name: tool.Name, Description: tool.Description, Parameters: schema,
		}}})
	}

	for _, message := range in.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			return Result{}, fmt.Errorf("unsupported message role %q", message.Role)
		}
		blocks, err := decodeBlocks(message.Content)
		if err != nil {
			return Result{}, fmt.Errorf("%s message: %w", message.Role, err)
		}
		content := gemini.Content{Role: role(message.Role)}
		for _, block := range blocks {
			parts, err := blockToPart(block, out.ToolsByID)
			if err != nil {
				return Result{}, err
			}
			if block.Type == "tool_use" {
				out.ToolsByID[block.ID] = block.Name
			}
			content.Parts = append(content.Parts, parts...)
		}
		if len(content.Parts) == 0 {
			return Result{}, fmt.Errorf("%s message has no content blocks", message.Role)
		}
		out.Request.Contents = append(out.Request.Contents, content)
	}
	return out, nil
}

func Response(in gemini.GenerateContentResponse, requestedModel string) (anthropic.Response, error) {
	if len(in.Candidates) == 0 {
		return anthropic.Response{}, fmt.Errorf("gemini returned no candidates")
	}
	candidate := in.Candidates[0]
	messageID := newMessageID()
	out := anthropic.Response{
		ID:         messageID,
		Type:       "message",
		Role:       "assistant",
		Model:      requestedModel,
		StopReason: stopReason(candidate.FinishReason),
		Usage: anthropic.Usage{
			InputTokens:  in.UsageMetadata.PromptTokenCount,
			OutputTokens: in.UsageMetadata.CandidatesTokenCount,
		},
	}
	for i, part := range candidate.Content.Parts {
		switch {
		case part.Text != "":
			out.Content = append(out.Content, anthropic.ContentBlock{Type: "text", Text: part.Text})
		case part.FunctionCall != nil:
			args, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return anthropic.Response{}, fmt.Errorf("marshal function call %d: %w", i, err)
			}
			out.Content = append(out.Content, anthropic.ContentBlock{
				Type: "tool_use", ID: fmt.Sprintf("toolu_%s_%d", messageID, i),
				Name: part.FunctionCall.Name, Input: args,
			})
		case part.InlineData != nil:
			return anthropic.Response{}, fmt.Errorf("gemini returned an unsupported inline output")
		}
	}
	if len(out.Content) == 0 {
		out.Content = []anthropic.ContentBlock{{Type: "text", Text: ""}}
	}
	for _, block := range out.Content {
		if block.Type == "tool_use" {
			out.StopReason = "tool_use"
			break
		}
	}
	return out, nil
}

func decodeBlocks(raw json.RawMessage) ([]anthropic.ContentBlock, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("content is empty")
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.HasPrefix(trimmed, []byte("\"")) {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		return []anthropic.ContentBlock{{Type: "text", Text: text}}, nil
	}
	var blocks []anthropic.ContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

func blockToPart(block anthropic.ContentBlock, toolsByID map[string]string) ([]gemini.Part, error) {
	switch block.Type {
	case "text":
		return []gemini.Part{{Text: block.Text}}, nil
	case "image":
		if block.Source == nil || block.Source.Data == "" || block.Source.MediaType == "" {
			return nil, fmt.Errorf("image block is missing source data")
		}
		return []gemini.Part{{InlineData: &gemini.Blob{MimeType: block.Source.MediaType, Data: block.Source.Data}}}, nil
	case "tool_use":
		if strings.TrimSpace(block.ID) == "" || strings.TrimSpace(block.Name) == "" {
			return nil, fmt.Errorf("tool_use requires id and name")
		}
		var args map[string]interface{}
		if len(block.Input) > 0 && string(block.Input) != "null" {
			if err := json.Unmarshal(block.Input, &args); err != nil {
				return nil, fmt.Errorf("tool_use %q input: %w", block.Name, err)
			}
		}
		return []gemini.Part{{FunctionCall: &gemini.FunctionCall{Name: block.Name, Args: args}}}, nil
	case "tool_result":
		if strings.TrimSpace(block.ToolUseID) == "" {
			return nil, fmt.Errorf("tool_result requires tool_use_id")
		}
		name := toolsByID[block.ToolUseID]
		if name == "" {
			return nil, fmt.Errorf("tool_result references unknown tool_use_id %q", block.ToolUseID)
		}
		response := map[string]interface{}{"content": rawContent(block.Content)}
		return []gemini.Part{{FunctionResponse: &gemini.FunctionResponse{Name: name, Response: response}}}, nil
	default:
		return nil, fmt.Errorf("unsupported content block type %q", block.Type)
	}
}

func newMessageID() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "msg_neoswitch"
	}
	return "msg_neoswitch_" + fmt.Sprintf("%x", data[:])
}

func rawContent(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return ""
	}
	var value interface{}
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	return strings.TrimSpace(string(raw))
}

func role(value string) string {
	if value == "assistant" {
		return "model"
	}
	return "user"
}

func stopReason(reason string) string {
	switch strings.ToUpper(reason) {
	case "MAX_TOKENS", "LENGTH":
		return "max_tokens"
	case "SAFETY", "BLOCKLIST", "PROHIBITED_CONTENT":
		return "refusal"
	default:
		return "end_turn"
	}
}
