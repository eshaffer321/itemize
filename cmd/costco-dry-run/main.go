package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	costcogo "github.com/costco-go/pkg/costco"
	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/matcher"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers/costco"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/storage"
)

func main() {
	fmt.Println("🛒 Costco → Monarch Money Sync (DRY RUN)")
	fmt.Println("=" + strings.Repeat("=", 50))
	fmt.Println()

	// Setup
	logger := slog.Default()
	ctx := context.Background()

	// Initialize storage
	// Use default database path
	dbPath := "costco_dry_run.db"
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Load Costco config and tokens
	savedConfig, err := costcogo.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load saved config: %v", err)
	}

	savedTokens, err := costcogo.LoadTokens()
	if err != nil {
		log.Fatalf("Failed to load saved tokens: %v", err)
	}

	// Create Costco client
	config := costcogo.Config{
		Email:           savedConfig.Email,
		Password:        "",
		WarehouseNumber: savedConfig.WarehouseNumber,
	}
	costcoClient := costcogo.NewClient(config)

	// Create provider
	provider := costco.NewProvider(costcoClient, logger)

	fmt.Println("📅 Configuration:")
	fmt.Printf("   Provider: %s\n", provider.DisplayName())
	fmt.Printf("   Email: %s\n", savedConfig.Email)
	fmt.Printf("   Warehouse: %s\n", savedConfig.WarehouseNumber)
	fmt.Printf("   Using saved tokens: %v\n", savedTokens != nil)
	fmt.Println()

	// Fetch Costco orders
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -14) // Last 14 days

	fmt.Printf("🛍️ Fetching Costco orders (%s to %s)...\n",
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"))

	orders, err := provider.FetchOrders(ctx, providers.FetchOptions{
		StartDate:      startDate,
		EndDate:        endDate,
		MaxOrders:      10,
		IncludeDetails: true,
	})
	if err != nil {
		log.Fatalf("Failed to fetch orders: %v", err)
	}

	fmt.Printf("   Found %d orders\n\n", len(orders))

	// Initialize Monarch client
	monarchToken := os.Getenv("MONARCH_TOKEN")
	if monarchToken == "" {
		fmt.Println("⚠️  MONARCH_TOKEN not set - simulating Monarch transactions")
		fmt.Println()

		// Simulate the sync process
		simulateDryRun(orders, store)
	} else {
		fmt.Println("💳 Connecting to Monarch Money...")
		monarchClient, err := monarch.NewClientWithToken(monarchToken)
		if err != nil {
			log.Fatalf("Failed to create Monarch client: %v", err)
		}

		// Fetch Monarch transactions
		fmt.Println("💳 Fetching Monarch transactions...")
		txList, err := monarchClient.Transactions.Query().
			Between(startDate.AddDate(0, 0, -7), endDate). // Add buffer for date matching
			Limit(500).
			Execute(ctx)
		if err != nil {
			log.Fatalf("Failed to fetch transactions: %v", err)
		}

		// Filter for Costco transactions
		var costcoTransactions []*monarch.Transaction
		for _, tx := range txList.Transactions {
			if tx.Merchant != nil && strings.Contains(strings.ToLower(tx.Merchant.Name), "costco") {
				costcoTransactions = append(costcoTransactions, tx)
			}
		}
		fmt.Printf("   Found %d Costco transactions in Monarch\n\n", len(costcoTransactions))

		// Process matches
		processDryRun(orders, costcoTransactions, store)
	}

	// Show statistics
	showStats(store)
}

func simulateDryRun(orders []providers.Order, store *storage.Storage) {
	fmt.Println("🔄 SIMULATED DRY RUN (no Monarch connection)")
	fmt.Println(strings.Repeat("-", 60))

	processedCount := 0
	for i, order := range orders {
		fmt.Printf("\n[%d/%d] Order %s\n", i+1, len(orders), order.GetID())
		fmt.Printf("   Date: %s\n", order.GetDate().Format("2006-01-02"))
		fmt.Printf("   Total: $%.2f\n", order.GetTotal())
		fmt.Printf("   Items: %d\n", len(order.GetItems()))

		// Check if already processed
		if store.IsProcessed(order.GetID()) {
			fmt.Printf("   ⏭️  Already processed\n")
			continue
		}

		// Simulate what would happen
		fmt.Printf("   📝 Would search for Monarch transaction around $%.2f\n", order.GetTotal())
		fmt.Printf("   ✂️  Would create %d splits for categories:\n", len(order.GetItems()))

		// Show sample of items
		maxItems := 3
		if len(order.GetItems()) < maxItems {
			maxItems = len(order.GetItems())
		}
		for j := 0; j < maxItems; j++ {
			item := order.GetItems()[j]
			fmt.Printf("      - %s ($%.2f)\n", item.GetName(), item.GetPrice())
		}
		if len(order.GetItems()) > maxItems {
			fmt.Printf("      ... and %d more items\n", len(order.GetItems())-maxItems)
		}

		// Record as dry-run
		record := &storage.ProcessingRecord{
			OrderID:     order.GetID(),
			OrderDate:   order.GetDate(),
			OrderAmount: order.GetTotal(),
			ItemCount:   len(order.GetItems()),
			ProcessedAt: time.Now(),
			Status:      "dry-run",
			Notes:       "Simulated dry run - no Monarch connection",
		}
		store.SaveRecord(record)
		processedCount++
	}

	fmt.Printf("\n✅ Simulated processing of %d orders\n", processedCount)
}

func processDryRun(orders []providers.Order, transactions []*monarch.Transaction, store *storage.Storage) {
	fmt.Println("🔄 DRY RUN PROCESSING")
	fmt.Println(strings.Repeat("-", 60))

	processedCount := 0
	matchedCount := 0
	noMatchCount := 0
	usedTransactionIDs := make(map[string]bool)

	for i, order := range orders {
		fmt.Printf("\n[%d/%d] Order %s\n", i+1, len(orders), order.GetID())
		fmt.Printf("   Date: %s\n", order.GetDate().Format("2006-01-02"))
		fmt.Printf("   Total: $%.2f\n", order.GetTotal())
		fmt.Printf("   Items: %d\n", len(order.GetItems()))

		// Check if already processed
		if store.IsProcessed(order.GetID()) {
			fmt.Printf("   ⏭️  Already processed\n")
			continue
		}

		// Find matching transaction using shared matcher
		matcherConfig := matcher.Config{
			AmountTolerance: 0.01,
			DateTolerance:   5, // Costco uses 5 days
		}
		transactionMatcher := matcher.NewMatcher(matcherConfig)

		matchResult, err := transactionMatcher.FindMatch(order, transactions, usedTransactionIDs)
		if err != nil {
			fmt.Printf("   ❌ Matching error: %v\n", err)
			continue
		}

		var match *monarch.Transaction
		var daysDiff float64
		if matchResult != nil {
			match = matchResult.Transaction
			daysDiff = matchResult.DateDiff
		}

		if match == nil {
			if order.GetTotal() < 0 {
				fmt.Printf("   ❌ No matching transaction found (return/refund: looking for +$%.2f in Monarch)\n", -order.GetTotal())
			} else {
				fmt.Printf("   ❌ No matching transaction found\n")
			}
			record := &storage.ProcessingRecord{
				OrderID:     order.GetID(),
				OrderDate:   order.GetDate(),
				OrderAmount: order.GetTotal(),
				ItemCount:   len(order.GetItems()),
				ProcessedAt: time.Now(),
				Status:      "no-match",
				Notes:       "No matching Monarch transaction found",
			}
			store.SaveRecord(record)
			noMatchCount++
			continue
		}

		fmt.Printf("   ✅ Matched transaction:\n")
		fmt.Printf("      ID: %s\n", match.ID)
		fmt.Printf("      Amount: $%.2f\n", math.Abs(match.Amount))
		fmt.Printf("      Date: %s\n", match.Date.Time.Format("2006-01-02"))
		fmt.Printf("      Date diff: %.0f days\n", daysDiff)

		// Mark transaction as used
		usedTransactionIDs[match.ID] = true

		if match.HasSplits {
			fmt.Printf("      ⚠️  Already has splits\n")
		} else {
			fmt.Printf("   ✂️  Would create %d splits:\n", len(order.GetItems()))

			// Show category breakdown
			categoryTotals := make(map[string]float64)
			for _, item := range order.GetItems() {
				// Simple category guessing based on item name
				category := guessCategory(item.GetName())
				categoryTotals[category] += item.GetPrice()
			}

			for category, total := range categoryTotals {
				fmt.Printf("      - %s: $%.2f\n", category, total)
			}
		}

		// Record the dry run
		record := &storage.ProcessingRecord{
			OrderID:         order.GetID(),
			TransactionID:   match.ID,
			OrderDate:       order.GetDate(),
			OrderAmount:     order.GetTotal(),
			ItemCount:       len(order.GetItems()),
			ProcessedAt:     time.Now(),
			Status:          "dry-run",
			MatchConfidence: matchResult.Confidence,
		}
		store.SaveRecord(record)
		processedCount++
		matchedCount++
	}

	fmt.Printf("\n📊 DRY RUN SUMMARY:\n")
	fmt.Printf("   Total orders: %d\n", len(orders))
	fmt.Printf("   Processed: %d\n", processedCount)
	fmt.Printf("   Matched: %d\n", matchedCount)
	fmt.Printf("   No match: %d\n", noMatchCount)
}

// Transaction matching is now handled by internal/matcher package

func guessCategory(itemName string) string {
	name := strings.ToLower(itemName)

	// Food categories
	if strings.Contains(name, "beef") || strings.Contains(name, "chicken") ||
		strings.Contains(name, "pork") || strings.Contains(name, "salmon") {
		return "Groceries - Meat"
	}
	if strings.Contains(name, "milk") || strings.Contains(name, "cheese") ||
		strings.Contains(name, "yogurt") {
		return "Groceries - Dairy"
	}
	if strings.Contains(name, "bread") || strings.Contains(name, "tortilla") {
		return "Groceries - Bakery"
	}
	if strings.Contains(name, "strawberry") || strings.Contains(name, "apple") ||
		strings.Contains(name, "banana") {
		return "Groceries - Produce"
	}

	// Household
	if strings.Contains(name, "paper") || strings.Contains(name, "towel") ||
		strings.Contains(name, "tissue") || strings.Contains(name, "wipe") {
		return "Household Supplies"
	}
	if strings.Contains(name, "battery") || strings.Contains(name, "duracell") {
		return "Electronics"
	}

	// Clothing
	if strings.Contains(name, "sock") || strings.Contains(name, "shirt") ||
		strings.Contains(name, "adidas") {
		return "Clothing"
	}

	return "General Merchandise"
}

func showStats(store *storage.Storage) {
	stats, err := store.GetStats()
	if err != nil || stats == nil {
		return
	}

	fmt.Println("\n📈 DATABASE STATISTICS:")
	fmt.Printf("   Total processed: %d\n", stats.TotalProcessed)
	fmt.Printf("   Total amount: $%.2f\n", stats.TotalAmount)
	if stats.TotalProcessed > 0 {
		fmt.Printf("   Average order: $%.2f\n", stats.TotalAmount/float64(stats.TotalProcessed))
	}
}
