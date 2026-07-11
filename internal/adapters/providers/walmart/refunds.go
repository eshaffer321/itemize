package walmart

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

type refundItemFetcher func(context.Context, string, bool) ([]providers.OrderItem, error)

type cookieFile struct {
	Cookies map[string]struct {
		Value string `json:"value"`
	} `json:"cookies"`
}

// newRefundItemFetcher reads the item-level returnId fields that Walmart's Go
// client currently omits from its typed Order model.
func newRefundItemFetcher() refundItemFetcher {
	home, err := os.UserHomeDir()
	if err != nil {
		return func(context.Context, string, bool) ([]providers.OrderItem, error) {
			return nil, fmt.Errorf("locating Walmart cookie file: %w", err)
		}
	}
	cookiePath := filepath.Join(home, ".walmart-api", "cookies.json")

	return func(ctx context.Context, orderID string, isInStore bool) ([]providers.OrderItem, error) {
		cookies, err := loadCookies(cookiePath)
		if err != nil {
			return nil, err
		}

		variables, err := json.Marshal(map[string]interface{}{
			"orderId": orderID, "orderIsInStore": isInStore, "clickThroughGroupId": "0",
			"enableIsWcpOrder": false, "enabledFeatures": []string{"csat-northstar-v1", "tips", "delivery-fees"},
			"enableSignOnDelivery": true, "includeTipDetails": true, "includeFeesDetails": true,
		})
		if err != nil {
			return nil, fmt.Errorf("encoding Walmart order request: %w", err)
		}
		endpoint, err := url.Parse("https://www.walmart.com/orchestra/orders/graphql/getOrder/d0622497daef19150438d07c506739d451cad6749cf45c3b4db95f2f5a0a65c4")
		if err != nil {
			return nil, fmt.Errorf("parsing Walmart order endpoint: %w", err)
		}
		query := endpoint.Query()
		query.Set("variables", string(variables))
		endpoint.RawQuery = query.Encode()

		client := &http.Client{Timeout: 30 * time.Second}
		for attempt := 0; attempt < 3; attempt++ {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
			if err != nil {
				return nil, fmt.Errorf("creating Walmart refund-item request: %w", err)
			}
			setRefundItemHeaders(req, cookies)

			resp, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("fetching Walmart refund items: %w", err)
			}
			if resp.StatusCode == http.StatusTooManyRequests && attempt < 2 {
				resp.Body.Close()
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(2 * time.Second):
					continue
				}
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return nil, fmt.Errorf("fetching Walmart refund items: HTTP %d", resp.StatusCode)
			}
			var payload json.RawMessage
			decodeErr := json.NewDecoder(resp.Body).Decode(&payload)
			resp.Body.Close()
			if decodeErr != nil {
				return nil, fmt.Errorf("decoding Walmart refund items: %w", decodeErr)
			}
			return findReturnedItems(payload), nil
		}
		return nil, fmt.Errorf("fetching Walmart refund items: rate limit retries exhausted")
	}
}

func setRefundItemHeaders(req *http.Request, cookies string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")
	req.Header.Set("X-Apollo-Operation-Name", "getOrder")
	req.Header.Set("X-O-Gql-Query", "query getOrder")
	req.Header.Set("X-O-Platform", "rweb")
	req.Header.Set("X-O-Bu", "WALMART-US")
	req.Header.Set("X-O-Mart", "B2C")
	req.Header.Set("X-O-Segment", "oaoh")
	correlationID := fmt.Sprintf("walmart-go-%d", time.Now().Unix())
	req.Header.Set("X-O-Correlation-Id", correlationID)
	req.Header.Set("Wm-Qos.Correlation_Id", correlationID)
	req.Header.Set("Wm-Mp", "true")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Dnt", "1")
	req.Header.Set("X-O-Platform-Version", "usweb-1.221.0")
	req.Header.Set("X-Enable-Server-Timing", "1")
	req.Header.Set("X-Latency-Trace", "1")
	req.Header.Set("Cookie", cookies)
}

func loadCookies(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading Walmart cookies: %w", err)
	}
	var parsed cookieFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decoding Walmart cookies: %w", err)
	}
	names := make([]string, 0, len(parsed.Cookies))
	for name := range parsed.Cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	pairs := make([]string, 0, len(names))
	for _, name := range names {
		pairs = append(pairs, name+"="+parsed.Cookies[name].Value)
	}
	return strings.Join(pairs, "; "), nil
}

func findReturnedItems(payload json.RawMessage) []providers.OrderItem {
	seen := make(map[string]bool)
	var items []providers.OrderItem
	var walk func(json.RawMessage)
	walk = func(value json.RawMessage) {
		var object map[string]json.RawMessage
		if json.Unmarshal(value, &object) == nil {
			if returnedItem, ok := parseReturnedItem(object); ok && !seen[returnedItem.key] {
				seen[returnedItem.key] = true
				items = append(items, returnedItem)
			}
			for _, child := range object {
				walk(child)
			}
			return
		}
		var array []json.RawMessage
		if json.Unmarshal(value, &array) == nil {
			for _, child := range array {
				walk(child)
			}
		}
	}
	walk(payload)
	return items
}

type returnedItem struct {
	key, name, sku  string
	price, quantity float64
}

func (i returnedItem) GetName() string      { return i.name }
func (i returnedItem) GetPrice() float64    { return i.price }
func (i returnedItem) GetQuantity() float64 { return i.quantity }
func (i returnedItem) GetUnitPrice() float64 {
	if i.quantity == 0 {
		return i.price
	}
	return i.price / i.quantity
}
func (i returnedItem) GetDescription() string { return i.name }
func (i returnedItem) GetSKU() string         { return i.sku }
func (i returnedItem) GetCategory() string    { return "" }

func parseReturnedItem(object map[string]json.RawMessage) (returnedItem, bool) {
	var returnID string
	if err := json.Unmarshal(object["returnId"], &returnID); err != nil || returnID == "" {
		return returnedItem{}, false
	}
	var product struct {
		Name string `json:"name"`
		SKU  string `json:"usItemId"`
	}
	var price struct {
		LinePrice struct {
			Value float64 `json:"value"`
		} `json:"linePrice"`
	}
	var quantity float64
	if json.Unmarshal(object["productInfo"], &product) != nil || json.Unmarshal(object["priceInfo"], &price) != nil || product.Name == "" {
		return returnedItem{}, false
	}
	_ = json.Unmarshal(object["quantity"], &quantity)
	return returnedItem{key: returnID + ":" + product.SKU, name: product.Name, sku: product.SKU, price: price.LinePrice.Value, quantity: quantity}, true
}
