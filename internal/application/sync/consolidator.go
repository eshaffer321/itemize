package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// Consolidator handles transaction consolidation for multi-delivery orders
type Consolidator struct {
	client  *monarch.Client
	logger  *slog.Logger
	storage storage.Repository
	runID   int64
}

// NewConsolidator creates a new consolidator
func NewConsolidator(client *monarch.Client, logger *slog.Logger, store storage.Repository, runID int64) *Consolidator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Consolidator{
		client:  client,
		logger:  logger.With(slog.String("component", "consolidator")),
		storage: store,
		runID:   runID,
	}
}

// SetRunID updates the run ID for API logging (called after orchestrator creates sync run)
func (c *Consolidator) SetRunID(runID int64) {
	c.runID = runID
}

// logAPICall logs an API call to the database
func (c *Consolidator) logAPICall(orderID, method string, request, response interface{}, err error, durationMs int64) {
	if c.storage == nil || c.runID == 0 {
		return // No storage or no run ID, skip logging
	}

	requestJSON, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		c.logger.Warn("Failed to marshal request for API log", "method", method, "error", marshalErr)
		requestJSON = []byte(fmt.Sprintf(`{"error": "failed to marshal: %v"}`, marshalErr))
	}

	responseJSON, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		c.logger.Warn("Failed to marshal response for API log", "method", method, "error", marshalErr)
		responseJSON = []byte(fmt.Sprintf(`{"error": "failed to marshal: %v"}`, marshalErr))
	}

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	apiCall := &storage.APICall{
		RunID:        c.runID,
		OrderID:      orderID,
		Method:       method,
		RequestJSON:  string(requestJSON),
		ResponseJSON: string(responseJSON),
		Error:        errStr,
		DurationMs:   durationMs,
	}

	if logErr := c.storage.LogAPICall(apiCall); logErr != nil {
		c.logger.Warn("Failed to log API call", "method", method, "error", logErr)
	}
}

// ConsolidationResult contains the result of consolidating transactions
type ConsolidationResult struct {
	ConsolidatedTransaction *monarch.Transaction
	FailedDeletions         []string // Transaction IDs that failed to delete
}

// ConsolidateTransactions merges multiple transactions into one
// - Updates first transaction with total amount
// - Adds note explaining consolidation
// - Deletes extra transactions
// Returns the consolidated (updated) transaction
func (c *Consolidator) ConsolidateTransactions(
	ctx context.Context,
	transactions []*monarch.Transaction,
	order providers.Order,
	dryRun bool,
) (*ConsolidationResult, error) {
	// Validate inputs
	if len(transactions) == 0 {
		return nil, fmt.Errorf("no transactions to consolidate")
	}

	if len(transactions) == 1 {
		c.logger.Warn("Single transaction provided to consolidator (no consolidation needed)")
		return &ConsolidationResult{
			ConsolidatedTransaction: transactions[0],
			FailedDeletions:         nil,
		}, nil
	}

	c.logger.Info("Consolidating transactions",
		"transaction_count", len(transactions),
		"order_id", order.GetID(),
		"dry_run", dryRun,
	)

	// Use first transaction as primary
	primary := transactions[0]
	extras := transactions[1:]

	// Update primary transaction
	updated, err := c.updatePrimaryTransaction(ctx, primary, order, transactions, dryRun)
	if err != nil {
		return nil, fmt.Errorf("failed to update primary transaction: %w", err)
	}

	// Delete extra transactions
	failedDeletions, err := c.deleteExtraTransactions(ctx, extras, order, dryRun)
	if err != nil {
		c.logger.Warn("Some transactions failed to delete",
			"failed_count", len(failedDeletions),
			"error", err,
		)
		// Don't return error - partial consolidation is acceptable
		// Caller will handle failed deletions
	}

	result := &ConsolidationResult{
		ConsolidatedTransaction: updated,
		FailedDeletions:         failedDeletions,
	}

	c.logger.Info("Consolidation complete",
		"consolidated_id", updated.ID,
		"deleted_count", len(extras)-len(failedDeletions),
		"failed_deletions", len(failedDeletions),
	)

	return result, nil
}

// updatePrimaryTransaction updates the first transaction with consolidated amount
func (c *Consolidator) updatePrimaryTransaction(
	ctx context.Context,
	primary *monarch.Transaction,
	order providers.Order,
	allTransactions []*monarch.Transaction,
	dryRun bool,
) (*monarch.Transaction, error) {
	// Build consolidation note
	note := c.buildConsolidationNote(allTransactions)

	// Calculate new amount (match sign of original transaction)
	newAmount := order.GetTotal()
	if primary.Amount > 0 {
		newAmount = math.Abs(newAmount)
	} else {
		newAmount = -math.Abs(newAmount)
	}

	c.logger.Debug("Updating primary transaction",
		"transaction_id", primary.ID,
		"old_amount", primary.Amount,
		"new_amount", newAmount,
	)

	if dryRun {
		c.logger.Info("[DRY RUN] Would update transaction",
			"transaction_id", primary.ID,
			"new_amount", newAmount,
			"note", note,
		)
		// Return copy with updated fields for dry-run
		updated := *primary
		updated.Amount = newAmount
		updated.Notes = note
		return &updated, nil
	}

	// Update via Monarch API
	params := &monarch.UpdateTransactionParams{
		Amount: &newAmount,
		Notes:  &note,
	}

	start := time.Now()
	updated, err := c.client.Transactions.Update(ctx, primary.ID, params)
	duration := time.Since(start).Milliseconds()

	// Log API call
	c.logAPICall(order.GetID(), "Transactions.Update", params, updated, err, duration)

	if err != nil {
		return nil, fmt.Errorf("failed to update transaction %s: %w", primary.ID, err)
	}

	c.logger.Info("Updated primary transaction",
		"transaction_id", updated.ID,
		"new_amount", updated.Amount,
	)

	return updated, nil
}

// deleteExtraTransactions removes the extra transactions after consolidation
// Returns list of transaction IDs that failed to delete
func (c *Consolidator) deleteExtraTransactions(
	ctx context.Context,
	extras []*monarch.Transaction,
	order providers.Order,
	dryRun bool,
) ([]string, error) {
	var failedDeletions []string
	var lastError error

	for _, txn := range extras {
		// Check if transaction has splits (safety check)
		if txn.HasSplits {
			c.logger.Error("Cannot delete transaction with splits",
				"transaction_id", txn.ID,
			)
			failedDeletions = append(failedDeletions, txn.ID)
			lastError = fmt.Errorf("transaction %s has splits", txn.ID)
			continue
		}

		c.logger.Debug("Deleting extra transaction", "transaction_id", txn.ID)

		if dryRun {
			c.logger.Info("[DRY RUN] Would delete transaction",
				"transaction_id", txn.ID,
				"amount", txn.Amount,
			)
			continue
		}

		// Delete via Monarch API
		start := time.Now()
		err := c.client.Transactions.Delete(ctx, txn.ID)
		duration := time.Since(start).Milliseconds()

		// Log API call
		c.logAPICall(order.GetID(), "Transactions.Delete", map[string]string{"transaction_id": txn.ID}, nil, err, duration)

		if err != nil {
			c.logger.Error("Failed to delete transaction",
				"transaction_id", txn.ID,
				"error", err,
			)
			failedDeletions = append(failedDeletions, txn.ID)
			lastError = err
			continue
		}

		c.logger.Info("Deleted extra transaction", "transaction_id", txn.ID)
	}

	if len(failedDeletions) > 0 {
		return failedDeletions, fmt.Errorf(
			"failed to delete %d of %d transactions: %w",
			len(failedDeletions), len(extras), lastError,
		)
	}

	return nil, nil
}

// buildConsolidationNote creates a note explaining the consolidation
func (c *Consolidator) buildConsolidationNote(transactions []*monarch.Transaction) string {
	if len(transactions) == 0 {
		return ""
	}

	charges := make([]string, len(transactions))
	for i, txn := range transactions {
		charges[i] = fmt.Sprintf("$%.2f", math.Abs(txn.Amount))
	}

	return fmt.Sprintf("Multi-delivery order (%d charges: %s)",
		len(transactions),
		strings.Join(charges, ", "))
}
