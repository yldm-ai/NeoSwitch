package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sen/neoswitch/internal/gemini"
)

type fakeGenerator struct {
	response gemini.GenerateContentResponse
}

func (f fakeGenerator) Generate(context.Context, gemini.GenerateContentRequest) (gemini.GenerateContentResponse, int, error) {
	return f.response, http.StatusOK, nil
}

func TestMessagesRequiresToken(t *testing.T) {
	handler := NewHandler(Config{Model: "gemini-4-neon", Token: "secret", Backend: fakeGenerator{}})
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"sonnet","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestMessagesJSONAndBufferedSSE(t *testing.T) {
	backend := fakeGenerator{response: gemini.GenerateContentResponse{
		Candidates:    []gemini.Candidate{{Content: gemini.Content{Parts: []gemini.Part{{Text: "hello"}}}, FinishReason: "STOP"}},
		UsageMetadata: gemini.UsageMetadata{PromptTokenCount: 3, CandidatesTokenCount: 4},
	}}
	handler := NewHandler(Config{Model: "gemini-4-neon", Token: "secret", Backend: backend})
	body := `{"model":"sonnet","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("JSON status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var decoded struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Content[0].Text != "hello" || decoded.StopReason != "end_turn" {
		t.Fatalf("decoded = %#v", decoded)
	}

	streamReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"sonnet","max_tokens":10,"stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	streamReq.Header.Set("X-NeoSwitch-Token", "secret")
	streamRecorder := httptest.NewRecorder()
	handler.ServeHTTP(streamRecorder, streamReq)
	stream := streamRecorder.Body.String()
	for _, event := range []string{"event: message_start", "event: content_block_start", "event: content_block_delta", "event: content_block_stop", "event: message_delta", "event: message_stop"} {
		if !strings.Contains(stream, event) {
			t.Fatalf("stream missing %q: %s", event, stream)
		}
	}
	if strings.Index(stream, "event: message_start") > strings.Index(stream, "event: message_stop") {
		t.Fatalf("stream events are out of order: %s", stream)
	}
}
