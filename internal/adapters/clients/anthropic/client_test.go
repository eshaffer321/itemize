package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eshaffer321/itemize/internal/domain/categorizer"
)

func float64Ptr(v float64) *float64 { return &v }

// newTestClient returns a Client wired to the given test server.
func newTestClient(srv *httptest.Server) *Client {
	c := NewClient("test-key")
	c.baseURL = srv.URL
	return c
}

func TestBuildMessagesRequest_MapsSystemAndUserMessages(t *testing.T) {
	req := categorizer.ChatCompletionRequest{
		Model:       "claude-haiku-4-5-20251001",
		Temperature: float64Ptr(0.1),
		Messages: []categorizer.Message{
			{Role: "system", Content: "you are a sorter"},
			{Role: "user", Content: "sort these"},
		},
	}

	got, prefilled := buildMessagesRequest(req)

	assert.False(t, prefilled)
	assert.Equal(t, "claude-haiku-4-5-20251001", got.Model)
	assert.Equal(t, defaultMaxTokens, got.MaxTokens)
	assert.Equal(t, "you are a sorter", got.System)
	require.NotNil(t, got.Temperature)
	assert.InDelta(t, 0.1, *got.Temperature, 1e-9)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, "user", got.Messages[0].Role)
	assert.Equal(t, "sort these", got.Messages[0].Content)
}

func TestBuildMessagesRequest_PrefillsWhenJSONRequested(t *testing.T) {
	req := categorizer.ChatCompletionRequest{
		Model: "claude-haiku-4-5-20251001",
		Messages: []categorizer.Message{
			{Role: "user", Content: "give me json"},
		},
		ResponseFormat: &categorizer.ResponseFormat{Type: "json_object"},
	}

	got, prefilled := buildMessagesRequest(req)

	assert.True(t, prefilled)
	require.Len(t, got.Messages, 2)
	assert.Equal(t, "assistant", got.Messages[1].Role)
	assert.Equal(t, "{", got.Messages[1].Content)
}

func TestBuildMessagesRequest_ConcatenatesMultipleSystemMessages(t *testing.T) {
	req := categorizer.ChatCompletionRequest{
		Model: "claude-haiku-4-5-20251001",
		Messages: []categorizer.Message{
			{Role: "system", Content: "first"},
			{Role: "system", Content: "second"},
			{Role: "user", Content: "go"},
		},
	}

	got, _ := buildMessagesRequest(req)

	assert.Equal(t, "first\n\nsecond", got.System)
}

func TestCreateChatCompletion_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/messages", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, defaultAPIVersion, r.Header.Get("anthropic-version"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body messagesRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "claude-haiku-4-5-20251001", body.Model)
		assert.Equal(t, defaultMaxTokens, body.MaxTokens)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"hello back"}]}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	resp, err := client.CreateChatCompletion(context.Background(), categorizer.ChatCompletionRequest{
		Model:    "claude-haiku-4-5-20251001",
		Messages: []categorizer.Message{{Role: "user", Content: "hi"}},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "hello back", resp.Choices[0].Message.Content)
}

func TestCreateChatCompletion_PrefillsAndReassemblesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body messagesRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		// Last message must be the "{" prefill on the assistant turn.
		require.Len(t, body.Messages, 2)
		assert.Equal(t, "assistant", body.Messages[1].Role)
		assert.Equal(t, "{", body.Messages[1].Content)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"\"ok\":true}"}]}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	resp, err := client.CreateChatCompletion(context.Background(), categorizer.ChatCompletionRequest{
		Model:          "claude-haiku-4-5-20251001",
		Messages:       []categorizer.Message{{Role: "user", Content: "json please"}},
		ResponseFormat: &categorizer.ResponseFormat{Type: "json_object"},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Choices, 1)
	// Prefill reassembled — full message must be valid JSON.
	assert.Equal(t, `{"ok":true}`, resp.Choices[0].Message.Content)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(resp.Choices[0].Message.Content), &parsed))
	assert.Equal(t, true, parsed["ok"])
}

func TestCreateChatCompletion_StructuredErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.CreateChatCompletion(context.Background(), categorizer.ChatCompletionRequest{
		Model:    "claude-haiku-4-5-20251001",
		Messages: []categorizer.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "anthropic API error")
	assert.Contains(t, err.Error(), "invalid x-api-key")
	assert.Contains(t, err.Error(), "authentication_error")
}

func TestCreateChatCompletion_OpaqueServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "upstream borked")
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.CreateChatCompletion(context.Background(), categorizer.ChatCompletionRequest{
		Model:    "claude-haiku-4-5-20251001",
		Messages: []categorizer.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
	// Categorizer's retry logic keys off the substring "502" — verify it's there.
	assert.True(t, strings.Contains(err.Error(), "502"))
}

func TestCreateChatCompletion_NoTextContentInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":[]}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.CreateChatCompletion(context.Background(), categorizer.ChatCompletionRequest{
		Model:    "claude-haiku-4-5-20251001",
		Messages: []categorizer.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no text content")
}
