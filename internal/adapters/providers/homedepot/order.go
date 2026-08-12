package homedepot

import (
	"fmt"
	"strings"
	"time"

	hdgo "github.com/fnziman/homedepot-go"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

// Order wraps a Home Depot OrderDetail and its originating OrderSummary
// to implement providers.Order. The summary is retained because in-store
// orders don't populate orderNumber on the detail — the store/transaction
// composite from the summary is used as the stable ID instead.
type Order struct {
	summary hdgo.OrderSummary
	detail  hdgo.OrderDetail
}

// GetID returns a stable unique identifier. Online orders use orderNumber
// directly; in-store orders use a composite of store number and transaction
// ID (both surfaced on the summary but empty on the detail).
func (o *Order) GetID() string {
	if o.detail.OrderNumber != "" {
		return o.detail.OrderNumber
	}
	if o.summary.TransactionID != "" {
		return fmt.Sprintf("hd-instore-%s-%s", o.summary.StoreNumber, o.summary.TransactionID)
	}
	return ""
}

// GetDate parses the ISO-8601 salesDate the API returns. Home Depot has
// been observed to use full RFC3339 timestamps and date-only strings;
// try both.
func (o *Order) GetDate() time.Time {
	raw := o.detail.SalesDate
	if raw == "" {
		raw = o.summary.SalesDate
	}
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (o *Order) GetTotal() float64    { return o.detail.GrandTotalAmount }
func (o *Order) GetSubtotal() float64 { return o.detail.SubTotalAmount }
func (o *Order) GetTax() float64      { return o.detail.TaxTotalAmount }

// GetTip returns 0 — Home Depot doesn't have driver tips.
func (o *Order) GetTip() float64 { return 0 }

// GetFees returns the sum of shipping and delivery charges. Home Depot
// reports these separately on the order detail.
func (o *Order) GetFees() float64 {
	return o.detail.ShippingCharge + o.detail.DeliveryCharge
}

// GetItems flattens line items across every fulfillment group.
func (o *Order) GetItems() []providers.OrderItem {
	all := o.detail.AllLineItems()
	out := make([]providers.OrderItem, 0, len(all))
	for i := range all {
		out = append(out, &OrderItem{item: all[i]})
	}
	return out
}

// GetProviderName is the human-readable provider identifier the domain
// layer uses in log lines and prompts.
func (o *Order) GetProviderName() string { return "Home Depot" }

// GetRawData exposes the underlying detail for handlers that want to peek
// at provider-specific fields.
func (o *Order) GetRawData() interface{} { return o.detail }

// OrderItem wraps a single homedepot-go LineItem to implement
// providers.OrderItem.
type OrderItem struct {
	item hdgo.LineItem
}

// GetName combines brand and description when both are present. Home Depot
// listings frequently split them ("RIDGID" + "18V Cordless Drill"), and
// concatenating gives the categorizer better signal than either alone.
func (i *OrderItem) GetName() string {
	brand := strings.TrimSpace(i.item.BrandName)
	desc := strings.TrimSpace(i.item.Description)
	switch {
	case brand != "" && desc != "":
		return brand + " " + desc
	case desc != "":
		return desc
	default:
		return brand
	}
}

// GetPrice returns the line total.
func (i *OrderItem) GetPrice() float64 { return i.item.TotalPrice }

// GetQuantity returns the purchased quantity (currentQuantity minus
// cancelledQuantity, clamped at zero). Cancellations happen fairly often
// on partial fulfillments; using CurrentQuantity as-is would over-report.
func (i *OrderItem) GetQuantity() float64 { return i.item.PurchasedQuantity() }

// GetUnitPrice returns the per-unit price.
func (i *OrderItem) GetUnitPrice() float64 { return i.item.UnitPrice }

// GetDescription returns the item description.
func (i *OrderItem) GetDescription() string { return i.item.Description }

// GetSKU prefers Home Depot's own thdSku (globally unique on
// homedepot.com) and falls back to skuNumber, then modelNumber.
func (i *OrderItem) GetSKU() string {
	switch {
	case i.item.THDSKU != "":
		return i.item.THDSKU
	case i.item.SKUNumber != "":
		return i.item.SKUNumber
	default:
		return i.item.ModelNumber
	}
}

// GetCategory returns "" — Home Depot's line-item response doesn't carry
// a category we can map cleanly to Monarch. The LLM categorizer does the
// mapping downstream.
func (i *OrderItem) GetCategory() string { return "" }
