package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	walmartclient "github.com/eshaffer321/walmart-client"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/matcher"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/splitter"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/storage"
)

// Config holds application configuration
type Config struct {
	DryRun       bool
	LookbackDays int
	DBPath       string
	Verbose      bool
	Force        bool
	MaxOrders    int
}

func main() {
	cmdConfig := parseFlags()

	fmt.Println("🛒 Walmart → Monarch Money Sync (with Dashboard)")
	fmt.Println("================================================\n")

	if cmdConfig.DryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes will be made")
	}
	fmt.Printf("📅 Looking back %d days for orders\n", cmdConfig.LookbackDays)

	// Load centralized configuration
	appConfig := config.LoadOrEnv()

	// Get and validate required API keys
	monarchToken, openaiKey, err := appConfig.MustGetAPIKeys()
	if err != nil {
		log.Fatalf("❌ %v", err)
	}

	// Use centralized config for database path (allow flag override)
	dbPath := appConfig.Storage.DatabasePath
	if cmdConfig.DBPath != "" && cmdConfig.DBPath != "processing.db" {
		// User explicitly overrode the database path via flag
		dbPath = cmdConfig.DBPath
	}
	fmt.Printf("💾 Using database: %s\n", dbPath)
	fmt.Println()

	// Initialize storage
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Initialize clients
	ctx := context.Background()
	clients, err := initializeClients(monarchToken, openaiKey)
	if err != nil {
		log.Fatalf("❌ Failed to initialize clients: %v", err)
	}

	// Get recent Walmart orders
	fmt.Println("🛍️ Fetching Walmart orders...")
	orders, err := fetchWalmartOrders(clients.walmart, cmdConfig.LookbackDays)
	if err != nil {
		log.Fatalf("❌ Failed to fetch orders: %v", err)
	}
	fmt.Printf("Found %d orders\n", len(orders))

	// Debug output for orders
	if cmdConfig.Verbose {
		fmt.Printf("📦 Walmart Orders:\n")
		for _, o := range orders {
			fmt.Printf("   - Order %s: $%.2f on %s\n", o.OrderID, o.TotalAmount, o.OrderDate.Format("2006-01-02"))
		}
	}
	fmt.Println()

	// Get Monarch transactions
	fmt.Println("💳 Fetching Monarch transactions...")
	transactions, err := fetchMonarchTransactions(ctx, clients.monarch, cmdConfig.LookbackDays)
	if err != nil {
		log.Fatalf("❌ Failed to fetch transactions: %v", err)
	}
	fmt.Printf("Found %d Walmart transactions\n\n", len(transactions))

	// Debug output for transactions
	if cmdConfig.Verbose {
		fmt.Printf("📋 Monarch Walmart Transactions:\n")
		for _, tx := range transactions {
			fmt.Printf("   - $%.2f on %s", math.Abs(tx.Amount), tx.Date.Time.Format("2006-01-02"))
			if tx.HasSplits {
				fmt.Printf(" (has splits)")
			}
			if tx.IsSplitTransaction {
				fmt.Printf(" (is a split)")
			}
			fmt.Println()
		}
		fmt.Println()
	}

	// Get categories
	categories, err := clients.monarch.Transactions.Categories().List(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to load categories: %v", err)
	}

	// Initialize splitter
	strategy := splitter.DefaultStrategy()
	split := splitter.NewSplitter(clients.categorizer, categories, strategy)

	// Start sync run tracking
	runID, _ := store.StartSyncRun(len(orders), cmdConfig.DryRun, cmdConfig.LookbackDays)

	// Process orders
	fmt.Println("🔄 Processing orders...")
	processedCount := 0
	skippedCount := 0
	errorCount := 0
	usedTransactionIDs := make(map[string]bool)

	for i, order := range orders {
		if cmdConfig.MaxOrders > 0 && processedCount >= cmdConfig.MaxOrders {
			break
		}

		// Check if already processed
		if !cmdConfig.Force && store.IsProcessed(order.OrderID) {
			if cmdConfig.Verbose {
				fmt.Printf("⏭️  Order %s already processed\n", order.OrderID)
			}
			skippedCount++
			continue
		}

		fmt.Printf("\n[%d/%d] Order %s ($%.2f)\n", 
			i+1, len(orders), order.OrderID, order.TotalAmount)

		// Create record
		record := &storage.ProcessingRecord{
			OrderID:     order.OrderID,
			OrderDate:   order.OrderDate,
			OrderAmount: order.TotalAmount,
			ItemCount:   order.ItemCount,
			ProcessedAt: time.Now(),
			Status:      "processing",
		}

		// Find matching transaction using shared matcher
		matcherConfig := matcher.Config{
			AmountTolerance: 0.01,
			DateTolerance:   3, // Walmart uses 3 days
		}
		transactionMatcher := matcher.NewMatcher(matcherConfig)

		adapter := &OrderInfoAdapter{OrderInfo: order}
		matchResult, err := transactionMatcher.FindMatch(adapter, transactions, usedTransactionIDs)
		if err != nil {
			fmt.Printf("  ❌ Matching error: %v\n", err)
			record.Status = "error"
			record.ErrorMessage = err.Error()
			if !cmdConfig.DryRun {
				store.SaveRecord(record)
			}
			continue
		}

		var match *monarch.Transaction
		if matchResult != nil {
			match = matchResult.Transaction
		}

		if match == nil {
			// Try using order ledger to find actual charges
			if cmdConfig.Verbose {
				fmt.Printf("  🔍 No direct match found, checking order ledger...\n")
			}

			ledger, err := clients.walmart.GetOrderLedger(order.OrderID)
			if err != nil {
				fmt.Printf("  ❌ Failed to get order ledger: %v\n", err)
			} else if ledger != nil {
				// Try to match using actual charge amounts from ledger
				match = findBestMatchUsingLedger(order, ledger, transactions, usedTransactionIDs)
				if match != nil {
					fmt.Printf("  ✅ Matched using ledger charges\n")
				}
			}

			if match == nil {
				fmt.Printf("  ❌ No matching transaction found\n")
				if cmdConfig.Verbose {
					// Show why no match found
					fmt.Printf("     Expected: $%.2f ±$0.01\n", order.TotalAmount)
					fmt.Printf("     Order date: %s (±3 days tolerance)\n", order.OrderDate.Format("2006-01-02"))
					if ledger != nil {
						fmt.Printf("     Ledger shows charges: ")
						for _, pm := range ledger.PaymentMethods {
							if pm.PaymentType == "CREDITCARD" {
								for _, charge := range pm.FinalCharges {
									fmt.Printf("$%.2f ", charge)
								}
							}
						}
						fmt.Println()
					}
				}
				record.Status = "failed"
				record.ErrorMessage = "No matching transaction"
				errorCount++
				store.SaveRecord(record)
				continue
			}
		}

		fmt.Printf("  ✅ Matched: $%.2f on %s\n",
			math.Abs(match.Amount), match.Date.Time.Format("2006-01-02"))
		record.TransactionID = match.ID

		// Mark transaction as used
		usedTransactionIDs[match.ID] = true

		// Check if already split
		if match.HasSplits {
			fmt.Printf("  ⚠️  Already has splits\n")
			record.Status = "skipped"
			record.Notes = "Transaction already split"
			skippedCount++
			store.SaveRecord(record)
			continue
		}

		// Get full order for splitting (use correct isInStore flag based on fulfillment type)
		isInStore := order.FulfillmentType == "IN_STORE"
		fullOrder, err := clients.walmart.GetOrder(order.OrderID, isInStore)
		if err != nil {
			fmt.Printf("  ❌ Failed to get full order: %v\n", err)
			record.Status = "failed"
			record.ErrorMessage = err.Error()
			errorCount++
			store.SaveRecord(record)
			continue
		}

		// Perform split
		fmt.Printf("  ✂️  Creating splits...\n")
		fmt.Printf("     Transaction amount: $%.2f\n", math.Abs(match.Amount))
		fmt.Printf("     Order amount: $%.2f\n", order.TotalAmount)
		
		// Show item details if verbose
		if cmdConfig.Verbose && fullOrder != nil && len(fullOrder.Groups) > 0 {
			fmt.Printf("     Items in order:\n")
			for _, group := range fullOrder.Groups {
				for _, item := range group.Items {
					itemName := "Unknown"
					itemPrice := 0.0
					if item.ProductInfo != nil {
						itemName = item.ProductInfo.Name
					}
					if item.PriceInfo != nil && item.PriceInfo.LinePrice != nil {
						itemPrice = item.PriceInfo.LinePrice.Value
					}
					fmt.Printf("       - %s: $%.2f", itemName, itemPrice)
					if item.Quantity > 1 {
						fmt.Printf(" (x%.0f)", item.Quantity)
					}
					fmt.Println()
				}
			}
		}
		
		splitResult, err := split.SplitTransaction(ctx, fullOrder, match)
		if err != nil {
			fmt.Printf("  ❌ Failed to split: %v\n", err)
			record.Status = "failed"
			record.ErrorMessage = err.Error()
			errorCount++
			store.SaveRecord(record)
			continue
		}
		
		// Log split details for debugging
		fmt.Printf("     Created %d splits:\n", len(splitResult.Splits))
		totalSplitAmount := 0.0
		for _, s := range splitResult.Splits {
			// Find category name for this split
			catName := "Unknown"
			for _, cat := range categories {
				if cat.ID == s.CategoryID {
					catName = cat.Name
					break
				}
			}
			fmt.Printf("       - %s: $%.2f", catName, math.Abs(s.Amount))
			if s.Notes != "" && cmdConfig.Verbose {
				fmt.Printf(" (%s)", s.Notes)
			}
			fmt.Println()
			totalSplitAmount += s.Amount
		}
		fmt.Printf("     Total of splits (raw): %.2f\n", totalSplitAmount)
		fmt.Printf("     Transaction amount (raw): %.2f\n", match.Amount)
		fmt.Printf("     Match validation: %.2f == %.2f? %v\n", 
			match.Amount, totalSplitAmount,
			fmt.Sprintf("%.2f", match.Amount) == fmt.Sprintf("%.2f", totalSplitAmount))

		record.SplitCount = len(splitResult.Splits)
		
		// Extract categories
		var categoryNames []string
		categoryMap := make(map[string]bool)
		for _, s := range splitResult.Splits {
			for _, cat := range categories {
				if cat.ID == s.CategoryID && !categoryMap[cat.Name] {
					categoryNames = append(categoryNames, cat.Name)
					categoryMap[cat.Name] = true
					break
				}
			}
		}
		record.Categories = categoryNames

		fmt.Printf("  📊 %d splits: %s\n", 
			record.SplitCount, strings.Join(categoryNames, ", "))

		// Apply splits if not dry run
		if !cmdConfig.DryRun {
			err = clients.monarch.Transactions.UpdateSplits(ctx, match.ID, splitResult.Splits)
			if err != nil {
				fmt.Printf("  ❌ Failed to apply: %v\n", err)
				record.Status = "failed"
				record.ErrorMessage = err.Error()
				errorCount++
			} else {
				fmt.Printf("  ✅ Applied successfully!\n")
				
				// If order has a driver tip, tag the transaction
				if splitResult.HasTip {
					fmt.Printf("  🏷️  Adding delivery tag (tip: $%.2f)...\n", splitResult.TipAmount)
					// Note: Need to create or find the delivery tag first
					// For now, we'll just log that we would tag it
					// TODO: Implement tag creation/lookup and application
					fmt.Printf("     [Would tag with #walmart-delivery]\n")
				}
				
				record.Status = "success"
				processedCount++
			}
		} else {
			fmt.Printf("  🔍 [DRY RUN] Would apply %d splits\n", len(splitResult.Splits))
			if splitResult.HasTip {
				fmt.Printf("     Including delivery tip: $%.2f\n", splitResult.TipAmount)
			}
			record.Status = "dry-run"
			processedCount++
		}

		store.SaveRecord(record)
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

	fmt.Printf("\n🌐 View dashboard at http://localhost:8080\n")
	fmt.Printf("   Run: go run cmd/dashboard/main.go\n")
}

func parseFlags() Config {
	cmdConfig := Config{}
	flag.BoolVar(&cmdConfig.DryRun, "dry-run", true, "Run without making changes")
	flag.IntVar(&cmdConfig.LookbackDays, "days", 14, "Number of days to look back")
	flag.StringVar(&cmdConfig.DBPath, "db", "processing.db", "Database file path")
	flag.BoolVar(&cmdConfig.Verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&cmdConfig.Force, "force", false, "Force reprocess")
	flag.IntVar(&cmdConfig.MaxOrders, "max", 0, "Maximum orders to process")
	flag.Parse()
	return cmdConfig
}

// ClientSet holds all initialized clients
type ClientSet struct {
	walmart     *walmartclient.WalmartClient
	monarch     *monarch.Client
	categorizer *categorizer.Categorizer
}

// OrderInfo holds order information
type OrderInfo struct {
	OrderID         string
	OrderDate       time.Time
	TotalAmount     float64
	ItemCount       int
	FulfillmentType string
}

// OrderInfoAdapter adapts OrderInfo to providers.Order interface
type OrderInfoAdapter struct {
	*OrderInfo
}

func (o *OrderInfoAdapter) GetItems() []providers.OrderItem { return nil }
func (o *OrderInfoAdapter) GetProviderName() string         { return "walmart" }
func (o *OrderInfoAdapter) GetSubtotal() float64            { return o.TotalAmount * 0.9 }
func (o *OrderInfoAdapter) GetTax() float64                 { return o.TotalAmount * 0.1 }
func (o *OrderInfoAdapter) GetTip() float64                 { return 0 }
func (o *OrderInfoAdapter) GetFees() float64                { return 0 }
func (o *OrderInfoAdapter) GetRawData() interface{}         { return o }
func (o *OrderInfoAdapter) GetID() string                   { return o.OrderID }
func (o *OrderInfoAdapter) GetDate() time.Time              { return o.OrderDate }
func (o *OrderInfoAdapter) GetTotal() float64               { return o.TotalAmount }

func initializeClients(monarchToken, openaiKey string) (*ClientSet, error) {
	monarchClient, err := monarch.NewClientWithToken(monarchToken)
	if err != nil {
		return nil, err
	}

	homeDir, _ := os.UserHomeDir()
	config := walmartclient.ClientConfig{
		RateLimit:  2 * time.Second,
		AutoSave:   true,
		CookieDir:  filepath.Join(homeDir, ".walmart-api"),
		CookieFile: filepath.Join(homeDir, ".walmart-api", "cookies.json"),
	}
	
	walmartClient, err := walmartclient.NewWalmartClient(config)
	if err != nil {
		return nil, err
	}

	openaiClient := categorizer.NewRealOpenAIClient(openaiKey)
	cache := categorizer.NewMemoryCache()
	cat := categorizer.NewCategorizer(openaiClient, cache)

	return &ClientSet{
		walmart:     walmartClient,
		monarch:     monarchClient,
		categorizer: cat,
	}, nil
}

func fetchWalmartOrders(client *walmartclient.WalmartClient, lookbackDays int) ([]*OrderInfo, error) {
	orders, err := client.GetRecentOrders(50)
	if err != nil {
		return nil, err
	}

	cutoffDate := time.Now().AddDate(0, 0, -lookbackDays)
	var result []*OrderInfo

	for _, order := range orders {
		// Parse order date from the summary first to avoid unnecessary API calls
		// OrderSummary uses DeliveredDate field
		if order.DeliveredDate == nil {
			continue // Skip orders without dates
		}
		orderDate, _ := time.Parse(time.RFC3339, *order.DeliveredDate)
		if orderDate.Before(cutoffDate) {
			continue // Skip orders outside our date range
		}
		
		// Only fetch full order details for orders within our date range
		fullOrder, err := client.GetOrder(order.OrderID, order.FulfillmentType == "IN_STORE")
		if err != nil {
			continue
		}

		total := 0.0
		if fullOrder.PriceDetails != nil && fullOrder.PriceDetails.GrandTotal != nil {
			total = fullOrder.PriceDetails.GrandTotal.Value
		}

		result = append(result, &OrderInfo{
			OrderID:         order.OrderID,
			OrderDate:       orderDate,
			TotalAmount:     total,
			ItemCount:       order.ItemCount,
			FulfillmentType: order.FulfillmentType,
		})
	}

	return result, nil
}

func fetchMonarchTransactions(ctx context.Context, client *monarch.Client, lookbackDays int) ([]*monarch.Transaction, error) {
	startDate := time.Now().AddDate(0, 0, -(lookbackDays + 7))
	endDate := time.Now()

	transactionList, err := client.Transactions.Query().
		Between(startDate, endDate).
		Limit(500).
		Execute(ctx)
	if err != nil {
		return nil, err
	}

	var walmartTransactions []*monarch.Transaction
	for _, tx := range transactionList.Transactions {
		// Skip split transactions - only match against parent transactions
		if tx.IsSplitTransaction {
			continue
		}
		if tx.Merchant != nil && strings.Contains(strings.ToLower(tx.Merchant.Name), "walmart") {
			walmartTransactions = append(walmartTransactions, tx)
		}
	}

	return walmartTransactions, nil
}

func findBestMatchUsingLedger(order *OrderInfo, ledger *walmartclient.OrderLedger, transactions []*monarch.Transaction, usedTransactionIDs map[string]bool) *monarch.Transaction {
	// Look for transactions that match the actual charges from the ledger
	// This handles cases where Walmart splits orders into multiple charges

	const AmountTolerance = 0.01 // Only allow 1 cent difference for rounding
	const DateToleranceDays = 3

	// Get all credit card charges from the ledger
	var creditCardCharges []float64
	for _, pm := range ledger.PaymentMethods {
		if pm.PaymentType == "CREDITCARD" {
			creditCardCharges = append(creditCardCharges, pm.FinalCharges...)
		}
	}

	if len(creditCardCharges) == 0 {
		return nil
	}

	// Try to find transactions that match ANY of the charge amounts
	for _, chargeAmount := range creditCardCharges {
		for _, tx := range transactions {
			// Skip if transaction already used
			if usedTransactionIDs[tx.ID] {
				continue
			}

			txAmount := math.Abs(tx.Amount)

			// Check for exact match (within a penny for rounding)
			if math.Abs(txAmount-chargeAmount) > AmountTolerance {
				continue
			}

			// Check date is within tolerance
			dateDiff := math.Abs(order.OrderDate.Sub(tx.Date.Time).Hours() / 24)
			if dateDiff > DateToleranceDays {
				continue
			}

			// Found a match!
			return tx
		}
	}

	return nil
}

// Transaction matching is now handled by internal/matcher package