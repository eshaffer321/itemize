package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromYAML(t *testing.T) {
	// Test loading from config.yaml - find it relative to project root
	// Try common locations
	configPaths := []string{
		"../../../config.yaml", // From internal/infrastructure/config
		"../../config.yaml",    // If directory structure changes
		"config.yaml",          // From root
	}

	var cfg *Config
	var err error
	found := false

	for _, path := range configPaths {
		cfg, err = Load(path)
		if err == nil {
			found = true
			break
		}
	}

	if !found {
		t.Skip("config.yaml not found in expected locations")
	}

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "monarch_sync.db", cfg.Storage.DatabasePath)
	assert.Equal(t, "gpt-4o", cfg.OpenAI.Model)
}

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("MONARCH_DB_PATH", "test.db")
	os.Setenv("MONARCH_TOKEN", "test-token")
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer func() {
		os.Unsetenv("MONARCH_DB_PATH")
		os.Unsetenv("MONARCH_TOKEN")
		os.Unsetenv("OPENAI_API_KEY")
	}()

	cfg := LoadFromEnv()
	assert.NotNil(t, cfg)
	assert.Equal(t, "test.db", cfg.Storage.DatabasePath)
	assert.Equal(t, "test-token", cfg.Monarch.APIKey)
	assert.Equal(t, "test-key", cfg.OpenAI.APIKey)
}

func TestLoadFromEnv_Defaults(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("MONARCH_DB_PATH")
	os.Unsetenv("OPENAI_MODEL")

	cfg := LoadFromEnv()
	assert.NotNil(t, cfg)
	assert.Equal(t, "monarch_sync.db", cfg.Storage.DatabasePath)
	assert.Equal(t, "gpt-4o", cfg.OpenAI.Model)
}

func TestLoadOrEnv(t *testing.T) {
	// Test that LoadOrEnv tries YAML first, falls back to env
	cfg := LoadOrEnv()
	assert.NotNil(t, cfg)
	assert.Equal(t, "monarch_sync.db", cfg.Storage.DatabasePath)
}

func TestLoadOrEnv_FallbackToEnv(t *testing.T) {
	// Test fallback when config file doesn't exist
	os.Setenv("MONARCH_DB_PATH", "fallback.db")
	defer os.Unsetenv("MONARCH_DB_PATH")

	// Try to load from non-existent file
	cfg := LoadOrEnv_WithPath("nonexistent.yaml")
	assert.NotNil(t, cfg)
	assert.Equal(t, "fallback.db", cfg.Storage.DatabasePath)
}

func TestEnvVarExpansion(t *testing.T) {
	// Create temp config file with env vars
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
storage:
  database_path: "${TEST_DB_PATH}"
monarch:
  api_key: "${TEST_MONARCH_TOKEN}"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set env vars
	os.Setenv("TEST_DB_PATH", "expanded.db")
	os.Setenv("TEST_MONARCH_TOKEN", "expanded-token")
	defer func() {
		os.Unsetenv("TEST_DB_PATH")
		os.Unsetenv("TEST_MONARCH_TOKEN")
	}()

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "expanded.db", cfg.Storage.DatabasePath)
	assert.Equal(t, "expanded-token", cfg.Monarch.APIKey)
}
