package sync

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/matcher"
)

// fetchOrders fetches orders from the provider based on the given options
func (o *Orchestrator) fetchOrders(ctx context.Context, opts Options) ([]providers.Order, error) {
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

	return orders, nil
}

// processOrder processes a single order, matching it to a transaction and creating splits
// Returns (processed, skipped, error)
func (o *Orchestrator) processOrder(
	ctx context.Context,
	order providers.Order,
	providerTransactions []*monarch.Transaction,
	usedTransactionIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	opts Options,
) (bool, bool, error) {
	if opts.Verbose {
		o.logger.Info("Processing order",
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
		return false, true, nil
	}

	// Match transaction
	matcherConfig := matcher.Config{
		AmountTolerance: 0.01,
		DateTolerance:   5,
	}
	transactionMatcher := matcher.NewMatcher(matcherConfig)

	matchResult, err := transactionMatcher.FindMatch(order, providerTransactions, usedTransactionIDs)
	if err != nil {
		o.logger.Error("Matching error", "order_id", order.GetID(), "error", err)
		o.recordError(order, err.Error())
		return false, false, fmt.Errorf("matching error: %w", err)
	}

	var match *monarch.Transaction
	var daysDiff float64
	if matchResult != nil {
		match = matchResult.Transaction
		daysDiff = matchResult.DateDiff
	}

	if match == nil {
		o.logger.Warn("No matching transaction found", "order_id", order.GetID())
		o.recordError(order, "No matching transaction")
		return false, false, fmt.Errorf("no matching transaction")
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
		return false, true, nil
	}

	// Categorize and split transaction
	if opts.Verbose {
		o.logger.Info("Processing transaction",
			"order_id", order.GetID(),
			"transaction_amount", match.Amount,
			"order_subtotal", order.GetSubtotal(),
			"order_tax", order.GetTax(),
		)
	}

	// Use splitter to determine if this is single or multi-category
	splits, err := o.splitter.CreateSplits(ctx, order, match, catCategories, monarchCategories)
	if err != nil {
		o.logger.Error("Failed to categorize order", "order_id", order.GetID(), "error", err)
		o.recordError(order, err.Error())
		return false, false, fmt.Errorf("failed to categorize order: %w", err)
	}

	// Handle single category (no splits needed)
	if splits == nil {
		if opts.Verbose {
			o.logger.Info("Single category detected - updating transaction category", "order_id", order.GetID())
		}

		categoryID, notes, err := o.splitter.GetSingleCategoryInfo(ctx, order, catCategories)
		if err != nil {
			o.logger.Error("Failed to get category info", "order_id", order.GetID(), "error", err)
			o.recordError(order, err.Error())
			return false, false, fmt.Errorf("failed to get category info: %w", err)
		}

		if !opts.DryRun {
			if opts.Verbose {
				o.logger.Info("Updating transaction category", "transaction_id", match.ID, "category_id", categoryID)
			}
			params := &monarch.UpdateTransactionParams{
				CategoryID: &categoryID,
				Notes:      &notes,
			}
			_, err = o.clients.Monarch.Transactions.Update(ctx, match.ID, params)
			if err != nil {
				o.logger.Error("Failed to update transaction", "order_id", order.GetID(), "error", err)
				o.recordError(order, err.Error())
				return false, false, fmt.Errorf("failed to update transaction: %w", err)
			}
			if opts.Verbose {
				o.logger.Info("Successfully updated transaction category", "order_id", order.GetID())
			}
		} else {
			if opts.Verbose {
				o.logger.Info("[DRY RUN] Would update transaction category", "order_id", order.GetID(), "category_id", categoryID)
			}
		}

		// Record success (no splits)
		o.recordSuccess(order, match, nil, daysDiff, opts.DryRun)
		return true, false, nil
	}

	// Handle multi-category (create splits)
	if opts.Verbose {
		o.logger.Info("Multiple categories detected - creating splits", "order_id", order.GetID(), "split_count", len(splits))

		// Log detailed split information
		for i, split := range splits {
			// Extract category name from notes (format: "CategoryName: items...")
			categoryName := ""
			if split.Category != nil {
				categoryName = split.Category.Name
			} else if colonIdx := strings.Index(split.Notes, ":"); colonIdx > 0 {
				categoryName = split.Notes[:colonIdx]
			}

			o.logger.Info("Split details",
				"split_index", i+1,
				"category_id", split.CategoryID,
				"category_name", categoryName,
				"amount", split.Amount,
				"notes", split.Notes,
			)
		}
	}

	if !opts.DryRun {
		if opts.Verbose {
			o.logger.Info("Applying splits to Monarch", "transaction_id", match.ID)
		}
		err = o.clients.Monarch.Transactions.UpdateSplits(ctx, match.ID, splits)
		if err != nil {
			o.logger.Error("Failed to apply splits", "order_id", order.GetID(), "error", err)
			o.recordError(order, err.Error())
			return false, false, fmt.Errorf("failed to apply splits: %w", err)
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
	o.recordSuccess(order, match, splits, daysDiff, opts.DryRun)
	return true, false, nil
}

// recordError records a processing error to storage
func (o *Orchestrator) recordError(order providers.Order, errorMsg string) {
	if o.storage != nil {
		record := &storage.ProcessingRecord{
			OrderID:      order.GetID(),
			Provider:     order.GetProviderName(),
			OrderDate:    order.GetDate(),
			OrderTotal:   order.GetTotal(),
			ItemCount:    len(order.GetItems()),
			ProcessedAt:  time.Now(),
			Status:       "failed",
			ErrorMessage: errorMsg,
		}
		o.storage.SaveRecord(record)
	}
}

// recordSuccess records a successful processing to storage
func (o *Orchestrator) recordSuccess(order providers.Order, transaction *monarch.Transaction, splits []*monarch.TransactionSplit, confidence float64, dryRun bool) {
	if o.storage != nil {
		record := &storage.ProcessingRecord{
			OrderID:           order.GetID(),
			Provider:          order.GetProviderName(),
			TransactionID:     transaction.ID,
			OrderDate:         order.GetDate(),
			OrderTotal:        order.GetTotal(),
			TransactionAmount: transaction.Amount,
			ItemCount:         len(order.GetItems()),
			SplitCount:        len(splits),
			ProcessedAt:       time.Now(),
			Status:            "success",
			MatchConfidence:   confidence,
			DryRun:            dryRun,
		}
		if dryRun {
			record.Status = "dry-run"
		}
		o.storage.SaveRecord(record)
	}
}

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
	orders, err := o.fetchOrders(ctx, opts)
	if err != nil {
		return nil, err
	}

	// 2. Fetch Monarch transactions
	// If no clients configured, return early (testing mode)
	if o.clients == nil {
		return result, nil
	}

	if opts.Verbose {
		o.logger.Info("Fetching Monarch transactions")
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -opts.LookbackDays)

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
			)
		}

		processed, skipped, err := o.processOrder(ctx, order, providerTransactions, usedTransactionIDs, catCategories, categories, opts)
		if err != nil {
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Errorf("order %s: %w", order.GetID(), err))
			continue
		}

		if processed {
			result.ProcessedCount++
		}
		if skipped {
			result.SkippedCount++
		}
	}

	// Complete sync run
	if o.storage != nil && runID > 0 {
		o.storage.CompleteSyncRun(runID, result.ProcessedCount, result.SkippedCount, result.ErrorCount)
	}

	return result, nil
}
