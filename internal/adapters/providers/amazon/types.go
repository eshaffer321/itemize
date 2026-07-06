package amazon

import "time"

// ParsedShipment is the internal representation of a shipment group.
type ParsedShipment struct {
	Status string
	Date   time.Time
	Items  []*ParsedOrderItem
}

// ParsedOrder is the internal representation used by itemize's Amazon handler.
type ParsedOrder struct {
	ID           string
	Date         time.Time
	Total        float64
	Subtotal     float64
	Tax          float64
	Shipping     float64
	Items        []*ParsedOrderItem
	Shipments    []*ParsedShipment
	Transactions []*ParsedTransaction
}

// ParsedOrderItem is the internal representation of an order item.
type ParsedOrderItem struct {
	Name     string
	Price    float64
	Quantity int
}

// ParsedTransaction is the internal representation of a transaction.
type ParsedTransaction struct {
	Date        time.Time
	Amount      float64
	Type        string // "charge" or "refund"
	Last4       string
	Description string
}
