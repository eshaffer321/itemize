package dto

import "time"

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// OrderResponse represents an order in API responses.
type OrderResponse struct {
	OrderID           string          `json:"order_id"`
	Provider          string          `json:"provider"`
	TransactionID     string          `json:"transaction_id,omitempty"`
	OrderDate         string          `json:"order_date"`
	ProcessedAt       string          `json:"processed_at"`
	OrderTotal        float64         `json:"order_total"`
	OrderSubtotal     float64         `json:"order_subtotal"`
	OrderTax          float64         `json:"order_tax"`
	OrderTip          float64         `json:"order_tip,omitempty"`
	TransactionAmount float64         `json:"transaction_amount"`
	Status            string          `json:"status"`
	ErrorMessage      string          `json:"error_message,omitempty"`
	ItemCount         int             `json:"item_count"`
	SplitCount        int             `json:"split_count"`
	MatchConfidence   float64         `json:"match_confidence"`
	DryRun            bool            `json:"dry_run"`
	Items             []ItemResponse  `json:"items,omitempty"`
	Splits            []SplitResponse `json:"splits,omitempty"`
}

// ItemResponse represents an item within an order.
type ItemResponse struct {
	Name       string  `json:"name"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	TotalPrice float64 `json:"total_price"`
	Category   string  `json:"category,omitempty"`
}

// SplitResponse represents a transaction split.
type SplitResponse struct {
	CategoryID   string         `json:"category_id"`
	CategoryName string         `json:"category_name"`
	Amount       float64        `json:"amount"`
	Items        []ItemResponse `json:"items,omitempty"`
	Notes        string         `json:"notes,omitempty"`
}

// OrderListResponse is returned when listing orders.
type OrderListResponse struct {
	Orders     []OrderResponse `json:"orders"`
	TotalCount int             `json:"total_count"`
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
}

// ItemSearchResultResponse represents an item found in search.
type ItemSearchResultResponse struct {
	OrderID   string  `json:"order_id"`
	Provider  string  `json:"provider"`
	OrderDate string  `json:"order_date"`
	ItemName  string  `json:"item_name"`
	ItemPrice float64 `json:"item_price"`
	Category  string  `json:"category,omitempty"`
}

// ItemSearchResponse is returned when searching items.
type ItemSearchResponse struct {
	Items []ItemSearchResultResponse `json:"items"`
	Query string                     `json:"query"`
	Count int                        `json:"count"`
}

// SyncRunResponse represents a sync run in API responses.
type SyncRunResponse struct {
	ID              int64  `json:"id"`
	Provider        string `json:"provider"`
	StartedAt       string `json:"started_at"`
	CompletedAt     string `json:"completed_at,omitempty"`
	LookbackDays    int    `json:"lookback_days"`
	DryRun          bool   `json:"dry_run"`
	OrdersFound     int    `json:"orders_found"`
	OrdersProcessed int    `json:"orders_processed"`
	OrdersSkipped   int    `json:"orders_skipped"`
	OrdersErrored   int    `json:"orders_errored"`
	Status          string `json:"status"`
}

// SyncRunListResponse is returned when listing sync runs.
type SyncRunListResponse struct {
	Runs  []SyncRunResponse `json:"runs"`
	Count int               `json:"count"`
}

// NewHealthResponse creates a health response with current timestamp.
func NewHealthResponse() HealthResponse {
	return HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
