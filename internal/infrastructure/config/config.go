// Package config provides centralized configuration management.
//
// Configuration can be loaded from:
//  1. YAML file (config.yaml)
//  2. Environment variables (fallback)
//
// Example usage:
//
//	cfg := config.LoadOrEnv()
//	dbPath := cfg.Storage.DatabasePath
//	monarchToken := cfg.Monarch.APIKey
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the entire application configuration
type Config struct {
	Providers     ProvidersConfig     `yaml:"providers"`
	Monarch       MonarchConfig       `yaml:"monarch"`
	OpenAI        OpenAIConfig        `yaml:"openai"`
	Anthropic     AnthropicConfig     `yaml:"anthropic"`
	Categorizer   CategorizerConfig   `yaml:"categorizer"`
	Storage       StorageConfig       `yaml:"storage"`
	Observability ObservabilityConfig `yaml:"observability"`
}

// StorageConfig holds database configuration
type StorageConfig struct {
	DatabasePath string `yaml:"database_path"`
}

// MonarchConfig holds Monarch API configuration
type MonarchConfig struct {
	APIKey string `yaml:"api_key"`
}

// OpenAIConfig holds OpenAI API configuration
type OpenAIConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// AnthropicConfig holds Anthropic (Claude) API configuration
type AnthropicConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// CategorizerConfig selects which LLM backend the categorizer uses.
// Provider may be "openai", "anthropic", or "" (auto-detect from which
// API key is set).
type CategorizerConfig struct {
	Provider string `yaml:"provider"`
}

// ProvidersConfig holds provider-specific configuration
type ProvidersConfig struct {
	Walmart WalmartConfig `yaml:"walmart"`
	Costco  CostcoConfig  `yaml:"costco"`
	Amazon  AmazonConfig  `yaml:"amazon"`
}

// WalmartConfig holds Walmart-specific settings
type WalmartConfig struct {
	Enabled      bool   `yaml:"enabled"`
	RateLimit    string `yaml:"rate_limit"`
	LookbackDays int    `yaml:"lookback_days"`
	MaxOrders    int    `yaml:"max_orders"`
	Debug        bool   `yaml:"debug"`
}

// CostcoConfig holds Costco-specific settings
type CostcoConfig struct {
	Enabled         bool   `yaml:"enabled"`
	RateLimit       string `yaml:"rate_limit"`
	LookbackDays    int    `yaml:"lookback_days"`
	MaxOrders       int    `yaml:"max_orders"`
	Debug           bool   `yaml:"debug"`
	Email           string `yaml:"email"`
	Password        string `yaml:"password"`
	WarehouseNumber string `yaml:"warehouse_number"`
}

// AmazonConfig holds Amazon-specific settings
type AmazonConfig struct {
	Enabled        bool   `yaml:"enabled"`
	RateLimit      string `yaml:"rate_limit"`
	LookbackDays   int    `yaml:"lookback_days"`
	MaxOrders      int    `yaml:"max_orders"`
	Debug          bool   `yaml:"debug"`
	AccountName    string `yaml:"account_name"`     // For multi-account support (optional)
	BrowserDataDir string `yaml:"browser_data_dir"` // Base directory for Amazon scraper browser profiles
}

// ObservabilityConfig holds observability settings
type ObservabilityConfig struct {
	Logging LoggingConfig `yaml:"logging"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	FilePath string `yaml:"file_path"` // Optional: path to log file (logs to both stdout and file)
}

// Load reads and parses the config file
func Load(path string) (*Config, error) {
	rootDir, configPath, err := validateConfigPath(path)
	if err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = root.Close()
	}()

	data, err := root.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Expand environment variables (e.g., ${MONARCH_TOKEN})
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateConfigPath(path string) (string, string, error) {
	if strings.TrimSpace(path) == "" {
		return "", "", fmt.Errorf("config path cannot be empty")
	}

	cleanPath := filepath.Clean(path)
	ext := strings.ToLower(filepath.Ext(cleanPath))
	if ext != ".yaml" && ext != ".yml" {
		return "", "", fmt.Errorf("config path must point to a YAML file")
	}

	rootDir := "."
	rootPath := cleanPath
	if !filepath.IsAbs(cleanPath) {
		if strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) || cleanPath == ".." {
			return "", "", fmt.Errorf("relative config path must stay within the working directory")
		}
	} else {
		rootDir = filepath.Dir(cleanPath)
		rootPath = filepath.Base(cleanPath)
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("config path must be a file")
	}

	return rootDir, rootPath, nil
}

// LoadFromEnv loads configuration from environment variables only
func LoadFromEnv() *Config {
	return &Config{
		Storage: StorageConfig{
			DatabasePath: getEnv("MONARCH_DB_PATH", "monarch_sync.db"),
		},
		Monarch: MonarchConfig{
			APIKey: os.Getenv("MONARCH_TOKEN"),
		},
		OpenAI: OpenAIConfig{
			APIKey: os.Getenv("OPENAI_API_KEY"),
			Model:  getEnv("OPENAI_MODEL", "gpt-5.4-nano"),
		},
		Anthropic: AnthropicConfig{
			APIKey: firstNonEmpty(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("CLAUDE_API_KEY")),
			Model:  getEnv("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001"),
		},
		Categorizer: CategorizerConfig{
			Provider: os.Getenv("CATEGORIZER_PROVIDER"),
		},
		Providers: ProvidersConfig{
			Walmart: WalmartConfig{
				Enabled:      true,
				LookbackDays: getEnvInt("WALMART_LOOKBACK_DAYS", 14),
				MaxOrders:    getEnvInt("WALMART_MAX_ORDERS", 0),
			},
			Costco: CostcoConfig{
				Enabled:      true,
				LookbackDays: getEnvInt("COSTCO_LOOKBACK_DAYS", 14),
				MaxOrders:    getEnvInt("COSTCO_MAX_ORDERS", 0),
			},
			Amazon: AmazonConfig{
				Enabled:        true,
				LookbackDays:   getEnvInt("AMAZON_LOOKBACK_DAYS", 14),
				MaxOrders:      getEnvInt("AMAZON_MAX_ORDERS", 0),
				AccountName:    getEnv("AMAZON_ACCOUNT_NAME", ""),
				BrowserDataDir: getEnv("AMAZON_BROWSER_DATA_DIR", getEnv("BROWSER_DATA_DIR", "")),
			},
		},
		Observability: ObservabilityConfig{
			Logging: LoggingConfig{
				Level:  getEnv("LOG_LEVEL", "info"),
				Format: getEnv("LOG_FORMAT", "text"),
			},
		},
	}
}

// LoadOrEnv tries to load from config.yaml, falls back to environment variables
func LoadOrEnv() *Config {
	return LoadOrEnv_WithPath("config.yaml")
}

// LoadOrEnv_WithPath tries to load from specified path, falls back to environment variables
func LoadOrEnv_WithPath(path string) *Config {
	if cfg, err := Load(path); err == nil {
		return cfg
	}
	return LoadFromEnv()
}

// firstNonEmpty returns the first non-empty string from its arguments.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// getEnv retrieves an environment variable with a fallback default
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// getEnvInt retrieves an integer environment variable with a fallback default
func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		var result int
		if _, err := fmt.Sscanf(val, "%d", &result); err == nil {
			return result
		}
	}
	return fallback
}

// GetAPIKey retrieves an API key from config first, then tries multiple environment variable names
// Usage: GetAPIKey(cfg.Monarch.APIKey, "MONARCH_TOKEN")
//
//	GetAPIKey(cfg.OpenAI.APIKey, "OPENAI_API_KEY", "OPENAI_APIKEY")
func (c *Config) GetAPIKey(configValue string, envVarNames ...string) string {
	// First, try the config value
	if configValue != "" {
		return configValue
	}

	// Then try each environment variable in order
	for _, envVar := range envVarNames {
		if val := os.Getenv(envVar); val != "" {
			return val
		}
	}

	return ""
}
