package handlers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	amazonprovider "github.com/eshaffer321/itemize/internal/adapters/providers/amazon"
	"github.com/eshaffer321/itemize/internal/domain/categorizer"
	"github.com/eshaffer321/itemize/internal/domain/matcher"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
)

const temporaryAmazonCategory = "[TEMP] Amazon"

// ProcessRefund categorizes one directly reported Amazon refund only when its
// item and matching Monarch credit are both unambiguous.
func (h *AmazonHandler) ProcessRefund(
	ctx context.Context,
	refund *amazonprovider.RefundOrder,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	dryRun bool,
) (*ProcessResult, error) {
	result := &ProcessResult{}
	if refund == nil {
		result.Skipped = true
		result.SkipReason = "missing Amazon refund record"
		return result, nil
	}
	record := refund.Record()
	switch {
	case !record.HasRefundTotal || record.RefundAmount <= 0:
		result.Skipped = true
		result.SkipReason = "Amazon did not report a refund total"
		return result, nil
	case record.RefundIssuedAt == nil:
		result.Skipped = true
		result.SkipReason = "Amazon did not report a refund-issued date"
		return result, nil
	case record.RMAID == "":
		result.Skipped = true
		result.SkipReason = "Amazon did not report an RMA ID"
		return result, nil
	case len(record.Items) != 1:
		result.Skipped = true
		result.SkipReason = "multiple returned items lack an authoritative item-level refund allocation"
		return result, nil
	}

	eligible := eligibleAmazonRefundTransactions(monarchTxns)

	matchResult, err := h.matcher.FindUniqueMatch(refund, eligible, usedTxnIDs)
	if errors.Is(err, matcher.ErrAmbiguousMatch) {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("ambiguous Amazon refund credit for $%.2f on %s", record.RefundAmount, record.RefundIssuedAt.Format("2006-01-02"))
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("amazon refund match error: %w", err)
	}
	if matchResult == nil {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("no unique temporary Amazon credit found for $%.2f", record.RefundAmount)
		return result, nil
	}

	transaction := matchResult.Transaction
	splits, err := h.splitter.CreateSplits(ctx, refund, transaction, catCategories, monarchCategories)
	if err != nil {
		return nil, fmt.Errorf("amazon refund split creation error: %w", err)
	}
	result.Splits = splits
	if splits == nil {
		categoryID, notes, categoryErr := h.splitter.GetSingleCategoryInfo(ctx, refund, catCategories)
		if categoryErr != nil {
			return nil, fmt.Errorf("amazon refund category error: %w", categoryErr)
		}
		if categoryID == "" {
			result.Skipped = true
			result.SkipReason = "categorizer did not return a valid Monarch category"
			return result, nil
		}
		result.CategoryID = categoryID
		result.MonarchNotes = notes
		if idx := strings.Index(notes, ":"); idx > 0 {
			result.CategoryName = notes[:idx]
		}
		if !dryRun {
			reviewed := false
			if updateErr := h.monarch.UpdateTransaction(ctx, transaction.ID, &monarch.UpdateTransactionParams{
				CategoryID:  &categoryID,
				Notes:       &notes,
				NeedsReview: &reviewed,
			}); updateErr != nil {
				return nil, fmt.Errorf("amazon refund transaction update error: %w", updateErr)
			}
		}
	} else if !dryRun {
		if updateErr := h.monarch.UpdateSplits(ctx, transaction.ID, splits); updateErr != nil {
			return nil, fmt.Errorf("amazon refund split update error: %w", updateErr)
		}
	}

	usedTxnIDs[transaction.ID] = true
	result.Transaction = transaction
	result.Processed = true
	h.logInfo("Categorized Amazon refund",
		"order_id", record.OrderID,
		"rma_id", record.RMAID,
		"transaction_id", transaction.ID,
		"refund_amount", record.RefundAmount,
		"item_asin", record.Items[0].ASIN,
		"dry_run", dryRun)
	return result, nil
}

// ProcessRefundGroup handles indistinguishable same-day/same-amount credits
// only when the group cardinality matches and every returned item resolves to
// the same category. No transaction-to-ASIN assignment is asserted.
func (h *AmazonHandler) ProcessRefundGroup(
	ctx context.Context,
	refunds []*amazonprovider.RefundOrder,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	dryRun bool,
) ([]*ProcessResult, string, error) {
	if len(refunds) < 2 {
		return nil, "refund group requires at least two records", nil
	}
	base := refunds[0].Record()
	if base.RefundIssuedAt == nil {
		return nil, "Amazon did not report a refund-issued date", nil
	}
	issuedDate := base.RefundIssuedAt.Format("2006-01-02")
	for _, refund := range refunds {
		if refund == nil {
			return nil, "missing Amazon refund record", nil
		}
		record := refund.Record()
		if !record.HasRefundTotal || record.RefundIssuedAt == nil || record.RMAID == "" || len(record.Items) != 1 {
			return nil, "refund group lacks authoritative single-item detail", nil
		}
		if math.Abs(record.RefundAmount-base.RefundAmount) > 0.001 || record.RefundIssuedAt.Format("2006-01-02") != issuedDate {
			return nil, "refund group does not share one amount and issued date", nil
		}
	}

	candidates := make([]*monarch.Transaction, 0, len(refunds))
	for _, tx := range eligibleAmazonRefundTransactions(monarchTxns) {
		if usedTxnIDs[tx.ID] || math.Abs(tx.Amount-base.RefundAmount) > 0.011 || tx.Date.Format("2006-01-02") != issuedDate {
			continue
		}
		candidates = append(candidates, tx)
	}
	if len(candidates) != len(refunds) {
		return nil, fmt.Sprintf("refund group has %d Amazon records but %d exact temporary credits", len(refunds), len(candidates)), nil
	}

	categoryID := ""
	categoryName := ""
	for _, refund := range refunds {
		splits, err := h.splitter.CreateSplits(ctx, refund, candidates[0], catCategories, monarchCategories)
		if err != nil {
			return nil, "", fmt.Errorf("amazon refund group categorization failed: %w", err)
		}
		if splits != nil {
			return nil, "returned item did not resolve to one category", nil
		}
		itemCategoryID, notes, err := h.splitter.GetSingleCategoryInfo(ctx, refund, catCategories)
		if err != nil {
			return nil, "", fmt.Errorf("amazon refund group category failed: %w", err)
		}
		if itemCategoryID == "" {
			return nil, "categorizer did not return a valid Monarch category", nil
		}
		itemCategoryName := notes
		if idx := strings.Index(notes, ":"); idx > 0 {
			itemCategoryName = notes[:idx]
		}
		if categoryID == "" {
			categoryID = itemCategoryID
			categoryName = itemCategoryName
			continue
		}
		if itemCategoryID != categoryID {
			return nil, "indistinguishable refund items have different categories", nil
		}
	}

	noteLines := []string{
		fmt.Sprintf("%s:", categoryName),
		fmt.Sprintf("- %d indistinguishable Amazon refunds of $%.2f issued %s", len(refunds), base.RefundAmount, issuedDate),
	}
	for _, refund := range refunds {
		record := refund.Record()
		noteLines = append(noteLines, fmt.Sprintf("- Returned: %s (ASIN %s)", record.Items[0].Name, record.Items[0].ASIN))
	}
	noteLines = append(noteLines, "- Amazon identifies the return group; the bank feed does not identify the individual credit-to-item mapping.")
	notes := strings.Join(noteLines, "\n")

	results := make([]*ProcessResult, 0, len(candidates))
	for _, transaction := range candidates {
		if !dryRun {
			reviewed := false
			if err := h.monarch.UpdateTransaction(ctx, transaction.ID, &monarch.UpdateTransactionParams{
				CategoryID:  &categoryID,
				Notes:       &notes,
				NeedsReview: &reviewed,
			}); err != nil {
				return results, "", fmt.Errorf("amazon refund group transaction update failed: %w", err)
			}
		}
		usedTxnIDs[transaction.ID] = true
		results = append(results, &ProcessResult{
			Processed:    true,
			Transaction:  transaction,
			CategoryID:   categoryID,
			CategoryName: categoryName,
			MonarchNotes: notes,
		})
	}
	h.logInfo("Categorized indistinguishable Amazon refund group",
		"refund_count", len(refunds),
		"refund_amount", base.RefundAmount,
		"issued_date", issuedDate,
		"category_id", categoryID,
		"dry_run", dryRun)
	return results, "", nil
}

func eligibleAmazonRefundTransactions(monarchTxns []*monarch.Transaction) []*monarch.Transaction {
	eligible := make([]*monarch.Transaction, 0, len(monarchTxns))
	for _, tx := range monarchTxns {
		if tx == nil || tx.Pending || tx.Amount <= 0 || tx.HasSplits {
			continue
		}
		if tx.Category != nil && !strings.EqualFold(normalizedCategoryName(tx.Category.Name), temporaryAmazonCategory) {
			continue
		}
		eligible = append(eligible, tx)
	}
	return eligible
}

func normalizedCategoryName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}
