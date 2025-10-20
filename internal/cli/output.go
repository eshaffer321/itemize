package cli

import (
	"fmt"
	"strings"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/sync"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// PrintHeader prints the application header
func PrintHeader(providerName string, dryRun bool) {
	mode := "PRODUCTION"
	if dryRun {
		mode = "DRY-RUN"
	}
	fmt.Printf("monarch-sync: %s (%s mode)\n", providerName, mode)
}

// PrintConfiguration prints sync configuration
func PrintConfiguration(providerName string, lookbackDays, maxOrders int, force bool) {
	fmt.Printf("Provider: %s | Lookback: %d days", providerName, lookbackDays)
	if maxOrders > 0 {
		fmt.Printf(" | Max orders: %d", maxOrders)
	}
	if force {
		fmt.Printf(" | Force: true")
	}
	fmt.Println("\n")
}

// PrintSyncSummary prints the sync result summary
func PrintSyncSummary(result *sync.Result, store *storage.Storage, dryRun bool) {
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Summary: Processed=%d Skipped=%d Errors=%d\n",
		result.ProcessedCount,
		result.SkippedCount,
		result.ErrorCount)

	// Print errors if any
	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, err := range result.Errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	// Get stats from database
	if store != nil {
		stats, _ := store.GetStats()
		if stats != nil && stats.TotalProcessed > 0 {
			successRate := 0.0
			if stats.TotalProcessed > 0 {
				successRate = float64(stats.SuccessCount) / float64(stats.TotalProcessed) * 100
			}
			fmt.Printf("\nAll-Time Stats: Orders=%d Splits=%d Amount=$%.2f Success=%.1f%%\n",
				stats.TotalProcessed,
				stats.TotalSplits,
				stats.TotalAmount,
				successRate)
		}
	}

	if !dryRun && result.ProcessedCount > 0 {
		fmt.Println("\nSync completed successfully.")
	}
}
