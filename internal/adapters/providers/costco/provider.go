package costco

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	costcogo "github.com/costco-go/pkg/costco"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
)

// Provider implements the OrderProvider interface for Costco
type Provider struct {
	client    *costcogo.Client
	logger    *slog.Logger
	rateLimit time.Duration
}

// NewProvider creates a new Costco provider
func NewProvider(client *costcogo.Client, logger *slog.Logger) *Provider {
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{
		client:    client,
		logger:    logger.With(slog.String("provider", "costco")),
		rateLimit: 3 * time.Second, // Conservative rate limit for Costco
	}
}

// Name returns the provider identifier
func (p *Provider) Name() string {
	return "costco"
}

// DisplayName returns the human-readable name
func (p *Provider) DisplayName() string {
	return "Costco"
}

// FetchOrders fetches orders within the specified date range
func (p *Provider) FetchOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
	p.logger.Info("fetching orders",
		slog.Time("start_date", opts.StartDate),
		slog.Time("end_date", opts.EndDate),
		slog.Int("max_orders", opts.MaxOrders),
	)

	var allOrders []providers.Order

	// Fetch online orders
	onlineOrders, err := p.fetchOnlineOrders(ctx, opts)
	if err != nil {
		p.logger.Error("failed to fetch online orders", slog.String("error", err.Error()))
		// Continue to try receipts even if online orders fail
	} else {
		allOrders = append(allOrders, onlineOrders...)
	}

	// Fetch in-store receipts
	receipts, err := p.fetchReceipts(ctx, opts)
	if err != nil {
		p.logger.Error("failed to fetch receipts", slog.String("error", err.Error()))
	} else {
		allOrders = append(allOrders, receipts...)
	}

	// Apply max orders limit if specified
	if opts.MaxOrders > 0 && len(allOrders) > opts.MaxOrders {
		allOrders = allOrders[:opts.MaxOrders]
	}

	p.logger.Info("fetched orders",
		slog.Int("total", len(allOrders)),
	)

	return allOrders, nil
}

func (p *Provider) fetchOnlineOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
	startDate := opts.StartDate.Format("2006-01-02")
	endDate := opts.EndDate.Format("2006-01-02")

	pageSize := 10
	if opts.MaxOrders > 0 && opts.MaxOrders < pageSize {
		pageSize = opts.MaxOrders
	}

	response, err := p.client.GetOnlineOrders(ctx, startDate, endDate, 1, pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get online orders: %w", err)
	}

	var orders []providers.Order
	for _, onlineOrder := range response.BCOrders {
		order := p.convertOnlineOrder(&onlineOrder, opts.IncludeDetails)
		orders = append(orders, order)
	}

	return orders, nil
}

func (p *Provider) fetchReceipts(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
	startDate := opts.StartDate.Format("2006-01-02")
	endDate := opts.EndDate.Format("2006-01-02")

	// Fetch warehouse receipts
	response, err := p.client.GetReceipts(ctx, startDate, endDate, "all", "all")
	if err != nil {
		return nil, fmt.Errorf("failed to get receipts: %w", err)
	}

	var orders []providers.Order
	for _, receipt := range response.Receipts {
		// If we need details, always fetch the full receipt (items from GetReceipts are just placeholders)
		if opts.IncludeDetails {
			// Determine the correct documentType based on the receipt
			documentType := "warehouse" // default
			if receipt.DocumentType == "FuelReceipts" {
				documentType = "fuel"
			}

			fullReceipt, err := p.client.GetReceiptDetail(ctx, receipt.TransactionBarcode, documentType)
			if err != nil {
				p.logger.Warn("failed to get receipt details",
					slog.String("barcode", receipt.TransactionBarcode),
					slog.String("documentType", receipt.DocumentType),
					slog.String("error", err.Error()),
				)
				// Fall back to the summary data
				order := p.convertReceipt(&receipt, false)
				orders = append(orders, order)
			} else {
				order := p.convertReceipt(fullReceipt, true)
				orders = append(orders, order)
			}
		} else {
			// Just use the summary data
			order := p.convertReceipt(&receipt, false)
			orders = append(orders, order)
		}
	}

	return orders, nil
}

// GetOrderDetails fetches detailed information for a specific order
func (p *Provider) GetOrderDetails(ctx context.Context, orderID string) (providers.Order, error) {
	// Try to parse as online order first
	// For now, we'll need to determine if this is a receipt barcode or online order ID
	// This is a simplified implementation

	// Assume it's a receipt barcode if it looks like one
	if len(orderID) > 10 {
		receipt, err := p.client.GetReceiptDetail(ctx, orderID, "warehouse")
		if err != nil {
			return nil, fmt.Errorf("failed to get receipt details: %w", err)
		}
		return p.convertReceipt(receipt, true), nil
	}

	// Otherwise treat as online order (would need to implement proper lookup)
	return nil, fmt.Errorf("online order detail lookup not yet implemented")
}

// SupportsDeliveryTips indicates whether this provider tracks delivery tips
func (p *Provider) SupportsDeliveryTips() bool {
	return false // Costco doesn't have delivery tips in the same way as other services
}

// SupportsRefunds indicates whether this provider can handle refunds
func (p *Provider) SupportsRefunds() bool {
	return false // Not implemented yet
}

// SupportsBulkFetch indicates whether this provider can fetch multiple orders at once
func (p *Provider) SupportsBulkFetch() bool {
	return true
}

// GetRateLimit returns the minimum time between API calls
func (p *Provider) GetRateLimit() time.Duration {
	return p.rateLimit
}

// HealthCheck verifies the provider is working
func (p *Provider) HealthCheck(ctx context.Context) error {
	// Try to fetch a minimal amount of data to verify connection
	endDate := time.Now().Format("2006-01-02")
	startDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	_, err := p.client.GetOnlineOrders(ctx, startDate, endDate, 1, 1)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}

// convertOnlineOrder converts a Costco online order to the generic Order interface
func (p *Provider) convertOnlineOrder(order *costcogo.OnlineOrder, includeDetails bool) providers.Order {
	orderDate, _ := time.Parse("2006-01-02T15:04:05", order.OrderPlacedDate)

	var items []providers.OrderItem
	if includeDetails {
		for _, lineItem := range order.OrderLineItems {
			items = append(items, &CostcoOrderItem{
				name:        lineItem.ItemDescription,
				quantity:    1, // Costco doesn't provide quantity in the response
				sku:         lineItem.ItemNumber,
				description: lineItem.ItemDescription,
			})
		}
	}

	return &CostcoOrder{
		id:           order.OrderNumber,
		date:         orderDate,
		total:        order.OrderTotal,
		items:        items,
		providerName: "Costco",
		orderType:    "online",
		rawData:      order,
	}
}

// convertReceipt converts a Costco receipt to the generic Order interface
func (p *Provider) convertReceipt(receipt *costcogo.Receipt, hasDetails bool) providers.Order {
	// Try different date formats
	var receiptDate time.Time
	var err error

	// Try ISO date format first
	receiptDate, err = time.Parse("2006-01-02", receipt.TransactionDate)
	if err != nil {
		// Try datetime format
		receiptDate, err = time.Parse("2006-01-02T15:04:05", receipt.TransactionDateTime)
		if err != nil {
			// Default to current time if parsing fails
			receiptDate = time.Now()
		}
	}

	var items []providers.OrderItem
	if hasDetails {
		for _, item := range receipt.ItemArray {
			items = append(items, &CostcoOrderItem{
				name:        item.ItemDescription01,
				price:       item.Amount,
				quantity:    float64(item.Unit),
				unitPrice:   item.ItemUnitPriceAmount,
				sku:         item.ItemNumber,
				description: fmt.Sprintf("%s %s", item.ItemDescription01, item.ItemDescription02),
			})
		}
	}

	return &CostcoOrder{
		id:           receipt.TransactionBarcode,
		date:         receiptDate,
		total:        receipt.Total,
		subtotal:     receipt.SubTotal,
		tax:          receipt.Taxes,
		items:        items,
		providerName: "Costco",
		orderType:    "receipt",
		rawData:      receipt,
	}
}

// CostcoOrder implements the Order interface for Costco
type CostcoOrder struct {
	id           string
	date         time.Time
	total        float64
	subtotal     float64
	tax          float64
	tip          float64
	fees         float64
	items        []providers.OrderItem
	providerName string
	orderType    string // "online" or "receipt"
	rawData      interface{}
}

func (o *CostcoOrder) GetID() string                   { return o.id }
func (o *CostcoOrder) GetDate() time.Time              { return o.date }
func (o *CostcoOrder) GetTotal() float64               { return o.total }
func (o *CostcoOrder) GetSubtotal() float64            { return o.subtotal }
func (o *CostcoOrder) GetTax() float64                 { return o.tax }
func (o *CostcoOrder) GetTip() float64                 { return o.tip }
func (o *CostcoOrder) GetFees() float64                { return o.fees }
func (o *CostcoOrder) GetItems() []providers.OrderItem { return o.items }
func (o *CostcoOrder) GetProviderName() string         { return o.providerName }
func (o *CostcoOrder) GetRawData() interface{}         { return o.rawData }

// CostcoOrderItem implements the OrderItem interface for Costco
type CostcoOrderItem struct {
	name        string
	price       float64
	quantity    float64
	unitPrice   float64
	description string
	sku         string
	category    string
}

func (i *CostcoOrderItem) GetName() string        { return i.name }
func (i *CostcoOrderItem) GetPrice() float64      { return i.price }
func (i *CostcoOrderItem) GetQuantity() float64   { return i.quantity }
func (i *CostcoOrderItem) GetUnitPrice() float64  { return i.unitPrice }
func (i *CostcoOrderItem) GetDescription() string { return i.description }
func (i *CostcoOrderItem) GetSKU() string         { return i.sku }
func (i *CostcoOrderItem) GetCategory() string    { return i.category }
