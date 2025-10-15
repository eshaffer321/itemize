package walmart

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProvider_ImplementsInterface verifies Provider implements OrderProvider
func TestProvider_ImplementsInterface(t *testing.T) {
	var _ providers.OrderProvider = (*Provider)(nil)
}

// TestNewProvider tests provider creation
func TestNewProvider(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	provider := NewProvider(nil, logger)

	assert.NotNil(t, provider)
	assert.Equal(t, "walmart", provider.Name())
	assert.Equal(t, "Walmart", provider.DisplayName())
	assert.Equal(t, 2*time.Second, provider.GetRateLimit())
}

// TestProvider_SupportsFeatures tests feature flags
func TestProvider_SupportsFeatures(t *testing.T) {
	provider := NewProvider(nil, nil)

	assert.True(t, provider.SupportsDeliveryTips(), "Walmart supports delivery tips")
	assert.True(t, provider.SupportsRefunds(), "Walmart supports refunds")
	assert.True(t, provider.SupportsBulkFetch(), "Walmart supports bulk fetch")
}

// TestProvider_FetchOrders_EmptyResult tests fetching with no orders
func TestProvider_FetchOrders_EmptyResult(t *testing.T) {
	// This test will fail until we implement the provider
	t.Skip("Skipping until we have a mock walmart client")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	provider := NewProvider(nil, logger)

	ctx := context.Background()
	opts := providers.FetchOptions{
		StartDate:      time.Now().AddDate(0, 0, -7),
		EndDate:        time.Now(),
		MaxOrders:      0,
		IncludeDetails: false,
	}

	orders, err := provider.FetchOrders(ctx, opts)

	require.NoError(t, err)
	assert.Empty(t, orders)
}

// TestProvider_FetchOrders_WithMaxOrders tests max orders limit
func TestProvider_FetchOrders_WithMaxOrders(t *testing.T) {
	t.Skip("Skipping until we have a mock walmart client")

	// TODO: Create mock client that returns multiple orders
	// Then verify MaxOrders limits the result
}

// TestProvider_GetOrderDetails tests fetching order details
func TestProvider_GetOrderDetails(t *testing.T) {
	provider := NewProvider(nil, nil)

	ctx := context.Background()
	order, err := provider.GetOrderDetails(ctx, "test-order-123")

	// This should return an error because we can't determine fulfillment type from ID alone
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "not supported")
}

// TestProvider_HealthCheck tests health check functionality
func TestProvider_HealthCheck(t *testing.T) {
	t.Skip("Skipping until we have a mock walmart client")

	// TODO: Test with mock client that succeeds/fails
}
