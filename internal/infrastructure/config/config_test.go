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
	assert.Equal(t, "gpt-5.4-nano", cfg.OpenAI.Model)
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
	assert.Equal(t, "gpt-5.4-nano", cfg.OpenAI.Model)
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

	err := os.WriteFile(configPath, []byte(configContent), 0600)
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

func TestValidateConfigPathRejectsUnsafePaths(t *testing.T) {
	tests := []string{
		"",
		"../config.yaml",
		"config.json",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			_, _, err := validateConfigPath(path)
			require.Error(t, err)
		})
	}
}

func TestValidateConfigPathAcceptsRelativeYAMLWithinWorkingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.Mkdir("config", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("config", "local.yml"), []byte("storage: {}\n"), 0600))

	rootDir, rootPath, err := validateConfigPath(filepath.Join("config", "local.yml"))

	require.NoError(t, err)
	assert.Equal(t, ".", rootDir)
	assert.Equal(t, filepath.Join("config", "local.yml"), rootPath)
}

func TestValidateConfigPathAcceptsAbsoluteYAML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("storage: {}\n"), 0600))

	rootDir, rootPath, err := validateConfigPath(configPath)

	require.NoError(t, err)
	assert.Equal(t, filepath.Dir(configPath), rootDir)
	assert.Equal(t, filepath.Base(configPath), rootPath)
}

func TestValidateConfigPathRejectsMissingFileAndDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, err := validateConfigPath(filepath.Join(tmpDir, "missing.yaml"))
	require.Error(t, err)

	_, _, err = validateConfigPath(tmpDir + string(filepath.Separator) + "config.yaml" + string(filepath.Separator))
	require.Error(t, err)

	dirPath := filepath.Join(tmpDir, "directory.yaml")
	require.NoError(t, os.Mkdir(dirPath, 0700))

	_, _, err = validateConfigPath(dirPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a file")
}
