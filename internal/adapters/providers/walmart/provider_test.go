package walmart

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	walmartclient "github.com/eshaffer321/walmart-client-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type detailCall struct {
	orderID   string
	groupID   string
	isInStore bool
}

type fakeWalmartClient struct {
	summaries   []walmartclient.OrderSummary
	historyErr  error
	detailErr   error
	detailCalls []detailCall
}

func (f *fakeWalmartClient) GetPurchaseHistory(_ context.Context, _ walmartclient.PurchaseHistoryRequest) (*walmartclient.PurchaseHistoryResponse, error) {
	if f.historyErr != nil {
		return nil, f.historyErr
	}
	resp := &walmartclient.PurchaseHistoryResponse{}
	resp.Data.OrderHistoryV2.OrderGroups = f.summaries
	return resp, nil
}

func (f *fakeWalmartClient) GetOrderWithGroup(_ context.Context, orderID, groupID string, isInStore bool) (*walmartclient.Order, error) {
	f.detailCalls = append(f.detailCalls, detailCall{orderID: orderID, groupID: groupID, isInStore: isInStore})
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	return &walmartclient.Order{ID: orderID}, nil
}

func (f *fakeWalmartClient) GetOrderLedger(_ context.Context, orderID string) (*walmartclient.OrderLedger, error) {
	return &walmartclient.OrderLedger{OrderID: orderID}, nil
}

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
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	provider := newProvider(&fakeWalmartClient{}, logger)

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
	client := &fakeWalmartClient{summaries: []walmartclient.OrderSummary{
		{OrderID: "order-1", GroupID: "group-1", FulfillmentType: "IN_STORE"},
		{OrderID: "order-2", GroupID: "group-2", FulfillmentType: "DFS"},
		{OrderID: "order-3", GroupID: "group-3", FulfillmentType: "IN_STORE"},
	}}
	provider := newProvider(client, nil)

	orders, err := provider.FetchOrders(context.Background(), providers.FetchOptions{
		StartDate:      time.Now().AddDate(0, 0, -7),
		EndDate:        time.Now(),
		MaxOrders:      2,
		IncludeDetails: true,
	})

	require.NoError(t, err)
	assert.Len(t, orders, 2)
	assert.Equal(t, []detailCall{
		{orderID: "order-1", groupID: "group-1", isInStore: true},
		{orderID: "order-2", groupID: "group-2", isInStore: false},
	}, client.detailCalls, "max orders must be applied before detail requests")
}

func TestProvider_FetchOrders_SkipsActiveOrdersBeforeMax(t *testing.T) {
	client := &fakeWalmartClient{summaries: []walmartclient.OrderSummary{
		{OrderID: "active-order", GroupID: "active-group", IsActive: true},
		{OrderID: "completed-order", GroupID: "completed-group", IsActive: false},
	}}
	provider := newProvider(client, nil)

	orders, err := provider.FetchOrders(context.Background(), providers.FetchOptions{
		StartDate: time.Now().AddDate(0, 0, -7),
		EndDate:   time.Now(),
		MaxOrders: 1,
	})

	require.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, []detailCall{
		{orderID: "completed-order", groupID: "completed-group", isInStore: false},
	}, client.detailCalls, "active orders must not consume detail requests or the max-orders budget")
}

func TestProvider_FetchOrders_PassesPurchaseHistoryGroupID(t *testing.T) {
	client := &fakeWalmartClient{summaries: []walmartclient.OrderSummary{
		{OrderID: "order-1", GroupID: "captured-group", FulfillmentType: "IN_STORE"},
	}}
	provider := newProvider(client, nil)

	_, err := provider.FetchOrders(context.Background(), providers.FetchOptions{
		StartDate: time.Now().AddDate(0, 0, -7),
		EndDate:   time.Now(),
	})

	require.NoError(t, err)
	require.Len(t, client.detailCalls, 1)
	assert.Equal(t, "captured-group", client.detailCalls[0].groupID)
}

func TestProvider_FetchOrders_StopsOnBotChallenge(t *testing.T) {
	challenge := fmt.Errorf("wrapped challenge: %w", walmartclient.ErrBotChallenge)
	client := &fakeWalmartClient{
		summaries: []walmartclient.OrderSummary{
			{OrderID: "order-1", GroupID: "group-1"},
			{OrderID: "order-2", GroupID: "group-2"},
		},
		detailErr: challenge,
	}
	provider := newProvider(client, nil)

	orders, err := provider.FetchOrders(context.Background(), providers.FetchOptions{
		StartDate: time.Now().AddDate(0, 0, -7),
		EndDate:   time.Now(),
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, walmartclient.ErrBotChallenge)
	assert.Empty(t, orders)
	assert.Len(t, client.detailCalls, 1, "bot challenge must stop further detail requests")
}

func TestDedupeOrderSummariesByID(t *testing.T) {
	orders := []walmartclient.OrderSummary{
		{OrderID: "duplicate", FulfillmentType: "DFS", ItemCount: 2},
		{OrderID: "unique", FulfillmentType: "IN_STORE", ItemCount: 1},
		{OrderID: "duplicate", FulfillmentType: "DFS", ItemCount: 2},
	}

	deduped, duplicates := dedupeOrderSummariesByID(orders)

	require.Len(t, deduped, 2)
	assert.Equal(t, "duplicate", deduped[0].OrderID)
	assert.Equal(t, "unique", deduped[1].OrderID)
	assert.Equal(t, []string{"duplicate"}, duplicates)
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
