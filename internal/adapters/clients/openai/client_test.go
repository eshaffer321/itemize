package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
)

// newTestClient returns a Client pointed at the given test server.
func newTestClient(srv *httptest.Server) *Client {
	c := NewClient("test-key")
	c.baseURL = srv.URL
	return c
}

func TestCreateChatCompletion_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	resp, err := client.CreateChatCompletion(context.Background(), categorizer.ChatCompletionRequest{
		Model:    "gpt-test",
		Messages: []categorizer.Message{{Role: "user", Content: "hi"}},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "hello", resp.Choices[0].Message.Content)
}

func TestCreateChatCompletion_StructuredErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"invalid api key","type":"invalid_request_error","code":"invalid_api_key"}}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.CreateChatCompletion(context.Background(), categorizer.ChatCompletionRequest{
		Model:    "gpt-test",
		Messages: []categorizer.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAI API error")
	assert.Contains(t, err.Error(), "invalid api key")
}

func TestCreateChatCompletion_OpaqueServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "upstream borked")
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.CreateChatCompletion(context.Background(), categorizer.ChatCompletionRequest{
		Model:    "gpt-test",
		Messages: []categorizer.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}
