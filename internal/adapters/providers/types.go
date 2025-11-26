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
