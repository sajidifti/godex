package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatStreamSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}

		var body struct {
			Model    string `json:"model"`
			Stream   bool   `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Model != "some/model" {
			t.Errorf("model = %q, want %q", body.Model, "some/model")
		}
		if !body.Stream {
			t.Errorf("stream = false, want true")
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" {
			t.Errorf("messages = %+v, want one user message", body.Messages)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		for _, delta := range []string{"hel", "lo"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", delta)
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClient("test-key", srv.URL)
	var got strings.Builder
	err := c.ChatStream(context.Background(), "some/model",
		[]Message{{Role: "user", Content: "hi"}},
		func(delta string) { got.WriteString(delta) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "hello" {
		t.Errorf("got %q, want %q", got.String(), "hello")
	}
}

func TestChatStreamAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"slow down"}}`)
	}))
	defer srv.Close()

	c := NewClient("k", srv.URL)
	err := c.ChatStream(context.Background(), "m", nil, func(string) {})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.Status != 429 {
		t.Errorf("status = %d, want 429", apiErr.Status)
	}
}

func TestChatStreamMidStreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"error\":{\"message\":\"provider melted\"}}\n\n")
	}))
	defer srv.Close()

	c := NewClient("k", srv.URL)
	err := c.ChatStream(context.Background(), "m", nil, func(string) {})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.Message != "provider melted" {
		t.Errorf("message = %q, want %q", apiErr.Message, "provider melted")
	}
}
