package amazon

import (
	"errors"
	"log/slog"
	"math"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

// ErrPaymentPending indicates an order has no bank charges yet because it hasn't shipped
var ErrPaymentPending = errors.New("payment pending: order has not been charged yet (awaiting shipment)")

// ErrGiftCardOrder indicates an order was paid entirely with gift cards or points —
// no bank transaction exists in Monarch to match against.
var ErrGiftCardOrder = errors.New("no bank charges found (order paid entirely with gift cards/points)")

// Order wraps a ParsedOrder to implement the providers.Order interface
type Order struct {
	parsedOrder *ParsedOrder
	items       []providers.OrderItem
	logger      *slog.Logger
}

// NewOrder creates a new Order adapter from a ParsedOrder
func NewOrder(parsedOrder *ParsedOrder, logger *slog.Logger) *Order {
	items := make([]providers.OrderItem, 0, len(parsedOrder.Items))
	for _, item := range parsedOrder.Items {
		items = append(items, &OrderItem{parsedItem: item})
	}
	return &Order{
		parsedOrder: parsedOrder,
		items:       items,
		logger:      logger,
	}
}

// GetID returns the order ID
func (o *Order) GetID() string {
	return o.parsedOrder.ID
}

// GetDate returns the order date
func (o *Order) GetDate() time.Time {
	return o.parsedOrder.Date
}

// GetTotal returns the total order amount
func (o *Order) GetTotal() float64 {
	return o.parsedOrder.Total
}

// GetSubtotal returns the subtotal before tax and fees
func (o *Order) GetSubtotal() float64 {
	return o.parsedOrder.Subtotal
}

// GetTax returns the tax amount
func (o *Order) GetTax() float64 {
	return o.parsedOrder.Tax
}

// GetTip returns the tip amount (Amazon doesn't support tips)
func (o *Order) GetTip() float64 {
	return 0
}

// GetFees returns shipping and handling fees
func (o *Order) GetFees() float64 {
	return o.parsedOrder.Shipping
}

// GetItems returns all items in the order
func (o *Order) GetItems() []providers.OrderItem {
	return o.items
}

// GetProviderName returns the provider name
func (o *Order) GetProviderName() string {
	return "Amazon"
}

// GetRawData returns the underlying parsed order
func (o *Order) GetRawData() interface{} {
	return o.parsedOrder
}

// GetFinalCharges returns the actual bank charges for this order
// Returns multiple charges for multi-shipment orders
// Filters out non-bank transactions like gift cards, points, etc.
func (o *Order) GetFinalCharges() ([]float64, error) {
	if len(o.parsedOrder.Transactions) == 0 {
		// No transactions at all - order hasn't been charged yet (awaiting shipment)
		return nil, ErrPaymentPending
	}

	var bankCharges []float64
	var hasNonBankPayments bool
	for _, tx := range o.parsedOrder.Transactions {
		// Skip refunds
		if tx.Type == "refund" {
			if o.logger != nil {
				o.logger.Warn("Skipping refund transaction (not yet supported)",
					"order_id", o.GetID(),
					"refund_amount", tx.Amount)
			}
			continue
		}

		// Skip non-positive amounts
		if tx.Amount <= 0 {
			continue
		}

		// Filter out non-bank transactions
		// Real bank charges have Last4 populated (card ending digits)
		// Points, gift cards, etc. have empty Last4
		if tx.Last4 == "" {
			hasNonBankPayments = true
			if o.logger != nil {
				o.logger.Debug("Skipping non-bank transaction",
					"order_id", o.GetID(),
					"amount", tx.Amount,
					"description", tx.Description,
					"reason", "no card digits - likely points or gift card")
			}
			continue
		}

		bankCharges = append(bankCharges, tx.Amount)
		if o.logger != nil {
			o.logger.Debug("Found bank charge in transaction",
				"order_id", o.GetID(),
				"amount", tx.Amount,
				"last4", tx.Last4,
				"date", tx.Date)
		}
	}

	if len(bankCharges) == 0 {
		if hasNonBankPayments {
			// Order was paid entirely with gift cards/points - no bank transaction to match
			return nil, ErrGiftCardOrder
		}
		// No bank charges and no non-bank payments processed yet - still pending
		return nil, ErrPaymentPending
	}

	return bankCharges, nil
}

// GetNonBankAmount returns the total amount paid via non-bank methods
// (Amazon Visa points, gift cards, promotional credits, etc.)
// These won't appear in Monarch as they aren't actual bank transactions
func (o *Order) GetNonBankAmount() (float64, error) {
	var nonBankTotal float64
	for _, tx := range o.parsedOrder.Transactions {
		// Skip refunds and non-positive amounts
		if tx.Type == "refund" || tx.Amount <= 0 {
			continue
		}

		// Non-bank transactions have empty Last4
		if tx.Last4 == "" {
			nonBankTotal += tx.Amount
			if o.logger != nil {
				o.logger.Debug("Found non-bank payment",
					"order_id", o.GetID(),
					"amount", tx.Amount,
					"description", tx.Description)
			}
		}
	}

	return nonBankTotal, nil
}

// GetItemsForCharge returns the items belonging to the shipment that best
// matches the given bank charge amount. When shipment data is available this
// lets each Monarch transaction be split using only the items that were
// actually in that box rather than pro-rating the entire order.
//
// Matching works by estimating each shipment's charge as:
//
//	shipmentSubtotal * (1 + taxRate)
//
// where taxRate = order.Tax / order.Subtotal. The shipment whose estimate is
// closest to chargeAmount wins. Falls back to all order items when no
// shipment data is present or when the order has only one shipment.
func (o *Order) GetItemsForCharge(chargeAmount float64) []providers.OrderItem {
	if len(o.parsedOrder.Shipments) <= 1 {
		return o.items
	}

	taxRate := 0.0
	if o.parsedOrder.Subtotal > 0 {
		taxRate = o.parsedOrder.Tax / o.parsedOrder.Subtotal
	}

	bestIdx := -1
	bestDiff := math.MaxFloat64
	for i, shipment := range o.parsedOrder.Shipments {
		var subtotal float64
		for _, item := range shipment.Items {
			subtotal += item.Price * float64(item.Quantity)
		}
		estimated := subtotal * (1 + taxRate)
		diff := math.Abs(estimated - chargeAmount)
		if diff < bestDiff {
			bestDiff = diff
			bestIdx = i
		}
	}

	if bestIdx < 0 {
		return o.items
	}

	shipment := o.parsedOrder.Shipments[bestIdx]
	items := make([]providers.OrderItem, 0, len(shipment.Items))
	for _, item := range shipment.Items {
		items = append(items, &OrderItem{parsedItem: item})
	}
	if len(items) == 0 {
		return o.items
	}
	return items
}

// IsMultiDelivery checks if order was split into multiple shipments/charges
// Returns true if there are multiple final charges
func (o *Order) IsMultiDelivery() (bool, error) {
	charges, err := o.GetFinalCharges()
	if err != nil {
		return false, err
	}
	return len(charges) > 1, nil
}

// GetTransactionDates returns the charge dates from transactions
// This is useful for matching to Monarch transactions since charge date
// may differ from order date
func (o *Order) GetTransactionDates() []time.Time {
	var dates []time.Time
	for _, tx := range o.parsedOrder.Transactions {
		if tx.Type == "charge" && !tx.Date.IsZero() {
			dates = append(dates, tx.Date)
		}
	}
	return dates
}

// OrderItem wraps a ParsedOrderItem to implement the providers.OrderItem interface
type OrderItem struct {
	parsedItem *ParsedOrderItem
}

// GetName returns the item name
func (i *OrderItem) GetName() string {
	return i.parsedItem.Name
}

// GetPrice returns the line total for this item
func (i *OrderItem) GetPrice() float64 {
	return i.parsedItem.Price
}

// GetQuantity returns the quantity of this item
func (i *OrderItem) GetQuantity() float64 {
	return float64(i.parsedItem.Quantity)
}

// GetUnitPrice returns the unit price of this item
func (i *OrderItem) GetUnitPrice() float64 {
	if i.parsedItem.Quantity > 0 {
		return i.parsedItem.Price / float64(i.parsedItem.Quantity)
	}
	return i.parsedItem.Price
}

// GetDescription returns the item description (same as name for Amazon)
func (i *OrderItem) GetDescription() string {
	return i.parsedItem.Name
}

// GetSKU returns the SKU (not available from CLI output)
func (i *OrderItem) GetSKU() string {
	return ""
}

// GetCategory returns the item category (not available from CLI output)
func (i *OrderItem) GetCategory() string {
	return ""
}

// Ensure interfaces are implemented at compile time
var (
	_ providers.Order     = (*Order)(nil)
	_ providers.OrderItem = (*OrderItem)(nil)
)
