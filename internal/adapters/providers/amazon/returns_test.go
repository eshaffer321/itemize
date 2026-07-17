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
	require.Len(t, order.GetItems(), 1)
	assert.Equal(t, "Insulated Sporty Cup", order.GetItems()[0].GetName())
	assert.Equal(t, 11.36, order.GetItems()[0].GetPrice())
	assert.Equal(t, "B0EXAMPLE1", order.GetItems()[0].GetSKU())
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

func TestNormalizeReturnCookieValueUnquotesBrowserCookie(t *testing.T) {
	cookie := &http.Cookie{Name: "browser-cookie", Value: `"saved-session"`}

	normalizeReturnCookieValue(cookie)

	assert.Equal(t, "saved-session", cookie.Value)
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}
