package target

import (
	"fmt"
	"math"
	"strings"
	"time"

	targetgo "github.com/daslicious/target-go"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

// Order kinds.
const (
	KindOnline = "online"
	KindStore  = "store"
)

// Order implements providers.Order for a Target online order or in-store receipt.
type Order struct {
	id       string
	kind     string
	date     time.Time
	total    float64
	subtotal float64
	tax      float64
	items    []providers.OrderItem
	raw      interface{}
}

func (o *Order) GetID() string                   { return o.id }
func (o *Order) GetDate() time.Time              { return o.date }
func (o *Order) GetTotal() float64               { return o.total }
func (o *Order) GetSubtotal() float64            { return o.subtotal }
func (o *Order) GetTax() float64                 { return o.tax }
func (o *Order) GetTip() float64                 { return 0 }
func (o *Order) GetFees() float64                { return 0 }
func (o *Order) GetItems() []providers.OrderItem { return o.items }
func (o *Order) GetProviderName() string         { return "Target" }
func (o *Order) GetRawData() interface{}         { return o.raw }

// Kind is KindOnline or KindStore.
func (o *Order) Kind() string { return o.kind }

// OrderItem implements providers.OrderItem.
type OrderItem struct {
	name      string
	price     float64
	quantity  float64
	unitPrice float64
	sku       string
	category  string
}

func (i *OrderItem) GetName() string        { return i.name }
func (i *OrderItem) GetPrice() float64      { return i.price }
func (i *OrderItem) GetQuantity() float64   { return i.quantity }
func (i *OrderItem) GetUnitPrice() float64  { return i.unitPrice }
func (i *OrderItem) GetDescription() string { return i.name }
func (i *OrderItem) GetSKU() string         { return i.sku }
func (i *OrderItem) GetCategory() string    { return i.category }

// convertStore turns an in-store receipt into an Order. The total is what the cards were charged:
// a gift card or a discount taken after the total (e.g. bringing your own bag) makes the card charge
// smaller than grand_total, and the card charge is what appears in Monarch.
func convertStore(s *targetgo.StoreOrder, fallback targetgo.OrderSummary) (*Order, error) {
	id := firstNonEmpty(s.StoreReceiptID, fallback.ID())
	date := orderDate(firstNonEmpty(s.OrderPurchaseDate, fallback.Date()))
	total := firstNonZero(float64(s.GrandTotal), float64(s.Summary.GrandTotal), float64(fallback.Summary.GrandTotal))
	lines, err := nameLines(s.OrderLines, fallback.Lines())
	if err != nil {
		return nil, err
	}
	return buildOrder(id, KindStore, date, total, cardCharge(s.PaymentTransactions, total), lines, s), nil
}

// convertOnline turns an online order into an Order. Items from every package are combined, and
// as with receipts, a gift-card share is left out of the total.
func convertOnline(o *targetgo.OnlineOrder, fallback targetgo.OrderSummary) (*Order, error) {
	id := firstNonEmpty(o.OrderNumber, fallback.ID())
	date := orderDate(firstNonEmpty(o.OrderDate, fallback.Date()))
	total := firstNonZero(float64(o.Summary.GrandTotal), float64(fallback.Summary.GrandTotal))
	lines := o.Lines()
	if len(lines) == 0 {
		lines = fallback.Lines()
	}
	lines, err := nameLines(lines, fallback.Lines())
	if err != nil {
		return nil, err
	}
	return buildOrder(id, KindOnline, date, total, onlineCardCharge(o.Payments, total), lines, o), nil
}

// convertInvoice turns one charge of an online order charged in parts into an Order with the given
// id, dated by the charge. Its items are the invoice's lines priced at their effective amount (after
// discounts, with their own tax), so only rounding and fees are left over as tax. Target's product
// type (and a name missing from the invoice) comes from the order's line for the same item. A line
// that isn't on the order is a fee (Target adds the bag fee as a PAPER_BAG line), not an item. As
// elsewhere, gift-card and coupon shares are left out of the total.
func convertInvoice(inv *targetgo.Invoice, online *targetgo.OnlineOrder, id string) (*Order, error) {
	types := map[string]targetgo.ProductClassification{}
	for _, line := range online.Lines() {
		if line.OrderLineKey != "" {
			types[line.OrderLineKey] = line.Item.ProductClassification
		}
	}
	lines := make([]targetgo.OrderLine, 0, len(inv.Lines))
	for _, l := range inv.Lines {
		if _, onOrder := types[l.OrderLineKey]; !onOrder && len(types) > 0 {
			continue
		}
		qty := l.Quantity
		if qty <= 0 {
			qty = 1
		}
		item := l.Item
		item.UnitPrice = l.EffectiveAmount / targetgo.Money(qty)
		item.ProductClassification = types[l.OrderLineKey]
		lines = append(lines, targetgo.OrderLine{OrderLineKey: l.OrderLineKey, Quantity: &qty, Item: item})
	}
	tenders := make([]targetgo.PaymentTransaction, 0, len(inv.Payments))
	for _, p := range inv.Payments {
		tenders = append(tenders, targetgo.PaymentTransaction{PaymentType: p.Type, PaymentSubType: p.SubType, Amount: p.TotalCharged})
	}
	lines, err := nameLines(lines, online.Lines())
	if err != nil {
		return nil, err
	}
	date := orderDate(firstNonEmpty(inv.Date, online.OrderDate))
	total := float64(inv.TotalAmount)
	return buildOrder(id, KindOnline, date, total, cardCharge(tenders, total), lines, inv), nil
}

// nameLines gives a line without a description the name of the same item (by TCIN) in other. Target
// sometimes sends an item line without its catalog fields (seen 2026-10-11 on a receipt whose history
// entry still had the name). A priced line left without a name is an error: buildOrder would drop it,
// and its price would end up in tax, split over the other items. The order then waits for the next run.
func nameLines(lines, other []targetgo.OrderLine) ([]targetgo.OrderLine, error) {
	names := map[string]string{}
	for _, l := range other {
		if l.Item.TCIN != "" && l.Item.Name() != "" {
			names[l.Item.TCIN] = l.Item.Description
		}
	}
	named := make([]targetgo.OrderLine, len(lines))
	for i, l := range lines {
		if l.Item.Name() == "" {
			l.Item.Description = names[l.Item.TCIN]
		}
		if l.Item.Name() == "" && l.Qty() != 0 && l.Item.UnitPrice != 0 {
			return nil, fmt.Errorf("target: item %s ($%.2f) came without a name", l.Item.TCIN, float64(l.Item.UnitPrice)*l.Qty())
		}
		named[i] = l
	}
	return named, nil
}

// buildOrder prices the items for the splitter. Item prices are the receipt's unit price times
// quantity, scaled by charge/total when only part of the total went on a card. The difference
// between the charge and the items (tax, minus any order-level discounts) is reported as tax, which
// the splitter spreads over the categories in proportion to their items.
//
// Lines priced $0 (paper bags) or with quantity 0 (cancelled) are dropped. If no line has a price, the items are merged into one
// item carrying the whole charge, so the order still gets a category and a note listing everything.
func buildOrder(id, kind string, date time.Time, total, charge float64, lines []targetgo.OrderLine, raw interface{}) *Order {
	ratio := 1.0
	if total != 0 && charge != total {
		ratio = charge / total
	}

	var items []providers.OrderItem
	var names []string
	subtotal := 0.0
	for _, line := range lines {
		name := line.Item.Name()
		qty := line.Qty()
		if name == "" || qty == 0 {
			continue
		}
		names = append(names, name)
		unit := float64(line.Item.UnitPrice)
		if unit == 0 {
			continue
		}
		price := roundCents(unit * qty * ratio)
		subtotal += price
		items = append(items, &OrderItem{
			name:      name,
			price:     price,
			quantity:  qty,
			unitPrice: roundCents(unit * ratio),
			sku:       line.Item.TCIN,
			category:  line.Item.ProductClassification.ProductTypeName,
		})
	}

	if len(items) == 0 && len(names) > 0 {
		joined := strings.Join(names, "; ")
		items = []providers.OrderItem{&OrderItem{name: joined, price: charge, quantity: 1, unitPrice: charge}}
		subtotal = charge
	}

	subtotal = roundCents(subtotal)
	return &Order{
		id:       id,
		kind:     kind,
		date:     date,
		total:    roundCents(charge),
		subtotal: subtotal,
		tax:      roundCents(charge - subtotal),
		items:    items,
		raw:      raw,
	}
}

// cardCharge returns the part of the receipt paid by card: not gift cards, cash, manufacturer
// coupons (MANUFACTURERCOUPON) or adjustments such as the reusable-bag discount
// (GENERALLEDGERADJUSTMENT / BAGDISCOUNT), which Target lists as tenders (see
// targetgo.IsCardTender). It's 0 if those paid it all; without tender amounts it's the total.
func cardCharge(tenders []targetgo.PaymentTransaction, total float64) float64 {
	card, other := 0.0, 0.0
	for _, t := range tenders {
		if t.IsCard() {
			card += float64(t.Amount)
		} else {
			other += float64(t.Amount)
		}
	}
	card = roundCents(card)
	switch {
	case card <= 0 && other > 0:
		return 0
	case card <= 0 || card > total:
		return total
	}
	return card
}

// onlineCardCharge returns what an online order charged to cards: the total less what gift cards and
// other non-card tenders paid (0 if they paid it all). The card amounts aren't added up as for a
// receipt: Target lists an online order's payments net of later refunds, and a bag fee refunded days
// after the charge left them below what the bank was charged.
func onlineCardCharge(tenders []targetgo.PaymentTransaction, total float64) float64 {
	other := 0.0
	for _, t := range tenders {
		if !t.IsCard() {
			other += float64(t.Amount)
		}
	}
	return math.Max(0, roundCents(total-other))
}

// orderDate is the day an order was placed, as seen where itemize runs, at midnight UTC like the
// other providers' order dates. Target's timestamps carry numeric offsets (in-store receipts come in
// Central time even for California stores), and a time whose zone is only an offset is stored in the
// dedup DB as "… -0500 -0500", which the SQLite driver can't read back.
func orderDate(s string) time.Time {
	t := targetgo.ParseTime(s)
	if t.IsZero() {
		return t
	}
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(values ...float64) float64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func roundCents(v float64) float64 {
	return math.Round(v*100) / 100
}
