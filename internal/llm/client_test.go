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
	_, err := c.ChatStream(context.Background(), "some/model",
		[]Message{{Role: "user", Content: "hi"}}, nil, false,
		func(d Delta) { got.WriteString(d.Content) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "hello" {
		t.Errorf("got %q, want %q", got.String(), "hello")
	}
}

func TestChatStreamSendsReasoning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Reasoning *struct {
				Enabled bool `json:"enabled"`
			} `json:"reasoning"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Reasoning == nil || !body.Reasoning.Enabled {
			t.Errorf("reasoning = %+v, want {enabled:true}", body.Reasoning)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClient("k", srv.URL)
	if _, err := c.ChatStream(context.Background(), "m", nil, nil, true, func(Delta) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChatStreamReasoning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"reasoning\":%q}}]}\n\n", "think ")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"reasoning\":%q}}]}\n\n", "hard")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", "answer")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClient("k", srv.URL)
	var reason, content strings.Builder
	resp, err := c.ChatStream(context.Background(), "m", nil, nil, true, func(d Delta) {
		if d.Reasoning != "" {
			reason.WriteString(d.Reasoning)
		}
		if d.Content != "" {
			content.WriteString(d.Content)
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reason.String() != "think hard" {
		t.Errorf("streamed reasoning = %q, want %q", reason.String(), "think hard")
	}
	if content.String() != "answer" {
		t.Errorf("streamed content = %q, want %q", content.String(), "answer")
	}
	if resp.Reasoning != "think hard" || resp.Content != "answer" {
		t.Errorf("resp = %+v, want reasoning %q content %q", resp, "think hard", "answer")
	}
}

func TestChatStreamAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"slow down"}}`)
	}))
	defer srv.Close()

	c := NewClient("k", srv.URL)
	_, err := c.ChatStream(context.Background(), "m", nil, nil, false, func(Delta) {})

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
	_, err := c.ChatStream(context.Background(), "m", nil, nil, false, func(Delta) {})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.Message != "provider melted" {
		t.Errorf("message = %q, want %q", apiErr.Message, "provider melted")
	}
}

func TestChatStreamToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":""}}]}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":\""}}]}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"main.go\"}"}}]}}]}`+"\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClient("k", srv.URL)
	resp, err := c.ChatStream(context.Background(), "m", nil, nil, false, func(Delta) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(resp.ToolCalls))
	}
	if tc := resp.ToolCalls[0]; tc.Function.Name != "read_file" || tc.Function.Arguments != `{"path":"main.go"}` {
		t.Errorf("call = %+v, want read_file {\"path\":\"main.go\"}", tc)
	}
}
