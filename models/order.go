// Package models contains data models for the Walmart-Monarch sync backend.
package models

import "time"

// Order represents a Walmart order received from the Chrome extension.
type Order struct {
	OrderNumber     string      `json:"orderNumber" binding:"required"`
	OrderDate       string      `json:"orderDate" binding:"required"`
	OrderTotal      *float64    `json:"orderTotal,omitempty"`
	Tax             *float64    `json:"tax,omitempty"`
	DeliveryCharges *float64    `json:"deliveryCharges,omitempty"`
	Tip             *float64    `json:"tip,omitempty"`
	Items           []OrderItem `json:"items,omitempty"`
}

// OrderItem represents an individual item within a Walmart order.
type OrderItem struct {
	Name       string  `json:"name" binding:"required"`
	Price      float64 `json:"price" binding:"required"`
	Quantity   int     `json:"quantity" binding:"required"`
	ProductURL string  `json:"productUrl,omitempty"`
	Category   string  `json:"category,omitempty"`
}

// OrderResponse represents the API response after processing an order.
type OrderResponse struct {
	Status       string    `json:"status"`
	Message      string    `json:"message,omitempty"`
	OrderID      string    `json:"orderId,omitempty"`
	ProcessingID string    `json:"processingId,omitempty"`
	ItemCount    int       `json:"itemCount,omitempty"`
	TotalAmount  *float64  `json:"totalAmount,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// BatchOrdersRequest represents a request containing multiple orders.
type BatchOrdersRequest struct {
	Orders []Order `json:"orders" binding:"required"`
}

// BatchOrdersResponse represents the response after processing multiple orders.
type BatchOrdersResponse struct {
	Success        bool               `json:"success"`
	ProcessedCount int                `json:"processedCount"`
	FailedCount    int                `json:"failedCount"`
	Results        []BatchOrderResult `json:"results"`
	Timestamp      time.Time          `json:"timestamp"`
}

// BatchOrderResult represents the result of processing a single order in a batch.
type BatchOrderResult struct {
	OrderNumber  string `json:"orderNumber"`
	Success      bool   `json:"success"`
	ProcessingID string `json:"processingId,omitempty"`
	Error        string `json:"error,omitempty"`
}

// SyncStatusResponse represents the sync status information.
type SyncStatusResponse struct {
	LastSyncTimestamp    *time.Time `json:"lastSyncTimestamp"`
	OrdersProcessedToday int        `json:"ordersProcessedToday"`
	OrdersProcessedTotal int        `json:"ordersProcessedTotal"`
	PendingErrors        []string   `json:"pendingErrors,omitempty"`
	Status               string     `json:"status"`
}
