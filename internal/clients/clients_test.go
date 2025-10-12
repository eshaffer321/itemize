package clients

import (
	"testing"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClients_Success(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Monarch: config.MonarchConfig{
			APIKey: "test-monarch-token",
		},
		OpenAI: config.OpenAIConfig{
			APIKey: "test-openai-key",
			Model:  "gpt-4o",
		},
	}

	// Act
	clients, err := NewClients(cfg)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, clients)
	assert.NotNil(t, clients.Monarch)
	assert.NotNil(t, clients.Categorizer)
}

func TestNewClients_WithExplicitModel(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Monarch: config.MonarchConfig{
			APIKey: "test-monarch-token",
		},
		OpenAI: config.OpenAIConfig{
			APIKey: "test-openai-key",
			Model:  "gpt-4-turbo",
		},
	}

	// Act
	clients, err := NewClients(cfg)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, clients)
	assert.NotNil(t, clients.Categorizer)
}

func TestNewClients_DefaultModel(t *testing.T) {
	// Arrange - Model is empty, should use default "gpt-4o"
	cfg := &config.Config{
		Monarch: config.MonarchConfig{
			APIKey: "test-monarch-token",
		},
		OpenAI: config.OpenAIConfig{
			APIKey: "test-openai-key",
			// Model is empty, should use default
		},
	}

	// Act
	clients, err := NewClients(cfg)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, clients)
	assert.NotNil(t, clients.Categorizer)
}
