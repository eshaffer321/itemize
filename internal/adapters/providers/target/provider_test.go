package target

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	targetgo "github.com/daslicious/target-go"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The testdata files are synthetic: they follow the response shapes other projects recorded from
// target.com in 2026, not captured responses.

func loadJSON(t *testing.T, name string, out any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, out))
}

type fakeAPI struct {
	history   map[string][]*targetgo.OrderHistoryResponse // purchase type -> pages
	online    map[string]*targetgo.OnlineOrder
	store     map[string]*targetgo.StoreOrder
	invoices  map[string]*targetgo.InvoiceList // order number -> its invoices; none if missing
	invoice   map[string]*targetgo.Invoice     // invoice id -> details
	detailErr map[string]error
	calls     []string
}

func (f *fakeAPI) OrderHistory(_ context.Context, purchaseType string, page, _ int) (*targetgo.OrderHistoryResponse, error) {
	f.calls = append(f.calls, fmt.Sprintf("history %s %d", purchaseType, page))
	pages := f.history[purchaseType]
	if page > len(pages) {
		return &targetgo.OrderHistoryResponse{}, nil
	}
	return pages[page-1], nil
}

func (f *fakeAPI) OnlineOrder(_ context.Context, id string) (*targetgo.OnlineOrder, error) {
	f.calls = append(f.calls, "online "+id)
	if err := f.detailErr[id]; err != nil {
		return nil, err
	}
	return f.online[id], nil
}

func (f *fakeAPI) StoreOrder(_ context.Context, id string) (*targetgo.StoreOrder, error) {
	f.calls = append(f.calls, "store "+id)
	if err := f.detailErr[id]; err != nil {
		return nil, err
	}
	return f.store[id], nil
}

func (f *fakeAPI) Invoices(_ context.Context, orderNumber string) (*targetgo.InvoiceList, error) {
	f.calls = append(f.calls, "invoices "+orderNumber)
	if list := f.invoices[orderNumber]; list != nil {
		return list, nil
	}
	return &targetgo.InvoiceList{}, nil
}

func (f *fakeAPI) Invoice(_ context.Context, orderNumber, invoiceID string) (*targetgo.Invoice, error) {
	f.calls = append(f.calls, "invoice "+orderNumber+" "+invoiceID)
	if err := f.detailErr[invoiceID]; err != nil {
		return nil, err
	}
	if inv := f.invoice[invoiceID]; inv != nil {
		return inv, nil
	}
	return nil, fmt.Errorf("target: HTTP 404 on invoice %s", invoiceID)
}

func fixtureAPI(t *testing.T) *fakeAPI {
	var storeHistory, onlineHistory targetgo.OrderHistoryResponse
	var store targetgo.StoreOrder
	var online targetgo.OnlineOrder
	loadJSON(t, "history_store.json", &storeHistory)
	loadJSON(t, "history_online.json", &onlineHistory)
	loadJSON(t, "store_order.json", &store)
	loadJSON(t, "online_order.json", &online)
	return &fakeAPI{
		history: map[string][]*targetgo.OrderHistoryResponse{targetgo.PurchaseStore: {&storeHistory}, targetgo.PurchaseOnline: {&onlineHistory}},
		online:  map[string]*targetgo.OnlineOrder{online.OrderNumber: &online},
		store:   map[string]*targetgo.StoreOrder{store.StoreReceiptID: &store},
	}
}

var window = providers.FetchOptions{
	StartDate:      time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
	EndDate:        time.Date(2026, 10, 9, 23, 0, 0, 0, time.UTC),
	IncludeDetails: true,
}

func itemSummary(order providers.Order) []string {
	var out []string
	for _, item := range order.GetItems() {
		out = append(out, fmt.Sprintf("%s x%g $%.2f [%s]", item.GetName(), item.GetQuantity(), item.GetPrice(), item.GetCategory()))
	}
	return out
}

func TestFetchOrders(t *testing.T) {
	api := fixtureAPI(t)
	orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
	require.NoError(t, err)

	// The return and the July receipt are skipped without reading their details.
	assert.Equal(t, []string{
		"history STORE 1", "store 6032-2290-0172-8918",
		"history ONLINE 1", "online 912002749053897", "invoices 912002749053897",
	}, api.calls)
	require.Len(t, orders, 2)

	store := orders[0]
	assert.Equal(t, "6032-2290-0172-8918", store.GetID())
	assert.Equal(t, "Target", store.GetProviderName())
	assert.Equal(t, KindStore, store.(*Order).Kind())
	assert.Equal(t, "2026-09-26", store.GetDate().Format("2006-01-02"))
	assert.InDelta(t, 24.42, store.GetTotal(), 1e-9)
	assert.InDelta(t, 22.77, store.GetSubtotal(), 1e-9)
	assert.InDelta(t, 1.65, store.GetTax(), 1e-9)
	assert.Equal(t, []string{
		"Whole Wheat Penne Pasta - 16oz x2 $6.98 [GROCERY]",
		"Men's Crew Socks 6pk - Gray x1 $12.00 [APPAREL]",
		"Freezer Storage Bags - 40ct - up&up™ x1 $3.79 [HOME]",
	}, itemSummary(store), "paper bag dropped, entities decoded")

	online := orders[1]
	assert.Equal(t, "912002749053897", online.GetID())
	assert.Equal(t, KindOnline, online.(*Order).Kind())
	assert.Equal(t, "2026-09-20", online.GetDate().Format("2006-01-02"))
	assert.InDelta(t, 35.67, online.GetTotal(), 1e-9)
	assert.InDelta(t, 2.19, online.GetTax(), 1e-9)
	assert.Equal(t, []string{
		"LEGO City F1 Williams Racing & Haas F1 Race Cars 60464 x1 $19.99 []",
		"Big Coloring Book x1 $13.49 []",
	}, itemSummary(online), "items from both packages, markup stripped")
}

func TestFetchOrdersPaging(t *testing.T) {
	page := func(dates ...string) *targetgo.OrderHistoryResponse {
		resp := &targetgo.OrderHistoryResponse{TotalPages: 5}
		for i, d := range dates {
			resp.Orders = append(resp.Orders, targetgo.OrderSummary{OrderNumber: fmt.Sprintf("9120%s%d", d[5:7]+d[8:10], i), PlacedDate: d, Summary: targetgo.Totals{GrandTotal: 10}})
		}
		return resp
	}
	api := &fakeAPI{history: map[string][]*targetgo.OrderHistoryResponse{targetgo.PurchaseOnline: {
		page("2026-10-01", "2026-09-25"),
		page("2026-09-12", "2026-09-01"), // passes the start date: stop here
		page("2026-08-20", "2026-08-10"),
	}}}
	p := NewProvider(api, nil)
	p.pageSize = 2

	opts := window
	opts.IncludeDetails = false
	orders, err := p.FetchOrders(context.Background(), opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"history STORE 1", "history ONLINE 1", "history ONLINE 2"}, api.calls)
	assert.Len(t, orders, 3)

	opts.MaxOrders = 2
	orders, err = p.FetchOrders(context.Background(), opts)
	require.NoError(t, err)
	assert.Len(t, orders, 2)
}

func TestFetchOrdersErrors(t *testing.T) {
	t.Run("one bad order is skipped", func(t *testing.T) {
		api := fixtureAPI(t)
		api.detailErr = map[string]error{"6032-2290-0172-8918": errors.New("target: HTTP 500")}
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, "912002749053897", orders[0].GetID())
	})

	for _, fatalErr := range []error{targetgo.ErrNotAuthenticated, targetgo.ErrBlocked} {
		t.Run(fatalErr.Error(), func(t *testing.T) {
			api := fixtureAPI(t)
			api.detailErr = map[string]error{"6032-2290-0172-8918": fmt.Errorf("%w (HTTP 401)", fatalErr)}
			_, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
			assert.True(t, errors.Is(err, fatalErr))
		})
	}
}

func TestGetOrderDetails(t *testing.T) {
	api := fixtureAPI(t)
	p := NewProvider(api, nil)
	store, err := p.GetOrderDetails(context.Background(), "6032-2290-0172-8918")
	require.NoError(t, err)
	assert.InDelta(t, 24.42, store.GetTotal(), 1e-9)
	online, err := p.GetOrderDetails(context.Background(), "912002749053897")
	require.NoError(t, err)
	assert.Len(t, online.GetItems(), 2)
	assert.Equal(t, []string{"store 6032-2290-0172-8918", "online 912002749053897", "invoices 912002749053897"}, api.calls)
}

// splitAPI serves an online order shipped in two packages and charged once per shipment.
func splitAPI(t *testing.T) *fakeAPI {
	var order targetgo.OnlineOrder
	var list targetgo.InvoiceList
	var first, second targetgo.Invoice
	loadJSON(t, "split_order.json", &order)
	loadJSON(t, "split_invoices.json", &list)
	loadJSON(t, "split_invoice_2001.json", &first)
	loadJSON(t, "split_invoice_2002.json", &second)
	summary := targetgo.OrderSummary{OrderNumber: order.OrderNumber, OrderDate: order.OrderDate, Summary: order.Summary, Packages: order.Packages}
	return &fakeAPI{
		history:  map[string][]*targetgo.OrderHistoryResponse{targetgo.PurchaseOnline: {{Orders: []targetgo.OrderSummary{summary}}}},
		online:   map[string]*targetgo.OnlineOrder{order.OrderNumber: &order},
		invoices: map[string]*targetgo.InvoiceList{order.OrderNumber: &list},
		invoice:  map[string]*targetgo.Invoice{"2001": &first, "2002": &second},
	}
}

func TestOrderChargedPerShipment(t *testing.T) {
	api := splitAPI(t)
	orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"history STORE 1", "history ONLINE 1", "online 912002800000001", "invoices 912002800000001",
		"invoice 912002800000001 2001", "invoice 912002800000001 2002",
	}, api.calls, "invoices read oldest first; the return is not read")
	require.Len(t, orders, 2)

	first := orders[0]
	assert.Equal(t, "912002800000001", first.GetID(), "the first charge keeps the order number")
	assert.Equal(t, KindOnline, first.(*Order).Kind())
	assert.Equal(t, "2026-09-21", first.GetDate().Format("2006-01-02"), "dated when it was charged")
	assert.InDelta(t, 25.82, first.GetTotal(), 1e-9)
	assert.InDelta(t, 25.62, first.GetSubtotal(), 1e-9)
	assert.InDelta(t, 0.20, first.GetTax(), 1e-9, "the bag fee is spread like tax")
	assert.Equal(t, []string{
		"LEGO City F1 Williams Racing & Haas F1 Race Cars 60464 x1 $21.54 [TOYS]",
		"Freezer Storage Bags - 40ct - up&up™ x1 $4.08 [HOME]",
	}, itemSummary(first), "items priced with their own tax, typed from the order lines; the bag fee line isn't an item")

	second := orders[1]
	assert.Equal(t, "912002800000001-2", second.GetID())
	assert.Equal(t, "2026-10-02", second.GetDate().Format("2006-01-02"))
	assert.InDelta(t, 46.87, second.GetTotal(), 1e-9)
	assert.InDelta(t, 0, second.GetTax(), 1e-9)
	assert.Equal(t, []string{
		"Big Coloring Book x1 $14.54 [BOOKS]",
		"Boys' Pajama Set - Blue 8 x2 $32.33 [APPAREL]",
	}, itemSummary(second))
}

func TestOrderChargedInParts(t *testing.T) {
	t.Run("an order charged once is read whole", func(t *testing.T) {
		// A digital manufacturer coupon is a tender on the invoice, so the invoice total is the
		// order total plus the coupon: one charge is read whole whatever its amount.
		api := splitAPI(t)
		api.invoices["912002800000001"] = &targetgo.InvoiceList{Invoices: []targetgo.InvoiceSummary{{ID: "2000", Type: "SHIPMENT", Date: "2026-09-21T15:00:00.000Z", Amount: 73.69}}}
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, "912002800000001", orders[0].GetID())
		assert.InDelta(t, 72.69, orders[0].GetTotal(), 1e-9)
		assert.Len(t, orders[0].GetItems(), 4)
		assert.NotContains(t, api.calls, "invoice 912002800000001 2000")
	})

	t.Run("a coupon's share left out of a charge", func(t *testing.T) {
		api := splitAPI(t)
		api.invoice["2002"].Payments = []targetgo.InvoicePayment{
			{Type: "MANUFACTURERCOUPON", SubType: "DIGITAL", TotalCharged: 1},
			{Type: "TARGETDEBIT", SubType: "NONE", TotalCharged: 45.87},
		}
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 2)
		assert.InDelta(t, 45.87, orders[1].GetTotal(), 1e-9)
	})

	t.Run("gift card share left out of a charge", func(t *testing.T) {
		api := splitAPI(t)
		api.invoice["2002"].Payments = []targetgo.InvoicePayment{
			{Type: "GIFTCARD", TotalCharged: 10},
			{Type: "VISA", TotalCharged: 36.87},
		}
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 2)
		second := orders[1]
		assert.InDelta(t, 36.87, second.GetTotal(), 1e-9)
		sum := 0.0
		for _, item := range second.GetItems() {
			sum += item.GetPrice()
		}
		assert.InDelta(t, second.GetSubtotal(), sum, 0.005)
		assert.InDelta(t, second.GetTotal(), second.GetSubtotal()+second.GetTax(), 0.005)
	})

	t.Run("an unreadable invoice fails the order", func(t *testing.T) {
		api := splitAPI(t)
		api.detailErr = map[string]error{"2002": errors.New("target: HTTP 500")}
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		assert.Empty(t, orders, "neither charge is synced until both can be read")

		api.detailErr = map[string]error{"2002": fmt.Errorf("%w (HTTP 401)", targetgo.ErrNotAuthenticated)}
		_, err = NewProvider(api, nil).FetchOrders(context.Background(), window)
		assert.ErrorIs(t, err, targetgo.ErrNotAuthenticated)
	})
}

func TestGetOrderDetailsCharge(t *testing.T) {
	p := NewProvider(splitAPI(t), nil)
	ctx := context.Background()

	second, err := p.GetOrderDetails(ctx, "912002800000001-2")
	require.NoError(t, err)
	assert.Equal(t, "912002800000001-2", second.GetID())
	assert.InDelta(t, 46.87, second.GetTotal(), 1e-9)

	first, err := p.GetOrderDetails(ctx, "912002800000001")
	require.NoError(t, err)
	assert.InDelta(t, 25.82, first.GetTotal(), 1e-9)

	_, err = p.GetOrderDetails(ctx, "912002800000001-3")
	assert.Error(t, err)

	all, err := p.FetchOrder(ctx, "912002800000001-2")
	require.NoError(t, err)
	assert.Len(t, all, 2, "every charge of the order")
}

// Target sometimes sends an item line without its catalog fields (description, product type), as on
// a receipt read on 2026-10-11 whose history entry still had the name. Dropping the line would put its
// price into tax and split it over the other items.
func TestItemWithoutName(t *testing.T) {
	const receipt = "6032-2290-0172-8918"
	unnamed := targetgo.Item{TCIN: "54191097", UnitPrice: 12}

	t.Run("named from the history entry", func(t *testing.T) {
		api := fixtureAPI(t)
		api.store[receipt].OrderLines[1].Item = unnamed
		api.history[targetgo.PurchaseStore][0].Orders[0].OrderLines = []targetgo.OrderLine{
			{Item: targetgo.Item{TCIN: "54191097", Description: "Men&#39;s Crew Socks 6pk - Gray"}},
		}
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 2)
		assert.Equal(t, receipt, orders[0].GetID())
		assert.InDelta(t, 1.65, orders[0].GetTax(), 1e-9)
		assert.Equal(t, []string{
			"Whole Wheat Penne Pasta - 16oz x2 $6.98 [GROCERY]",
			"Men's Crew Socks 6pk - Gray x1 $12.00 []",
			"Freezer Storage Bags - 40ct - up&up™ x1 $3.79 [HOME]",
		}, itemSummary(orders[0]))
	})

	t.Run("an order with an unnamed item waits for the next run", func(t *testing.T) {
		api := fixtureAPI(t)
		api.store[receipt].OrderLines[1].Item = unnamed
		p := NewProvider(api, nil)
		orders, err := p.FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, "912002749053897", orders[0].GetID())

		_, err = p.GetOrderDetails(context.Background(), receipt)
		assert.ErrorContains(t, err, "item 54191097 ($12.00) came without a name")
	})

	t.Run("unpriced and cancelled lines need no name", func(t *testing.T) {
		api := fixtureAPI(t)
		zero := 0.0
		lines := api.store[receipt].OrderLines
		lines[2].Item.Description = ""
		api.store[receipt].OrderLines = append(lines, targetgo.OrderLine{Quantity: &zero, Item: targetgo.Item{TCIN: "5", UnitPrice: 9}})
		order, err := NewProvider(api, nil).GetOrderDetails(context.Background(), receipt)
		require.NoError(t, err)
		assert.Len(t, order.GetItems(), 3)
	})

	t.Run("an online order line named from the history entry", func(t *testing.T) {
		api := fixtureAPI(t)
		api.online["912002749053897"].Packages[0].OrderLines[0].Item = targetgo.Item{TCIN: "89120321", UnitPrice: 19.99}
		_, err := NewProvider(api, nil).GetOrderDetails(context.Background(), "912002749053897")
		require.Error(t, err, "nothing to name it from without the history entry")

		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 2)
		assert.Equal(t, []string{
			"LEGO City F1 Williams Racing & Haas F1 Race Cars 60464 x1 $19.99 []",
			"Big Coloring Book x1 $13.49 []",
		}, itemSummary(orders[1]))
	})

	t.Run("an invoice line named from its order line", func(t *testing.T) {
		api := splitAPI(t)
		api.invoice["2002"].Lines[0].Item.Description = ""
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 2)
		assert.Equal(t, []string{
			"Big Coloring Book x1 $14.54 [BOOKS]",
			"Boys' Pajama Set - Blue 8 x2 $32.33 [APPAREL]",
		}, itemSummary(orders[1]))

		api.online["912002800000001"].Packages[1].OrderLines[0].Item.Description = ""
		_, err = NewProvider(api, nil).GetOrderDetails(context.Background(), "912002800000001-2")
		assert.ErrorContains(t, err, "item 77777777 ($14.54) came without a name")
	})

	t.Run("a name missing only from an order charged in parts", func(t *testing.T) {
		// The charges are read from the invoices, which have every name.
		api := splitAPI(t)
		api.online["912002800000001"].Packages[0].OrderLines[0].Item.Description = ""
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		assert.Len(t, orders, 2)
	})
}

func TestGiftCardShareLeavesCardCharge(t *testing.T) {
	var receipt targetgo.StoreOrder
	loadJSON(t, "store_order.json", &receipt)
	receipt.PaymentTransactions = []targetgo.PaymentTransaction{
		{PaymentType: "GIFT_CARD", Amount: 4.42},
		{PaymentType: "CREDIT_CARD", PaymentSubType: "VISA", Amount: 20.00},
	}
	order, err := convertStore(&receipt, targetgo.OrderSummary{})
	require.NoError(t, err)
	assert.InDelta(t, 20.00, order.GetTotal(), 1e-9, "Monarch sees the card charge")

	receipt.PaymentTransactions = []targetgo.PaymentTransaction{
		{PaymentType: "MANUFACTURERCOUPON", Amount: 1.00},
		{PaymentType: "TARGETDEBIT", Amount: 23.42},
	}
	couponed, err := convertStore(&receipt, targetgo.OrderSummary{})
	require.NoError(t, err)
	assert.InDelta(t, 23.42, couponed.GetTotal(), 1e-9, "a coupon isn't a card charge")

	sum := 0.0
	for _, item := range order.GetItems() {
		sum += item.GetPrice()
	}
	assert.InDelta(t, order.GetSubtotal(), sum, 0.005)
	assert.InDelta(t, order.GetTotal(), order.GetSubtotal()+order.GetTax(), 0.005)
	assert.Less(t, order.GetSubtotal(), 22.77)
}

// Bug (found 2026-10-11 in a May receipt): the reusable-bag discount is taken after the total and
// listed as a tender, GENERALLEDGERADJUSTMENT / BAGDISCOUNT. It was counted as card money, so the
// order's total was grand_total, 15 cents more than the card charge, and it could never match.
func TestBagDiscountIsNotACardCharge(t *testing.T) {
	var receipt targetgo.StoreOrder
	loadJSON(t, "store_order.json", &receipt)
	require.NoError(t, json.Unmarshal([]byte(`[
		{"amount": "0.15", "payment_type": "GENERALLEDGERADJUSTMENT", "payment_sub_type": "BAGDISCOUNT", "payment_sub_type_value": "Reusable Bag Discount"},
		{"amount": "24.27", "payment_type": "TARGETDEBIT", "payment_sub_type": "NONE", "payment_sub_type_value": "Target Circle Debit Card"}
	]`), &receipt.PaymentTransactions))
	order, err := convertStore(&receipt, targetgo.OrderSummary{})
	require.NoError(t, err)
	assert.InDelta(t, 24.27, order.GetTotal(), 1e-9, "Monarch sees the card charge")
	assert.InDelta(t, order.GetTotal(), order.GetSubtotal()+order.GetTax(), 0.005)
}

// Bug (found 2026-10-13 by reading the whole order history): when Target cancels part of an online
// order (out of stock at the store, or the guest declined a later date), the cancelled items move to
// a package of their own with status STAT_CANCELED, and their quantity stays at 1. They were synced as
// bought: listed in the notes and given a share of the charge. 15 of 94 online orders had one.
func TestCancelledPackageIsLeftOut(t *testing.T) {
	var order targetgo.OnlineOrder
	require.NoError(t, json.Unmarshal([]byte(`{
		"order_number": "912002900000001", "order_date": "2026-09-20T18:00:00.000Z",
		"summary": {"grand_total": "26.75", "total_product_price": "25.00", "total_taxes": "1.75"},
		"payments": [{"payment_type": "TARGETDEBIT", "payment_subtype": "NONE", "amount": "26.75"}],
		"packages": [
			{"grouping_metadata": {"status": "STAT_ORDER_PICKED_UP"}, "fulfillment": {"status": {"key": "STAT_ORDER_PICKED_UP", "code": "3700.003"}},
			 "order_lines": [{"order_line_key": "k1", "quantity": 1, "original_quantity": 1,
				"item": {"tcin": "11111111", "description": "Ceramic Coffee Mug", "unit_price": 25.00, "product_classification": {"product_type_name": "APPAREL"}}}]},
			{"grouping_metadata": {"status": "STAT_CANCELED", "cancellation": {"cancel_reason_code": "CANCEL_REPROMTIMEEXCEEDED", "cancel_reason_code_description": "Out of stock at selected store"}},
			 "fulfillment": {"status": {"key": "STAT_CANCELED", "code": "9000"}},
			 "order_lines": [{"order_line_key": "k2", "quantity": 1, "original_quantity": 1, "cancellation": {"cancel_reason_code": "CANCEL_REPROMTIMEEXCEEDED"},
				"item": {"tcin": "22222222", "description": "Glass Food Storage Set", "unit_price": 19.99, "product_classification": {"product_type_name": "HOME"}}}]}
		]}`), &order))

	o, err := convertOnline(&order, targetgo.OrderSummary{})
	require.NoError(t, err)
	assert.Equal(t, []string{"Ceramic Coffee Mug x1 $25.00 [APPAREL]"}, itemSummary(o))
	assert.InDelta(t, 26.75, o.GetTotal(), 1e-9)
	assert.InDelta(t, 1.75, o.GetTax(), 1e-9)
}

// Bug (found 2026-10-13): an online order's payments are listed net of later refunds. When Target
// refunded a pickup order's bag fee days after charging it, the card payment read $83.00 while the
// bank had been charged the full $83.40 (7 orders from 2024-10 to 2025-04), so the order could never
// match. What the card was charged is the total less what gift cards paid (71 of 72 orders matched
// Monarch this way, against 65 using the card payment).
func TestOnlineChargeIsTotalLessGiftCards(t *testing.T) {
	var order targetgo.OnlineOrder
	loadJSON(t, "online_order.json", &order)

	order.Payments = []targetgo.PaymentTransaction{{PaymentType: "TARGETDEBIT", PaymentSubtype: "NONE", Amount: 35.17}}
	o, err := convertOnline(&order, targetgo.OrderSummary{})
	require.NoError(t, err)
	assert.InDelta(t, 35.67, o.GetTotal(), 1e-9, "a refund 50 cents later doesn't lower the charge")

	order.Payments = []targetgo.PaymentTransaction{
		{PaymentType: "TARGETDEBIT", PaymentSubtype: "NONE", Amount: 25.17},
		{PaymentType: "TARGETGIFTCARD", PaymentSubtype: "NONE", Amount: 10},
	}
	o, err = convertOnline(&order, targetgo.OrderSummary{})
	require.NoError(t, err)
	assert.InDelta(t, 25.67, o.GetTotal(), 1e-9, "the gift card's share is still left out")
}

// Bug (found 2026-10-13): an order paid entirely with gift cards has no card charge, but its total was
// taken as the amount to match, so it waited for a bank transaction that never comes and could
// take another Target charge of the same amount. The history had 13 such orders
// and charges, and a cancelled online order with a $0 total that would have waited the same way.
func TestOrdersNotChargedToACardAreLeftOut(t *testing.T) {
	giftCard := func(amount float64) []targetgo.PaymentTransaction {
		return []targetgo.PaymentTransaction{{PaymentType: "TARGETGIFTCARD", PaymentSubType: "NONE", Amount: targetgo.Money(amount)}}
	}

	t.Run("a receipt paid with a gift card", func(t *testing.T) {
		api := fixtureAPI(t)
		api.store["6032-2290-0172-8918"].PaymentTransactions = giftCard(24.42)
		p := NewProvider(api, nil)
		orders, err := p.FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, "912002749053897", orders[0].GetID())

		_, err = p.GetOrderDetails(context.Background(), "6032-2290-0172-8918")
		assert.ErrorContains(t, err, "card")
	})

	t.Run("an online order paid with a gift card", func(t *testing.T) {
		api := fixtureAPI(t)
		api.online["912002749053897"].Payments = giftCard(35.67)
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, "6032-2290-0172-8918", orders[0].GetID())
	})

	t.Run("a cancelled online order", func(t *testing.T) {
		api := fixtureAPI(t)
		order := api.online["912002749053897"]
		order.Summary = targetgo.Totals{}
		order.Payments = []targetgo.PaymentTransaction{{PaymentType: "TARGETDEBIT", PaymentSubtype: "NONE", Amount: 6.12}}
		for i, entry := range api.history[targetgo.PurchaseOnline][0].Orders {
			if entry.OrderNumber == order.OrderNumber {
				api.history[targetgo.PurchaseOnline][0].Orders[i].Summary = targetgo.Totals{}
			}
		}
		orders, err := NewProvider(api, nil).FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, "6032-2290-0172-8918", orders[0].GetID())
	})

	t.Run("a charge paid with a gift card", func(t *testing.T) {
		api := splitAPI(t)
		api.invoice["2001"].Payments = []targetgo.InvoicePayment{{Type: "TARGETGIFTCARD", SubType: "NONE", TotalCharged: 25.82}}
		p := NewProvider(api, nil)
		orders, err := p.FetchOrders(context.Background(), window)
		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, "912002800000001-2", orders[0].GetID(), "the other charge keeps its id")
		assert.InDelta(t, 46.87, orders[0].GetTotal(), 1e-9)

		_, err = p.GetOrderDetails(context.Background(), "912002800000001")
		assert.Error(t, err)
	})
}

func TestUnpricedItemsMerge(t *testing.T) {
	var history targetgo.OrderHistoryResponse
	loadJSON(t, "history_online.json", &history)
	order := convertSummary(history.Orders[0], targetgo.PurchaseOnline)
	require.Len(t, order.GetItems(), 1)
	item := order.GetItems()[0]
	assert.Equal(t, "LEGO City F1 Williams Racing & Haas F1 Race Cars 60464", item.GetName())
	assert.InDelta(t, 35.67, item.GetPrice(), 1e-9)
	assert.InDelta(t, 0, order.GetTax(), 1e-9)
}

// The sample_*.json files keep the shape of target.com's responses (cut down to the fields the
// parser reads, with Target's HTML entities and string amounts); ids, stores, dates and items are made up.

func TestSampleStoreReceipt(t *testing.T) {
	var receipt targetgo.StoreOrder
	loadJSON(t, "sample_store_order.json", &receipt)
	order, err := convertStore(&receipt, targetgo.OrderSummary{})
	require.NoError(t, err)

	assert.Equal(t, "6000-1234-0000-0001", order.GetID())
	assert.Equal(t, "2026-09-24", order.GetDate().Format("2006-01-02"))
	assert.InDelta(t, 20.46, order.GetTotal(), 1e-9, "charged to the Target Circle Debit Card")
	assert.InDelta(t, 19.99, order.GetSubtotal(), 1e-9)
	assert.InDelta(t, 0.47, order.GetTax(), 1e-9, "$1.47 tax less the $1.00 Circle Card discount")
	assert.Equal(t, []string{"Kid's Building Blocks Set - 100pc x1 $19.99 [TOYS]"}, itemSummary(order))
}

func TestSampleOnlineOrder(t *testing.T) {
	var online targetgo.OnlineOrder
	loadJSON(t, "sample_online_order.json", &online)
	order, err := convertOnline(&online, targetgo.OrderSummary{})
	require.NoError(t, err)

	assert.Equal(t, "102000000000001", order.GetID())
	assert.Equal(t, "2026-08-05", order.GetDate().Format("2006-01-02"))
	assert.InDelta(t, 22.72, order.GetTotal(), 1e-9)
	assert.InDelta(t, 23.92, order.GetSubtotal(), 1e-9)
	assert.InDelta(t, -1.20, order.GetTax(), 1e-9, "no tax on groceries, 5% Circle Card discount")
	assert.Equal(t, []string{
		"Sliced Sharp Cheddar Cheese - 8oz - Good & Gather™ x2 $7.18 [GROCERY]",
		"Gala Apples - 3lb Bag x2 $8.38 [GROCERY]",
		"Organic Baby Spinach - 5oz - Good & Gather™ x3 $5.37 [GROCERY]",
		"Fresh Blueberries - 1pt x1 $2.99 [GROCERY]",
	}, itemSummary(order))
}

func TestSampleInvoice(t *testing.T) {
	var online targetgo.OnlineOrder
	var list targetgo.InvoiceList
	var invoice targetgo.Invoice
	loadJSON(t, "sample_online_order.json", &online)
	loadJSON(t, "sample_invoices.json", &list)
	loadJSON(t, "sample_invoice.json", &invoice)

	require.Len(t, list.Invoices, 1)
	assert.Equal(t, "62000000000000001", list.Invoices[0].ID)
	assert.InDelta(t, 22.72, float64(list.Invoices[0].Amount), 1e-9)

	order, err := convertInvoice(&invoice, &online, "102000000000001")
	require.NoError(t, err)
	assert.Equal(t, "102000000000001", order.GetID())
	assert.Equal(t, "2026-08-05", order.GetDate().Format("2006-01-02"))
	assert.InDelta(t, 22.72, order.GetTotal(), 1e-9)
	assert.InDelta(t, 22.72, order.GetSubtotal(), 1e-9)
	assert.InDelta(t, 0, order.GetTax(), 1e-9)
	assert.Equal(t, []string{
		"Fresh Blueberries - 1pt x1 $2.84 [GROCERY]",
		"Organic Baby Spinach - 5oz - Good & Gather™ x3 $5.10 [GROCERY]",
		"Gala Apples - 3lb Bag x2 $7.96 [GROCERY]",
		"Sliced Sharp Cheddar Cheese - 8oz - Good & Gather™ x2 $6.82 [GROCERY]",
	}, itemSummary(order), "5% Circle Card discount already taken off each line")
}

func TestSampleHistories(t *testing.T) {
	var online, store targetgo.OrderHistoryResponse
	loadJSON(t, "sample_history_online.json", &online)
	loadJSON(t, "sample_history_store.json", &store)

	require.Len(t, online.Orders, 3)
	first := online.Orders[0]
	assert.Equal(t, "102000000000001", first.ID())
	assert.Equal(t, "2026-08-05", targetgo.ParseTime(first.Date()).Format("2006-01-02"))
	assert.Len(t, first.Lines(), 4, "online history keeps lines in packages")
	assert.InDelta(t, 22.72, float64(first.Summary.GrandTotal), 1e-9)

	require.Len(t, store.Orders, 3)
	assert.Equal(t, "6000-1234-0000-0002", store.Orders[0].ID())
	assert.Equal(t, "2026-10-06", targetgo.ParseTime(store.Orders[0].Date()).Format("2006-01-02"))
	assert.Equal(t, 3, store.TotalPages)
}

func TestCancelledLineIsDropped(t *testing.T) {
	zero, two := 0.0, 2.0
	order := buildOrder("1", KindOnline, time.Now(), 10, 10, []targetgo.OrderLine{
		{Quantity: &zero, OriginalQuantity: 1, Item: targetgo.Item{Description: "Out of stock", UnitPrice: 5}},
		{Quantity: &two, Item: targetgo.Item{Description: "Kept", UnitPrice: 4.5}},
	}, nil)
	assert.Equal(t, []string{"Kept x2 $9.00 []"}, itemSummary(order))
}

func TestOrderDateIsLocalDay(t *testing.T) {
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	la, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)
	time.Local = la

	// 23:17 Central is 21:17 in California: still the 6th, not the 7th as in UTC.
	assert.Equal(t, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), orderDate("2026-10-06T23:17:42-05:00"))
	assert.Equal(t, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), orderDate("2026-08-06T01:09:52.000Z"))
	assert.True(t, orderDate("not a date").IsZero())
}

// TestMain pins the local zone: order dates are the day as seen where itemize runs, so the
// expected dates in these tests depend on it.
func TestMain(m *testing.M) {
	if loc, err := time.LoadLocation("America/Denver"); err == nil {
		time.Local = loc
	}
	os.Exit(m.Run())
}
