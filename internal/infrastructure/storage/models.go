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

// LedgerState represents the current state of an order's ledger
type LedgerState string

const (
	LedgerStatePending       LedgerState = "payment_pending"
	LedgerStateCharged       LedgerState = "charged"
	LedgerStateRefunded      LedgerState = "refunded"
	LedgerStatePartialRefund LedgerState = "partial_refund"
)

// OrderLedger represents a snapshot of an order's ledger at a point in time
type OrderLedger struct {
	ID                 int64       `json:"id"`
	OrderID            string      `json:"order_id"`
	SyncRunID          int64       `json:"sync_run_id,omitempty"`
	Provider           string      `json:"provider"`
	FetchedAt          time.Time   `json:"fetched_at"`
	LedgerState        LedgerState `json:"ledger_state"`
	LedgerVersion      int         `json:"ledger_version"`
	LedgerJSON         string      `json:"ledger_json"`          // Raw provider JSON
	TotalCharged       float64     `json:"total_charged"`        // Sum of all charges
	ChargeCount        int         `json:"charge_count"`         // Number of charges
	PaymentMethodTypes string      `json:"payment_method_types"` // Comma-separated: "CREDITCARD,GIFTCARD"
	HasRefunds         bool        `json:"has_refunds"`
	IsValid            bool        `json:"is_valid"`
	ValidationNotes    string      `json:"validation_notes,omitempty"`

	// Populated from ledger_charges table
	Charges []LedgerCharge `json:"charges,omitempty"`
}

// LedgerCharge represents a single charge within a ledger
type LedgerCharge struct {
	ID                   int64     `json:"id"`
	OrderLedgerID        int64     `json:"order_ledger_id"`
	OrderID              string    `json:"order_id"`
	SyncRunID            int64     `json:"sync_run_id,omitempty"`
	ChargeSequence       int       `json:"charge_sequence"` // Order within ledger
	ChargeAmount         float64   `json:"charge_amount"`
	ChargeType           string    `json:"charge_type"`            // "payment", "refund"
	PaymentMethod        string    `json:"payment_method"`         // "CREDITCARD", "GIFTCARD"
	CardType             string    `json:"card_type,omitempty"`    // "VISA", "AMEX"
	CardLastFour         string    `json:"card_last_four,omitempty"`
	MonarchTransactionID string    `json:"monarch_transaction_id,omitempty"`
	IsMatched            bool      `json:"is_matched"`
	MatchConfidence      float64   `json:"match_confidence,omitempty"`
	MatchedAt            time.Time `json:"matched_at,omitempty"`
	SplitCount           int       `json:"split_count,omitempty"`
}

// LedgerFilters defines filters for querying ledgers
type LedgerFilters struct {
	OrderID  string      // Filter by order ID
	Provider string      // Filter by provider
	State    LedgerState // Filter by ledger state
	Limit    int         // Max results (0 = default 50)
	Offset   int         // Pagination offset
}

// LedgerListResult contains paginated ledger results
type LedgerListResult struct {
	Ledgers    []*OrderLedger `json:"ledgers"`
	TotalCount int            `json:"total_count"`
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
}
