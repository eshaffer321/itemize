package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	amazongo "github.com/eshaffer321/amazon-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const returnHistoryFixture = `<!doctype html>
<html><head><title>Online Return Center</title></head><body>
  <div class="a-box-group a-spacing-extra-large">
    <div class="a-box a-last"><div class="a-box-inner">
      <div>RETURN CREATED Jun 29, 2026</div>
      <div>REFUND TOTAL $11.36</div>
      <div>ORDER # 112-1111111-2222222</div>
      <div>RMA ID : D8ExampleRRMA</div>
      <div class="a-row">
        <div class="a-column a-span9">
          <div class="a-section"><h4>Return in transit</h4></div>
          <div class="a-fixed-left-grid-col a-col-right">
            <div class="a-row"><a href="/gp/product/B0EXAMPLE1">Insulated Sporty Cup</a></div>
            <div class="a-row"><span>$10.72</span></div>
          </div>
        </div>
        <div class="a-column a-span3 a-span-last">
          <a href="/spr/returns/cart?orderId=112-1111111-2222222&amp;returnSessionId=session">View return/refund status</a>
        </div>
      </div>
    </div></div>
  </div>
  <div class="a-box-group a-spacing-extra-large">
    <div class="a-box a-last"><div class="a-box-inner">
      <div>RETURN CREATED Jun 14, 2026</div>
      <div>REFUND TOTAL $165.95</div>
      <div>ORDER # 112-3333333-4444444</div>
      <div>RMA ID : DrExampleRRMA</div>
      <div class="a-row">
        <div class="a-column a-span9">
          <div class="a-section"><h4>Refund issued</h4></div>
          <div class="a-fixed-left-grid-col a-col-right">
            <div class="a-row"><a href="https://www.amazon.com/gp/product/B0EXAMPLE2">Caterpillar Play Tunnel</a></div>
            <div class="a-row"><span>$156.56</span></div>
          </div>
        </div>
        <a href="/spr/returns/cart?orderId=112-3333333-4444444">View return/refund status</a>
      </div>
    </div></div>
  </div>
  <div class="a-box-group a-spacing-extra-large">
    <div class="a-box a-last"><div class="a-box-inner">
      <div>RETURN CREATED Jul 6, 2026</div>
      <div>ORDER # 112-5555555-6666666</div>
      <div>RMA ID : D2PendingRRMA</div>
      <div class="a-section"><h4>Return requested</h4></div>
      <div class="a-fixed-left-grid-col a-col-right">
        <a href="/gp/product/B0EXAMPLE3">Replacement Power Cord</a>
        <span>$11.95</span>
      </div>
      <a href="/spr/returns/cart?orderId=112-5555555-6666666">View return/refund status</a>
    </div></div>
  </div>
</body></html>`

func TestParseReturnHistory_MapsAmazonReturnRecords(t *testing.T) {
	base, err := url.Parse("https://www.amazon.com")
	require.NoError(t, err)

	returns, err := parseReturnHistory(strings.NewReader(returnHistoryFixture), base)

	require.NoError(t, err)
	require.Len(t, returns, 3)
	assert.Equal(t, ReturnRecord{
		OrderID:        "112-1111111-2222222",
		RMAID:          "D8ExampleRRMA",
		CreatedAt:      time.Date(2026, time.June, 29, 0, 0, 0, 0, time.UTC),
		RefundAmount:   11.36,
		HasRefundTotal: true,
		Status:         "Return in transit",
		Items: []ReturnedItem{{
			ASIN:  "B0EXAMPLE1",
			Name:  "Insulated Sporty Cup",
			Price: 10.72,
		}},
		statusURL: "https://www.amazon.com/spr/returns/cart?orderId=112-1111111-2222222&returnSessionId=session",
	}, returns[0])
	assert.Equal(t, "Refund issued", returns[1].Status)
	assert.Equal(t, 165.95, returns[1].RefundAmount)
	assert.Equal(t, "B0EXAMPLE2", returns[1].Items[0].ASIN)
	assert.Equal(t, "Caterpillar Play Tunnel", returns[1].Items[0].Name)
	assert.False(t, returns[2].HasRefundTotal)
	assert.Zero(t, returns[2].RefundAmount)
	assert.Equal(t, "Return requested", returns[2].Status)
}

func TestReturnHistoryClient_FetchesWithSavedAmazonCookies(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies-wife.json")
	data := `{"cookies":[{"name":"session-id","value":"saved-session","domain":".amazon.com","path":"/"}],"updated_at":"2026-07-16T00:00:00Z"}`
	require.NoError(t, os.WriteFile(cookieFile, []byte(data), 0o600))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session-id")
		require.NoError(t, err)
		assert.Equal(t, "saved-session", cookie.Value)
		if r.URL.Path == "/spr/returns/cart" {
			amount := "$11.36"
			if r.URL.Query().Get("orderId") == "112-3333333-4444444" {
				amount = "$165.95"
			}
			_, _ = fmt.Fprintf(w, `<html><body><div>%s refund issued on Jul 3, 2026</div></body></html>`, amount)
			return
		}
		_, _ = w.Write([]byte(returnHistoryFixture))
	}))
	t.Cleanup(server.Close)

	client := &returnHistoryClient{
		httpClient: &http.Client{Timeout: time.Second},
		cookieFile: cookieFile,
		historyURL: server.URL,
	}
	returns, err := client.Fetch(context.Background())

	require.NoError(t, err)
	assert.Len(t, returns, 3)
	require.NotNil(t, returns[0].RefundIssuedAt)
	assert.Equal(t, time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC), *returns[0].RefundIssuedAt)
	require.NotNil(t, returns[1].RefundIssuedAt)
	assert.Equal(t, time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC), *returns[1].RefundIssuedAt)
	assert.Nil(t, returns[2].RefundIssuedAt)
}

func TestParseRefundIssuedAtRequiresMatchingRefundAmount(t *testing.T) {
	body := []byte(`<div><span>$14.41</span><span> refund issued on </span><span>Jul 3, 2026</span></div>
		<div><span>$11.36</span><span> refund issued on </span><span>Jul 5, 2026</span></div>`)

	issuedAt, ok := parseRefundIssuedAt(body, 11.36)

	require.True(t, ok)
	assert.Equal(t, time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC), issuedAt)
}

func TestNewRefundOrderUsesIssuedDateAndSoleRefundItem(t *testing.T) {
	issuedAt := time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC)
	record := ReturnRecord{
		OrderID:        "112-1111111-2222222",
		RMAID:          "D8ExampleRRMA",
		RefundAmount:   11.36,
		HasRefundTotal: true,
		RefundIssuedAt: &issuedAt,
		Items:          []ReturnedItem{{ASIN: "B0EXAMPLE1", Name: "Insulated Sporty Cup", Price: 10.72}},
	}

	order := NewRefundOrder(record)

	assert.Equal(t, "amazon-refund:D8ExampleRRMA", order.GetID())
	assert.Equal(t, issuedAt, order.GetDate())
	assert.Equal(t, -11.36, order.GetTotal())
	assert.Equal(t, 11.36, order.GetSubtotal())
	assert.Zero(t, order.GetTax())
	assert.Zero(t, order.GetTip())
	assert.Zero(t, order.GetFees())
	assert.Equal(t, "Amazon", order.GetProviderName())
	assert.Equal(t, record, order.GetRawData())
	assert.Equal(t, record, order.Record())
	require.Len(t, order.GetItems(), 1)
	item := order.GetItems()[0]
	assert.Equal(t, "Insulated Sporty Cup", item.GetName())
	assert.Equal(t, 11.36, item.GetPrice())
	assert.Equal(t, 1.0, item.GetQuantity())
	assert.Equal(t, 11.36, item.GetUnitPrice())
	assert.Empty(t, item.GetDescription())
	assert.Equal(t, "B0EXAMPLE1", item.GetSKU())
	assert.Empty(t, item.GetCategory())

	createdAt := time.Date(2026, time.June, 29, 0, 0, 0, 0, time.UTC)
	multiple := NewRefundOrder(ReturnRecord{
		CreatedAt: createdAt,
		Items: []ReturnedItem{
			{ASIN: "one", Name: "One", Price: 3.25},
			{ASIN: "two", Name: "Two", Price: 4.75},
		},
	})
	assert.Equal(t, createdAt, multiple.GetDate())
	assert.Equal(t, 3.25, multiple.GetItems()[0].GetPrice())
}

func TestReturnHistoryClientConstructionAndCookieResolution(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "cookies.json")
	provider := &Provider{cookieFile: explicit}

	resolved, err := provider.resolvedCookieFile()
	require.NoError(t, err)
	assert.Equal(t, explicit, resolved)

	client := newReturnHistoryClient(explicit)
	assert.Equal(t, explicit, client.cookieFile)
	assert.Equal(t, amazonReturnHistoryURL, client.historyURL)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)

	home := t.TempDir()
	t.Setenv("HOME", home)
	provider = &Provider{profile: "wife"}
	resolved, err = provider.resolvedCookieFile()
	require.NoError(t, err)
	assert.Contains(t, resolved, "wife")

	provider = &Provider{}
	resolved, err = provider.resolvedCookieFile()
	require.NoError(t, err)
	assert.NotEmpty(t, resolved)
}

func TestReturnHistoryClient_UsesBrowserFingerprintAndSkipsExpiredCookies(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies-wife.json")
	stored := amazongo.CookieFile{Cookies: []*amazongo.Cookie{
		{Name: "session-id", Value: "active", Domain: ".amazon.com", Path: "/"},
		{Name: "ak_bmsc", Value: "expired", Domain: ".amazon.com", Path: "/", Expires: time.Now().Add(-time.Hour).Unix()},
	}}
	data, err := jsonMarshal(stored)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cookieFile, data, 0o600))

	const browserUserAgent = "Mozilla/5.0 Chrome/143.0.7499.4 Safari/537.36"
	var receivedUserAgent string
	var receivedSecCHUA string
	var receivedSecCHUAMobile string
	var receivedSecCHUAPlatform string
	var receivedCookies string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUserAgent = r.UserAgent()
		receivedSecCHUA = r.Header.Get("Sec-CH-UA")
		receivedSecCHUAMobile = r.Header.Get("Sec-CH-UA-Mobile")
		receivedSecCHUAPlatform = r.Header.Get("Sec-CH-UA-Platform")
		receivedCookies = r.Header.Get("Cookie")
		_, _ = w.Write([]byte(`<html><title>Online Return Center</title><body></body></html>`))
	}))
	t.Cleanup(server.Close)

	fingerprint := &amazongo.BrowserFingerprint{
		UserAgent:       browserUserAgent,
		SecCHUA:         `"Chromium";v="143", "Not A(Brand";v="24"`,
		SecCHUAMobile:   "?0",
		SecCHUAPlatform: `"macOS"`,
	}
	client := newReturnHistoryClient(cookieFile, fingerprint)
	client.httpClient = server.Client()
	client.historyURL = server.URL
	_, err = client.Fetch(context.Background())

	require.NoError(t, err)
	assert.Equal(t, browserUserAgent, receivedUserAgent)
	assert.Equal(t, fingerprint.SecCHUA, receivedSecCHUA)
	assert.Equal(t, fingerprint.SecCHUAMobile, receivedSecCHUAMobile)
	assert.Equal(t, fingerprint.SecCHUAPlatform, receivedSecCHUAPlatform)
	assert.Contains(t, receivedCookies, "session-id=active")
	assert.NotContains(t, receivedCookies, "ak_bmsc=")
}

func TestReturnHistoryClient_RejectsAmazonSignInPage(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies-wife.json")
	stored := amazongo.CookieFile{Cookies: []*amazongo.Cookie{{Name: "session-id", Value: "expired", Domain: ".amazon.com", Path: "/"}}}
	data, err := jsonMarshal(stored)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cookieFile, data, 0o600))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Amazon Sign-In</title><input id="ap_email"></html>`))
	}))
	t.Cleanup(server.Close)

	client := &returnHistoryClient{httpClient: server.Client(), cookieFile: cookieFile, historyURL: server.URL}
	_, err = client.Fetch(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "return-center authentication required")
}

func TestNormalizeReturnCookieValueSecuresAndUnquotesBrowserCookie(t *testing.T) {
	cookie := &http.Cookie{Name: "browser-cookie", Value: `"saved-session"`}

	normalizeReturnCookieValue(cookie)

	assert.Equal(t, "saved-session", cookie.Value)
	assert.True(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestReturnHistoryClientReportsRequestFailures(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte(`{"cookies":[]}`), 0o600))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	client := &returnHistoryClient{httpClient: server.Client(), cookieFile: cookieFile, historyURL: server.URL}
	_, err := client.Fetch(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "return-history request failed")
	assert.Contains(t, err.Error(), "status 503")

	client.historyURL = "://bad-url"
	_, _, err = client.fetchPage(context.Background(), client.historyURL, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestReturnHistoryClientRejectsSignInOnReturnDetail(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte(`{"cookies":[]}`), 0o600))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/spr/returns/cart" {
			_, _ = w.Write([]byte(`<html><input id="ap_email"><input id="ap_password"></html>`))
			return
		}
		_, _ = w.Write([]byte(returnHistoryFixture))
	}))
	t.Cleanup(server.Close)

	client := &returnHistoryClient{httpClient: server.Client(), cookieFile: cookieFile, historyURL: server.URL}
	_, err := client.Fetch(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication required while reading RMA D8ExampleRRMA")
}

func TestReturnParsingRejectsIncompleteRecordsAndHandlesURLVariants(t *testing.T) {
	returns, err := parseReturnHistory(strings.NewReader(`<div class="a-box-group a-spacing-extra-large">
		<div>RETURN CREATED Not a date</div><div>ORDER # 112-1111111-2222222</div>
		<div class="a-fixed-left-grid-col a-col-right"><a href="/gp/product/B0BADDATE">Item</a></div>
	</div><div class="a-box-group a-spacing-extra-large"><div>RETURN CREATED Jul 1, 2026</div></div>`), nil)
	require.NoError(t, err)
	assert.Empty(t, returns)

	assert.Equal(t, "https://example.com/detail", resolveReturnURL(nil, "https://example.com/detail"))
	assert.Empty(t, resolveReturnURL(nil, "://bad-url"))
	assert.Nil(t, responseURL(nil))
	assert.False(t, isReturnSignInPage(nil, []byte("ordinary return page")))
	assert.True(t, isReturnSignInPage(&url.URL{Path: "/ap/signin"}, nil))

	normalizeReturnCookieValue(nil)
	plain := &http.Cookie{Name: "plain", Value: "value"}
	normalizeReturnCookieValue(plain)
	assert.Equal(t, "value", plain.Value)
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func TestRefundOrderUsesCardPortionWhenRefundWasSplitAcrossTenders(t *testing.T) {
	issuedAt := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC)
	record := ReturnRecord{
		OrderID:        "112-1111111-2222222",
		RMAID:          "DsplitRRMA",
		RefundAmount:   42.39,
		HasRefundTotal: true,
		RefundIssuedAt: &issuedAt,
		Items:          []ReturnedItem{{ASIN: "B0EXAMPLE1", Name: "Tumbler", Price: 39.99}},
		RefundTenders: []RefundTender{
			{Date: issuedAt, Amount: 31.24, Last4: "4242", Description: "Prime Visa ****4242"},
			{Date: issuedAt, Amount: 11.15, Description: "Amazon Visa points"},
		},
	}

	assert.InDelta(t, 31.24, record.BankRefundAmount(), 0.001)

	order := NewRefundOrder(record)
	assert.InDelta(t, -31.24, order.GetTotal(), 0.001)
	assert.InDelta(t, 31.24, order.GetSubtotal(), 0.001)
	require.Len(t, order.GetItems(), 1)
	assert.InDelta(t, 31.24, order.GetItems()[0].GetPrice(), 0.001)
	assert.InDelta(t, 31.24, order.GetItems()[0].GetUnitPrice(), 0.001)

	pointsOnly := ReturnRecord{RefundAmount: 11.15, HasRefundTotal: true, RefundTenders: []RefundTender{{Amount: 11.15, Description: "Amazon Visa points"}}}
	assert.Zero(t, pointsOnly.BankRefundAmount())

	withoutBreakdown := ReturnRecord{RefundAmount: 11.36, HasRefundTotal: true}
	assert.InDelta(t, 11.36, withoutBreakdown.BankRefundAmount(), 0.001)
}

func TestAttachRefundTendersAssignsOrderRefundRowsToReturns(t *testing.T) {
	oct3 := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC)
	oct5 := time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC)
	refundRow := func(date time.Time, amount float64, last4 string) *amazongo.Transaction {
		method := "Amazon Visa points"
		if last4 != "" {
			method = "Prime Visa ****" + last4
		}
		return &amazongo.Transaction{Date: date, Amount: amount, LastFour: last4, PaymentMethod: method, IsRefund: true}
	}
	charge := func(date time.Time, amount float64, last4 string) *amazongo.Transaction {
		return &amazongo.Transaction{Date: date, Amount: amount, LastFour: last4}
	}
	client := &fakeAmazonClient{transactionsByOrderID: map[string][]*amazongo.Transaction{
		"split-order": {
			refundRow(oct3, 31.24, "4242"), refundRow(oct3, 11.15, ""),
			charge(oct3.AddDate(0, 0, -5), 31.24, "4242"), charge(oct3.AddDate(0, 0, -6), 11.15, ""),
		},
		// The refund row is dated a day after the Return Center's issued date.
		"late-row-order": {refundRow(oct3.AddDate(0, 0, 1), 20.00, "4242")},
		"two-returns-order": {
			refundRow(oct3, 10.00, "4242"),
			refundRow(oct5, 15.00, "4242"), refundRow(oct5, 5.00, ""),
		},
		"same-day-order":  {refundRow(oct3, 30.00, "4242"), refundRow(oct3, 10.00, "")},
		"mismatch-order":  {refundRow(oct3, 7.00, "4242")},
		"no-refund-order": {charge(oct3, 9.00, "4242")},
	}}
	provider := NewProvider(nil, nil)
	provider.client = client
	record := func(orderID, rma string, amount float64, issued *time.Time) ReturnRecord {
		return ReturnRecord{OrderID: orderID, RMAID: rma, RefundAmount: amount, HasRefundTotal: issued != nil, RefundIssuedAt: issued}
	}
	returns := []ReturnRecord{
		record("split-order", "split", 42.39, &oct3),
		record("late-row-order", "late", 20.00, &oct3),
		record("two-returns-order", "first", 10.00, &oct3),
		record("two-returns-order", "second", 20.00, &oct5),
		record("same-day-order", "same-a", 20.00, &oct3),
		record("same-day-order", "same-b", 20.00, &oct3),
		record("mismatch-order", "mismatch", 9.00, &oct3),
		record("no-refund-order", "none", 9.00, &oct3),
		record("pending-order", "pending", 0, nil),
		record("failing-order", "failing", 5.00, &oct3),
	}

	provider.attachRefundTenders(context.Background(), returns)

	byRMA := map[string]ReturnRecord{}
	for _, r := range returns {
		byRMA[r.RMAID] = r
	}
	require.Len(t, byRMA["split"].RefundTenders, 2)
	assert.Equal(t, RefundTender{Date: oct3, Amount: 31.24, Last4: "4242", Description: "Prime Visa ****4242"}, byRMA["split"].RefundTenders[0])
	assert.InDelta(t, 31.24, byRMA["split"].BankRefundAmount(), 0.001)
	assert.InDelta(t, 20.00, byRMA["late"].BankRefundAmount(), 0.001)
	require.Len(t, byRMA["late"].RefundTenders, 1)
	assert.InDelta(t, 10.00, byRMA["first"].BankRefundAmount(), 0.001)
	require.Len(t, byRMA["first"].RefundTenders, 1)
	assert.InDelta(t, 15.00, byRMA["second"].BankRefundAmount(), 0.001)
	require.Len(t, byRMA["second"].RefundTenders, 2)
	// Two returns refunded the same day cannot be told apart, and a breakdown
	// that disagrees with the Return Center total is not trusted.
	for _, rma := range []string{"same-a", "same-b", "mismatch", "none", "pending", "failing"} {
		assert.Empty(t, byRMA[rma].RefundTenders, rma)
	}
	assert.InDelta(t, 9.00, byRMA["mismatch"].BankRefundAmount(), 0.001)
	// One request per order, and none for returns without an issued refund.
	assert.Equal(t, map[string]int{
		"split-order": 1, "late-row-order": 1, "two-returns-order": 1, "same-day-order": 1,
		"mismatch-order": 1, "no-refund-order": 1, "failing-order": 1,
	}, client.transactionFetches)
}
