package amazon

import "time"

// CLIOutput represents the JSON output from amazon-order-scraper CLI
type CLIOutput struct {
	Orders []CLIOrder `json:"orders"`
}

// CLIShipment represents a shipment group from the CLI output
type CLIShipment struct {
	Status string         `json:"status"` // "Delivered", "Arriving", "Shipped"
	Date   string         `json:"date"`   // ISO 8601: "2025-12-15"
	Items  []CLIOrderItem `json:"items"`
}

// CLIOrder represents an order from the CLI output
type CLIOrder struct {
	OrderID      string           `json:"orderId"`
	OrderDate    string           `json:"orderDate"`    // ISO 8601: "2025-12-13"
	Total        string           `json:"total"`        // "$116.20"
	Subtotal     string           `json:"subtotal"`     // "$109.62"
	Tax          string           `json:"tax"`          // "$6.58"
	Shipping     string           `json:"shipping"`     // "$0.00"
	Items        []CLIOrderItem   `json:"items"`
	Shipments    []CLIShipment    `json:"shipments"`
	Transactions []CLITransaction `json:"transactions"`
}

// CLIOrderItem represents an item from the CLI output
type CLIOrderItem struct {
	Name     string `json:"name"`
	Price    string `json:"price"`    // "$14.99"
	Quantity int    `json:"quantity"` // numeric
}

// CLITransaction represents a payment transaction from the CLI output
type CLITransaction struct {
	Date        string `json:"date"`        // ISO 8601: "2025-12-13"
	Amount      string `json:"amount"`      // "$116.20"
	Type        string `json:"type"`        // "charge" or "refund"
	Last4       string `json:"last4"`       // "1211"
	Description string `json:"description"` // "Prime Visa ****1211..."
}

// ParsedShipment is the internal representation of a shipment group
type ParsedShipment struct {
	Status string
	Date   time.Time
	Items  []*ParsedOrderItem
}

// ParsedOrder is the internal representation after parsing CLI output
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

// ParsedOrderItem is the internal representation of an order item
type ParsedOrderItem struct {
	Name     string
	Price    float64
	Quantity int
}

// ParsedTransaction is the internal representation of a transaction
type ParsedTransaction struct {
	Date        time.Time
	Amount      float64
	Type        string // "charge" or "refund"
	Last4       string
	Description string
}
