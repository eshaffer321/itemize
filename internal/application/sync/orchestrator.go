package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/sync/handlers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// fetchOrders fetches orders from the provider based on the given options
func (o *Orchestrator) fetchOrders(ctx context.Context, opts Options) ([]providers.Order, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -opts.LookbackDays)

	o.logger.Debug("Fetching orders",
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"),
	)

	orders, err := o.provider.FetchOrders(ctx, providers.FetchOptions{
		StartDate:      startDate,
		EndDate:        endDate,
		MaxOrders:      opts.MaxOrders,
		IncludeDetails: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}

	o.logger.Debug("Fetched orders", "count", len(orders))

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
	o.logger.Debug("Processing order",
		"order_id", order.GetID(),
		"order_date", order.GetDate().Format("2006-01-02"),
		"order_total", order.GetTotal(),
		"item_count", len(order.GetItems()),
	)

	// Check if already processed
	if !opts.Force && o.storage != nil && o.storage.IsProcessed(order.GetID()) {
		o.logger.Debug("Skipping already processed order", "order_id", order.GetID())
		return false, true, nil
	}

	// Use Amazon handler for Amazon orders (uses pro-rata allocation)
	if amazonOrder, ok := handlers.AsAmazonOrder(order); ok && o.amazonHandler != nil {
		o.logger.Debug("Using Amazon handler for order", "order_id", order.GetID())
		result, err := o.amazonHandler.ProcessOrder(ctx, amazonOrder, providerTransactions, usedTransactionIDs, catCategories, monarchCategories, opts.DryRun)
		if err != nil {
			o.logger.Error("Amazon handler error", "order_id", order.GetID(), "error", err)
			o.recordError(order, err.Error())
			return false, false, err
		}
		if result.Skipped {
			o.logger.Warn("Amazon order skipped", "order_id", order.GetID(), "reason", result.SkipReason)
			o.recordError(order, result.SkipReason)
			return false, false, fmt.Errorf("skipped: %s", result.SkipReason)
		}
		// Record success
		if result.Processed {
			o.recordSuccess(order, nil, result.Splits, 0, opts.DryRun)
		}
		return result.Processed, result.Skipped, nil
	}

	// Check if this is a multi-delivery order or has ledger-based amounts (Walmart specific)
	type MultiDeliveryOrder interface {
		providers.Order
		IsMultiDelivery() (bool, error)
		GetFinalCharges() ([]float64, error)
	}

	if mdOrder, ok := order.(MultiDeliveryOrder); ok {
		charges, err := mdOrder.GetFinalCharges()
		if err != nil {
			o.logger.Error("Failed to get ledger charges", "order_id", order.GetID(), "error", err)
			// Fall through to regular processing using GetTotal()
		} else if len(charges) > 1 {
			// Multiple charges - multi-delivery order
			o.logger.Info("Detected multi-delivery order", "order_id", order.GetID())
			return o.processMultiDeliveryOrder(ctx, mdOrder, providerTransactions, usedTransactionIDs, catCategories, monarchCategories, opts)
		} else if len(charges) == 1 {
			chargeAmount := charges[0]
			orderTotal := order.GetTotal()

			// Check if ledger amount differs from order total (gift card, refund, etc.)
			const epsilon = 0.01 // Allow 1 cent difference for floating point
			if math.Abs(chargeAmount-orderTotal) > epsilon {
				o.logger.Info("Using ledger amount for matching (differs from order total)",
					"order_id", order.GetID(),
					"order_total", orderTotal,
					"ledger_charge", chargeAmount,
					"difference", math.Abs(chargeAmount-orderTotal))

				// Create a wrapper order that returns the ledger amount as its total
				wrappedOrder := &ledgerAmountOrder{
					Order:        order,
					ledgerAmount: chargeAmount,
				}

				// Use wrapped order for matching
				matchResult, err := o.matcher.FindMatch(wrappedOrder, providerTransactions, usedTransactionIDs)
				if err != nil {
					o.logger.Error("Matching error", "order_id", order.GetID(), "error", err)
					o.recordError(order, err.Error())
					return false, false, fmt.Errorf("matching error: %w", err)
				}

				if matchResult == nil {
					o.logger.Warn("No matching transaction found for ledger amount",
						"order_id", order.GetID(),
						"ledger_amount", chargeAmount)
					o.recordError(order, "no matching transaction")
					return false, false, fmt.Errorf("no matching transaction")
				}

				o.logger.Debug("Matched transaction using ledger amount",
					"order_id", order.GetID(),
					"transaction_id", matchResult.Transaction.ID,
					"ledger_amount", chargeAmount,
					"transaction_amount", math.Abs(matchResult.Transaction.Amount),
					"date_diff_days", matchResult.DateDiff)

				// Mark as used
				usedTransactionIDs[matchResult.Transaction.ID] = true

				// Apply categorization and splits using original order (not wrapped)
				return o.categorizeAndApplySplits(ctx, order, matchResult.Transaction, catCategories, monarchCategories, opts, nil)
			}
			// else: ledger amount equals order total, fall through to regular processing
		}
	}

	// Match transaction (regular single-delivery order)
	matchResult, err := o.matcher.FindMatch(order, providerTransactions, usedTransactionIDs)
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

	o.logger.Debug("Matched transaction",
		"order_id", order.GetID(),
		"transaction_id", match.ID,
		"amount", math.Abs(match.Amount),
		"date_diff_days", daysDiff,
	)

	// Mark transaction as used
	usedTransactionIDs[match.ID] = true

	// Apply categorization and splits to matched transaction
	return o.categorizeAndApplySplits(ctx, order, match, catCategories, monarchCategories, opts, nil)
}

// processMultiDeliveryOrder handles orders with multiple charge transactions
func (o *Orchestrator) processMultiDeliveryOrder(
	ctx context.Context,
	order interface {
		providers.Order
		IsMultiDelivery() (bool, error)
		GetFinalCharges() ([]float64, error)
	},
	providerTransactions []*monarch.Transaction,
	usedTransactionIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	opts Options,
) (bool, bool, error) {
	o.logger.Info("Processing multi-delivery order",
		"order_id", order.GetID(),
		"order_total", order.GetTotal(),
	)

	// Get charge amounts
	charges, err := order.GetFinalCharges()
	if err != nil {
		o.logger.Error("Failed to get final charges", "order_id", order.GetID(), "error", err)
		o.recordError(order, err.Error())
		return false, false, fmt.Errorf("failed to get final charges: %w", err)
	}

	o.logger.Info("Found multiple charges", "count", len(charges), "charges", charges)

	// Find matching transactions for each charge
	multiMatchResult, err := o.matcher.FindMultipleMatches(order, providerTransactions, usedTransactionIDs, charges)
	if err != nil {
		o.logger.Error("Multi-match error", "order_id", order.GetID(), "error", err)
		o.recordError(order, err.Error())
		return false, false, fmt.Errorf("multi-match error: %w", err)
	}

	if !multiMatchResult.AllFound {
		o.logger.Warn("Could not find all matching transactions",
			"order_id", order.GetID(),
			"expected", len(charges),
			"found", len(multiMatchResult.Matches),
		)
		o.recordError(order, "Incomplete multi-transaction match")
		return false, false, fmt.Errorf("incomplete multi-transaction match")
	}

	// Extract matched transactions
	matchedTransactions := make([]*monarch.Transaction, 0, len(multiMatchResult.Matches))
	originalTransactionIDs := make([]string, 0, len(multiMatchResult.Matches))
	for i, match := range multiMatchResult.Matches {
		if match == nil {
			o.logger.Error("Nil match in result", "index", i)
			continue
		}
		matchedTransactions = append(matchedTransactions, match.Transaction)
		originalTransactionIDs = append(originalTransactionIDs, match.Transaction.ID)
		o.logger.Debug("Matched charge transaction",
			"charge_index", i,
			"charge_amount", charges[i],
			"transaction_id", match.Transaction.ID,
			"transaction_amount", math.Abs(match.Transaction.Amount),
			"date_diff_days", match.DateDiff,
		)
	}

	// Mark all transactions as used
	for _, txnID := range originalTransactionIDs {
		usedTransactionIDs[txnID] = true
	}

	// Consolidate transactions into one
	consolidationResult, err := o.consolidator.ConsolidateTransactions(ctx, matchedTransactions, order, opts.DryRun)
	if err != nil {
		o.logger.Error("Failed to consolidate transactions", "order_id", order.GetID(), "error", err)
		o.recordError(order, err.Error())
		return false, false, fmt.Errorf("failed to consolidate transactions: %w", err)
	}

	consolidatedTxn := consolidationResult.ConsolidatedTransaction

	if len(consolidationResult.FailedDeletions) > 0 {
		o.logger.Warn("Some transactions failed to delete (manual cleanup required)",
			"order_id", order.GetID(),
			"failed_transaction_ids", consolidationResult.FailedDeletions,
		)
	}

	o.logger.Info("Consolidated transactions",
		"order_id", order.GetID(),
		"consolidated_id", consolidatedTxn.ID,
		"original_ids", originalTransactionIDs,
		"failed_deletions", len(consolidationResult.FailedDeletions),
	)

	// Apply categorization and splits to consolidated transaction
	return o.categorizeAndApplySplits(ctx, order, consolidatedTxn, catCategories, monarchCategories, opts, &storage.MultiDeliveryInfo{
		IsMultiDelivery:           true,
		ChargeCount:               len(charges),
		OriginalTransactionIDs:    originalTransactionIDs,
		ChargeAmounts:             charges,
		ConsolidatedTransactionID: consolidatedTxn.ID,
	})
}

// categorizeAndApplySplits applies categorization and splits to a transaction
// This is shared logic used by both regular and multi-delivery order processing
func (o *Orchestrator) categorizeAndApplySplits(
	ctx context.Context,
	order providers.Order,
	transaction *monarch.Transaction,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	opts Options,
	multiDeliveryInfo *storage.MultiDeliveryInfo,
) (bool, bool, error) {
	// Check if already has splits
	if transaction.HasSplits {
		o.logger.Debug("Transaction already has splits", "transaction_id", transaction.ID)
		return false, true, nil
	}

	// Categorize and split transaction
	o.logger.Debug("Processing transaction",
		"order_id", order.GetID(),
		"transaction_amount", transaction.Amount,
		"order_subtotal", order.GetSubtotal(),
		"order_tax", order.GetTax(),
	)

	// Use splitter to determine if this is single or multi-category
	splits, err := o.splitter.CreateSplits(ctx, order, transaction, catCategories, monarchCategories)
	if err != nil {
		o.logger.Error("Failed to categorize order", "order_id", order.GetID(), "error", err)
		o.recordError(order, err.Error())
		return false, false, fmt.Errorf("failed to categorize order: %w", err)
	}

	// Handle single category (no splits needed)
	if splits == nil {
		o.logger.Debug("Single category detected - updating transaction category", "order_id", order.GetID())

		categoryID, notes, err := o.splitter.GetSingleCategoryInfo(ctx, order, catCategories)
		if err != nil {
			o.logger.Error("Failed to get category info", "order_id", order.GetID(), "error", err)
			o.recordError(order, err.Error())
			return false, false, fmt.Errorf("failed to get category info: %w", err)
		}

		if !opts.DryRun {
			o.logger.Debug("Updating transaction category", "transaction_id", transaction.ID, "category_id", categoryID)
			params := &monarch.UpdateTransactionParams{
				CategoryID: &categoryID,
				Notes:      &notes,
			}

			start := time.Now()
			result, err := o.clients.Monarch.Transactions.Update(ctx, transaction.ID, params)
			duration := time.Since(start).Milliseconds()

			// Log API call
			o.logAPICall(order.GetID(), "Transactions.Update", params, result, err, duration)

			if err != nil {
				o.logger.Error("Failed to update transaction", "order_id", order.GetID(), "error", err)
				o.recordError(order, err.Error())
				return false, false, fmt.Errorf("failed to update transaction: %w", err)
			}
			o.logger.Debug("Successfully updated transaction category", "order_id", order.GetID())
		} else {
			o.logger.Debug("[DRY RUN] Would update transaction category", "order_id", order.GetID(), "category_id", categoryID)
		}

		// Record success (no splits)
		o.recordSuccessWithMultiDelivery(order, transaction, nil, 0.0, opts.DryRun, multiDeliveryInfo)
		return true, false, nil
	}

	// Handle multi-category (create splits)
	o.logger.Debug("Multiple categories detected - creating splits", "order_id", order.GetID(), "split_count", len(splits))

	// Log detailed split information
	for i, split := range splits {
		// Extract category name from notes (format: "CategoryName: items...")
		categoryName := ""
		if split.Category != nil {
			categoryName = split.Category.Name
		} else if colonIdx := strings.Index(split.Notes, ":"); colonIdx > 0 {
			categoryName = split.Notes[:colonIdx]
		}

		o.logger.Debug("Split details",
			"split_index", i+1,
			"category_id", split.CategoryID,
			"category_name", categoryName,
			"amount", split.Amount,
			"notes", split.Notes,
		)
	}

	if !opts.DryRun {
		o.logger.Debug("Applying splits to Monarch", "transaction_id", transaction.ID)

		start := time.Now()
		err = o.clients.Monarch.Transactions.UpdateSplits(ctx, transaction.ID, splits)
		duration := time.Since(start).Milliseconds()

		// Log API call
		o.logAPICall(order.GetID(), "Transactions.UpdateSplits", splits, nil, err, duration)

		if err != nil {
			o.logger.Error("Failed to apply splits", "order_id", order.GetID(), "error", err)
			o.recordError(order, err.Error())
			return false, false, fmt.Errorf("failed to apply splits: %w", err)
		}
		o.logger.Debug("Successfully applied splits", "order_id", order.GetID())
	} else {
		o.logger.Debug("[DRY RUN] Would apply splits", "order_id", order.GetID(), "split_count", len(splits))
	}

	// Record success
	o.recordSuccessWithMultiDelivery(order, transaction, splits, 0.0, opts.DryRun, multiDeliveryInfo)
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
	o.recordSuccessWithMultiDelivery(order, transaction, splits, confidence, dryRun, nil)
}

// recordSuccessWithMultiDelivery records a successful processing with optional multi-delivery info
func (o *Orchestrator) recordSuccessWithMultiDelivery(
	order providers.Order,
	transaction *monarch.Transaction,
	splits []*monarch.TransactionSplit,
	confidence float64,
	dryRun bool,
	multiDeliveryInfo *storage.MultiDeliveryInfo,
) {
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

		// Add multi-delivery metadata if provided
		if multiDeliveryInfo != nil {
			if err := record.SetMultiDeliveryInfo(multiDeliveryInfo); err != nil {
				o.logger.Error("Failed to set multi-delivery info", "error", err)
			}
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
	o.logger.Debug("Starting sync",
		"provider", o.provider.DisplayName(),
		"lookback_days", opts.LookbackDays,
		"max_orders", opts.MaxOrders,
		"dry_run", opts.DryRun,
		"force", opts.Force,
	)

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

	o.logger.Debug("Fetching Monarch transactions")

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

	o.logger.Debug("Fetched transactions",
		"total", len(txList.Transactions),
		"provider_transactions", len(providerTransactions),
	)

	// 3. Get Monarch categories
	o.logger.Debug("Loading Monarch categories")

	categories, err := o.clients.Monarch.Transactions.Categories().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load categories: %w", err)
	}

	o.logger.Debug("Loaded categories", "count", len(categories))

	// Convert to categorizer format
	catCategories := make([]categorizer.Category, len(categories))
	for i, cat := range categories {
		catCategories[i] = categorizer.Category{
			ID:   cat.ID,
			Name: cat.Name,
		}
	}

	// Start sync run
	if o.storage != nil {
		var err error
		o.runID, err = o.storage.StartSyncRun(o.provider.DisplayName(), opts.LookbackDays, opts.DryRun)
		if err != nil {
			o.logger.Warn("Failed to start sync run tracking", "error", err)
			// Continue anyway - tracking failure shouldn't block sync
		}
		// Set runID on consolidator for API logging
		if o.consolidator != nil {
			o.consolidator.SetRunID(o.runID)
		}
	}

	// 4. Process orders
	usedTransactionIDs := make(map[string]bool)

	for i, order := range orders {
		// If OrderID filter is set, skip all other orders
		if opts.OrderID != "" && order.GetID() != opts.OrderID {
			o.logger.Debug("Skipping order (not matching -order-id filter)",
				"order_id", order.GetID(),
				"filter", opts.OrderID,
			)
			continue
		}

		o.logger.Debug("Processing order",
			"index", i+1,
			"total", len(orders),
		)

		processed, skipped, err := o.processOrder(ctx, order, providerTransactions, usedTransactionIDs, catCategories, categories, opts)
		if err != nil {
			result.ErrorCount++
			result.Errors = append(result.Errors, fmt.Errorf("order %s (%s, $%.2f): %w",
				order.GetID(),
				order.GetDate().Format("2006-01-02"),
				order.GetTotal(),
				err))
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
	if o.storage != nil && o.runID > 0 {
		o.storage.CompleteSyncRun(o.runID, len(orders), result.ProcessedCount, result.SkippedCount, result.ErrorCount)
	}

	return result, nil
}

// logAPICall logs an API call to the database for audit trail
func (o *Orchestrator) logAPICall(orderID, method string, request, response interface{}, err error, durationMs int64) {
	if o.storage == nil || o.runID == 0 {
		return // No storage or no run ID, skip logging
	}

	requestJSON, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		o.logger.Warn("Failed to marshal request for API log", "method", method, "error", marshalErr)
		requestJSON = []byte(fmt.Sprintf(`{"error": "failed to marshal: %v"}`, marshalErr))
	}

	responseJSON, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		o.logger.Warn("Failed to marshal response for API log", "method", method, "error", marshalErr)
		responseJSON = []byte(fmt.Sprintf(`{"error": "failed to marshal: %v"}`, marshalErr))
	}

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	apiCall := &storage.APICall{
		RunID:        o.runID,
		OrderID:      orderID,
		Method:       method,
		RequestJSON:  string(requestJSON),
		ResponseJSON: string(responseJSON),
		Error:        errStr,
		DurationMs:   durationMs,
	}

	if err := o.storage.LogAPICall(apiCall); err != nil {
		o.logger.Warn("Failed to log API call", "method", method, "error", err)
	}
}

// ledgerAmountOrder wraps an order to override GetTotal() with a ledger-based amount
// This is used when the actual bank charge differs from the order total
// (e.g., due to gift cards, refunds, or other adjustments)
type ledgerAmountOrder struct {
	providers.Order
	ledgerAmount float64
}

// GetTotal returns the ledger amount instead of the original order total
func (l *ledgerAmountOrder) GetTotal() float64 {
	return l.ledgerAmount
}
