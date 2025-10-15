package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	costcogo "github.com/costco-go/pkg/costco"
	walmartclient "github.com/eshaffer321/walmart-client"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers/costco"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers/walmart"
)

// NewCostcoProvider creates a new Costco provider
func NewCostcoProvider(cfg *config.Config, logger *slog.Logger) (providers.OrderProvider, error) {
	// Load Costco config
	savedConfig, err := costcogo.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load Costco config: %w", err)
	}

	costcoConfig := costcogo.Config{
		Email:           savedConfig.Email,
		Password:        "",
		WarehouseNumber: savedConfig.WarehouseNumber,
	}

	costcoClient := costcogo.NewClient(costcoConfig)
	return costco.NewProvider(costcoClient, logger), nil
}

// NewWalmartProvider creates a new Walmart provider
func NewWalmartProvider(cfg *config.Config, logger *slog.Logger) (providers.OrderProvider, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	walmartConfig := walmartclient.ClientConfig{
		RateLimit:  2 * time.Second,
		AutoSave:   true,
		CookieDir:  filepath.Join(homeDir, ".walmart-api"),
		CookieFile: filepath.Join(homeDir, ".walmart-api", "cookies.json"),
	}

	walmartClient, err := walmartclient.NewWalmartClient(walmartConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Walmart client: %w", err)
	}

	return walmart.NewProvider(walmartClient, logger), nil
}
