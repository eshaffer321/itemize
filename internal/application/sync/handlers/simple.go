package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/matcher"
)

// SimpleHandler processes orders for simple providers like Costco
// that don't have multi-delivery, gift cards, or other special handling.
// It matches orders to transactions by total amount and applies categorization.
type SimpleHandler struct {
	matcher  *matcher.Matcher
	splitter CategorySplitter
	monarch  MonarchClient
	logger   *slog.Logger
}

// NewSimpleHandler creates a new simple order handler
func NewSimpleHandler(
	matcher *matcher.Matcher,
	splitter CategorySplitter,
	monarch MonarchClient,
	logger *slog.Logger,
) *SimpleHandler {
	return &SimpleHandler{
		matcher:  matcher,
		splitter: splitter,
		monarch:  monarch,
		logger:   logger,
	}
}

// ProcessOrder processes a simple order by matching to a transaction and applying splits
func (h *SimpleHandler) ProcessOrder(
	ctx context.Context,
	order providers.Order,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	dryRun bool,
) (*ProcessResult, error) {
	result := &ProcessResult{}

	// Step 1: Match transaction using order total
	matchResult, err := h.matcher.FindMatch(order, monarchTxns, usedTxnIDs)
	if err != nil {
		return nil, fmt.Errorf("matching error: %w", err)
	}

	if matchResult == nil {
		result.Skipped = true
		result.SkipReason = "no matching transaction found"
		h.logWarn("No matching transaction found", "order_id", order.GetID())
		return result, nil
	}

	transaction := matchResult.Transaction
	usedTxnIDs[transaction.ID] = true

	h.logDebug("Matched transaction",
		"order_id", order.GetID(),
		"transaction_id", transaction.ID,
		"amount", math.Abs(transaction.Amount),
		"date_diff_days", matchResult.DateDiff,
	)

	// Step 2: Check if transaction already has splits
	if transaction.HasSplits {
		h.logDebug("Transaction already has splits", "transaction_id", transaction.ID)
		result.Skipped = true
		result.SkipReason = "transaction already has splits"
		return result, nil
	}

	// Step 3: Categorize and create splits
	splits, err := h.splitter.CreateSplits(ctx, order, transaction, catCategories, monarchCategories)
	if err != nil {
		return nil, fmt.Errorf("split creation error: %w", err)
	}

	result.Splits = splits

	// Step 4: Apply to Monarch
	if splits == nil {
		// Single category - update transaction category
		return h.applySingleCategory(ctx, order, transaction, catCategories, dryRun, result)
	}

	// Multiple categories - apply splits
	return h.applyMultiCategorySplits(ctx, order, transaction, splits, dryRun, result)
}

// applySingleCategory updates a transaction with a single category (no splits)
func (h *SimpleHandler) applySingleCategory(
	ctx context.Context,
	order providers.Order,
	transaction *monarch.Transaction,
	catCategories []categorizer.Category,
	dryRun bool,
	result *ProcessResult,
) (*ProcessResult, error) {
	h.logDebug("Single category detected - updating transaction category", "order_id", order.GetID())

	categoryID, notes, err := h.splitter.GetSingleCategoryInfo(ctx, order, catCategories)
	if err != nil {
		return nil, fmt.Errorf("get category info error: %w", err)
	}

	if !dryRun {
		params := &monarch.UpdateTransactionParams{
			CategoryID: &categoryID,
			Notes:      &notes,
		}
		if err := h.monarch.UpdateTransaction(ctx, transaction.ID, params); err != nil {
			return nil, fmt.Errorf("update transaction error: %w", err)
		}
		h.logDebug("Updated transaction category",
			"order_id", order.GetID(),
			"transaction_id", transaction.ID,
			"category_id", categoryID)
	} else {
		h.logDebug("[DRY RUN] Would update transaction category",
			"order_id", order.GetID(),
			"category_id", categoryID)
	}

	result.Processed = true
	return result, nil
}

// applyMultiCategorySplits creates splits for a transaction with multiple categories
func (h *SimpleHandler) applyMultiCategorySplits(
	ctx context.Context,
	order providers.Order,
	transaction *monarch.Transaction,
	splits []*monarch.TransactionSplit,
	dryRun bool,
	result *ProcessResult,
) (*ProcessResult, error) {
	h.logDebug("Multiple categories detected - creating splits",
		"order_id", order.GetID(),
		"split_count", len(splits))

	// Log detailed split information
	for i, split := range splits {
		categoryName := ""
		if split.Category != nil {
			categoryName = split.Category.Name
		} else if colonIdx := strings.Index(split.Notes, ":"); colonIdx > 0 {
			categoryName = split.Notes[:colonIdx]
		}

		h.logDebug("Split details",
			"split_index", i+1,
			"category_id", split.CategoryID,
			"category_name", categoryName,
			"amount", split.Amount,
			"notes", split.Notes,
		)
	}

	if !dryRun {
		if err := h.monarch.UpdateSplits(ctx, transaction.ID, splits); err != nil {
			return nil, fmt.Errorf("update splits error: %w", err)
		}
		h.logDebug("Applied splits",
			"order_id", order.GetID(),
			"transaction_id", transaction.ID,
			"split_count", len(splits))

		// Populate SplitDetails ONLY after successful Monarch API call
		// Check if the splitter supports returning detailed split information
		if detailedSplitter, ok := h.splitter.(CategorySplitterWithDetails); ok {
			result.SplitDetails = detailedSplitter.GetSplitDetails()
		}
	} else {
		h.logDebug("[DRY RUN] Would apply splits",
			"order_id", order.GetID(),
			"split_count", len(splits))
	}

	result.Processed = true
	return result, nil
}

// Nil-safe logging helpers
func (h *SimpleHandler) logDebug(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Debug(msg, args...)
	}
}

func (h *SimpleHandler) logWarn(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Warn(msg, args...)
	}
}
