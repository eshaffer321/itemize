package sync

import (
	"context"
	"fmt"
	"time"

	amazonprovider "github.com/eshaffer321/itemize/internal/adapters/providers/amazon"
	"github.com/eshaffer321/itemize/internal/domain/categorizer"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
)

type amazonReturnProvider interface {
	FetchReturns(ctx context.Context) ([]amazonprovider.ReturnRecord, error)
}

func (o *Orchestrator) fetchAmazonReturns(ctx context.Context) ([]amazonprovider.ReturnRecord, error) {
	provider, ok := o.provider.(amazonReturnProvider)
	if !ok {
		return nil, nil
	}
	started := time.Now()
	returns, err := provider.FetchReturns(ctx)
	o.logProviderFetch("amazon_returns", map[string]any{}, map[string]any{
		"return_count": len(returns),
	}, err, time.Since(started), 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Amazon returns: %w", err)
	}
	o.logger.Info("Fetched Amazon return ledger", "return_count", len(returns))
	return returns, nil
}

func (o *Orchestrator) processAmazonReturns(
	ctx context.Context,
	returns []amazonprovider.ReturnRecord,
	transactions []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	opts Options,
	now time.Time,
	result *Result,
) {
	if o.amazonHandler == nil || result == nil {
		return
	}
	type refundGroup struct {
		key     string
		records []amazonprovider.ReturnRecord
	}
	groupsByKey := make(map[string]*refundGroup)
	groups := make([]*refundGroup, 0)
	for _, record := range returns {
		if opts.OrderID != "" && record.OrderID != opts.OrderID && record.RMAID != opts.OrderID {
			continue
		}
		if !returnFallsWithinLookback(record, opts.LookbackDays, now) {
			continue
		}
		refund := amazonprovider.NewRefundOrder(record)
		if !opts.Force && o.storage != nil && o.storage.IsProcessed(refund.GetID()) {
			o.logger.Debug("Skipping already processed Amazon refund", "refund_id", refund.GetID())
			continue
		}
		key := fmt.Sprintf("%s|%.2f", record.RefundIssuedAt.Format("2006-01-02"), record.RefundAmount)
		group := groupsByKey[key]
		if group == nil {
			group = &refundGroup{key: key}
			groupsByKey[key] = group
			groups = append(groups, group)
		}
		group.records = append(group.records, record)
	}

	for _, group := range groups {
		if len(group.records) > 1 {
			o.processAmazonRefundGroup(ctx, group.key, group.records, transactions, usedTxnIDs, catCategories, monarchCategories, opts, result)
			continue
		}
		record := group.records[0]
		refund := amazonprovider.NewRefundOrder(record)
		refundCtx := withAuditContext(ctx, refund.GetID(), opts.DryRun)
		processed, err := o.amazonHandler.ProcessRefund(refundCtx, refund, transactions, usedTxnIDs, catCategories, monarchCategories, opts.DryRun)
		if err != nil {
			result.ErrorCount++
			wrapped := fmt.Errorf("amazon refund %s (order %s, $%.2f): %w", record.RMAID, record.OrderID, record.RefundAmount, err)
			result.Errors = append(result.Errors, wrapped)
			o.logger.Error("Amazon refund processing failed", "order_id", record.OrderID, "rma_id", record.RMAID, "error", err)
			o.recordError(refund, err.Error(), nil)
			continue
		}
		if processed.Skipped {
			result.RefundSkippedCount++
			o.logger.Warn("Amazon refund left untouched", "order_id", record.OrderID, "rma_id", record.RMAID, "reason", processed.SkipReason)
			continue
		}
		if processed.Processed {
			result.RefundProcessedCount++
			o.recordSuccessWithResult(refund, processed.Transaction, processed.Splits, 1, opts.DryRun, processed, nil)
		}
	}
}

func (o *Orchestrator) processAmazonRefundGroup(
	ctx context.Context,
	groupKey string,
	records []amazonprovider.ReturnRecord,
	transactions []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	opts Options,
	result *Result,
) {
	refunds := make([]*amazonprovider.RefundOrder, 0, len(records))
	for _, record := range records {
		refunds = append(refunds, amazonprovider.NewRefundOrder(record))
	}
	groupCtx := withAuditContext(ctx, "amazon-refund-group:"+groupKey, opts.DryRun)
	processed, skipReason, err := o.amazonHandler.ProcessRefundGroup(groupCtx, refunds, transactions, usedTxnIDs, catCategories, monarchCategories, opts.DryRun)
	if err != nil {
		result.ErrorCount++
		wrapped := fmt.Errorf("amazon refund group %s: %w", groupKey, err)
		result.Errors = append(result.Errors, wrapped)
		o.logger.Error("Amazon refund group processing failed", "group", groupKey, "error", err)
		return
	}
	if skipReason != "" {
		result.RefundSkippedCount += len(records)
		o.logger.Warn("Amazon refund group left untouched", "group", groupKey, "refund_count", len(records), "reason", skipReason)
		return
	}
	if len(processed) != len(records) {
		result.ErrorCount++
		result.Errors = append(result.Errors, fmt.Errorf("amazon refund group %s returned %d results for %d records", groupKey, len(processed), len(records)))
		return
	}
	for i, refund := range refunds {
		// Record each authoritative return as complete for deduplication, but do
		// not persist an individual Monarch transaction association: Amazon and
		// the bank feed cannot identify which indistinguishable credit belongs
		// to which returned item.
		auditResult := *processed[i]
		auditResult.Transaction = nil
		o.recordSuccessWithResult(refund, nil, processed[i].Splits, 1, opts.DryRun, &auditResult, nil)
	}
	result.RefundProcessedCount += len(processed)
}

func returnFallsWithinLookback(record amazonprovider.ReturnRecord, lookbackDays int, now time.Time) bool {
	if record.RefundIssuedAt == nil {
		return false
	}
	cutoff := now.AddDate(0, 0, -lookbackDays)
	issuedKey := dateKey(*record.RefundIssuedAt)
	return issuedKey >= dateKey(cutoff) && issuedKey <= dateKey(now)
}

func dateKey(value time.Time) int {
	year, month, day := value.Date()
	return year*10000 + int(month)*100 + day
}
