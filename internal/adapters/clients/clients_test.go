package clients

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	anthropicclient "github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients/anthropic"
	openaiclient "github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients/openai"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// clearLLMEnv removes env vars that newChatClient consults via cfg.GetAPIKey
// so each test controls inputs purely through the config struct.
func clearLLMEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"OPENAI_API_KEY", "OPENAI_APIKEY",
		"ANTHROPIC_API_KEY", "CLAUDE_API_KEY",
	} {
		t.Setenv(k, "")
	}
}

func TestNewChatClient_ExplicitOpenAI(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{
		OpenAI:      config.OpenAIConfig{APIKey: "open-key", Model: "gpt-test"},
		Categorizer: config.CategorizerConfig{Provider: "openai"},
	}

	client, model, err := newChatClient(cfg, discardLogger())

	require.NoError(t, err)
	_, ok := client.(*openaiclient.Client)
	assert.True(t, ok, "expected openai client")
	assert.Equal(t, "gpt-test", model)
}

func TestNewChatClient_ExplicitOpenAI_MissingKey(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{
		Categorizer: config.CategorizerConfig{Provider: "openai"},
	}

	_, _, err := newChatClient(cfg, discardLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY")
}

func TestNewChatClient_ExplicitAnthropic(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{
		Anthropic:   config.AnthropicConfig{APIKey: "anth-key", Model: "claude-test"},
		Categorizer: config.CategorizerConfig{Provider: "anthropic"},
	}

	client, model, err := newChatClient(cfg, discardLogger())

	require.NoError(t, err)
	_, ok := client.(*anthropicclient.Client)
	assert.True(t, ok, "expected anthropic client")
	assert.Equal(t, "claude-test", model)
}

func TestNewChatClient_ExplicitAnthropic_MissingKey(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{
		Categorizer: config.CategorizerConfig{Provider: "anthropic"},
	}

	_, _, err := newChatClient(cfg, discardLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
}

func TestNewChatClient_UnknownProvider(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{
		Categorizer: config.CategorizerConfig{Provider: "gemini"},
	}

	_, _, err := newChatClient(cfg, discardLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown categorizer provider")
}

func TestNewChatClient_AutoDetect_OpenAIOnly(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{
		OpenAI: config.OpenAIConfig{APIKey: "open-key", Model: "gpt-test"},
	}

	client, model, err := newChatClient(cfg, discardLogger())

	require.NoError(t, err)
	_, ok := client.(*openaiclient.Client)
	assert.True(t, ok)
	assert.Equal(t, "gpt-test", model)
}

func TestNewChatClient_AutoDetect_AnthropicOnly(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{
		Anthropic: config.AnthropicConfig{APIKey: "anth-key", Model: "claude-test"},
	}

	client, model, err := newChatClient(cfg, discardLogger())

	require.NoError(t, err)
	_, ok := client.(*anthropicclient.Client)
	assert.True(t, ok)
	assert.Equal(t, "claude-test", model)
}

func TestNewChatClient_AutoDetect_BothKeysPrefersOpenAI(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{
		OpenAI:    config.OpenAIConfig{APIKey: "open-key", Model: "gpt-test"},
		Anthropic: config.AnthropicConfig{APIKey: "anth-key", Model: "claude-test"},
	}

	client, model, err := newChatClient(cfg, discardLogger())

	require.NoError(t, err)
	_, ok := client.(*openaiclient.Client)
	assert.True(t, ok, "both keys set without explicit provider should default to openai")
	assert.Equal(t, "gpt-test", model)
}

func TestNewChatClient_AutoDetect_NoKeys(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{}

	_, _, err := newChatClient(cfg, discardLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no LLM API key configured")
}

func TestNewChatClient_CaseInsensitiveProvider(t *testing.T) {
	clearLLMEnv(t)
	cfg := &config.Config{
		Anthropic:   config.AnthropicConfig{APIKey: "anth-key", Model: "claude-test"},
		Categorizer: config.CategorizerConfig{Provider: "  Anthropic  "},
	}

	client, _, err := newChatClient(cfg, discardLogger())

	require.NoError(t, err)
	_, ok := client.(*anthropicclient.Client)
	assert.True(t, ok)
}
