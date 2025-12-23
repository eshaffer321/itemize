package storage

import (
	"encoding/json"
	"time"
)

// ProcessingRecord includes detailed split information
type ProcessingRecord struct {
	ID                int64     `json:"id"`
	OrderID           string    `json:"order_id"`
	Provider          string    `json:"provider"`
	TransactionID     string    `json:"transaction_id"`
	OrderDate         time.Time `json:"order_date"`
	ProcessedAt       time.Time `json:"processed_at"`
	OrderTotal        float64   `json:"order_total"`
	OrderSubtotal     float64   `json:"order_subtotal"`
	OrderTax          float64   `json:"order_tax"`
	OrderTip          float64   `json:"order_tip"`
	TransactionAmount float64   `json:"transaction_amount"`
	SplitCount        int       `json:"split_count"`
	Status            string    `json:"status"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	ItemCount         int       `json:"item_count"`
	MatchConfidence   float64   `json:"match_confidence"`
	DryRun            bool      `json:"dry_run"`

	// Detailed data stored as JSON
	Items      []OrderItem   `json:"items"`
	Splits     []SplitDetail `json:"splits"`
	ItemsJSON  string        `json:"-"` // For DB storage
	SplitsJSON string        `json:"-"` // For DB storage

	// Multi-delivery tracking (JSON)
	MultiDeliveryData string `json:"multi_delivery_data,omitempty"` // For DB storage
}

// OrderItem represents an item in the order
type OrderItem struct {
	Name       string  `json:"name"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	TotalPrice float64 `json:"total_price"`
	Category   string  `json:"category,omitempty"`
}

// SplitDetail represents how the transaction was split
type SplitDetail struct {
	CategoryID   string      `json:"category_id"`
	CategoryName string      `json:"category_name"`
	Amount       float64     `json:"amount"`
	Items        []OrderItem `json:"items"`
	Notes        string      `json:"notes"`
}

// MultiDeliveryInfo tracks multi-delivery consolidation metadata
type MultiDeliveryInfo struct {
	IsMultiDelivery           bool      `json:"is_multi_delivery"`
	ChargeCount               int       `json:"charge_count"`
	OriginalTransactionIDs    []string  `json:"original_transaction_ids"`
	ChargeAmounts             []float64 `json:"charge_amounts"`
	ConsolidatedTransactionID string    `json:"consolidated_transaction_id"`
}

// SetMultiDeliveryInfo serializes multi-delivery metadata to JSON for storage
func (r *ProcessingRecord) SetMultiDeliveryInfo(info *MultiDeliveryInfo) error {
	if info == nil {
		r.MultiDeliveryData = ""
		return nil
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	r.MultiDeliveryData = string(data)
	return nil
}

// GetMultiDeliveryInfo deserializes multi-delivery metadata from JSON
func (r *ProcessingRecord) GetMultiDeliveryInfo() (*MultiDeliveryInfo, error) {
	if r.MultiDeliveryData == "" {
		return nil, nil
	}

	var info MultiDeliveryInfo
	err := json.Unmarshal([]byte(r.MultiDeliveryData), &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// Stats contains enhanced statistics
type Stats struct {
	TotalProcessed     int                      `json:"total_processed"`
	SuccessCount       int                      `json:"success_count"`
	FailedCount        int                      `json:"failed_count"`
	SkippedCount       int                      `json:"skipped_count"`
	DryRunCount        int                      `json:"dry_run_count"`
	TotalAmount        float64                  `json:"total_amount"`
	AverageOrderAmount float64                  `json:"average_order_amount"`
	TotalSplits        int                      `json:"total_splits"`
	ProviderStats      map[string]ProviderStats `json:"provider_stats"`
}

// ProviderStats contains per-provider statistics
type ProviderStats struct {
	Count        int     `json:"count"`
	SuccessCount int     `json:"success_count"`
	TotalAmount  float64 `json:"total_amount"`
}

// APICall represents a logged API call
type APICall struct {
	RunID        int64
	OrderID      string
	Method       string
	RequestJSON  string
	ResponseJSON string
	Error        string
	DurationMs   int64
}
