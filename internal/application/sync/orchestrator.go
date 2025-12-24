package sync

import (
	"context"
	"fmt"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/sync/handlers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
)

// handleResult processes the result from a provider handler and records success/error
// Returns (processed, skipped, error) matching processOrder signature
func (o *Orchestrator) handleResult(order providers.Order, result *handlers.ProcessResult, err error, opts Options) (bool, bool, error) {
	if err != nil {
		o.logger.Error("Handler error", "order_id", order.GetID(), "error", err)
		o.recordError(order, err.Error())
		return false, false, err
	}
	if result.Skipped {
		o.logger.Warn("Order skipped", "order_id", order.GetID(), "reason", result.SkipReason)
		// Don't treat "payment pending" as an error - it's expected for new orders
		if result.SkipReason == "payment pending" {
			return false, true, nil
		}
		// Don't treat "already has splits" as an error - just skip silently
		if result.SkipReason == "transaction already has splits" {
			return false, true, nil
		}
		o.recordError(order, result.SkipReason)
		return false, false, fmt.Errorf("skipped: %s", result.SkipReason)
	}
	if result.Processed {
		o.recordSuccess(order, nil, result.Splits, 0, opts.DryRun)
	}
	return result.Processed, result.Skipped, nil
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
		return o.handleResult(order, result, err, opts)
	}

	// Use Walmart handler for Walmart orders (handles multi-delivery and gift cards)
	if walmartOrder, ok := handlers.AsWalmartOrder(order); ok && o.walmartHandler != nil {
		o.logger.Debug("Using Walmart handler for order", "order_id", order.GetID())
		result, err := o.walmartHandler.ProcessOrder(ctx, walmartOrder, providerTransactions, usedTransactionIDs, catCategories, monarchCategories, opts.DryRun)
		return o.handleResult(order, result, err, opts)
	}

	// Use Simple handler for all other providers (Costco, etc.)
	if o.simpleHandler != nil {
		o.logger.Debug("Using Simple handler for order", "order_id", order.GetID())
		result, err := o.simpleHandler.ProcessOrder(ctx, order, providerTransactions, usedTransactionIDs, catCategories, monarchCategories, opts.DryRun)
		return o.handleResult(order, result, err, opts)
	}

	// No handler available (testing mode without clients)
	o.logger.Warn("No handler available for order", "order_id", order.GetID())
	return false, true, nil
}

// Run executes the sync process for the configured provider
func (o *Orchestrator) Run(ctx context.Context, opts Options) (*Result, error) {
	result := &Result{
		Errors: make([]error, 0),
	}

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

	// 2. Fetch Monarch transactions (if clients configured)
	if o.clients == nil {
		return result, nil // Testing mode
	}

	providerTransactions, err := o.fetchMonarchTransactions(ctx, opts)
	if err != nil {
		return nil, err
	}

	// 3. Get Monarch categories
	catCategories, monarchCategories, err := o.fetchCategories(ctx)
	if err != nil {
		return nil, err
	}

	// 4. Start sync run tracking
	if o.storage != nil {
		o.runID, err = o.storage.StartSyncRun(o.provider.DisplayName(), opts.LookbackDays, opts.DryRun)
		if err != nil {
			o.logger.Warn("Failed to start sync run tracking", "error", err)
		}
		if o.consolidator != nil {
			o.consolidator.SetRunID(o.runID)
		}
		// Set up ledger storage for Walmart handler
		if o.walmartHandler != nil {
			o.walmartHandler.SetLedgerStorage(&ledgerStorageAdapter{repo: o.storage}, o.runID)
		}
	}

	// 5. Process orders
	usedTransactionIDs := make(map[string]bool)

	for i, order := range orders {
		if opts.OrderID != "" && order.GetID() != opts.OrderID {
			o.logger.Debug("Skipping order (not matching -order-id filter)",
				"order_id", order.GetID(),
				"filter", opts.OrderID,
			)
			continue
		}

		o.logger.Debug("Processing order", "index", i+1, "total", len(orders))

		processed, skipped, err := o.processOrder(ctx, order, providerTransactions, usedTransactionIDs, catCategories, monarchCategories, opts)
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

	// 6. Complete sync run
	if o.storage != nil && o.runID > 0 {
		if err := o.storage.CompleteSyncRun(o.runID, len(orders), result.ProcessedCount, result.SkippedCount, result.ErrorCount); err != nil {
			o.logger.Error("Failed to complete sync run", "run_id", o.runID, "error", err)
		}
	}

	return result, nil
}
