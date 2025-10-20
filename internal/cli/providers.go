package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	costcogo "github.com/costco-go/pkg/costco"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers/costco"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers/walmart"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/logging"
	walmartclient "github.com/eshaffer321/walmart-client"
)

// NewCostcoProvider creates a new Costco provider with a system-scoped logger
func NewCostcoProvider(cfg *config.Config) (providers.OrderProvider, error) {
	// Load Costco config
	savedConfig, err := costcogo.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load Costco config: %w", err)
	}

	// Create a costco-scoped logger
	costcoLogger := logging.NewLoggerWithSystem(cfg.Observability.Logging, "costco")

	costcoConfig := costcogo.Config{
		Email:           savedConfig.Email,
		Password:        "",
		WarehouseNumber: savedConfig.WarehouseNumber,
		Logger:          costcoLogger,
	}

	costcoClient := costcogo.NewClient(costcoConfig)
	return costco.NewProvider(costcoClient, costcoLogger), nil
}

// NewWalmartProvider creates a new Walmart provider with a system-scoped logger
func NewWalmartProvider(cfg *config.Config) (providers.OrderProvider, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create a walmart-scoped logger
	walmartLogger := logging.NewLoggerWithSystem(cfg.Observability.Logging, "walmart")

	walmartConfig := walmartclient.ClientConfig{
		RateLimit:  2 * time.Second,
		AutoSave:   true,
		CookieDir:  filepath.Join(homeDir, ".walmart-api"),
		CookieFile: filepath.Join(homeDir, ".walmart-api", "cookies.json"),
		Logger:     walmartLogger,
	}

	walmartClient, err := walmartclient.NewWalmartClient(walmartConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Walmart client: %w", err)
	}

	return walmart.NewProvider(walmartClient, walmartLogger), nil
}
