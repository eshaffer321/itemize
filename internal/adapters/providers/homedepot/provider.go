// Package homedepot implements providers.OrderProvider for Home Depot
// purchases. It wraps github.com/fnziman/homedepot-go, which speaks the
// internal /oms/customer/order/v1 endpoints homedepot.com itself uses via
// cookie-replay auth.
package homedepot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	hdgo "github.com/fnziman/homedepot-go"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

// hdClient is the subset of homedepot-go's *Client the provider depends on.
// Extracting it as an interface lets us mock the client in tests without
// spinning up an httptest server.
type hdClient interface {
	ListOrders(ctx context.Context, start, end time.Time) ([]hdgo.OrderSummary, error)
	GetOrder(ctx context.Context, summary hdgo.OrderSummary) (hdgo.OrderDetail, error)
	HealthCheck(ctx context.Context) error
}

// Provider is the Home Depot OrderProvider.
type Provider struct {
	client    hdClient
	logger    *slog.Logger
	rateLimit time.Duration
}

// NewProvider constructs a Home Depot provider from a homedepot-go client.
// Callers are responsible for scoping the logger (e.g. system="homedepot").
func NewProvider(client *hdgo.Client, logger *slog.Logger) *Provider {
	if client == nil {
		return newProvider(nil, logger)
	}
	return newProvider(client, logger)
}

func newProvider(client hdClient, logger *slog.Logger) *Provider {
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{
		client:    client,
		logger:    logger,
		rateLimit: 1 * time.Second,
	}
}

// Name is the CLI-facing identifier.
func (p *Provider) Name() string { return "homedepot" }

// DisplayName is the human-readable identifier. It's also what the sync
// orchestrator uses when substring-matching against Monarch merchant
// names (see internal/application/sync/fetch.go), so "Home Depot"
// matches "THE HOME DEPOT" (case-insensitive) cleanly.
func (p *Provider) DisplayName() string { return "Home Depot" }

// FetchOrders returns full-detail orders whose salesDate falls in the
// given range. Follows the same "always fetch details" pattern as the
// Walmart provider — the domain layer requires them for categorization.
func (p *Provider) FetchOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
	p.logger.Info("fetching orders",
		slog.Time("start_date", opts.StartDate),
		slog.Time("end_date", opts.EndDate),
		slog.Int("max_orders", opts.MaxOrders),
	)

	summaries, err := p.client.ListOrders(ctx, opts.StartDate, opts.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to list home depot orders: %w", err)
	}

	if opts.MaxOrders > 0 && len(summaries) > opts.MaxOrders {
		summaries = summaries[:opts.MaxOrders]
	}

	orders := make([]providers.Order, 0, len(summaries))
	for _, summary := range summaries {
		if err := ctx.Err(); err != nil {
			return orders, fmt.Errorf("cancelled during order fetch: %w", err)
		}

		detail, err := p.client.GetOrder(ctx, summary)
		if err != nil {
			p.logger.Warn("failed to fetch home depot order details, skipping",
				slog.String("order_ref", summaryRef(summary)),
				slog.String("origin", summary.OrderOrigin),
				slog.String("error", err.Error()))
			continue
		}
		orders = append(orders, &Order{summary: summary, detail: detail})
	}

	p.logger.Info("fetched orders", slog.Int("total", len(orders)))
	return orders, nil
}

// GetOrderDetails is not supported for Home Depot: /orderdetails needs the
// orderOrigin plus store/register/transaction fields to route to the online
// vs in-store schema, none of which are derivable from a bare order ID.
// Callers should use FetchOrders, which already returns full details.
func (p *Provider) GetOrderDetails(ctx context.Context, orderID string) (providers.Order, error) {
	p.logger.Warn("GetOrderDetails called without OrderSummary context",
		slog.String("order_id", orderID))
	return nil, fmt.Errorf("GetOrderDetails not supported for Home Depot without OrderSummary")
}

// SupportsDeliveryTips — Home Depot orders don't include driver tips.
func (p *Provider) SupportsDeliveryTips() bool { return false }

// SupportsRefunds — refund handling is planned but not implemented in v1;
// the API exposes returnTotal on OrderDetail but we don't wire refund flows
// through the orchestrator yet.
func (p *Provider) SupportsRefunds() bool { return false }

// SupportsBulkFetch — ListOrders returns everything in the range.
func (p *Provider) SupportsBulkFetch() bool { return true }

// GetRateLimit is the desired pacing between successive syncs. The
// underlying homedepot-go client already paces itself between pages and
// per-order detail fetches, so this is a coarse outer-loop pacing hint.
func (p *Provider) GetRateLimit() time.Duration { return p.rateLimit }

// HealthCheck confirms cookies are valid by issuing a small /orderhistory
// request via homedepot-go.
func (p *Provider) HealthCheck(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("home depot client not initialized")
	}
	if err := p.client.HealthCheck(ctx); err != nil {
		return fmt.Errorf("home depot health check failed: %w", err)
	}
	return nil
}

// summaryRef returns a short human-readable identifier for the summary,
// used only in log lines. Handles both online (orderNumber) and in-store
// (store + transactionId) cases.
func summaryRef(s hdgo.OrderSummary) string {
	if len(s.OrderNumbers) > 0 && s.OrderNumbers[0] != "" {
		return s.OrderNumbers[0]
	}
	if s.TransactionID != "" {
		return fmt.Sprintf("store-%s/txn-%s", s.StoreNumber, s.TransactionID)
	}
	return "unknown"
}
