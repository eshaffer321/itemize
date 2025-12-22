package sync

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// Recording and audit trail functions for the sync orchestrator.
// These handle persisting processing results and API call logs to storage.

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
		if err := o.storage.SaveRecord(record); err != nil {
			o.logger.Error("Failed to save error record", "order_id", order.GetID(), "error", err)
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

		if err := o.storage.SaveRecord(record); err != nil {
			o.logger.Error("Failed to save success record", "order_id", order.GetID(), "error", err)
		}
	}
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
