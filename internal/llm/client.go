package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultBaseURL = "https://openrouter.ai/api/v1"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
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

func (c *Client) Chat(ctx context.Context, model string, msgs []Message) (string, error) {
	body, err := json.Marshal(chatRequest{Model: model, Messages: msgs})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var out chatResponse
	jsonErr := json.Unmarshal(data, &out)

	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(data))

		if jsonErr == nil && out.Error != nil {
			msg = out.Error.Message
		}
		return "", &APIError{Status: resp.StatusCode, Message: msg}
	}
	if jsonErr != nil {
		return "", fmt.Errorf("decode response: %w", jsonErr)
	}

	if out.Error != nil {
		return "", &APIError{Status: resp.StatusCode, Message: out.Error.Message}
	}
	if len(out.Choices) == 0 {
		return "", errors.New("openrouter: response contained no choice")
	}
	return out.Choices[0].Message.Content, nil
}
