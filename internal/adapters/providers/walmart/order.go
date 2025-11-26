package walmart

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	walmartclient "github.com/eshaffer321/walmart-client-go"
)

// Order wraps a Walmart order and implements providers.Order interface
type Order struct {
	walmartOrder *walmartclient.Order
	client       *walmartclient.WalmartClient
	logger       *slog.Logger

	// ledgerCache stores the order ledger to avoid duplicate API calls.
	// Note: Assumes single-threaded access per Order instance.
	// The sync orchestrator processes orders sequentially, so no mutex needed.
	ledgerCache *walmartclient.OrderLedger
}

// GetID returns the order ID
func (o *Order) GetID() string {
	return o.walmartOrder.ID
}

// GetDate returns the order date
func (o *Order) GetDate() time.Time {
	// Try multiple date formats that Walmart uses
	formats := []string{
		time.RFC3339,                   // "2006-01-02T15:04:05Z07:00"
		"2006-01-02T15:04:05.000-0700", // Walmart's format with milliseconds and timezone
		"2006-01-02T15:04:05-0700",     // Without milliseconds
		"2006-01-02",                   // Date only
	}

	for _, format := range formats {
		t, err := time.Parse(format, o.walmartOrder.OrderDate)
		if err == nil {
			return t
		}
	}

	// If all parsing fails, log warning and return zero time
	if o.logger != nil {
		o.logger.Warn("failed to parse order date with all formats",
			slog.String("order_id", o.walmartOrder.ID),
			slog.String("date_string", o.walmartOrder.OrderDate))
	}
	return time.Time{}
}

// GetTotal returns the total order amount (including tip if present)
func (o *Order) GetTotal() float64 {
	if o.walmartOrder.PriceDetails == nil {
		return 0.0
	}

	total := 0.0

	// Start with grand total
	if o.walmartOrder.PriceDetails.GrandTotal != nil {
		total = o.walmartOrder.PriceDetails.GrandTotal.Value
	}

	// Add driver tip if present
	if o.walmartOrder.PriceDetails.DriverTip != nil {
		total += o.walmartOrder.PriceDetails.DriverTip.Value
	}

	return total
}

// GetSubtotal returns the order subtotal (before tax and fees)
func (o *Order) GetSubtotal() float64 {
	if o.walmartOrder.PriceDetails == nil || o.walmartOrder.PriceDetails.SubTotal == nil {
		return 0.0
	}
	return o.walmartOrder.PriceDetails.SubTotal.Value
}

// GetTax returns the tax amount
func (o *Order) GetTax() float64 {
	if o.walmartOrder.PriceDetails == nil || o.walmartOrder.PriceDetails.TaxTotal == nil {
		return 0.0
	}
	return o.walmartOrder.PriceDetails.TaxTotal.Value
}

// GetTip returns the driver tip amount
func (o *Order) GetTip() float64 {
	if o.walmartOrder.PriceDetails == nil || o.walmartOrder.PriceDetails.DriverTip == nil {
		return 0.0
	}
	return o.walmartOrder.PriceDetails.DriverTip.Value
}

// GetFees returns the sum of all fees
func (o *Order) GetFees() float64 {
	if o.walmartOrder.PriceDetails == nil {
		return 0.0
	}

	total := 0.0
	for _, fee := range o.walmartOrder.PriceDetails.Fees {
		total += fee.Value
	}
	return total
}

// GetItems returns the order items
func (o *Order) GetItems() []providers.OrderItem {
	var items []providers.OrderItem

	for _, group := range o.walmartOrder.Groups {
		for _, item := range group.Items {
			items = append(items, &OrderItem{item: item})
		}
	}

	return items
}

// GetProviderName returns the provider name
func (o *Order) GetProviderName() string {
	return "walmart"
}

// GetRawData returns the raw Walmart order data
func (o *Order) GetRawData() interface{} {
	return o.walmartOrder
}

// OrderItem wraps a Walmart order item and implements providers.OrderItem interface
type OrderItem struct {
	item walmartclient.OrderItem
}

// GetName returns the item name
func (i *OrderItem) GetName() string {
	if i.item.ProductInfo != nil {
		return i.item.ProductInfo.Name
	}
	return ""
}

// GetPrice returns the line price (total for this item including quantity)
func (i *OrderItem) GetPrice() float64 {
	if i.item.PriceInfo != nil && i.item.PriceInfo.LinePrice != nil {
		return i.item.PriceInfo.LinePrice.Value
	}
	return 0.0
}

// GetQuantity returns the item quantity
func (i *OrderItem) GetQuantity() float64 {
	return i.item.Quantity
}

// GetUnitPrice returns the unit price
func (i *OrderItem) GetUnitPrice() float64 {
	if i.item.PriceInfo != nil && i.item.PriceInfo.UnitPrice != nil {
		return i.item.PriceInfo.UnitPrice.Value
	}
	return 0.0
}

// GetDescription returns the item description
func (i *OrderItem) GetDescription() string {
	if i.item.ProductInfo != nil {
		return i.item.ProductInfo.Name // Walmart doesn't separate name/description
	}
	return ""
}

// GetSKU returns the item SKU
func (i *OrderItem) GetSKU() string {
	if i.item.ProductInfo != nil {
		return i.item.ProductInfo.USItemID
	}
	return ""
}

// GetCategory returns the item category (if available from provider)
func (i *OrderItem) GetCategory() string {
	// Walmart API doesn't provide category in order items
	return ""
}

// GetFinalCharges returns the actual bank charges for this order
// Returns multiple charges for multi-delivery orders
// Uses cached ledger result to avoid duplicate API calls
func (o *Order) GetFinalCharges() ([]float64, error) {
	var ledger *walmartclient.OrderLedger

	// Check cache first
	if o.ledgerCache != nil {
		ledger = o.ledgerCache
	} else {
		// Client is required for ledger API
		if o.client == nil {
			return nil, fmt.Errorf("client not available")
		}

		// Fetch ledger from API
		var err error
		ledger, err = o.client.GetOrderLedger(o.GetID())
		if err != nil {
			return nil, fmt.Errorf("failed to get order ledger: %w", err)
		}

		// Cache for future calls
		o.ledgerCache = ledger
	}

	// Extract and validate charges
	if len(ledger.PaymentMethods) == 0 {
		return nil, fmt.Errorf("no payment methods in ledger")
	}

	charges := ledger.PaymentMethods[0].FinalCharges

	// Validate charges
	if len(charges) == 0 {
		return nil, fmt.Errorf("no final charges in ledger")
	}

	// TODO: Handle refund transactions (negative charges)
	// Currently we skip negative charges in the ledger, which means refund
	// transactions in Monarch Money won't be categorized. Future enhancement:
	// 1. Match the refund transaction (positive amount in Monarch)
	// 2. Determine which item(s) were refunded from the ledger
	// 3. Categorize the refund with the same category as the refunded item

	// Filter to positive charges only (actual bank charges)
	// Skip negative charges (refunds/credits) and zero charges
	var positiveCharges []float64
	for _, charge := range charges {
		if charge > 0 {
			positiveCharges = append(positiveCharges, charge)
		} else if charge < 0 {
			if o.logger != nil {
				o.logger.Warn("Skipping refund in ledger (not yet supported)",
					"order_id", o.GetID(),
					"refund_amount", charge)
			}
		}
	}

	if len(positiveCharges) == 0 {
		return nil, fmt.Errorf("no positive charges found (order may be fully refunded)")
	}

	return positiveCharges, nil
}

// IsMultiDelivery checks if order was split into multiple deliveries
// Returns true if there are multiple final charges
func (o *Order) IsMultiDelivery() (bool, error) {
	charges, err := o.GetFinalCharges()
	if err != nil {
		return false, err
	}
	return len(charges) > 1, nil
}
