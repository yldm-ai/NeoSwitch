package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sen/neoswitch/internal/gemini"
)

type Gemini struct {
	Client  *http.Client
	BaseURL string
	APIKey  string
	Model   string
}

func NewGeminiFromEnv() (*Gemini, error) {
	key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if key == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GEMINI_BASE_URL")), "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		model = "gemini-4-neon"
	}
	return &Gemini{
		Client:  &http.Client{Timeout: 120 * time.Second},
		BaseURL: base,
		APIKey:  key,
		Model:   model,
	}, nil
}

func (g *Gemini) Generate(ctx context.Context, request gemini.GenerateContentRequest) (gemini.GenerateContentResponse, int, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return gemini.GenerateContentResponse{}, 0, err
	}
	url := fmt.Sprintf("%s/models/%s:generateContent", g.BaseURL, g.Model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return gemini.GenerateContentResponse{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.APIKey)
	resp, err := g.Client.Do(req)
	if err != nil {
		return gemini.GenerateContentResponse{}, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return gemini.GenerateContentResponse{}, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return gemini.GenerateContentResponse{}, resp.StatusCode, fmt.Errorf("gemini returned HTTP %d: %s", resp.StatusCode, sanitizeProviderError(data))
	}
	var out gemini.GenerateContentResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return gemini.GenerateContentResponse{}, resp.StatusCode, fmt.Errorf("decode gemini response: %w", err)
	}
	return out, resp.StatusCode, nil
}

func sanitizeProviderError(data []byte) string {
	var value struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &value) == nil && value.Error.Message != "" {
		return value.Error.Message
	}
	return string(data)
}
