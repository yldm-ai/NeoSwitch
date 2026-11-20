package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sen/neoswitch/internal/gemini"
)

func TestGeminiGenerateUsesExpectedEndpointAndAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1beta/models/gemini-4-neon:generateContent" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatalf("missing API key header")
		}
		var request gemini.GenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Contents) != 1 || request.Contents[0].Parts[0].Text != "hello" {
			t.Fatalf("request body = %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hi"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2}}`))
	}))
	defer server.Close()

	client := &Gemini{Client: server.Client(), BaseURL: server.URL + "/v1beta", APIKey: "test-key", Model: "gemini-4-neon"}
	response, status, err := client.Generate(context.Background(), gemini.GenerateContentRequest{Contents: []gemini.Content{{Parts: []gemini.Part{{Text: "hello"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || response.Candidates[0].Content.Parts[0].Text != "hi" {
		t.Fatalf("status=%d response=%#v", status, response)
	}
}

func TestGeminiGenerateSanitizesJSONProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer server.Close()
	client := &Gemini{Client: server.Client(), BaseURL: server.URL, APIKey: "bad", Model: "gemini-4-neon"}
	_, status, err := client.Generate(context.Background(), gemini.GenerateContentRequest{})
	if status != http.StatusUnauthorized || err == nil || err.Error() != "gemini returned HTTP 401: invalid key" {
		t.Fatalf("status=%d err=%v", status, err)
	}
}
