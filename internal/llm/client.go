package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultBaseURL = "https://openrouter.ai/api/v1"

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolDef struct {
	Type     string   `json:"type"`
	Function ToolFunc `json:"function"`
}

type ToolFunc struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Response struct {
	Content   string
	ToolCalls []ToolCall
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
	Stream   bool      `json:"stream,omitempty"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string          `json:"content"`
			ToolCalls []toolCallDelta `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type toolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized:
		return "invalid API key (401)"
	case http.StatusPaymentRequired:
		return "insufficient credits for this model (402)"
	case http.StatusTooManyRequests:
		return "rate limited (429): " + e.Message
	}
	return fmt.Sprintf("openrouter error %d: %s", e.Status, e.Message)
}

type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 2 * time.Minute},
	}
}

// ChatStream sends msgs to the model and invokes onDelta for every content
// token as it arrives from the server-sent event stream.
func (c *Client) ChatStream(ctx context.Context, model string, msgs []Message, tools []ToolDef, onDelta func(string)) (Response, error) {
	body, err := json.Marshal(chatRequest{Model: model, Messages: msgs, Tools: tools, Stream: true})
	if err != nil {
		return Response{}, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Response{}, readAPIError(resp)
	}
	return readStream(resp.Body, onDelta)
}

func readAPIError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	msg := strings.TrimSpace(string(data))

	var payload struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Error != nil {
		msg = payload.Error.Message
	}
	return &APIError{Status: resp.StatusCode, Message: msg}
}

func readStream(r io.Reader, onDelta func(string)) (Response, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var resp Response
	var content strings.Builder

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		switch {
		case data == "":
			continue
		case data == "[DONE]":
			resp.Content = content.String()
			return resp, nil
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return Response{}, &APIError{Status: http.StatusOK, Message: chunk.Error.Message}
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			content.WriteString(delta.Content)
			onDelta(delta.Content)
		}
		for _, tc := range delta.ToolCalls {
			appendToolCall(&resp.ToolCalls, tc)
		}
	}
	resp.Content = content.String()
	return resp, sc.Err()
}

func appendToolCall(calls *[]ToolCall, tc toolCallDelta) {
	for len(*calls) <= tc.Index {
		*calls = append(*calls, ToolCall{Type: "function"})
	}
	c := &(*calls)[tc.Index]
	if tc.ID != "" {
		c.ID = tc.ID
	}
	if tc.Type != "" {
		c.Type = tc.Type
	}
	if tc.Function.Name != "" {
		c.Function.Name = tc.Function.Name
	}
	c.Function.Arguments += tc.Function.Arguments
}
