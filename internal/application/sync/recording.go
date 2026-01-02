package sync

import (
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// convertOrderItems converts provider order items to storage order items
func convertOrderItems(items []providers.OrderItem) []storage.OrderItem {
	if items == nil {
		return nil
	}

	result := make([]storage.OrderItem, len(items))
	for i, item := range items {
		result[i] = storage.OrderItem{
			Name:       item.GetName(),
			Quantity:   item.GetQuantity(),
			UnitPrice:  item.GetUnitPrice(),
			TotalPrice: item.GetPrice(),
			Category:   item.GetCategory(),
		}
	}
	return result
}

// convertSplits converts monarch splits to storage split details
func convertSplits(splits []*monarch.TransactionSplit) []storage.SplitDetail {
	if splits == nil {
		return nil
	}

	result := make([]storage.SplitDetail, len(splits))
	for i, split := range splits {
		result[i] = storage.SplitDetail{
			CategoryID: split.CategoryID,
			Amount:     split.Amount,
			Notes:      split.Notes,
		}
	}
	return result
}

// Recording and audit trail functions for the sync orchestrator.
// These handle persisting processing results and API call logs to storage.

// recordError records a processing error to storage
func (o *Orchestrator) recordError(order providers.Order, errorMsg string) {
	if o.storage != nil {
		record := &storage.ProcessingRecord{
			OrderID:       order.GetID(),
			Provider:      order.GetProviderName(),
			OrderDate:     order.GetDate(),
			OrderTotal:    order.GetTotal(),
			OrderSubtotal: order.GetSubtotal(),
			OrderTax:      order.GetTax(),
			OrderTip:      order.GetTip(),
			ItemCount:     len(order.GetItems()),
			ProcessedAt:   time.Now(),
			Status:        "failed",
			ErrorMessage:  errorMsg,
			Items:         convertOrderItems(order.GetItems()),
		}
		if err := o.storage.SaveRecord(record); err != nil {
			o.logger.Error("Failed to save error record", "order_id", order.GetID(), "error", err)
		}
	}
}

// recordPending records an order that is pending (not yet charged/shipped)
// This allows tracking without blocking retries on future syncs
func (o *Orchestrator) recordPending(order providers.Order, reason string) {
	if o.storage != nil {
		record := &storage.ProcessingRecord{
			OrderID:       order.GetID(),
			Provider:      order.GetProviderName(),
			OrderDate:     order.GetDate(),
			OrderTotal:    order.GetTotal(),
			OrderSubtotal: order.GetSubtotal(),
			OrderTax:      order.GetTax(),
			OrderTip:      order.GetTip(),
			ItemCount:     len(order.GetItems()),
			ProcessedAt:   time.Now(),
			Status:        "pending",
			ErrorMessage:  reason,
			Items:         convertOrderItems(order.GetItems()),
		}
		if err := o.storage.SaveRecord(record); err != nil {
			o.logger.Error("Failed to save pending record", "order_id", order.GetID(), "error", err)
		}
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
			OrderID:         order.GetID(),
			Provider:        order.GetProviderName(),
			OrderDate:       order.GetDate(),
			OrderTotal:      order.GetTotal(),
			OrderSubtotal:   order.GetSubtotal(),
			OrderTax:        order.GetTax(),
			OrderTip:        order.GetTip(),
			ItemCount:       len(order.GetItems()),
			SplitCount:      len(splits),
			ProcessedAt:     time.Now(),
			Status:          "success",
			MatchConfidence: confidence,
			DryRun:          dryRun,
			Items:           convertOrderItems(order.GetItems()),
			Splits:          convertSplits(splits),
		}

		// Add transaction info if available
		if transaction != nil {
			record.TransactionID = transaction.ID
			record.TransactionAmount = transaction.Amount
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

		if err := o.storage.SaveRecord(record); err != nil {
			o.logger.Error("Failed to save success record", "order_id", order.GetID(), "error", err)
		}
	}
}
