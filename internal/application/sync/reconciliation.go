package sync

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/eshaffer321/itemize/internal/infrastructure/storage"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
)

const provisionalStatus = "provisional"

// transactionReconciliationClient is the subset of Monarch operations needed
// to follow a pending transaction to its posted replacement and restore the
// exact mutation payload cached in SQLite.
type transactionReconciliationClient interface {
	GetTransaction(ctx context.Context, id string) (*monarch.TransactionDetails, error)
	GetSplits(ctx context.Context, id string) ([]*monarch.TransactionSplit, error)
	UpdateTransaction(ctx context.Context, id string, params *monarch.UpdateTransactionParams) error
	UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error
}

// reconcileStoredOrder handles an order whose desired Monarch state is already
// cached locally. It returns handled=true whenever normal categorization should
// not run, which prevents repeated LLM calls for provisional orders.
func (o *Orchestrator) reconcileStoredOrder(
	ctx context.Context,
	order providers.Order,
	record *storage.ProcessingRecord,
	providerTransactions []*monarch.Transaction,
	usedTransactionIDs map[string]bool,
	dryRun bool,
) (handled, processed, skipped bool, err error) {
	if record == nil ||
		record.DryRun ||
		(record.Status != "success" && record.Status != provisionalStatus) {
		return false, false, false, nil
	}
	if o.reconciliationClient == nil || record.TransactionID == "" {
		return true, false, true, nil
	}

	details := transactionDetailsByID(providerTransactions, record.TransactionID)
	var getErr error
	if details == nil {
		// The list snapshot no longer contains the stored ID. Monarch's Get
		// endpoint follows pending rows to their posted replacements.
		details, getErr = o.reconciliationClient.GetTransaction(ctx, record.TransactionID)
	}
	if getErr != nil && !errors.Is(getErr, monarch.ErrNotFound) {
		// A read failure must not fall through to normal processing, which would
		// repeat the LLM call and risk a second mutation. Keep the cached state
		// unchanged and retry reconciliation on the next sync.
		o.logger.Warn("Could not resolve cached Monarch transaction; will retry",
			"order_id", order.GetID(),
			"transaction_id", record.TransactionID,
			"error", getErr)
		return true, false, true, nil
	}

	if getErr != nil || details == nil || details.Transaction == nil {
		replacement, ambiguous := findPostedReplacement(record, providerTransactions)
		if ambiguous {
			o.logger.Warn("Pending transaction replacement is ambiguous; leaving cached categorization provisional",
				"order_id", order.GetID(),
				"transaction_id", record.TransactionID,
				"amount", record.TransactionAmount)
			return true, false, true, nil
		}
		if replacement == nil {
			o.logger.Debug("Pending transaction has no posted replacement yet",
				"order_id", order.GetID(),
				"transaction_id", record.TransactionID)
			return true, false, true, nil
		}
		details = &monarch.TransactionDetails{Transaction: replacement}
	}

	transaction := details.Transaction
	if !matchesRecordedAmount(record, transaction) {
		return true, false, false, fmt.Errorf(
			"resolved Monarch transaction amount %.2f does not match cached amount %.2f for order %s",
			transaction.Amount,
			record.TransactionAmount,
			order.GetID(),
		)
	}
	usedTransactionIDs[transaction.ID] = true

	if transaction.Pending {
		// This also repairs legacy records that were incorrectly marked as a
		// final success while their matched Monarch row was still pending.
		if record.Status == "success" && !dryRun {
			provisional := *record
			provisional.RunID = o.runID
			provisional.Status = provisionalStatus
			provisional.ProcessedAt = time.Now()
			if saveErr := o.storage.SaveRecord(&provisional); saveErr != nil {
				return true, false, false, fmt.Errorf("save provisional record: %w", saveErr)
			}
		}
		o.logger.Debug("Categorized transaction is still pending",
			"order_id", order.GetID(),
			"transaction_id", transaction.ID)
		return true, false, true, nil
	}

	redirected := transaction.ID != record.TransactionID
	needsRepair := record.Status == provisionalStatus ||
		redirected ||
		sameIDSafeRepairNeeded(record, transaction)
	if !needsRepair {
		// A final same-ID transaction with a non-temporary category may have
		// been edited by the user. Leave it alone.
		return true, false, true, nil
	}

	stateMatches, compareErr := o.cachedStateMatches(ctx, record, transaction)
	if compareErr != nil {
		return true, false, false, compareErr
	}

	if !stateMatches && !dryRun {
		if applyErr := o.applyCachedState(ctx, record, transaction); applyErr != nil {
			return true, false, false, applyErr
		}
	}

	if !dryRun {
		finalized := *record
		finalized.RunID = o.runID
		finalized.TransactionID = transaction.ID
		finalized.TransactionAmount = transaction.Amount
		finalized.Status = "success"
		finalized.ErrorMessage = ""
		finalized.ProcessedAt = time.Now()
		if saveErr := o.storage.SaveRecord(&finalized); saveErr != nil {
			return true, false, false, fmt.Errorf("save reconciled record: %w", saveErr)
		}
		if saveErr := o.storage.SaveOrderTransaction(&storage.OrderTransaction{
			RunID:         o.runID,
			OrderID:       order.GetID(),
			TransactionID: transaction.ID,
			Role:          "posted_replacement",
			Amount:        transaction.Amount,
			CategoryID:    finalized.CategoryID,
			CategoryName:  finalized.CategoryName,
			Notes:         finalized.MonarchNotes,
		}); saveErr != nil {
			o.logger.Warn("Failed to save posted replacement transaction",
				"order_id", order.GetID(),
				"transaction_id", transaction.ID,
				"error", saveErr)
		}
	}

	o.logger.Info("Reconciled categorized order to posted Monarch transaction",
		"order_id", order.GetID(),
		"previous_transaction_id", record.TransactionID,
		"transaction_id", transaction.ID,
		"redirected", redirected,
		"state_reapplied", !stateMatches)
	return true, true, false, nil
}

func (o *Orchestrator) cachedStateMatches(
	ctx context.Context,
	record *storage.ProcessingRecord,
	transaction *monarch.Transaction,
) (bool, error) {
	if len(record.Splits) == 0 {
		if record.CategoryID == "" {
			return true, nil
		}
		if transaction.Category == nil || transaction.Category.ID != record.CategoryID {
			return false, nil
		}
		return record.MonarchNotes == "" || transaction.Notes == record.MonarchNotes, nil
	}

	current, err := o.reconciliationClient.GetSplits(ctx, transaction.ID)
	if err != nil {
		return false, fmt.Errorf("get splits for posted transaction %s: %w", transaction.ID, err)
	}
	return cachedSplitsMatch(record.Splits, current), nil
}

func (o *Orchestrator) applyCachedState(
	ctx context.Context,
	record *storage.ProcessingRecord,
	transaction *monarch.Transaction,
) error {
	if len(record.Splits) > 0 {
		splits := make([]*monarch.TransactionSplit, len(record.Splits))
		total := 0.0
		for i, cached := range record.Splits {
			total += cached.Amount
			splits[i] = &monarch.TransactionSplit{
				Amount:     cached.Amount,
				CategoryID: cached.CategoryID,
				Notes:      cached.Notes,
			}
		}
		if math.Abs(total-transaction.Amount) > 0.011 {
			return fmt.Errorf(
				"cached splits total %.2f does not match posted transaction %.2f",
				total,
				transaction.Amount,
			)
		}
		if err := o.reconciliationClient.UpdateSplits(ctx, transaction.ID, splits); err != nil {
			return fmt.Errorf("restore cached splits on transaction %s: %w", transaction.ID, err)
		}
		return nil
	}

	if record.CategoryID == "" {
		return nil
	}
	categoryID := record.CategoryID
	notes := record.MonarchNotes
	reviewed := false
	params := &monarch.UpdateTransactionParams{
		CategoryID:  &categoryID,
		Notes:       &notes,
		NeedsReview: &reviewed,
	}
	if err := o.reconciliationClient.UpdateTransaction(ctx, transaction.ID, params); err != nil {
		return fmt.Errorf("restore cached category on transaction %s: %w", transaction.ID, err)
	}
	return nil
}

func findPostedReplacement(
	record *storage.ProcessingRecord,
	transactions []*monarch.Transaction,
) (*monarch.Transaction, bool) {
	var candidates []*monarch.Transaction
	for _, transaction := range transactions {
		if transaction == nil || transaction.Pending || transaction.IsSplitTransaction {
			continue
		}
		if !matchesRecordedAmount(record, transaction) {
			continue
		}
		if !record.OrderDate.IsZero() && !transaction.Date.IsZero() {
			days := math.Abs(transaction.Date.Sub(record.OrderDate).Hours() / 24)
			if days > 5 {
				continue
			}
		}
		candidates = append(candidates, transaction)
	}

	if len(candidates) == 1 {
		return candidates[0], false
	}
	if record.MonarchNotes != "" {
		var noteMatch *monarch.Transaction
		for _, candidate := range candidates {
			if strings.TrimSpace(candidate.Notes) != strings.TrimSpace(record.MonarchNotes) {
				continue
			}
			if noteMatch != nil {
				return nil, true
			}
			noteMatch = candidate
		}
		if noteMatch != nil {
			return noteMatch, false
		}
	}
	return nil, len(candidates) > 1
}

func transactionDetailsByID(
	transactions []*monarch.Transaction,
	transactionID string,
) *monarch.TransactionDetails {
	for _, transaction := range transactions {
		if transaction != nil && transaction.ID == transactionID {
			return &monarch.TransactionDetails{Transaction: transaction}
		}
	}
	return nil
}

func matchesRecordedAmount(record *storage.ProcessingRecord, transaction *monarch.Transaction) bool {
	if transaction == nil {
		return false
	}
	return math.Abs(math.Abs(record.TransactionAmount)-math.Abs(transaction.Amount)) <= 0.011
}

func sameIDSafeRepairNeeded(record *storage.ProcessingRecord, transaction *monarch.Transaction) bool {
	if len(record.Splits) > 0 {
		return false
	}
	if record.CategoryID == "" {
		return false
	}
	return transaction.Category == nil ||
		strings.Contains(strings.ToLower(transaction.Category.Name), "temp")
}

func cachedSplitsMatch(cached []storage.SplitDetail, current []*monarch.TransactionSplit) bool {
	if len(cached) != len(current) {
		return false
	}
	matched := make([]bool, len(current))
	for _, expected := range cached {
		found := false
		for i, actual := range current {
			if matched[i] || actual == nil {
				continue
			}
			categoryID := actual.CategoryID
			if categoryID == "" && actual.Category != nil {
				categoryID = actual.Category.ID
			}
			if categoryID == expected.CategoryID &&
				math.Abs(actual.Amount-expected.Amount) <= 0.011 &&
				actual.Notes == expected.Notes {
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
