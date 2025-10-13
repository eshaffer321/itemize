package cli

import (
	"fmt"
	"strings"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/storage"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/sync"
)

// PrintHeader prints the application header
func PrintHeader(providerName string, dryRun bool) {
	fmt.Printf("🛒 %s → Monarch Money Sync\n", providerName)
	fmt.Println("=" + strings.Repeat("=", 50))
	if dryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes will be made")
	} else {
		fmt.Println("⚠️  PRODUCTION MODE - Will update Monarch transactions!")
	}
	fmt.Println()
}

// PrintConfiguration prints sync configuration
func PrintConfiguration(providerName string, lookbackDays, maxOrders int, force bool) {
	fmt.Println("📅 Configuration:")
	fmt.Printf("   Provider: %s\n", providerName)
	fmt.Printf("   Lookback: %d days\n", lookbackDays)
	fmt.Printf("   Max orders: %d\n", maxOrders)
	fmt.Printf("   Force reprocess: %v\n", force)
	fmt.Println()
}

// PrintSyncSummary prints the sync result summary
func PrintSyncSummary(result *sync.Result, store *storage.Storage, dryRun bool) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 SUMMARY")
	fmt.Printf("   Processed: %d\n", result.ProcessedCount)
	fmt.Printf("   Skipped:   %d\n", result.SkippedCount)
	fmt.Printf("   Errors:    %d\n", result.ErrorCount)

	// Print errors if any
	if len(result.Errors) > 0 {
		fmt.Println("\n⚠️  ERRORS:")
		for _, err := range result.Errors {
			fmt.Printf("   - %v\n", err)
		}
	}

	// Get stats from database
	if store != nil {
		stats, _ := store.GetStats()
		if stats != nil && stats.TotalProcessed > 0 {
			fmt.Printf("\n📈 ALL TIME STATS\n")
			fmt.Printf("   Total Orders: %d\n", stats.TotalProcessed)
			fmt.Printf("   Total Splits: %d\n", stats.TotalSplits)
			fmt.Printf("   Total Amount: $%.2f\n", stats.TotalAmount)
			fmt.Printf("   Success Rate: %.1f%%\n", stats.SuccessRate)
		}
	}

	if !dryRun && result.ProcessedCount > 0 {
		fmt.Println("\n✅ Successfully updated Monarch Money transactions!")
	}
}
