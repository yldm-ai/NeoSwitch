package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sen/neoswitch/internal/anthropic"
	"github.com/sen/neoswitch/internal/gemini"
	"github.com/sen/neoswitch/internal/translate"
)

type Generator interface {
	Generate(context.Context, gemini.GenerateContentRequest) (gemini.GenerateContentResponse, int, error)
}

type Config struct {
	Backend Generator
	Model   string
	Token   string
}

func NewHandler(config Config) http.Handler {
	if config.Model == "" {
		config.Model = "gemini-4-neon"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []map[string]interface{}{{
			"id": config.Model, "type": "model", "display_name": "Gemini 4 Neon", "owned_by": "google",
		}}})
	})
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if config.Token != "" && !authorized(r, config.Token) {
			writeAPIError(w, http.StatusUnauthorized, "authentication_error", "invalid NeoSwitch token")
			return
		}
		if config.Backend == nil {
			writeAPIError(w, http.StatusInternalServerError, "configuration_error", "no Gemini backend configured")
			return
		}
		var request anthropic.MessagesRequest
		body := http.MaxBytesReader(w, r.Body, 32<<20)
		defer body.Close()
		if err := json.NewDecoder(body).Decode(&request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("invalid JSON: %v", err))
			return
		}
		translated, err := translate.Request(request)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 130*time.Second)
		defer cancel()
		result, status, err := config.Backend.Generate(ctx, translated.Request)
		if err != nil {
			message := err.Error()
			if status == 0 {
				status = http.StatusBadGateway
			} else if status < 400 {
				status = http.StatusBadGateway
			} else {
				status = http.StatusBadGateway
			}
			writeAPIError(w, status, "upstream_error", message)
			return
		}
		response, err := translate.Response(result, request.Model)
		if err != nil {
			writeAPIError(w, http.StatusBadGateway, "upstream_error", err.Error())
			return
		}
		if request.Stream {
			writeSSE(w, response)
			return
		}
		writeJSON(w, http.StatusOK, response)
	})
	return requestLogger(mux)
}

func authorized(r *http.Request, token string) bool {
	if r.Header.Get("X-NeoSwitch-Token") == token {
		return true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	return strings.HasPrefix(auth, "Bearer ") && strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")) == token
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, typ, message string) {
	writeJSON(w, status, anthropic.ErrorResponse{Type: "error", Error: anthropic.Error{Type: typ, Message: message}})
}

func writeSSE(w http.ResponseWriter, response anthropic.Response) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)
	id := response.ID
	if id == "" {
		id = newID("msg")
	}
	partial := response
	partial.ID = id
	partial.Content = []anthropic.ContentBlock{}
	writeEvent(w, flusher, "message_start", map[string]interface{}{"type": "message_start", "message": partial})
	for index, block := range response.Content {
		start := map[string]interface{}{"type": "content_block_start", "index": index, "content_block": block}
		if block.Type == "tool_use" {
			start["content_block"] = map[string]interface{}{"type": "tool_use", "id": block.ID, "name": block.Name, "input": map[string]interface{}{}}
		}
		writeEvent(w, flusher, "content_block_start", start)
		switch block.Type {
		case "text":
			writeEvent(w, flusher, "content_block_delta", map[string]interface{}{"type": "content_block_delta", "index": index, "delta": map[string]interface{}{"type": "text_delta", "text": block.Text}})
		case "tool_use":
			writeEvent(w, flusher, "content_block_delta", map[string]interface{}{"type": "content_block_delta", "index": index, "delta": map[string]interface{}{"type": "input_json_delta", "partial_json": string(block.Input)}})
		}
		writeEvent(w, flusher, "content_block_stop", map[string]interface{}{"type": "content_block_stop", "index": index})
	}
	writeEvent(w, flusher, "message_delta", map[string]interface{}{"type": "message_delta", "delta": map[string]interface{}{"stop_reason": response.StopReason, "stop_sequence": response.StopSequence}, "usage": response.Usage})
	writeEvent(w, flusher, "message_stop", map[string]interface{}{"type": "message_stop"})
}

func writeEvent(w io.Writer, flusher http.Flusher, event string, value interface{}) {
	b, _ := json.Marshal(value)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	if flusher != nil {
		flusher.Flush()
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately no prompt, response, header, or query logging.
		next.ServeHTTP(w, r)
	})
}

func newID(prefix string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return prefix + "_neoswitch"
	}
	return prefix + "_" + hex.EncodeToString(random[:])
}
