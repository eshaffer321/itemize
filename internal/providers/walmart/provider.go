package walmart

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	walmartclient "github.com/eshaffer321/walmart-client"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
)

// Provider implements the OrderProvider interface for Walmart
type Provider struct {
	client    *walmartclient.WalmartClient
	logger    *slog.Logger
	rateLimit time.Duration
}

// NewProvider creates a new Walmart provider
func NewProvider(client *walmartclient.WalmartClient, logger *slog.Logger) *Provider {
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{
		client:    client,
		logger:    logger.With(slog.String("provider", "walmart")),
		rateLimit: 2 * time.Second,
	}
}

// Name returns the provider identifier
func (p *Provider) Name() string {
	return "walmart"
}

// DisplayName returns the human-readable name
func (p *Provider) DisplayName() string {
	return "Walmart"
}

// FetchOrders fetches orders within the specified date range
func (p *Provider) FetchOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
	p.logger.Info("fetching orders",
		slog.Time("start_date", opts.StartDate),
		slog.Time("end_date", opts.EndDate),
		slog.Int("max_orders", opts.MaxOrders),
	)

	// Calculate timestamps for Walmart API
	minTimestamp := opts.StartDate.Unix()
	maxTimestamp := opts.EndDate.Unix()

	// Create request for purchase history
	req := walmartclient.PurchaseHistoryRequest{
		MinTimestamp: &minTimestamp,
		MaxTimestamp: &maxTimestamp,
	}

	// Fetch purchase history from Walmart API
	resp, err := p.client.GetPurchaseHistory(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch walmart orders: %w", err)
	}

	// Convert OrderSummary to full Orders
	var providerOrders []providers.Order
	for _, summary := range resp.Data.OrderHistoryV2.OrderGroups {
		// Fetch full order details if requested
		if opts.IncludeDetails {
			isInStore := summary.FulfillmentType == "IN_STORE"
			fullOrder, err := p.client.GetOrder(summary.OrderID, isInStore)
			if err != nil {
				p.logger.Warn("failed to fetch order details, skipping",
					slog.String("order_id", summary.OrderID),
					slog.String("error", err.Error()))
				continue
			}

			providerOrders = append(providerOrders, &Order{
				walmartOrder: fullOrder,
				client:       p.client,
				logger:       p.logger,
			})
		} else {
			// For basic listing, we'd need to create a minimal Order
			// For now, always fetch details
			isInStore := summary.FulfillmentType == "IN_STORE"
			fullOrder, err := p.client.GetOrder(summary.OrderID, isInStore)
			if err != nil {
				p.logger.Warn("failed to fetch order details, skipping",
					slog.String("order_id", summary.OrderID),
					slog.String("error", err.Error()))
				continue
			}

			providerOrders = append(providerOrders, &Order{
				walmartOrder: fullOrder,
				client:       p.client,
				logger:       p.logger,
			})
		}
	}

	// Apply max orders limit if specified
	if opts.MaxOrders > 0 && len(providerOrders) > opts.MaxOrders {
		providerOrders = providerOrders[:opts.MaxOrders]
	}

	p.logger.Info("fetched orders", slog.Int("total", len(providerOrders)))

	return providerOrders, nil
}

// GetOrderDetails fetches full details for a specific order
func (p *Provider) GetOrderDetails(ctx context.Context, orderID string) (providers.Order, error) {
	// Note: We need to know if it's in-store or not, which we can't determine from just the ID
	// This is a limitation we'll need to handle
	p.logger.Warn("GetOrderDetails called without fulfillment type context",
		slog.String("order_id", orderID))
	return nil, fmt.Errorf("GetOrderDetails not supported for Walmart without fulfillment type")
}

// SupportsDeliveryTips returns true if provider supports delivery tips
func (p *Provider) SupportsDeliveryTips() bool {
	return true // Walmart has delivery orders with tips
}

// SupportsRefunds returns true if provider supports refunds
func (p *Provider) SupportsRefunds() bool {
	return true
}

// SupportsBulkFetch returns true if provider supports bulk fetching
func (p *Provider) SupportsBulkFetch() bool {
	return true
}

// GetRateLimit returns the rate limit for API calls
func (p *Provider) GetRateLimit() time.Duration {
	return p.rateLimit
}

// HealthCheck verifies the provider is accessible
func (p *Provider) HealthCheck(ctx context.Context) error {
	// Try to fetch recent purchase history as a health check
	now := time.Now()
	oneWeekAgo := now.AddDate(0, 0, -7).Unix()
	nowTimestamp := now.Unix()

	req := walmartclient.PurchaseHistoryRequest{
		MinTimestamp: &oneWeekAgo,
		MaxTimestamp: &nowTimestamp,
	}

	_, err := p.client.GetPurchaseHistory(req)
	if err != nil {
		return fmt.Errorf("walmart health check failed: %w", err)
	}
	return nil
}
