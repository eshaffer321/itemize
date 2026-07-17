package amazon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	amazongo "github.com/eshaffer321/amazon-go"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

const (
	amazonReturnHistoryURL = "https://www.amazon.com/spr/returns/history"
	amazonReturnUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var (
	returnOrderIDPattern = regexp.MustCompile(`\bORDER\s*#\s*(\d{3}-\d{7}-\d{7})\b`)
	returnRMAIDPattern   = regexp.MustCompile(`\bRMA\s+ID\s*:\s*([A-Za-z0-9]+RRMA)\b`)
	returnCreatedPattern = regexp.MustCompile(`\bRETURN\s+CREATED\s+([A-Z][a-z]{2}\s+\d{1,2},\s+\d{4})\b`)
	refundTotalPattern   = regexp.MustCompile(`\bREFUND\s+TOTAL\s+\$([\d,]+\.\d{2})\b`)
	refundIssuedPattern  = regexp.MustCompile(`(?i)\$([\d,]+\.\d{2})\s+refund issued on\s+([A-Z][a-z]{2}\s+\d{1,2},\s+\d{4})`)
	returnPricePattern   = regexp.MustCompile(`\$([\d,]+\.\d{2})`)
	returnASINPattern    = regexp.MustCompile(`/gp/product/([A-Z0-9]+)`)
)

// ReturnRecord is one return event reported by Amazon's Return Center.
// RefundAmount is authoritative only when HasRefundTotal is true.
type ReturnRecord struct {
	OrderID        string         `json:"order_id"`
	RMAID          string         `json:"rma_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	RefundAmount   float64        `json:"refund_amount,omitempty"`
	HasRefundTotal bool           `json:"has_refund_total"`
	RefundIssuedAt *time.Time     `json:"refund_issued_at,omitempty"`
	Status         string         `json:"status"`
	Items          []ReturnedItem `json:"items"`
	statusURL      string
}

// ReturnedItem identifies an item included in an Amazon return event.
type ReturnedItem struct {
	ASIN  string  `json:"asin"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// RefundOrder adapts one authoritative Amazon return record to the generic
// order interface used by categorization and matching.
type RefundOrder struct {
	record ReturnRecord
}

// NewRefundOrder creates a read-only order view for a return record.
func NewRefundOrder(record ReturnRecord) *RefundOrder {
	return &RefundOrder{record: record}
}

func (o *RefundOrder) GetID() string { return "amazon-refund:" + o.record.RMAID }
func (o *RefundOrder) GetDate() time.Time {
	if o.record.RefundIssuedAt != nil {
		return *o.record.RefundIssuedAt
	}
	return o.record.CreatedAt
}
func (o *RefundOrder) GetTotal() float64       { return -o.record.RefundAmount }
func (o *RefundOrder) GetSubtotal() float64    { return o.record.RefundAmount }
func (o *RefundOrder) GetTax() float64         { return 0 }
func (o *RefundOrder) GetTip() float64         { return 0 }
func (o *RefundOrder) GetFees() float64        { return 0 }
func (o *RefundOrder) GetProviderName() string { return "Amazon" }
func (o *RefundOrder) GetRawData() interface{} { return o.record }
func (o *RefundOrder) Record() ReturnRecord    { return o.record }
func (o *RefundOrder) GetItems() []providers.OrderItem {
	items := make([]providers.OrderItem, 0, len(o.record.Items))
	for _, item := range o.record.Items {
		price := item.Price
		if len(o.record.Items) == 1 && o.record.HasRefundTotal {
			price = o.record.RefundAmount
		}
		items = append(items, refundOrderItem{item: item, price: price})
	}
	return items
}

type refundOrderItem struct {
	item  ReturnedItem
	price float64
}

func (i refundOrderItem) GetName() string        { return i.item.Name }
func (i refundOrderItem) GetPrice() float64      { return i.price }
func (i refundOrderItem) GetQuantity() float64   { return 1 }
func (i refundOrderItem) GetUnitPrice() float64  { return i.price }
func (i refundOrderItem) GetDescription() string { return "" }
func (i refundOrderItem) GetSKU() string         { return i.item.ASIN }
func (i refundOrderItem) GetCategory() string    { return "" }

type returnHistoryClient struct {
	httpClient *http.Client
	cookieFile string
	historyURL string
}

func newReturnHistoryClient(cookieFile string) *returnHistoryClient {
	return &returnHistoryClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cookieFile: cookieFile,
		historyURL: amazonReturnHistoryURL,
	}
}

// FetchReturns reads Amazon's Return Center without mutating the saved cookie file.
func (p *Provider) FetchReturns(ctx context.Context) ([]ReturnRecord, error) {
	cookieFile, err := p.resolvedCookieFile()
	if err != nil {
		return nil, err
	}
	returns, err := newReturnHistoryClient(cookieFile).Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Amazon returns: %w", err)
	}
	return returns, nil
}

func (p *Provider) resolvedCookieFile() (string, error) {
	if p.cookieFile != "" {
		return p.cookieFile, nil
	}
	if p.profile != "" {
		path, err := amazongo.CookiePathForAccount(p.profile)
		if err != nil {
			return "", fmt.Errorf("failed to resolve Amazon account cookie file: %w", err)
		}
		return path, nil
	}
	path, err := amazongo.DefaultCookiePath()
	if err != nil {
		return "", fmt.Errorf("failed to resolve default Amazon cookie file: %w", err)
	}
	return path, nil
}

func (c *returnHistoryClient) Fetch(ctx context.Context) ([]ReturnRecord, error) {
	store, err := amazongo.NewCookieStore(c.cookieFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load Amazon cookies: %w", err)
	}

	cookies := store.ToHTTPCookies()
	for _, cookie := range cookies {
		normalizeReturnCookieValue(cookie)
	}
	body, finalURL, err := c.fetchPage(ctx, c.historyURL, cookies)
	if err != nil {
		return nil, fmt.Errorf("amazon return-history request failed: %w", err)
	}
	if isReturnSignInPage(finalURL, body) {
		return nil, fmt.Errorf("return-center authentication required: open Amazon's Return Center with the account browser profile and sign in again")
	}

	baseURL, err := url.Parse(c.historyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Amazon return-history URL: %w", err)
	}
	returns, err := parseReturnHistory(bytes.NewReader(body), baseURL)
	if err != nil {
		return nil, err
	}
	for i := range returns {
		if !returns[i].HasRefundTotal || returns[i].statusURL == "" {
			continue
		}
		detail, detailURL, detailErr := c.fetchPage(ctx, returns[i].statusURL, cookies)
		if detailErr != nil {
			return nil, fmt.Errorf("amazon return status request failed for RMA %s: %w", returns[i].RMAID, detailErr)
		}
		if isReturnSignInPage(detailURL, detail) {
			return nil, fmt.Errorf("return-center authentication required while reading RMA %s", returns[i].RMAID)
		}
		if issuedAt, ok := parseRefundIssuedAt(detail, returns[i].RefundAmount); ok {
			returns[i].RefundIssuedAt = &issuedAt
		}
	}
	return returns, nil
}

func (c *returnHistoryClient) fetchPage(ctx context.Context, target string, cookies []*http.Cookie) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("User-Agent", amazonReturnUserAgent)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, responseURL(resp), fmt.Errorf("returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, responseURL(resp), fmt.Errorf("failed to read response: %w", err)
	}
	return body, responseURL(resp), nil
}

func responseURL(resp *http.Response) *url.URL {
	if resp != nil && resp.Request != nil {
		return resp.Request.URL
	}
	return nil
}

func parseReturnHistory(reader io.Reader, baseURL *url.URL) ([]ReturnRecord, error) {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Amazon return history: %w", err)
	}

	returns := make([]ReturnRecord, 0)
	doc.Find(".a-box-group.a-spacing-extra-large").Each(func(_ int, group *goquery.Selection) {
		text := normalizedReturnText(group.Text())
		orderMatch := returnOrderIDPattern.FindStringSubmatch(text)
		createdMatch := returnCreatedPattern.FindStringSubmatch(text)
		if len(orderMatch) != 2 || len(createdMatch) != 2 {
			return
		}

		createdAt, parseErr := time.ParseInLocation("Jan 2, 2006", createdMatch[1], time.UTC)
		if parseErr != nil {
			return
		}

		record := ReturnRecord{
			OrderID:   orderMatch[1],
			CreatedAt: createdAt,
			Status:    normalizedReturnText(group.Find("h4").First().Text()),
			Items:     parseReturnedItems(group),
		}
		if len(record.Items) == 0 {
			return
		}
		if rmaMatch := returnRMAIDPattern.FindStringSubmatch(text); len(rmaMatch) == 2 {
			record.RMAID = rmaMatch[1]
		}
		if refundMatch := refundTotalPattern.FindStringSubmatch(text); len(refundMatch) == 2 {
			if amount, amountErr := parseReturnAmount(refundMatch[1]); amountErr == nil {
				record.RefundAmount = amount
				record.HasRefundTotal = true
			}
		}
		group.Find("a").EachWithBreak(func(_ int, link *goquery.Selection) bool {
			if !strings.EqualFold(normalizedReturnText(link.Text()), "View return/refund status") {
				return true
			}
			href, exists := link.Attr("href")
			if exists {
				record.statusURL = resolveReturnURL(baseURL, href)
			}
			return false
		})
		returns = append(returns, record)
	})

	return returns, nil
}

func parseReturnedItems(group *goquery.Selection) []ReturnedItem {
	items := make([]ReturnedItem, 0)
	seen := make(map[string]bool)
	group.Find(`a[href*="/gp/product/"]`).Each(func(_ int, link *goquery.Selection) {
		href, exists := link.Attr("href")
		if !exists {
			return
		}
		asinMatch := returnASINPattern.FindStringSubmatch(href)
		if len(asinMatch) != 2 || seen[asinMatch[1]] {
			return
		}
		name := normalizedReturnText(link.Text())
		if name == "" {
			return
		}
		item := ReturnedItem{ASIN: asinMatch[1], Name: name}
		itemText := normalizedReturnText(link.Closest(".a-fixed-left-grid-col.a-col-right").Text())
		priceMatches := returnPricePattern.FindAllStringSubmatch(itemText, -1)
		if len(priceMatches) > 0 {
			item.Price, _ = parseReturnAmount(priceMatches[len(priceMatches)-1][1])
		}
		seen[item.ASIN] = true
		items = append(items, item)
	})
	return items
}

func parseReturnAmount(value string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
}

func parseRefundIssuedAt(body []byte, refundAmount float64) (time.Time, bool) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return time.Time{}, false
	}
	text := normalizedReturnText(doc.Text())
	for _, match := range refundIssuedPattern.FindAllStringSubmatch(text, -1) {
		if len(match) != 3 {
			continue
		}
		amount, err := parseReturnAmount(match[1])
		if err != nil || math.Abs(amount-refundAmount) > 0.001 {
			continue
		}
		issuedAt, err := time.ParseInLocation("Jan 2, 2006", match[2], time.UTC)
		if err == nil {
			return issuedAt, true
		}
	}
	return time.Time{}, false
}

func resolveReturnURL(baseURL *url.URL, href string) string {
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if baseURL == nil {
		return parsed.String()
	}
	return baseURL.ResolveReference(parsed).String()
}

func normalizeReturnCookieValue(cookie *http.Cookie) {
	if cookie == nil {
		return
	}
	// These cookies are only sent back to Amazon over HTTPS. Mark the imported
	// browser values with conservative attributes before attaching them to a
	// request; AddCookie serializes only the name and value.
	cookie.Secure = true
	cookie.HttpOnly = true
	cookie.SameSite = http.SameSiteLaxMode
	if len(cookie.Value) < 2 || cookie.Value[0] != '"' || cookie.Value[len(cookie.Value)-1] != '"' {
		return
	}
	unquoted, err := strconv.Unquote(cookie.Value)
	if err == nil {
		cookie.Value = unquoted
	}
}

func normalizedReturnText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func isReturnSignInPage(finalURL *url.URL, body []byte) bool {
	if finalURL != nil && strings.Contains(finalURL.Path, "/ap/signin") {
		return true
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "amazon sign-in") ||
		(strings.Contains(lower, "ap_email") && strings.Contains(lower, "ap_password"))
}
