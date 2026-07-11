package walmart

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRefundItemFetcher_FetchesReturnedItem(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cookieDir := filepath.Join(home, ".walmart-api")
	require.NoError(t, os.MkdirAll(cookieDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(cookieDir, "cookies.json"), []byte(`{"cookies":{"auth":{"value":"test-cookie"}}}`), 0600))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-cookie", r.Header.Get("Cookie")[5:])
		assert.Equal(t, "getOrder", r.Header.Get("X-Apollo-Operation-Name"))
		_, err := w.Write([]byte(`{"data":{"order":{"groups_2101":[{"categories":[{"items":[{"returnId":"1","quantity":1,"productInfo":{"name":"Refunded creamer","usItemId":"sku-1"},"priceInfo":{"linePrice":{"value":5.26}}}]}]}]}}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	originalEndpoint := refundItemEndpoint
	refundItemEndpoint = server.URL
	t.Cleanup(func() { refundItemEndpoint = originalEndpoint })

	items, err := newRefundItemFetcher()(context.Background(), "order-1", false)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Refunded creamer", items[0].GetName())
	assert.Equal(t, 1.0, items[0].GetQuantity())
	assert.InDelta(t, 5.26, items[0].GetUnitPrice(), 0.001)
	assert.Equal(t, "Refunded creamer", items[0].GetDescription())
	assert.Empty(t, items[0].GetCategory())
}

func TestFindReturnedItems_UsesWalmartReturnIDAndDeduplicatesUIViews(t *testing.T) {
	payload := []byte(`{
  "data": {"order": {"groups_2101": [{
    "categories": [{"items": [{"returnId": "1", "quantity": 1, "productInfo": {"name": "Chobani Coffee Creamer", "usItemId": "15162760729"}, "priceInfo": {"linePrice": {"value": 5.26}}}]}],
    "subGroups": [{"categories": [{"items": [{"returnId": "1", "quantity": 1, "productInfo": {"name": "Chobani Coffee Creamer", "usItemId": "15162760729"}, "priceInfo": {"linePrice": {"value": 5.26}}}]}]}]
  }]}}
}`)

	items := findReturnedItems(payload)
	require.Len(t, items, 1)
	assert.Equal(t, "Chobani Coffee Creamer", items[0].GetName())
	assert.Equal(t, "15162760729", items[0].GetSKU())
	assert.InDelta(t, 5.26, items[0].GetPrice(), 0.001)
}
