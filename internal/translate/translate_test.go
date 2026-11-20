package translate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yldm-ai/NeoSwitch/internal/anthropic"
	"github.com/yldm-ai/NeoSwitch/internal/gemini"
)

func TestRequestMapsSystemToolsImagesAndToolResults(t *testing.T) {
	request := anthropic.MessagesRequest{
		Model:     "claude-sonnet",
		MaxTokens: 512,
		System:    json.RawMessage(`"You are precise."`),
		Tools: []anthropic.Tool{{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		}},
		Messages: []anthropic.Message{
			{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"Inspect this"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="}}]`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"call-1","name":"read_file","input":{"path":"README.md"}}]`)},
			{Role: "user", Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"call-1","content":"file contents"}]`)},
		},
	}

	result, err := Request(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Request.SystemInstruction == nil || result.Request.SystemInstruction.Parts[0].Text != "You are precise." {
		t.Fatalf("system instruction was not mapped: %#v", result.Request.SystemInstruction)
	}
	if len(result.Request.Tools) != 1 || result.Request.Tools[0].FunctionDeclarations[0].Name != "read_file" {
		t.Fatalf("tools were not mapped: %#v", result.Request.Tools)
	}
	if got := len(result.Request.Contents); got != 3 {
		t.Fatalf("contents = %d, want 3", got)
	}
	if result.Request.Contents[0].Parts[1].InlineData == nil {
		t.Fatalf("image was not mapped: %#v", result.Request.Contents[0].Parts)
	}
	responsePart := result.Request.Contents[2].Parts[0].FunctionResponse
	if responsePart == nil || responsePart.Name != "read_file" {
		t.Fatalf("tool result was not mapped: %#v", responsePart)
	}
}

func TestRequestRejectsUnknownBlockAndMissingTool(t *testing.T) {
	base := anthropic.MessagesRequest{MaxTokens: 10, Messages: []anthropic.Message{{Role: "user", Content: json.RawMessage(`[{"type":"audio"}]`)}}}
	if _, err := Request(base); err == nil || !strings.Contains(err.Error(), "unsupported content block") {
		t.Fatalf("unknown block error = %v", err)
	}
	base.Messages[0].Content = json.RawMessage(`[{"type":"tool_result","tool_use_id":"missing","content":"x"}]`)
	if _, err := Request(base); err == nil || !strings.Contains(err.Error(), "unknown tool_use_id") {
		t.Fatalf("missing tool error = %v", err)
	}
}

func TestResponseMapsTextAndToolUse(t *testing.T) {
	response, err := Response(gemini.GenerateContentResponse{
		Candidates: []gemini.Candidate{{
			Content: gemini.Content{Parts: []gemini.Part{
				{Text: "I will inspect it."},
				{FunctionCall: &gemini.FunctionCall{Name: "read_file", Args: map[string]interface{}{"path": "README.md"}}},
			}},
			FinishReason: "STOP",
		}},
		UsageMetadata: gemini.UsageMetadata{PromptTokenCount: 7, CandidatesTokenCount: 9},
	}, "claude-sonnet")
	if err != nil {
		t.Fatal(err)
	}
	if response.StopReason != "tool_use" || len(response.Content) != 2 {
		t.Fatalf("response = %#v", response)
	}
	if response.Content[1].Type != "tool_use" || response.Content[1].Name != "read_file" {
		t.Fatalf("tool response = %#v", response.Content[1])
	}
	if response.Usage.InputTokens != 7 || response.Usage.OutputTokens != 9 {
		t.Fatalf("usage = %#v", response.Usage)
	}
}
