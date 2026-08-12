package homedepot

import (
	"context"
	"errors"
	"testing"
	"time"

	hdgo "github.com/fnziman/homedepot-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

// mockClient implements the hdClient interface with configurable behavior.
type mockClient struct {
	listErr        error
	summaries      []hdgo.OrderSummary
	details        map[string]hdgo.OrderDetail // keyed by summary orderNumber or transactionId
	detailErrs     map[string]error
	healthErr      error
	listCalls      int
	getOrderCalls  int
	healthCalls    int
}

func (m *mockClient) ListOrders(_ context.Context, _, _ time.Time) ([]hdgo.OrderSummary, error) {
	m.listCalls++
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.summaries, nil
}

func (m *mockClient) GetOrder(_ context.Context, s hdgo.OrderSummary) (hdgo.OrderDetail, error) {
	m.getOrderCalls++
	key := summaryKey(s)
	if err := m.detailErrs[key]; err != nil {
		return hdgo.OrderDetail{}, err
	}
	if d, ok := m.details[key]; ok {
		return d, nil
	}
	return hdgo.OrderDetail{}, errors.New("no fixture for " + key)
}

func (m *mockClient) HealthCheck(_ context.Context) error {
	m.healthCalls++
	return m.healthErr
}

func summaryKey(s hdgo.OrderSummary) string {
	if len(s.OrderNumbers) > 0 && s.OrderNumbers[0] != "" {
		return s.OrderNumbers[0]
	}
	return s.TransactionID
}

func TestProvider_NameAndDisplayName(t *testing.T) {
	p := newProvider(&mockClient{}, nil)
	assert.Equal(t, "homedepot", p.Name())
	// DisplayName intentionally matches Monarch's "THE HOME DEPOT" merchant
	// name via case-insensitive substring; do not change casually.
	assert.Equal(t, "Home Depot", p.DisplayName())
}

func TestProvider_Capabilities(t *testing.T) {
	p := newProvider(&mockClient{}, nil)
	assert.False(t, p.SupportsDeliveryTips())
	assert.False(t, p.SupportsRefunds())
	assert.True(t, p.SupportsBulkFetch())
	assert.Equal(t, time.Second, p.GetRateLimit())
}

func TestProvider_FetchOrders_HappyPath(t *testing.T) {
	summaries := []hdgo.OrderSummary{
		{OrderOrigin: "online", OrderNumbers: []string{"WD-000-0001"}, SalesDate: "2026-07-01T00:00:00Z", TotalAmount: 42.99},
		{OrderOrigin: "instore", StoreNumber: "0000", TransactionID: "TXN-000-0001", SalesDate: "2026-07-02T00:00:00Z", TotalAmount: 27.50},
	}
	details := map[string]hdgo.OrderDetail{
		"WD-000-0001": {
			OrderNumber: "WD-000-0001", OrderOrigin: "online", SalesDate: "2026-07-01T00:00:00Z",
			SubTotalAmount: 39.99, TaxTotalAmount: 3.00, GrandTotalAmount: 42.99,
			FulfillmentGroups: []hdgo.FulfillmentGroup{{LineItems: []hdgo.LineItem{
				{THDSKU: "000-000-000", BrandName: "Test Brand", Description: "Sample Hammer", UnitPrice: 19.99, TotalPrice: 19.99, CurrentQuantity: 1},
			}}},
		},
		"TXN-000-0001": {
			OrderNumber: "", OrderOrigin: "instore", SalesDate: "2026-07-02T00:00:00Z", StoreNumber: "0000",
			SubTotalAmount: 25.00, TaxTotalAmount: 2.50, GrandTotalAmount: 27.50,
			FulfillmentGroups: []hdgo.FulfillmentGroup{{LineItems: []hdgo.LineItem{
				{THDSKU: "000-000-002", Description: "Sample Paint, 1 Gallon", UnitPrice: 25.00, TotalPrice: 25.00, CurrentQuantity: 1},
			}}},
		},
	}
	m := &mockClient{summaries: summaries, details: details}
	p := newProvider(m, nil)

	orders, err := p.FetchOrders(context.Background(), providers.FetchOptions{
		StartDate: time.Now().AddDate(0, -1, 0),
		EndDate:   time.Now(),
	})

	require.NoError(t, err)
	require.Len(t, orders, 2)
	assert.Equal(t, 1, m.listCalls)
	assert.Equal(t, 2, m.getOrderCalls)

	// First order — online — uses orderNumber as ID.
	assert.Equal(t, "WD-000-0001", orders[0].GetID())
	assert.InDelta(t, 42.99, orders[0].GetTotal(), 0.001)

	// Second order — in-store — uses composite ID because orderNumber is empty.
	assert.Equal(t, "hd-instore-0000-TXN-000-0001", orders[1].GetID())
	assert.InDelta(t, 27.50, orders[1].GetTotal(), 0.001)
}

func TestProvider_FetchOrders_MaxOrdersCap(t *testing.T) {
	summaries := []hdgo.OrderSummary{
		{OrderOrigin: "online", OrderNumbers: []string{"A"}},
		{OrderOrigin: "online", OrderNumbers: []string{"B"}},
		{OrderOrigin: "online", OrderNumbers: []string{"C"}},
	}
	details := map[string]hdgo.OrderDetail{
		"A": {OrderNumber: "A"}, "B": {OrderNumber: "B"}, "C": {OrderNumber: "C"},
	}
	m := &mockClient{summaries: summaries, details: details}
	p := newProvider(m, nil)

	orders, err := p.FetchOrders(context.Background(), providers.FetchOptions{
		StartDate: time.Now().AddDate(0, -1, 0),
		EndDate:   time.Now(),
		MaxOrders: 2,
	})

	require.NoError(t, err)
	assert.Len(t, orders, 2)
	assert.Equal(t, 2, m.getOrderCalls, "should not fetch details beyond MaxOrders")
}

func TestProvider_FetchOrders_ListError(t *testing.T) {
	m := &mockClient{listErr: errors.New("boom")}
	p := newProvider(m, nil)

	_, err := p.FetchOrders(context.Background(), providers.FetchOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	assert.Equal(t, 0, m.getOrderCalls)
}

func TestProvider_FetchOrders_SkipsFailedDetail(t *testing.T) {
	summaries := []hdgo.OrderSummary{
		{OrderOrigin: "online", OrderNumbers: []string{"OK"}},
		{OrderOrigin: "online", OrderNumbers: []string{"BAD"}},
		{OrderOrigin: "online", OrderNumbers: []string{"ALSO_OK"}},
	}
	m := &mockClient{
		summaries: summaries,
		details: map[string]hdgo.OrderDetail{
			"OK":      {OrderNumber: "OK"},
			"ALSO_OK": {OrderNumber: "ALSO_OK"},
		},
		detailErrs: map[string]error{"BAD": errors.New("detail failed")},
	}
	p := newProvider(m, nil)

	orders, err := p.FetchOrders(context.Background(), providers.FetchOptions{})
	require.NoError(t, err)
	assert.Len(t, orders, 2, "the failed order should be skipped, not fatal")
}

func TestProvider_FetchOrders_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	m := &mockClient{summaries: []hdgo.OrderSummary{{OrderNumbers: []string{"X"}, OrderOrigin: "online"}}}
	p := newProvider(m, nil)

	orders, err := p.FetchOrders(ctx, providers.FetchOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
	assert.Empty(t, orders)
}

func TestProvider_HealthCheck(t *testing.T) {
	m := &mockClient{}
	p := newProvider(m, nil)
	assert.NoError(t, p.HealthCheck(context.Background()))
	assert.Equal(t, 1, m.healthCalls)
}

func TestProvider_HealthCheck_Error(t *testing.T) {
	m := &mockClient{healthErr: errors.New("nope")}
	p := newProvider(m, nil)
	err := p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}

func TestProvider_HealthCheck_NilClient(t *testing.T) {
	p := newProvider(nil, nil)
	err := p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestProvider_GetOrderDetails_NotSupported(t *testing.T) {
	p := newProvider(&mockClient{}, nil)
	_, err := p.GetOrderDetails(context.Background(), "WD-000-0001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestSummaryRef(t *testing.T) {
	cases := []struct {
		name string
		s    hdgo.OrderSummary
		want string
	}{
		{"online with order number", hdgo.OrderSummary{OrderNumbers: []string{"WD-1"}}, "WD-1"},
		{"instore with txn", hdgo.OrderSummary{StoreNumber: "0100", TransactionID: "TXN-9"}, "store-0100/txn-TXN-9"},
		{"neither", hdgo.OrderSummary{}, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, summaryRef(tc.s))
		})
	}
}

// TestProvider_ImplementsInterface is a compile-time check that Provider
// still satisfies providers.OrderProvider (guards against interface drift).
func TestProvider_ImplementsInterface(t *testing.T) {
	var _ providers.OrderProvider = (*Provider)(nil)
}
