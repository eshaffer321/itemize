package providers

import (
	"context"
	"time"
)

// Order represents a generic order from any provider
type Order interface {
	GetID() string
	GetDate() time.Time
	GetTotal() float64    // Total amount charged
	GetSubtotal() float64 // Subtotal before tax/fees
	GetTax() float64      // Tax amount
	GetTip() float64      // Delivery tip if applicable
	GetFees() float64     // Other fees (service, delivery, etc)
	GetItems() []OrderItem
	GetProviderName() string // "Walmart", "Costco", etc.
	GetRawData() interface{} // Provider-specific data if needed
}

// OrderItem represents a single item in an order
type OrderItem interface {
	GetName() string
	GetPrice() float64 // Line total for this item
	GetQuantity() float64
	GetUnitPrice() float64
	GetDescription() string
	GetSKU() string
	GetCategory() string // Provider's category if available
}

// FetchOptions configures how orders are fetched
type FetchOptions struct {
	StartDate      time.Time
	EndDate        time.Time
	MaxOrders      int
	IncludeDetails bool // Whether to fetch full order details
}

// OrderProvider is the interface that all providers must implement
type OrderProvider interface {
	// Provider identification
	Name() string        // "walmart", "costco", etc.
	DisplayName() string // "Walmart", "Costco", etc.

	// Order operations
	FetchOrders(ctx context.Context, opts FetchOptions) ([]Order, error)
	GetOrderDetails(ctx context.Context, orderID string) (Order, error)

	// Provider capabilities
	SupportsDeliveryTips() bool
	SupportsRefunds() bool
	SupportsBulkFetch() bool

	// Rate limiting
	GetRateLimit() time.Duration

	// Health check
	HealthCheck(ctx context.Context) error
}

// ProviderConfig is common configuration for providers
type ProviderConfig struct {
	Enabled      bool          `json:"enabled"`
	RateLimit    time.Duration `json:"rate_limit"`
	LookbackDays int           `json:"lookback_days"`
	MaxOrders    int           `json:"max_orders"`
	Debug        bool          `json:"debug"`
}

// ProviderStats tracks provider performance
type ProviderStats struct {
	Provider         string
	OrdersFetched    int
	OrdersProcessed  int
	OrdersFailed     int
	LastFetchTime    time.Time
	LastProcessTime  time.Time
	AverageMatchRate float64
	TotalAmount      float64
}

// ProviderError represents provider-specific errors
type ProviderError struct {
	Provider string
	Code     string
	Message  string
	Err      error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return e.Provider + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Provider + ": " + e.Message
}

// TransactionMatcher matches orders with bank transactions
type TransactionMatcher interface {
	FindMatch(order Order, transactions []interface{}) (interface{}, float64, error)
}
