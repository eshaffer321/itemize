package sync

import (
	"encoding/json"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/sync/handlers"
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

// recordSuccessWithResult records a successful processing with optional handler result and multi-delivery info
func (o *Orchestrator) recordSuccessWithResult(
	order providers.Order,
	transaction *monarch.Transaction,
	splits []*monarch.TransactionSplit,
	confidence float64,
	dryRun bool,
	result *handlers.ProcessResult,
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

		// Add audit trail data from handler result
		if result != nil {
			record.CategoryID = result.CategoryID
			record.CategoryName = result.CategoryName
			record.MonarchNotes = result.MonarchNotes
		}

		// Serialize raw order data for audit trail
		if rawData := order.GetRawData(); rawData != nil {
			if rawJSON, err := json.Marshal(rawData); err == nil {
				record.RawOrderJSON = string(rawJSON)
			}
		}

		// Extract fees breakdown if available (provider-specific)
		if feesData := extractFeesBreakdown(order); feesData != "" {
			record.OrderFeesJSON = feesData
		}

		if err := o.storage.SaveRecord(record); err != nil {
			o.logger.Error("Failed to save success record", "order_id", order.GetID(), "error", err)
		}
	}
}

// extractFeesBreakdown extracts fee breakdown from provider-specific order data
func extractFeesBreakdown(order providers.Order) string {
	rawData := order.GetRawData()
	if rawData == nil {
		return ""
	}

	// Try to extract fees from the raw data structure
	// Different providers have different fee structures
	type feesExtractor interface {
		GetFees() interface{}
	}

	if extractor, ok := rawData.(feesExtractor); ok {
		if fees := extractor.GetFees(); fees != nil {
			if feesJSON, err := json.Marshal(fees); err == nil {
				return string(feesJSON)
			}
		}
	}

	// Fallback: try to extract from a map structure
	if dataMap, ok := rawData.(map[string]interface{}); ok {
		if priceDetails, ok := dataMap["priceDetails"].(map[string]interface{}); ok {
			if fees, ok := priceDetails["fees"]; ok {
				if feesJSON, err := json.Marshal(fees); err == nil {
					return string(feesJSON)
				}
			}
		}
	}

	return ""
}
