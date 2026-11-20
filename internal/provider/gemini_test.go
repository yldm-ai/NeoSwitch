package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/genai"

	"github.com/yldm-ai/neon-switch/internal/anthropic"
	"github.com/yldm-ai/neon-switch/internal/translate"
)

// isolate clears every variable genai reads so a developer's real credentials can never reach the test server.
func isolate(t *testing.T, baseURL string) {
	t.Helper()
	for _, name := range []string{"GOOGLE_API_KEY", "GOOGLE_GENAI_USE_VERTEXAI", "GOOGLE_GENAI_USE_ENTERPRISE", "GOOGLE_CLOUD_PROJECT", "GOOGLE_CLOUD_LOCATION", "GOOGLE_CLOUD_REGION", "GOOGLE_GEMINI_BASE_URL", "GOOGLE_VERTEX_BASE_URL", "GOOGLE_GENAI_API_VERSION", "GEMINI_MODEL"} {
		t.Setenv(name, "")
	}
	t.Setenv("GEMINI_API_KEY", "test-key")
	t.Setenv("GEMINI_BASE_URL", baseURL)
}

func translated(t *testing.T, body string) translate.Request {
	t.Helper()
	var req anthropic.MessagesRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	out, err := translate.ToGemini(req)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestGenerateWireFormat checks what actually goes over the wire, through genai's own serialization: the v1beta path, the API key header, JSON Schema in parametersJsonSchema and the replayed thought signature.
func TestGenerateWireFormat(t *testing.T) {
	var body map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1beta/models/gemini-4-neon:generateContent" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Error("missing API key header")
		}
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"Read","args":{"p":"x"}},"thoughtSignature":"c2lnMg=="}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2}}`))
	}))
	defer upstream.Close()
	// The trailing /v1beta is what earlier NeoSwitch versions documented; it must not be doubled.
	isolate(t, upstream.URL+"/v1beta")
	backend, err := NewGeminiFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	first, err := backend.Generate(context.Background(), translated(t, `{"max_tokens":5,
		"tools":[{"name":"Read","input_schema":{"$schema":"s","type":"object","additionalProperties":false}}],
		"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	decl := body["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)[0].(map[string]any)
	if _, ok := decl["parametersJsonSchema"]; !ok || decl["parameters"] != nil {
		t.Fatalf("declaration on the wire = %v", decl)
	}
	response, err := translate.FromGemini(first, "claude")
	if err != nil {
		t.Fatal(err)
	}

	// Replay the returned tool_use the way Claude Code does on the next turn.
	toolUse, _ := json.Marshal(response.Content[0])
	_, err = backend.Generate(context.Background(), translated(t, `{"max_tokens":5,"messages":[
		{"role":"user","content":"hi"},
		{"role":"assistant","content":[`+string(toolUse)+`]},
		{"role":"user","content":[{"type":"tool_result","tool_use_id":"`+response.Content[0].ID+`","content":"done"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	contents := body["contents"].([]any)
	call := contents[1].(map[string]any)["parts"].([]any)[0].(map[string]any)
	if call["thoughtSignature"] != "c2lnMg==" {
		t.Fatalf("thought signature not replayed on the wire: %v", call)
	}
	result := contents[2].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)
	if result["name"] != "Read" || result["response"].(map[string]any)["output"] != "done" {
		t.Fatalf("function response = %v", result)
	}
}

func TestStreamUsesSSEEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-4-neon:streamGenerateContent" || r.URL.Query().Get("alt") != "sse" {
			t.Errorf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"a\"}]}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"b\"}]},\"finishReason\":\"STOP\"}]}\n\n")
	}))
	defer upstream.Close()
	isolate(t, upstream.URL)
	backend, err := NewGeminiFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for resp, err := range backend.Stream(context.Background(), translated(t, `{"max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`)) {
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(resp.Candidates[0].Content.Parts[0].Text)
	}
	if text.String() != "ab" {
		t.Fatalf("streamed %q", text.String())
	}
}

func TestUpstreamErrorKeepsStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"quota exceeded","status":"RESOURCE_EXHAUSTED"}}`))
	}))
	defer upstream.Close()
	isolate(t, upstream.URL)
	backend, err := NewGeminiFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.Generate(context.Background(), translated(t, `{"max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`))
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 429 || apiErr.Message != "quota exceeded" {
		t.Fatalf("err = %#v", err)
	}
}

func TestCountTokensOnDeveloperAPIIncludesSystemAndTools(t *testing.T) {
	var body map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ":countTokens") {
			t.Errorf("path = %s", r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &body)
		_, _ = w.Write([]byte(`{"totalTokens":17}`))
	}))
	defer upstream.Close()
	isolate(t, upstream.URL)
	backend, err := NewGeminiFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	n, err := backend.CountTokens(context.Background(), translated(t, `{"system":"rules","tools":[{"name":"Read","input_schema":{"type":"object"}}],"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil || n != 17 {
		t.Fatalf("count = %d, %v", n, err)
	}
	if contents := body["contents"].([]any); len(contents) != 3 {
		t.Fatalf("system and tools must be counted as preamble contents: %v", contents)
	}
}

func TestMissingCredentials(t *testing.T) {
	isolate(t, "")
	t.Setenv("GEMINI_API_KEY", "")
	if _, err := NewGeminiFromEnv(context.Background()); err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckLooksUpAndGenerates(t *testing.T) {
	var paths []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"name":"models/gemini-4-neon"}`))
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"OK"}]},"finishReason":"STOP"}]}`))
	}))
	defer upstream.Close()
	isolate(t, upstream.URL)
	backend, err := NewGeminiFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /v1beta/models/gemini-4-neon", "POST /v1beta/models/gemini-4-neon:generateContent"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestCheckNamesTheFailure(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failOn    string
		status    int
		body      string
		wantStep  string
		wantCause string
	}{
		{"invalid key", http.MethodGet, 400, `{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT","details":[{"reason":"API_KEY_INVALID"}]}}`, "look up model", "authentication failed"},
		{"missing model", http.MethodGet, 404, `{"error":{"code":404,"message":"models/gemini-4-neon is not found","status":"NOT_FOUND"}}`, "look up model", "model not found"},
		{"no access", http.MethodGet, 403, `{"error":{"code":403,"message":"Permission denied on resource project","status":"PERMISSION_DENIED"}}`, "look up model", "permission denied"},
		{"region", http.MethodPost, 400, `{"error":{"code":400,"message":"User location is not supported for the API use.","status":"FAILED_PRECONDITION"}}`, "generate with", "region restricted"},
		{"quota", http.MethodPost, 429, `{"error":{"code":429,"message":"quota exceeded","status":"RESOURCE_EXHAUSTED"}}`, "generate with", "quota or rate limit"},
		{"outage", http.MethodPost, 503, `{"error":{"code":503,"message":"overloaded","status":"UNAVAILABLE"}}`, "generate with", "service unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == tc.failOn {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
					return
				}
				_, _ = w.Write([]byte(`{"name":"models/gemini-4-neon"}`))
			}))
			defer upstream.Close()
			isolate(t, upstream.URL)
			backend, err := NewGeminiFromEnv(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			err = backend.Check(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.wantStep) || !strings.Contains(err.Error(), tc.wantCause) {
				t.Fatalf("err = %v, want step %q and cause %q", err, tc.wantStep, tc.wantCause)
			}
			if strings.Contains(err.Error(), "test-key") {
				t.Fatalf("error leaks the API key: %v", err)
			}
		})
	}
}

func TestCheckReportsUnreachableUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	upstream.Close()
	isolate(t, upstream.URL)
	backend, err := NewGeminiFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "could not reach Gemini") {
		t.Fatalf("err = %v", err)
	}
}

func TestListModelsFollowsPagesAndKeepsGenerativeModels(t *testing.T) {
	var queries []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1beta/models" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		queries = append(queries, r.URL.Query().Get("pageToken"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("pageToken") {
		case "":
			_, _ = w.Write([]byte(`{"models":[
				{"name":"models/gemini-z-pro","supportedGenerationMethods":["generateContent","countTokens"]},
				{"name":"models/text-embedding-9","supportedGenerationMethods":["embedContent"]}
			],"nextPageToken":"p2"}`))
		case "p2":
			_, _ = w.Write([]byte(`{"models":[
				{"name":"models/gemini-4-neon","supportedGenerationMethods":["generateContent"]},
				{"name":"models/imagen-9","supportedGenerationMethods":["predict"]},
				{"name":"models/aqa"}
			]}`))
		default:
			t.Errorf("unexpected page token %q", r.URL.Query().Get("pageToken"))
		}
	}))
	defer upstream.Close()
	isolate(t, upstream.URL)
	backend, err := NewGeminiFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"gemini-4-neon", "gemini-z-pro"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("models = %v, want %v", got, want)
	}
	if strings.Join(queries, ",") != ",p2" {
		t.Fatalf("page tokens requested = %q", queries)
	}
}

func TestListModelsNamesTheFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-4-neon","supportedGenerationMethods":["generateContent"]}],"nextPageToken":"p2"}`))
			return
		}
		// The failure on a later page must not be swallowed into a partial list.
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT","details":[{"reason":"API_KEY_INVALID"}]}}`))
	}))
	defer upstream.Close()
	isolate(t, upstream.URL)
	backend, err := NewGeminiFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	models, err := backend.ListModels(context.Background())
	if err == nil || !strings.Contains(err.Error(), "list models") || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("models = %v, err = %v", models, err)
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Fatalf("error leaks the API key: %v", err)
	}
}
