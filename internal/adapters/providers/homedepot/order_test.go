package homedepot

import (
	"testing"
	"time"

	hdgo "github.com/fnziman/homedepot-go"
	"github.com/stretchr/testify/assert"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

func TestOrder_GetID_OnlineUsesOrderNumber(t *testing.T) {
	o := &Order{
		summary: hdgo.OrderSummary{OrderNumbers: []string{"WD-0001"}, OrderOrigin: "online"},
		detail:  hdgo.OrderDetail{OrderNumber: "WD-0001"},
	}
	assert.Equal(t, "WD-0001", o.GetID())
}

func TestOrder_GetID_InstoreUsesComposite(t *testing.T) {
	o := &Order{
		summary: hdgo.OrderSummary{StoreNumber: "0100", TransactionID: "TXN-42", OrderOrigin: "instore"},
		detail:  hdgo.OrderDetail{OrderNumber: "", OrderOrigin: "instore"},
	}
	assert.Equal(t, "hd-instore-0100-TXN-42", o.GetID())
}

func TestOrder_GetID_EmptyWhenNoIdentifier(t *testing.T) {
	o := &Order{summary: hdgo.OrderSummary{}, detail: hdgo.OrderDetail{}}
	assert.Empty(t, o.GetID())
}

func TestOrder_GetDate_ParsesMultipleFormats(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"RFC3339 with Z", "2026-07-01T00:00:00Z"},
		{"date only", "2026-07-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &Order{detail: hdgo.OrderDetail{SalesDate: tc.raw}}
			got := o.GetDate()
			assert.False(t, got.IsZero(), "expected valid parse of %q", tc.raw)
			assert.Equal(t, 2026, got.Year())
			assert.Equal(t, time.July, got.Month())
		})
	}
}

func TestOrder_GetDate_FallsBackToSummary(t *testing.T) {
	// Detail salesDate empty but summary has one.
	o := &Order{
		summary: hdgo.OrderSummary{SalesDate: "2026-07-02T00:00:00Z"},
		detail:  hdgo.OrderDetail{SalesDate: ""},
	}
	got := o.GetDate()
	assert.False(t, got.IsZero())
	assert.Equal(t, 2, got.Day())
}

func TestOrder_GetDate_UnparseableReturnsZero(t *testing.T) {
	o := &Order{detail: hdgo.OrderDetail{SalesDate: "not-a-date"}}
	assert.True(t, o.GetDate().IsZero())
}

func TestOrder_Amounts(t *testing.T) {
	o := &Order{detail: hdgo.OrderDetail{
		SubTotalAmount:   39.99,
		TaxTotalAmount:   3.00,
		GrandTotalAmount: 42.99,
		ShippingCharge:   5.00,
		DeliveryCharge:   2.00,
	}}
	assert.InDelta(t, 42.99, o.GetTotal(), 0.001)
	assert.InDelta(t, 39.99, o.GetSubtotal(), 0.001)
	assert.InDelta(t, 3.00, o.GetTax(), 0.001)
	assert.InDelta(t, 0, o.GetTip(), 0.001, "Home Depot doesn't have driver tips")
	assert.InDelta(t, 7.00, o.GetFees(), 0.001, "fees should sum shipping + delivery")
}

func TestOrder_GetItems_FlattensAcrossFulfillmentGroups(t *testing.T) {
	o := &Order{detail: hdgo.OrderDetail{FulfillmentGroups: []hdgo.FulfillmentGroup{
		{LineItems: []hdgo.LineItem{{THDSKU: "1"}, {THDSKU: "2"}}},
		{LineItems: []hdgo.LineItem{{THDSKU: "3"}}},
	}}}
	items := o.GetItems()
	assert.Len(t, items, 3)
	assert.Equal(t, "1", items[0].GetSKU())
	assert.Equal(t, "3", items[2].GetSKU())
}

func TestOrder_GetProviderName(t *testing.T) {
	o := &Order{}
	assert.Equal(t, "Home Depot", o.GetProviderName())
}

func TestOrder_GetRawData(t *testing.T) {
	o := &Order{detail: hdgo.OrderDetail{OrderNumber: "WD-1"}}
	raw, ok := o.GetRawData().(hdgo.OrderDetail)
	assert.True(t, ok, "raw data should be the underlying OrderDetail")
	assert.Equal(t, "WD-1", raw.OrderNumber)
}

func TestOrderItem_GetName_BrandPlusDescription(t *testing.T) {
	cases := []struct {
		name string
		item hdgo.LineItem
		want string
	}{
		{"brand + desc", hdgo.LineItem{BrandName: "RIDGID", Description: "Cordless Drill"}, "RIDGID Cordless Drill"},
		{"desc only", hdgo.LineItem{Description: "Sample Hammer"}, "Sample Hammer"},
		{"brand only", hdgo.LineItem{BrandName: "Kohler"}, "Kohler"},
		{"trims whitespace", hdgo.LineItem{BrandName: "  Milwaukee  ", Description: "  Saw  "}, "Milwaukee Saw"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := &OrderItem{item: tc.item}
			assert.Equal(t, tc.want, item.GetName())
		})
	}
}

func TestOrderItem_GetSKU_PrefersTHDSKU(t *testing.T) {
	cases := []struct {
		name string
		item hdgo.LineItem
		want string
	}{
		{"prefers thdSku", hdgo.LineItem{THDSKU: "T-1", SKUNumber: "S-1", ModelNumber: "M-1"}, "T-1"},
		{"falls back to skuNumber", hdgo.LineItem{SKUNumber: "S-1", ModelNumber: "M-1"}, "S-1"},
		{"falls back to modelNumber", hdgo.LineItem{ModelNumber: "M-1"}, "M-1"},
		{"empty when nothing", hdgo.LineItem{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := &OrderItem{item: tc.item}
			assert.Equal(t, tc.want, item.GetSKU())
		})
	}
}

func TestOrderItem_GetQuantity_HandlesCancellations(t *testing.T) {
	item := &OrderItem{item: hdgo.LineItem{CurrentQuantity: 3, CancelledQuantity: 1}}
	assert.InDelta(t, 2.0, item.GetQuantity(), 0.001)
}

func TestOrderItem_Prices(t *testing.T) {
	item := &OrderItem{item: hdgo.LineItem{UnitPrice: 12.50, TotalPrice: 25.00}}
	assert.InDelta(t, 25.00, item.GetPrice(), 0.001)
	assert.InDelta(t, 12.50, item.GetUnitPrice(), 0.001)
}

func TestOrderItem_GetDescription(t *testing.T) {
	item := &OrderItem{item: hdgo.LineItem{Description: "Sample Paint"}}
	assert.Equal(t, "Sample Paint", item.GetDescription())
}

func TestOrderItem_GetCategory_AlwaysEmpty(t *testing.T) {
	// Home Depot's response doesn't expose a Monarch-mappable category;
	// the LLM categorizer does the work downstream.
	item := &OrderItem{item: hdgo.LineItem{}}
	assert.Empty(t, item.GetCategory())
}

// TestOrder_ImplementsInterface is a compile-time check.
func TestOrder_ImplementsInterface(t *testing.T) {
	var _ providers.Order = (*Order)(nil)
	var _ providers.OrderItem = (*OrderItem)(nil)
}
