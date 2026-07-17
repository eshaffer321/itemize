package amazon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	amazongo "github.com/eshaffer321/amazon-go"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProvider_ImplementsInterface verifies the provider implements the interface.
func TestProvider_ImplementsInterface(t *testing.T) {
	var _ providers.OrderProvider = (*Provider)(nil)
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider(nil, nil)
	assert.Equal(t, "amazon", provider.Name())
	assert.Equal(t, "Amazon", provider.DisplayName())
}

func TestProvider_SupportsFeatures(t *testing.T) {
	provider := NewProvider(nil, nil)
	assert.False(t, provider.SupportsDeliveryTips(), "Amazon doesn't support delivery tips")
	assert.True(t, provider.SupportsRefunds(), "Amazon supports refunds")
	assert.True(t, provider.SupportsBulkFetch(), "Amazon supports bulk fetch")
}

func TestProvider_GetRateLimit(t *testing.T) {
	provider := NewProvider(nil, nil)
	assert.Equal(t, time.Second, provider.GetRateLimit())
}

func TestProvider_MerchantSearchTerms(t *testing.T) {
	provider := NewProvider(nil, nil)
	terms := provider.MerchantSearchTerms()

	assert.Contains(t, terms, "Amazon")
	assert.Contains(t, terms, "AMZN")
	assert.Contains(t, terms, "AMZN Mktp US")
	assert.Contains(t, terms, "Whole Foods")
}

func TestProvider_WithConfig(t *testing.T) {
	provider := NewProvider(nil, &ProviderConfig{
		Profile:    "wife",
		CookieFile: "/tmp/amazon-cookies.json",
	})

	assert.Equal(t, "wife", provider.profile)
	assert.Equal(t, "/tmp/amazon-cookies.json", provider.cookieFile)
}

func TestNewProvider_RejectsUnsafeProfile(t *testing.T) {
	provider := NewProvider(nil, &ProviderConfig{Profile: "wife;rm-rf"})
	assert.Empty(t, provider.profile)
}

func TestProvider_FetchOrdersUsesAmazonGoClientAndTransactions(t *testing.T) {
	orderDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	txDate := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	client := &fakeAmazonClient{
		orders: []*amazongo.Order{
			{
				ID:           "114-9733092-9360267",
				Date:         orderDate,
				Total:        44.91,
				Subtotal:     42.37,
				Tax:          2.54,
				ShippingFees: 0,
				Items: []*amazongo.OrderItem{
					{Name: "Cable", Price: 14.99, Quantity: 1, UnitPrice: 14.99},
					{Name: "Adapter", UnitPrice: 13.69, Quantity: 2},
				},
			},
		},
		transactionsByOrderID: map[string][]*amazongo.Transaction{
			"114-9733092-9360267": {
				{
					OrderID:       "114-9733092-9360267",
					Date:          txDate,
					Amount:        44.91,
					PaymentMethod: "Prime Visa ****1211",
					LastFour:      "1211",
					Status:        "Completed",
				},
			},
		},
	}
	provider := NewProviderWithClient(nil, nil, client)

	orders, err := provider.FetchOrders(context.Background(), providers.FetchOptions{
		StartDate:      orderDate.AddDate(0, 0, -1),
		EndDate:        orderDate.AddDate(0, 0, 1),
		MaxOrders:      10,
		IncludeDetails: true,
	})

	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, amazongo.FetchOptions{
		StartDate:      orderDate.AddDate(0, 0, -1),
		EndDate:        orderDate.AddDate(0, 0, 1),
		MaxOrders:      10,
		IncludeDetails: true,
	}, client.fetchOptions)

	amazonOrder, ok := orders[0].(*Order)
	require.True(t, ok)
	assert.Equal(t, "114-9733092-9360267", amazonOrder.GetID())
	assert.Equal(t, orderDate, amazonOrder.GetDate())
	assert.Equal(t, 44.91, amazonOrder.GetTotal())
	assert.Equal(t, 42.37, amazonOrder.GetSubtotal())
	assert.Equal(t, 2.54, amazonOrder.GetTax())

	items := amazonOrder.GetItems()
	require.Len(t, items, 2)
	assert.Equal(t, "Cable", items[0].GetName())
	assert.Equal(t, 14.99, items[0].GetPrice())
	assert.Equal(t, "Adapter", items[1].GetName())
	assert.Equal(t, 27.38, items[1].GetPrice())

	charges, err := amazonOrder.GetFinalCharges()
	require.NoError(t, err)
	assert.Equal(t, []float64{44.91}, charges)
	assert.Equal(t, []time.Time{txDate}, amazonOrder.GetTransactionDates())
	assert.True(t, client.cookiesSaved)
}

func TestProvider_FetchOrdersKeepsOrderWhenTransactionsFail(t *testing.T) {
	client := &fakeAmazonClient{
		orders: []*amazongo.Order{
			{ID: "114-0000000-0000000", Date: time.Now(), Total: 50},
		},
		fetchTransactionsErr: errors.New("transactions unavailable"),
	}
	provider := NewProviderWithClient(nil, nil, client)

	orders, err := provider.FetchOrders(context.Background(), providers.FetchOptions{IncludeDetails: true})

	require.NoError(t, err)
	require.Len(t, orders, 1)
	amazonOrder := orders[0].(*Order)
	charges, err := amazonOrder.GetFinalCharges()
	assert.Nil(t, charges)
	assert.ErrorIs(t, err, ErrPaymentPending)
}

func TestProvider_FetchOrdersReturnsHealthCheckError(t *testing.T) {
	client := &fakeAmazonClient{healthErr: errors.New("missing essential cookies")}
	provider := NewProviderWithClient(nil, &ProviderConfig{Profile: "wife"}, client)

	orders, err := provider.FetchOrders(context.Background(), providers.FetchOptions{IncludeDetails: true})

	require.Error(t, err)
	assert.Nil(t, orders)
	assert.Contains(t, err.Error(), "amazon auth check failed")
	assert.Contains(t, err.Error(), "itemize amazon setup -account wife")
	assert.True(t, client.healthChecked)
}

func TestProvider_FetchOrdersRewordsExpiredCookieError(t *testing.T) {
	client := &fakeAmazonClient{healthErr: errors.New("authentication failed: cookies are expired, please re-import cookies from browser")}
	provider := NewProviderWithClient(nil, &ProviderConfig{Profile: "wife"}, client)

	_, err := provider.FetchOrders(context.Background(), providers.FetchOptions{IncludeDetails: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "amazon auth check failed")
	assert.Contains(t, err.Error(), "Amazon rejected the saved cookies or opened a sign-in page")
	assert.NotContains(t, err.Error(), "cookies are expired")
}

func TestProvider_FailedHealthCheckDoesNotOverwriteSavedCookies(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies-wife.json")
	stored := amazongo.CookieFile{
		Cookies: []*amazongo.Cookie{
			{Name: "session-id", Value: "original-session-id", Domain: ".amazon.com", Path: "/"},
			{Name: "session-token", Value: "original-session-token", Domain: ".amazon.com", Path: "/"},
			{Name: "ubid-main", Value: "original-ubid", Domain: ".amazon.com", Path: "/"},
			{Name: "at-main", Value: "original-at", Domain: ".amazon.com", Path: "/"},
		},
		UpdatedAt: time.Now(),
	}
	data, err := json.Marshal(stored)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cookieFile, data, 0o600))

	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Set-Cookie": []string{"session-token=poisoned-by-sign-in; Path=/; Domain=.amazon.com"},
			},
			Body:    io.NopCloser(strings.NewReader(`<html><title>Amazon Sign-In</title><input id="ap_email"></html>`)),
			Request: req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	provider := NewProvider(nil, &ProviderConfig{Profile: "wife", CookieFile: cookieFile})
	_, err = provider.FetchOrders(context.Background(), providers.FetchOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amazon auth check failed")

	after, err := os.ReadFile(cookieFile)
	require.NoError(t, err)
	var saved amazongo.CookieFile
	require.NoError(t, json.Unmarshal(after, &saved))
	for _, cookie := range saved.Cookies {
		if cookie.Name == "session-token" {
			assert.Equal(t, "original-session-token", cookie.Value)
			return
		}
	}
	t.Fatal("saved session-token cookie not found")
}

func TestProvider_HealthCheckRewordsExpiredCookieError(t *testing.T) {
	client := &fakeAmazonClient{healthErr: errors.New("authentication failed: cookies are expired, please re-import cookies from browser")}
	provider := NewProviderWithClient(nil, &ProviderConfig{Profile: "wife"}, client)

	err := provider.HealthCheck(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Amazon rejected the saved cookies or opened a sign-in page")
	assert.NotContains(t, err.Error(), "cookies are expired")
}

func TestProvider_GetOrderDetailsFetchesOrderWithTransactions(t *testing.T) {
	client := &fakeAmazonClient{
		detailOrder: &amazongo.Order{
			ID:       "114-9733092-9360267",
			Date:     time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			Total:    12.34,
			Subtotal: 11.11,
			Tax:      1.23,
			Items:    []*amazongo.OrderItem{{Name: "Notebook", Price: 11.11, Quantity: 1}},
		},
		detailTransactions: []*amazongo.Transaction{
			{Amount: 12.34, LastFour: "1211", Status: "Completed"},
		},
	}
	provider := NewProviderWithClient(nil, nil, client)

	order, err := provider.GetOrderDetails(context.Background(), "114-9733092-9360267")

	require.NoError(t, err)
	assert.Equal(t, "114-9733092-9360267", client.detailOrderID)
	assert.Equal(t, "114-9733092-9360267", order.GetID())
	charges, err := order.(*Order).GetFinalCharges()
	require.NoError(t, err)
	assert.Equal(t, []float64{12.34}, charges)
	assert.True(t, client.cookiesSaved)
}

func TestProvider_HealthCheckUsesAmazonGoClient(t *testing.T) {
	client := &fakeAmazonClient{}
	provider := NewProviderWithClient(nil, nil, client)

	require.NoError(t, provider.HealthCheck(context.Background()))
	assert.True(t, client.healthChecked)
}

func TestProvider_HealthCheckWrapsAuthErrorWithLoginCommand(t *testing.T) {
	client := &fakeAmazonClient{healthErr: errors.New("missing essential cookies")}
	provider := NewProviderWithClient(nil, &ProviderConfig{Profile: "wife"}, client)

	err := provider.HealthCheck(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "amazon auth check failed")
	assert.Contains(t, err.Error(), "itemize amazon setup -account wife")
}

func TestConvertGoTransactionMapsRefundAndPendingStatuses(t *testing.T) {
	refund := convertGoTransaction(&amazongo.Transaction{Amount: 10, Status: "Refunded"})
	pending := convertGoTransaction(&amazongo.Transaction{Amount: 10, Status: "Pending"})
	charge := convertGoTransaction(&amazongo.Transaction{Amount: 10, Status: "Completed"})

	assert.Equal(t, "refund", refund.Type)
	assert.Equal(t, "pending", pending.Type)
	assert.Equal(t, "charge", charge.Type)
}

func TestLoginCommandUsesExplicitCookieFile(t *testing.T) {
	provider := NewProvider(nil, &ProviderConfig{CookieFile: "/tmp/amazon-cookies.json"})

	assert.Contains(t, provider.loginCommand(), `-cookie-file "/tmp/amazon-cookies.json"`)
	assert.Contains(t, provider.loginCommand(), "itemize amazon setup")
}

type fakeAmazonClient struct {
	orders                []*amazongo.Order
	transactionsByOrderID map[string][]*amazongo.Transaction
	fetchOrdersErr        error
	fetchTransactionsErr  error
	fetchOptions          amazongo.FetchOptions
	detailOrderID         string
	detailOrder           *amazongo.Order
	detailTransactions    []*amazongo.Transaction
	fetchOrderWithTxErr   error
	healthChecked         bool
	healthErr             error
	cookiesSaved          bool
	saveCookiesErr        error
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (f *fakeAmazonClient) FetchOrders(_ context.Context, opts amazongo.FetchOptions) ([]*amazongo.Order, error) {
	f.fetchOptions = opts
	return f.orders, f.fetchOrdersErr
}

func (f *fakeAmazonClient) FetchOrderWithTransactions(_ context.Context, orderID string) (*amazongo.Order, []*amazongo.Transaction, error) {
	f.detailOrderID = orderID
	return f.detailOrder, f.detailTransactions, f.fetchOrderWithTxErr
}

func (f *fakeAmazonClient) SaveCookies() error {
	f.cookiesSaved = true
	return f.saveCookiesErr
}

func (f *fakeAmazonClient) FetchTransactions(_ context.Context, orderID string) ([]*amazongo.Transaction, error) {
	if f.fetchTransactionsErr != nil {
		return nil, f.fetchTransactionsErr
	}
	return f.transactionsByOrderID[orderID], nil
}

func (f *fakeAmazonClient) HealthCheck() error {
	f.healthChecked = true
	return f.healthErr
}
