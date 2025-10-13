package sync

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/matcher"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/storage"
)

// Run executes the sync process for the configured provider
func (o *Orchestrator) Run(ctx context.Context, opts Options) (*Result, error) {
	result := &Result{
		Errors: make([]error, 0),
	}

	// Log configuration
	if opts.Verbose {
		o.logger.Info("Starting sync",
			"provider", o.provider.DisplayName(),
			"lookback_days", opts.LookbackDays,
			"max_orders", opts.MaxOrders,
			"dry_run", opts.DryRun,
			"force", opts.Force,
		)
	}

	// 1. Fetch orders from provider
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -opts.LookbackDays)

	if opts.Verbose {
		o.logger.Info("Fetching orders",
			"start_date", startDate.Format("2006-01-02"),
			"end_date", endDate.Format("2006-01-02"),
		)
	}

	orders, err := o.provider.FetchOrders(ctx, providers.FetchOptions{
		StartDate:      startDate,
		EndDate:        endDate,
		MaxOrders:      opts.MaxOrders,
		IncludeDetails: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}

	if opts.Verbose {
		o.logger.Info("Fetched orders", "count", len(orders))
	}

	// 2. Fetch Monarch transactions
	// If no clients configured, return early (testing mode)
	if o.clients == nil {
		return result, nil
	}

	if opts.Verbose {
		o.logger.Info("Fetching Monarch transactions")
	}

	txList, err := o.clients.Monarch.Transactions.Query().
		Between(startDate.AddDate(0, 0, -7), endDate). // Add buffer for date matching
		Limit(500).
		Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}

	// Filter for provider transactions (excluding splits)
	var providerTransactions []*monarch.Transaction
	providerName := strings.ToLower(o.provider.DisplayName())
	for _, tx := range txList.Transactions {
		// Skip split transactions - only process parent transactions
		if tx.IsSplitTransaction {
			continue
		}
		if tx.Merchant != nil && strings.Contains(strings.ToLower(tx.Merchant.Name), providerName) {
			providerTransactions = append(providerTransactions, tx)
		}
	}

	if opts.Verbose {
		o.logger.Info("Fetched transactions",
			"total", len(txList.Transactions),
			"provider_transactions", len(providerTransactions),
		)
	}

	// 3. Get Monarch categories
	if opts.Verbose {
		o.logger.Info("Loading Monarch categories")
	}

	categories, err := o.clients.Monarch.Transactions.Categories().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load categories: %w", err)
	}

	if opts.Verbose {
		o.logger.Info("Loaded categories", "count", len(categories))
	}

	// Convert to categorizer format
	catCategories := make([]categorizer.Category, len(categories))
	for i, cat := range categories {
		catCategories[i] = categorizer.Category{
			ID:   cat.ID,
			Name: cat.Name,
		}
	}

	// Start sync run
	var runID int64
	if o.storage != nil {
		runID, _ = o.storage.StartSyncRun(len(orders), opts.DryRun, opts.LookbackDays)
	}

	// 4. Process orders
	usedTransactionIDs := make(map[string]bool)

	for i, order := range orders {
		if opts.Verbose {
			o.logger.Info("Processing order",
				"index", i+1,
				"total", len(orders),
				"order_id", order.GetID(),
				"order_date", order.GetDate().Format("2006-01-02"),
				"order_total", order.GetTotal(),
				"item_count", len(order.GetItems()),
			)
		}

		// Check if already processed
		if !opts.Force && o.storage != nil && o.storage.IsProcessed(order.GetID()) {
			if opts.Verbose {
				o.logger.Info("Skipping already processed order", "order_id", order.GetID())
			}
			result.SkippedCount++
			continue
		}

		// Match transaction
		matcherConfig := matcher.Config{
			AmountTolerance: 0.01,
			DateTolerance:   5, // 5 days tolerance
		}
		transactionMatcher := matcher.NewMatcher(matcherConfig)

		matchResult, err := transactionMatcher.FindMatch(order, providerTransactions, usedTransactionIDs)
		if err != nil {
			o.logger.Error("Matching error", "order_id", order.GetID(), "error", err)
			if o.storage != nil {
				record := &storage.ProcessingRecord{
					OrderID:      order.GetID(),
					OrderDate:    order.GetDate(),
					OrderAmount:  order.GetTotal(),
					ItemCount:    len(order.GetItems()),
					ProcessedAt:  time.Now(),
					Status:       "error",
					ErrorMessage: err.Error(),
				}
				o.storage.SaveRecord(record)
			}
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Errorf("order %s: %w", order.GetID(), err))
			continue
		}

		var match *monarch.Transaction
		var daysDiff float64
		if matchResult != nil {
			match = matchResult.Transaction
			daysDiff = matchResult.DateDiff
		}

		if match == nil {
			o.logger.Warn("No matching transaction found", "order_id", order.GetID())
			if o.storage != nil {
				record := &storage.ProcessingRecord{
					OrderID:      order.GetID(),
					OrderDate:    order.GetDate(),
					OrderAmount:  order.GetTotal(),
					ItemCount:    len(order.GetItems()),
					ProcessedAt:  time.Now(),
					Status:       "failed",
					ErrorMessage: "No matching transaction",
				}
				o.storage.SaveRecord(record)
			}
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Errorf("order %s: no matching transaction", order.GetID()))
			continue
		}

		if opts.Verbose {
			o.logger.Info("Matched transaction",
				"order_id", order.GetID(),
				"transaction_id", match.ID,
				"amount", math.Abs(match.Amount),
				"date_diff_days", daysDiff,
			)
		}

		// Mark transaction as used
		usedTransactionIDs[match.ID] = true

		// Check if already has splits
		if match.HasSplits {
			if opts.Verbose {
				o.logger.Info("Transaction already has splits", "transaction_id", match.ID)
			}
			result.SkippedCount++
			continue
		}

		// Create splits
		if opts.Verbose {
			o.logger.Info("Creating splits",
				"order_id", order.GetID(),
				"transaction_amount", match.Amount,
				"order_subtotal", order.GetSubtotal(),
				"order_tax", order.GetTax(),
			)
		}

		splits, err := o.createSplits(ctx, order, match, catCategories, categories)
		if err != nil {
			o.logger.Error("Failed to create splits", "order_id", order.GetID(), "error", err)
			if o.storage != nil {
				record := &storage.ProcessingRecord{
					OrderID:      order.GetID(),
					OrderDate:    order.GetDate(),
					OrderAmount:  order.GetTotal(),
					ItemCount:    len(order.GetItems()),
					ProcessedAt:  time.Now(),
					Status:       "failed",
					ErrorMessage: err.Error(),
				}
				o.storage.SaveRecord(record)
			}
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Errorf("order %s: failed to split: %w", order.GetID(), err))
			continue
		}

		if opts.Verbose {
			o.logger.Info("Created splits", "order_id", order.GetID(), "split_count", len(splits))
		}

		// Apply splits if not dry run
		if !opts.DryRun {
			if opts.Verbose {
				o.logger.Info("Applying splits to Monarch", "transaction_id", match.ID)
			}
			err = o.clients.Monarch.Transactions.UpdateSplits(ctx, match.ID, splits)
			if err != nil {
				o.logger.Error("Failed to apply splits", "order_id", order.GetID(), "error", err)
				if o.storage != nil {
					record := &storage.ProcessingRecord{
						OrderID:      order.GetID(),
						OrderDate:    order.GetDate(),
						OrderAmount:  order.GetTotal(),
						ItemCount:    len(order.GetItems()),
						ProcessedAt:  time.Now(),
						Status:       "failed",
						ErrorMessage: err.Error(),
					}
					o.storage.SaveRecord(record)
				}
				result.ErrorCount++
				result.Errors = append(result.Errors, fmt.Errorf("order %s: failed to apply splits: %w", order.GetID(), err))
				continue
			}
			if opts.Verbose {
				o.logger.Info("Successfully applied splits", "order_id", order.GetID())
			}
		} else {
			if opts.Verbose {
				o.logger.Info("[DRY RUN] Would apply splits", "order_id", order.GetID(), "split_count", len(splits))
			}
		}

		// Record success
		if o.storage != nil {
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
			if opts.DryRun {
				record.Status = "dry-run"
			}
			o.storage.SaveRecord(record)
		}
		result.ProcessedCount++
	}

	// Complete sync run
	if o.storage != nil && runID > 0 {
		o.storage.CompleteSyncRun(runID, result.ProcessedCount, result.SkippedCount, result.ErrorCount)
	}

	return result, nil
}

// createSplits creates transaction splits for an order
func (o *Orchestrator) createSplits(
	ctx context.Context,
	order providers.Order,
	transaction *monarch.Transaction,
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
	result, err := o.clients.Categorizer.CategorizeItems(ctx, items, catCategories)
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
				if item.Quantity > 1 {
					itemDetails = append(itemDetails, fmt.Sprintf("%s (x%d)", item.Name, item.Quantity))
				} else {
					itemDetails = append(itemDetails, item.Name)
				}
			}

			// Add proportional tax
			categoryTax := categorySubtotal * taxRate
			mainAmount := categorySubtotal
			taxAmount := categoryTax

			// Monarch shows purchases as negative
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
			categorySubtotal := 0.0
			itemDetails := []string{}

			for _, item := range items {
				categorySubtotal += item.Price
				if item.Quantity > 1 {
					itemDetails = append(itemDetails, fmt.Sprintf("%s (x%d)", item.Name, item.Quantity))
				} else {
					itemDetails = append(itemDetails, item.Name)
				}
			}

			// Add proportional tax
			categoryTax := categorySubtotal * taxRate
			categoryTotal := categorySubtotal + categoryTax

			// Monarch shows purchases as negative for purchases, positive for returns
			if order.GetTotal() > 0 {
				categoryTotal = -categoryTotal
			}

			// Create split with detailed notes
			noteContent := strings.Join(itemDetails, ", ")
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
