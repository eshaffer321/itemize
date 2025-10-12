package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math"
	"strings"
	"time"

	costcogo "github.com/costco-go/pkg/costco"
	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/categorizer"
	appconfig "github.com/eshaffer321/monarchmoney-sync-backend/internal/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/matcher"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers/costco"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/storage"
)

type Config struct {
	DryRun       bool
	LookbackDays int
	MaxOrders    int
	Force        bool
}

func main() {
	// Parse flags
	cmdConfig := Config{}
	flag.BoolVar(&cmdConfig.DryRun, "dry-run", false, "Run without making changes")
	flag.IntVar(&cmdConfig.LookbackDays, "days", 14, "Number of days to look back")
	flag.IntVar(&cmdConfig.MaxOrders, "max", 0, "Maximum orders to process (0 = unlimited)")
	flag.BoolVar(&cmdConfig.Force, "force", false, "Force reprocess already processed orders")
	flag.Parse()

	fmt.Println("🛒 Costco → Monarch Money Sync")
	fmt.Println("=" + strings.Repeat("=", 50))

	if cmdConfig.DryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes will be made")
	} else {
		fmt.Println("⚠️  PRODUCTION MODE - Will update Monarch transactions!")
	}
	fmt.Println()

	// Setup
	logger := slog.Default()
	ctx := context.Background()

	// Load centralized configuration
	cfg := appconfig.LoadOrEnv()

	// Get and validate required API keys
	monarchToken, openaiKey, err := cfg.MustGetAPIKeys()
	if err != nil {
		log.Fatalf("❌ %v", err)
	}

	// Initialize storage using centralized config
	dbPath := cfg.Storage.DatabasePath
	fmt.Printf("💾 Using database: %s\n", dbPath)
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Initialize Monarch client
	monarchClient, err := monarch.NewClientWithToken(monarchToken)
	if err != nil {
		log.Fatalf("Failed to create Monarch client: %v", err)
	}

	// Initialize categorizer
	openaiClient := categorizer.NewRealOpenAIClient(openaiKey)
	cache := categorizer.NewMemoryCache()
	cat := categorizer.NewCategorizer(openaiClient, cache)

	// Load Costco config and create client
	savedConfig, err := costcogo.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load saved config: %v", err)
	}

	config_costco := costcogo.Config{
		Email:           savedConfig.Email,
		Password:        "",
		WarehouseNumber: savedConfig.WarehouseNumber,
	}
	costcoClient := costcogo.NewClient(config_costco)

	// Create provider
	provider := costco.NewProvider(costcoClient, logger)

	fmt.Println("📅 Configuration:")
	fmt.Printf("   Provider: %s\n", provider.DisplayName())
	fmt.Printf("   Lookback: %d days\n", cmdConfig.LookbackDays)
	fmt.Printf("   Max orders: %d\n", cmdConfig.MaxOrders)
	fmt.Printf("   Force reprocess: %v\n", cmdConfig.Force)
	fmt.Println()

	// Fetch Costco orders
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -cmdConfig.LookbackDays)

	fmt.Printf("🛍️ Fetching Costco orders (%s to %s)...\n",
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"))

	orders, err := provider.FetchOrders(ctx, providers.FetchOptions{
		StartDate:      startDate,
		EndDate:        endDate,
		MaxOrders:      cmdConfig.MaxOrders,
		IncludeDetails: true,
	})
	if err != nil {
		log.Fatalf("Failed to fetch orders: %v", err)
	}

	fmt.Printf("   Found %d orders\n\n", len(orders))

	// Fetch Monarch transactions
	fmt.Println("💳 Fetching Monarch transactions...")
	txList, err := monarchClient.Transactions.Query().
		Between(startDate.AddDate(0, 0, -7), endDate). // Add buffer for date matching
		Limit(500).
		Execute(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch transactions: %v", err)
	}

	// Filter for Costco transactions (excluding splits)
	var costcoTransactions []*monarch.Transaction
	for _, tx := range txList.Transactions {
		// IMPORTANT: Skip split transactions (they have IsSplitTransaction = true)
		// Only process parent transactions to avoid matching against splits
		if tx.IsSplitTransaction {
			continue
		}
		if tx.Merchant != nil && strings.Contains(strings.ToLower(tx.Merchant.Name), "costco") {
			costcoTransactions = append(costcoTransactions, tx)
		}
	}
	fmt.Printf("   Found %d Costco transactions\n\n", len(costcoTransactions))

	// Get Monarch categories
	fmt.Println("📊 Loading Monarch categories...")
	categories, err := monarchClient.Transactions.Categories().List(ctx)
	if err != nil {
		log.Fatalf("Failed to load categories: %v", err)
	}
	fmt.Printf("   Found %d categories\n\n", len(categories))

	// Convert to categorizer format
	catCategories := make([]categorizer.Category, len(categories))
	for i, cat := range categories {
		catCategories[i] = categorizer.Category{
			ID:   cat.ID,
			Name: cat.Name,
		}
	}

	// We'll create splits directly since the splitter is Walmart-specific

	// Start sync run
	runID, _ := store.StartSyncRun(len(orders), cmdConfig.DryRun, cmdConfig.LookbackDays)

	// Process orders
	fmt.Println("🔄 Processing orders...")
	fmt.Println(strings.Repeat("-", 60))

	processedCount := 0
	skippedCount := 0
	errorCount := 0
	usedTransactionIDs := make(map[string]bool)

	for i, order := range orders {
		fmt.Printf("\n[%d/%d] Order %s\n", i+1, len(orders), order.GetID())
		fmt.Printf("   Date: %s\n", order.GetDate().Format("2006-01-02"))
		fmt.Printf("   Total: $%.2f\n", order.GetTotal())
		fmt.Printf("   Items: %d\n", len(order.GetItems()))

		// Check if already processed
		if !cmdConfig.Force && store.IsProcessed(order.GetID()) {
			fmt.Printf("   ⏭️  Already processed\n")
			skippedCount++
			continue
		}

		// Find matching transaction using shared matcher
		matcherConfig := matcher.Config{
			AmountTolerance: 0.01,
			DateTolerance:   5, // Costco uses 5 days
		}
		transactionMatcher := matcher.NewMatcher(matcherConfig)

		matchResult, err := transactionMatcher.FindMatch(order, costcoTransactions, usedTransactionIDs)
		if err != nil {
			fmt.Printf("   ❌ Matching error: %v\n", err)
			record := &storage.ProcessingRecord{
				OrderID:      order.GetID(),
				OrderDate:    order.GetDate(),
				OrderAmount:  order.GetTotal(),
				ItemCount:    len(order.GetItems()),
				ProcessedAt:  time.Now(),
				Status:       "error",
				ErrorMessage: err.Error(),
			}
			store.SaveRecord(record)
			errorCount++
			continue
		}

		var match *monarch.Transaction
		var daysDiff float64
		if matchResult != nil {
			match = matchResult.Transaction
			daysDiff = matchResult.DateDiff
		}

		if match == nil {
			fmt.Printf("   ❌ No matching transaction found\n")
			record := &storage.ProcessingRecord{
				OrderID:      order.GetID(),
				OrderDate:    order.GetDate(),
				OrderAmount:  order.GetTotal(),
				ItemCount:    len(order.GetItems()),
				ProcessedAt:  time.Now(),
				Status:       "failed",
				ErrorMessage: "No matching transaction",
			}
			store.SaveRecord(record)
			errorCount++
			continue
		}

		fmt.Printf("   ✅ Matched: Transaction %s\n", match.ID)
		fmt.Printf("      Amount: $%.2f (date diff: %.0f days)\n",
			math.Abs(match.Amount), daysDiff)

		// Mark transaction as used
		usedTransactionIDs[match.ID] = true

		// Check if already has splits
		if match.HasSplits {
			fmt.Printf("   ⚠️  Already has splits\n")
			skippedCount++
			continue
		}

		// Create splits
		fmt.Printf("   ✂️  Creating splits...\n")
		fmt.Printf("      Transaction amount: $%.2f\n", match.Amount)
		fmt.Printf("      Order subtotal: $%.2f, tax: $%.2f\n", order.GetSubtotal(), order.GetTax())
		splits, err := createSplits(ctx, order, match, cat, catCategories, categories)
		if err != nil {
			fmt.Printf("   ❌ Failed to split: %v\n", err)
			record := &storage.ProcessingRecord{
				OrderID:      order.GetID(),
				OrderDate:    order.GetDate(),
				OrderAmount:  order.GetTotal(),
				ItemCount:    len(order.GetItems()),
				ProcessedAt:  time.Now(),
				Status:       "failed",
				ErrorMessage: err.Error(),
			}
			store.SaveRecord(record)
			errorCount++
			continue
		}

		fmt.Printf("      Created %d splits\n", len(splits))

		// Show category breakdown
		categoryTotals := make(map[string]float64)
		for _, s := range splits {
			for _, cat := range categories {
				if cat.ID == s.CategoryID {
					categoryTotals[cat.Name] += s.Amount
					break
				}
			}
		}

		for category, total := range categoryTotals {
			fmt.Printf("      - %s: $%.2f\n", category, total)
		}

		// Apply splits if not dry run
		if !cmdConfig.DryRun {
			fmt.Printf("   💾 Applying splits to Monarch...\n")
			err = monarchClient.Transactions.UpdateSplits(ctx, match.ID, splits)
			if err != nil {
				fmt.Printf("   ❌ Failed to apply splits: %v\n", err)
				record := &storage.ProcessingRecord{
					OrderID:      order.GetID(),
					OrderDate:    order.GetDate(),
					OrderAmount:  order.GetTotal(),
					ItemCount:    len(order.GetItems()),
					ProcessedAt:  time.Now(),
					Status:       "failed",
					ErrorMessage: err.Error(),
				}
				store.SaveRecord(record)
				errorCount++
				continue
			}
			fmt.Printf("   ✅ Successfully applied splits!\n")
		} else {
			fmt.Printf("   🔍 [DRY RUN] Would apply %d splits\n", len(splits))
		}

		// Record success
		record := &storage.ProcessingRecord{
			OrderID:         order.GetID(),
			TransactionID:   match.ID,
			OrderDate:       order.GetDate(),
			OrderAmount:     order.GetTotal(),
			ItemCount:       len(order.GetItems()),
			SplitCount:      len(splits),
			ProcessedAt:     time.Now(),
			Status:          "success",
			MatchConfidence: daysDiff,
		}
		if cmdConfig.DryRun {
			record.Status = "dry-run"
		}
		store.SaveRecord(record)
		processedCount++
	}

	// Complete sync run
	if runID > 0 {
		store.CompleteSyncRun(runID, processedCount, skippedCount, errorCount)
	}

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 SUMMARY")
	fmt.Printf("   Processed: %d\n", processedCount)
	fmt.Printf("   Skipped:   %d\n", skippedCount)
	fmt.Printf("   Errors:    %d\n", errorCount)

	// Get stats from database
	stats, _ := store.GetStats()
	if stats != nil && stats.TotalProcessed > 0 {
		fmt.Printf("\n📈 ALL TIME STATS\n")
		fmt.Printf("   Total Orders: %d\n", stats.TotalProcessed)
		fmt.Printf("   Total Splits: %d\n", stats.TotalSplits)
		fmt.Printf("   Total Amount: $%.2f\n", stats.TotalAmount)
		fmt.Printf("   Success Rate: %.1f%%\n", stats.SuccessRate)
	}

	if !cmdConfig.DryRun && processedCount > 0 {
		fmt.Println("\n✅ Successfully updated Monarch Money transactions!")
	}
}

// Transaction matching is now handled by internal/matcher package

// createSplits creates transaction splits for a Costco order
func createSplits(
	ctx context.Context,
	order providers.Order,
	transaction *monarch.Transaction,
	cat *categorizer.Categorizer,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
) ([]*monarch.TransactionSplit, error) {

	// Convert items for categorization
	items := make([]categorizer.Item, len(order.GetItems()))
	for i, orderItem := range order.GetItems() {
		items[i] = categorizer.Item{
			Name:     orderItem.GetName(),
			Price:    orderItem.GetPrice(),
			Quantity: int(orderItem.GetQuantity()),
		}
	}

	// Get categories from AI
	result, err := cat.CategorizeItems(ctx, items, catCategories)
	if err != nil {
		return nil, fmt.Errorf("failed to categorize items: %w", err)
	}

	// Group items by category
	categoryGroups := make(map[string][]categorizer.Item)
	categoryIDs := make(map[string]string)

	// Map categorizations back to items
	for i, cat := range result.Categorizations {
		if i < len(items) {
			categoryGroups[cat.CategoryName] = append(categoryGroups[cat.CategoryName], items[i])
			categoryIDs[cat.CategoryName] = cat.CategoryID
		}
	}

	// Calculate tax proportion
	subtotal := order.GetSubtotal()
	tax := order.GetTax()
	taxRate := 0.0
	if subtotal > 0 {
		taxRate = tax / subtotal
	}

	// Create splits
	var splits []*monarch.TransactionSplit

	// If there's only one category, we need to create at least 2 splits for Monarch
	if len(categoryGroups) == 1 {
		for categoryName, items := range categoryGroups {
			// Calculate total for this category
			categorySubtotal := 0.0
			itemDetails := []string{}

			for _, item := range items {
				categorySubtotal += item.Price
				// Include quantity if more than 1
				if item.Quantity > 1 {
					itemDetails = append(itemDetails, fmt.Sprintf("%s (x%d)", item.Name, item.Quantity))
				} else {
					itemDetails = append(itemDetails, item.Name)
				}
			}

			// Add proportional tax
			categoryTax := categorySubtotal * taxRate
			categoryTotal := categorySubtotal + categoryTax

			// Monarch shows purchases as negative
			if order.GetTotal() > 0 {
				categoryTotal = -categoryTotal
			}

			// Split into main and tax portions to have 2 splits
			mainAmount := categorySubtotal
			taxAmount := categoryTax

			if order.GetTotal() > 0 {
				mainAmount = -mainAmount
				taxAmount = -taxAmount
			}

			// Main split
			splits = append(splits, &monarch.TransactionSplit{
				Amount:     mainAmount,
				CategoryID: categoryIDs[categoryName],
				Notes:      fmt.Sprintf("%s: %s", categoryName, strings.Join(itemDetails, ", ")),
			})

			// Tax split (use same category)
			if math.Abs(taxAmount) > 0.01 {
				splits = append(splits, &monarch.TransactionSplit{
					Amount:     taxAmount,
					CategoryID: categoryIDs[categoryName],
					Notes:      fmt.Sprintf("%s (tax)", categoryName),
				})
			} else {
				// If no tax, create a minimal second split to meet Monarch's requirement
				// Use opposite sign to balance
				adjustmentAmount := 0.01
				if order.GetTotal() > 0 {
					adjustmentAmount = -0.01
				}
				splits = append(splits, &monarch.TransactionSplit{
					Amount:     adjustmentAmount,
					CategoryID: categoryIDs[categoryName],
					Notes:      "Rounding adjustment",
				})
				// Adjust main split to compensate
				splits[0].Amount = transaction.Amount - adjustmentAmount
			}
		}
	} else {
		// Multiple categories - create normal splits
		for categoryName, items := range categoryGroups {
			// Calculate total for this category
			categorySubtotal := 0.0
			itemDetails := []string{}

			for _, item := range items {
				categorySubtotal += item.Price
				// Include quantity if more than 1
				if item.Quantity > 1 {
					itemDetails = append(itemDetails, fmt.Sprintf("%s (x%d)", item.Name, item.Quantity))
				} else {
					itemDetails = append(itemDetails, item.Name)
				}
			}

			// Add proportional tax
			categoryTax := categorySubtotal * taxRate
			categoryTotal := categorySubtotal + categoryTax

			// Monarch shows purchases as negative (money leaving account)
			// and returns as positive (money coming back)
			if order.GetTotal() > 0 {
				// Regular purchase - make negative for Monarch
				categoryTotal = -categoryTotal
			}
			// For returns (order.GetTotal() < 0), keep positive

			// Create split with detailed notes
			noteContent := strings.Join(itemDetails, ", ")
			// Add item count if there are many items
			if len(items) > 3 {
				noteContent = fmt.Sprintf("(%d items) %s", len(items), noteContent)
			}
			split := &monarch.TransactionSplit{
				Amount:     categoryTotal,
				CategoryID: categoryIDs[categoryName],
				Notes:      fmt.Sprintf("%s: %s", categoryName, noteContent),
			}

			splits = append(splits, split)
		}
	}

	// Adjust for rounding to ensure splits sum to transaction amount
	totalSplits := 0.0
	for _, s := range splits {
		totalSplits += s.Amount
	}

	diff := transaction.Amount - totalSplits
	if math.Abs(diff) > 0.01 && len(splits) > 0 {
		// Add difference to largest split
		largestIdx := 0
		largestAmount := 0.0
		for i, s := range splits {
			if math.Abs(s.Amount) > largestAmount {
				largestAmount = math.Abs(s.Amount)
				largestIdx = i
			}
		}
		splits[largestIdx].Amount += diff
	}

	return splits, nil
}
