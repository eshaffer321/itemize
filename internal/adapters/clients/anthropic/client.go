// Package anthropic contains the Anthropic (Claude) HTTP adapter
// implementing categorizer.ChatClient.
//
// It translates the OpenAI-shaped chat-completion request the categorizer
// uses into Anthropic's Messages API request, and translates the response
// back. When the caller asks for JSON (ResponseFormat.Type == "json_object"),
// the adapter prefills the assistant turn with "{" so the model emits a
// JSON object suitable for the categorizer's existing json.Unmarshal step.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
)

const (
	defaultBaseURL       = "https://api.anthropic.com/v1"
	defaultAPIVersion    = "2023-06-01"
	defaultMaxTokens     = 4096
	defaultClientTimeout = 30 * time.Second
)

// Client is the HTTP-backed Anthropic implementation of categorizer.ChatClient.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	apiVersion string
}

// NewClient creates a new Anthropic client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		apiVersion: defaultAPIVersion,
		httpClient: &http.Client{Timeout: defaultClientTimeout},
	}
}

// CreateChatCompletion translates the request to Anthropic's Messages API,
// posts it, and returns the response in the categorizer's vocabulary.
func (c *Client) CreateChatCompletion(ctx context.Context, request categorizer.ChatCompletionRequest) (*categorizer.ChatCompletionResponse, error) {
	msgReq, prefilled := buildMessagesRequest(request)

	requestBody, err := json.Marshal(msgReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", c.apiVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("anthropic API error: %s (type: %s)",
				errorResp.Error.Message, errorResp.Error.Type)
		}
		return nil, fmt.Errorf("anthropic API returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseMessagesResponse(body, prefilled)
}

// messagesRequest is Anthropic's Messages API request body.
type messagesRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens"`
	System      string           `json:"system,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	Messages    []messagesTurn   `json:"messages"`
}

type messagesTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// messagesResponse is the relevant subset of Anthropic's Messages API response.
type messagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// buildMessagesRequest translates a ChatCompletionRequest into Anthropic's
// Messages API shape. Returns the request plus a bool indicating whether the
// assistant turn was prefilled with "{" — needed to reconstruct the JSON on
// the response side.
func buildMessagesRequest(req categorizer.ChatCompletionRequest) (messagesRequest, bool) {
	out := messagesRequest{
		Model:       req.Model,
		MaxTokens:   defaultMaxTokens,
		Temperature: req.Temperature,
	}

	for _, m := range req.Messages {
		if m.Role == "system" {
			// Anthropic takes system as a top-level field; concatenate
			// if multiple system messages are passed.
			if out.System == "" {
				out.System = m.Content
			} else {
				out.System += "\n\n" + m.Content
			}
			continue
		}
		out.Messages = append(out.Messages, messagesTurn{Role: m.Role, Content: m.Content})
	}

	prefilled := false
	if req.ResponseFormat != nil && req.ResponseFormat.Type == "json_object" {
		out.Messages = append(out.Messages, messagesTurn{Role: "assistant", Content: "{"})
		prefilled = true
	}

	return out, prefilled
}

// parseMessagesResponse extracts the assistant text from Anthropic's response
// and wraps it in the categorizer's ChatCompletionResponse vocabulary. If the
// assistant turn was prefilled with "{", that character is prepended so the
// caller can json.Unmarshal directly.
func parseMessagesResponse(body []byte, prefilled bool) (*categorizer.ChatCompletionResponse, error) {
	var resp messagesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var text string
	for _, block := range resp.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}
	if text == "" {
		return nil, fmt.Errorf("anthropic response contained no text content")
	}

	if prefilled {
		text = "{" + text
	}

	return &categorizer.ChatCompletionResponse{
		Choices: []categorizer.Choice{
			{Message: categorizer.Message{Role: "assistant", Content: text}},
		},
	}, nil
}
