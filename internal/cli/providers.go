package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	costcogo "github.com/eshaffer321/costco-go/pkg/costco"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
	amazonprovider "github.com/eshaffer321/itemize/internal/adapters/providers/amazon"
	"github.com/eshaffer321/itemize/internal/adapters/providers/costco"
	"github.com/eshaffer321/itemize/internal/adapters/providers/walmart"
	"github.com/eshaffer321/itemize/internal/infrastructure/config"
	"github.com/eshaffer321/itemize/internal/infrastructure/logging"
	walmartclient "github.com/eshaffer321/walmart-client-go/v2"
)

// NewCostcoProvider creates a new Costco provider with a system-scoped logger
func NewCostcoProvider(cfg *config.Config, verbose bool) (providers.OrderProvider, error) {
	// Load Costco config
	savedConfig, err := costcogo.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load Costco config: %w", err)
	}

	// Create a costco-scoped logger with verbose flag
	loggingCfg := cfg.Observability.Logging
	if verbose {
		loggingCfg.Level = "debug"
	}
	costcoLogger := logging.NewLoggerWithSystem(loggingCfg, "costco")

	costcoConfig := costcogo.Config{
		Email:           savedConfig.Email,
		WarehouseNumber: savedConfig.WarehouseNumber,
		Logger:          costcoLogger,
	}

	costcoClient := costcogo.NewClient(costcoConfig)
	return costco.NewProvider(costcoClient, costcoLogger), nil
}

// NewWalmartProvider creates a new Walmart provider with a system-scoped logger
func NewWalmartProvider(cfg *config.Config, verbose bool) (providers.OrderProvider, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create a walmart-scoped logger with verbose flag
	loggingCfg := cfg.Observability.Logging
	if verbose {
		loggingCfg.Level = "debug"
	}
	walmartLogger := logging.NewLoggerWithSystem(loggingCfg, "walmart")

	walmartConfig := walmartclient.ClientConfig{
		RateLimit:       2 * time.Second,  // General rate limit for orders
		LedgerRateLimit: 30 * time.Second, // Stricter limit for ledger API (v1.0.6)
		MaxRetries:      3,                // Auto-retry on 429 with exponential backoff (v1.0.6)
		AutoSave:        true,
		CookieDir:       filepath.Join(homeDir, ".walmart-api"),
		CookieFile:      filepath.Join(homeDir, ".walmart-api", "cookies.json"),
		Logger:          walmartLogger,
	}

	walmartClient, err := walmartclient.NewWalmartClient(walmartConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Walmart client: %w", err)
	}

	return walmart.NewProvider(walmartClient, walmartLogger), nil
}

// NewAmazonProvider creates a new Amazon provider with a system-scoped logger
// Uses the amazon-order-scraper CLI (npm package) for fetching orders.
// account, if non-empty, overrides cfg.Providers.Amazon.AccountName (and thus
// AMAZON_ACCOUNT_NAME) — it's the value of the -account flag.
func NewAmazonProvider(cfg *config.Config, verbose bool, account string) (providers.OrderProvider, error) {
	// Create an amazon-scoped logger with verbose flag
	loggingCfg := cfg.Observability.Logging
	if verbose {
		loggingCfg.Level = "debug"
	}
	amazonLogger := logging.NewLoggerWithSystem(loggingCfg, "amazon")

	browserDataDir, err := resolveAmazonBrowserDataDir(cfg.Providers.Amazon.BrowserDataDir)
	if err != nil {
		return nil, err
	}

	profile := cfg.Providers.Amazon.AccountName
	if account != "" {
		profile = account
	}

	// Build provider config
	providerCfg := &amazonprovider.ProviderConfig{
		Profile:        profile,
		Headless:       false, // Default to non-headless for interactive use
		BrowserDataDir: browserDataDir,
	}

	return amazonprovider.NewProvider(amazonLogger, providerCfg), nil
}

// ListAmazonAccounts returns the names of saved Amazon browser profiles found
// under the resolved browser data directory (each profile is a subdirectory
// created the first time `amazon-scraper --login --profile <name>` runs).
func ListAmazonAccounts(cfg *config.Config) ([]string, error) {
	browserDataDir, err := resolveAmazonBrowserDataDir(cfg.Providers.Amazon.BrowserDataDir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(browserDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read browser data directory: %w", err)
	}

	var accounts []string
	for _, entry := range entries {
		if entry.IsDir() {
			accounts = append(accounts, entry.Name())
		}
	}
	return accounts, nil
}

func resolveAmazonBrowserDataDir(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	if envDir := os.Getenv("AMAZON_BROWSER_DATA_DIR"); envDir != "" {
		return envDir, nil
	}
	if envDir := os.Getenv("BROWSER_DATA_DIR"); envDir != "" {
		return envDir, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".itemize", "amazon"), nil
}
