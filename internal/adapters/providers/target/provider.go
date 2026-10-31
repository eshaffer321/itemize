package target

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	targetgo "github.com/daslicious/target-go"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

// API is the part of targetgo.Client the provider uses (replaced in tests).
type API interface {
	OrderHistory(ctx context.Context, purchaseType string, page, pageSize int) (*targetgo.OrderHistoryResponse, error)
	OnlineOrder(ctx context.Context, orderNumber string) (*targetgo.OnlineOrder, error)
	StoreOrder(ctx context.Context, receiptID string) (*targetgo.StoreOrder, error)
	Invoices(ctx context.Context, orderNumber string) (*targetgo.InvoiceList, error)
	Invoice(ctx context.Context, orderNumber, invoiceID string) (*targetgo.Invoice, error)
}

// Provider implements providers.OrderProvider for Target: in-store receipts (linked to the Target
// Circle account) and online orders.
type Provider struct {
	api      API
	logger   *slog.Logger
	pageSize int
	maxPages int
}

// NewProvider creates a Target provider.
func NewProvider(api API, logger *slog.Logger) *Provider {
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{api: api, logger: logger, pageSize: 10, maxPages: 10}
}

func (p *Provider) Name() string                { return "target" }
func (p *Provider) DisplayName() string         { return "Target" }
func (p *Provider) SupportsDeliveryTips() bool  { return false }
func (p *Provider) SupportsRefunds() bool       { return false }
func (p *Provider) SupportsBulkFetch() bool     { return true }
func (p *Provider) GetRateLimit() time.Duration { return 3 * time.Second }

// FetchOrders lists in-store and online orders placed between opts.StartDate and opts.EndDate and,
// with IncludeDetails, reads each one's items (an online order charged in parts gives one Order per
// charge). A rejected login or a bot challenge fails the whole fetch; any other error on a single
// order is logged and that order is left for the next run.
func (p *Provider) FetchOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
	var orders []providers.Order
	for _, purchaseType := range []string{targetgo.PurchaseStore, targetgo.PurchaseOnline} {
		summaries, err := p.listSince(ctx, purchaseType, opts.StartDate)
		if err != nil {
			return nil, err
		}
		for _, s := range summaries {
			if opts.MaxOrders > 0 && len(orders) >= opts.MaxOrders {
				return orders, nil
			}
			if skip := p.skipReason(s, opts); skip != "" {
				p.logger.Debug("skipping Target order", "order_id", s.ID(), "reason", skip)
				continue
			}
			if !opts.IncludeDetails {
				orders = append(orders, convertSummary(s, purchaseType))
				continue
			}
			parts, err := p.details(ctx, s, purchaseType)
			if err != nil {
				if fatal(err) {
					return nil, err
				}
				p.logger.Error("failed to fetch Target order", "order_id", s.ID(), "error", err)
				continue
			}
			for _, order := range parts {
				orders = append(orders, order)
			}
		}
	}
	p.logger.Info("fetched Target orders", "count", len(orders))
	return orders, nil
}

// laterCharge matches the id of a later charge of an online order charged in parts: <order number>-<n>.
var laterCharge = regexp.MustCompile(`^([0-9]+)-[0-9]+$`)

// GetOrderDetails fetches one order as the sync sees it: an in-store receipt, an online order, or one
// charge of an online order charged in parts.
func (p *Provider) GetOrderDetails(ctx context.Context, orderID string) (providers.Order, error) {
	orders, err := p.FetchOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	for _, order := range orders {
		if order.GetID() == orderID {
			return order, nil
		}
	}
	if laterCharge.MatchString(orderID) {
		return nil, fmt.Errorf("target: no charge %s (the order has %d charged to a card)", orderID, len(orders))
	}
	if len(orders) == 0 || laterCharge.MatchString(orders[0].GetID()) {
		return nil, fmt.Errorf("target: %s wasn't charged to a card (paid with gift cards, or cancelled)", orderID)
	}
	return orders[0], nil
}

// FetchOrder fetches the order orderID belongs to: one Order, or one per charge for an online order
// charged in parts. Receipt ids are digit groups joined by dashes, online order numbers are digits,
// and later charges of an online order are <order number>-2, -3, ...
func (p *Provider) FetchOrder(ctx context.Context, orderID string) ([]providers.Order, error) {
	summary, purchaseType := targetgo.OrderSummary{OrderNumber: orderID}, targetgo.PurchaseOnline
	if m := laterCharge.FindStringSubmatch(orderID); m != nil {
		summary.OrderNumber = m[1]
	} else if strings.Contains(orderID, "-") {
		summary, purchaseType = targetgo.OrderSummary{StoreReceiptID: orderID}, targetgo.PurchaseStore
	}
	parts, err := p.details(ctx, summary, purchaseType)
	if err != nil {
		return nil, err
	}
	orders := make([]providers.Order, len(parts))
	for i, order := range parts {
		orders[i] = order
	}
	return orders, nil
}

// HealthCheck reads the first page of online orders, which fails if the login has expired.
func (p *Provider) HealthCheck(ctx context.Context) error {
	_, err := p.api.OrderHistory(ctx, targetgo.PurchaseOnline, 1, 1)
	return err
}

// details reads an order's items. An online order shipped in several packages can be charged once
// per shipment, and each charge is its own transaction in Monarch, so an order charged more than once
// is read from its invoices: one Order per charge, oldest first. The first keeps the order number as
// its id, so the id doesn't change from when the order had one charge; later charges are
// <order number>-2, -3, ... An order charged once (or not yet) is read whole: until a later shipment
// is charged, its total won't match the first charge, and it waits like an order not charged yet.
// Orders and charges paid without a card are left out (see charged).
func (p *Provider) details(ctx context.Context, s targetgo.OrderSummary, purchaseType string) ([]*Order, error) {
	if purchaseType == targetgo.PurchaseStore {
		receipt, err := p.api.StoreOrder(ctx, s.ID())
		if err != nil {
			return nil, err
		}
		order, err := convertStore(receipt, s)
		if err != nil {
			return nil, err
		}
		return p.charged([]*Order{order}), nil
	}
	online, err := p.api.OnlineOrder(ctx, s.ID())
	if err != nil {
		return nil, err
	}
	number := firstNonEmpty(online.OrderNumber, s.ID())
	list, err := p.api.Invoices(ctx, number)
	if err != nil {
		return nil, fmt.Errorf("reading the invoices of order %s: %w", number, err)
	}
	charges := list.Charges()
	if len(charges) < 2 {
		whole, err := convertOnline(online, s)
		if err != nil {
			return nil, err
		}
		return p.charged([]*Order{whole}), nil
	}

	orders := make([]*Order, 0, len(charges))
	for i, charge := range charges {
		invoice, err := p.api.Invoice(ctx, number, charge.ID)
		if err != nil {
			return nil, fmt.Errorf("reading invoice %s of order %s: %w", charge.ID, number, err)
		}
		id := number
		if i > 0 {
			id = fmt.Sprintf("%s-%d", number, i+1)
		}
		order, err := convertInvoice(invoice, online, id)
		if err != nil {
			return nil, fmt.Errorf("invoice %s of order %s: %w", charge.ID, number, err)
		}
		orders = append(orders, order)
	}
	p.logger.Debug("Target order charged in parts", "order_id", number, "charges", len(orders))
	return p.charged(orders), nil
}

// charged leaves out the orders (or charges) nothing was charged to a card for: paid with gift cards,
// or cancelled. No bank transaction will ever match them. Later charges keep their ids.
func (p *Provider) charged(orders []*Order) []*Order {
	kept := make([]*Order, 0, len(orders))
	for _, order := range orders {
		if order.total <= 0 {
			p.logger.Info("Target order not charged to a card, nothing to match", "order_id", order.id)
			continue
		}
		kept = append(kept, order)
	}
	return kept
}

// listSince pages through the order history (newest first) until it passes start.
func (p *Provider) listSince(ctx context.Context, purchaseType string, start time.Time) ([]targetgo.OrderSummary, error) {
	all, complete, err := targetgo.HistorySince(ctx, p.api, purchaseType, start, p.pageSize, p.maxPages)
	if err != nil {
		return nil, fmt.Errorf("listing Target %s orders: %w", strings.ToLower(purchaseType), err)
	}
	if !complete {
		p.logger.Warn("stopped listing Target orders at the page limit", "purchase_type", purchaseType, "pages", p.maxPages)
	}
	return all, nil
}

func (p *Provider) skipReason(s targetgo.OrderSummary, opts providers.FetchOptions) string {
	if s.ID() == "" {
		return "no order id"
	}
	if strings.Contains(strings.ToLower(s.OrderType), "return") {
		return "return"
	}
	placed := targetgo.ParseTime(s.Date())
	if placed.IsZero() {
		return "unreadable date " + s.Date()
	}
	if placed.Before(opts.StartDate) || (!opts.EndDate.IsZero() && placed.After(opts.EndDate)) {
		return "outside date range"
	}
	if s.Summary.GrandTotal <= 0 && len(s.Lines()) == 0 {
		return "no total"
	}
	return ""
}

// convertSummary builds an Order from a history entry alone (no item prices).
func convertSummary(s targetgo.OrderSummary, purchaseType string) *Order {
	kind := KindOnline
	if purchaseType == targetgo.PurchaseStore {
		kind = KindStore
	}
	total := float64(s.Summary.GrandTotal)
	return buildOrder(s.ID(), kind, orderDate(s.Date()), total, total, s.Lines(), s)
}

func fatal(err error) bool {
	return errors.Is(err, targetgo.ErrNotAuthenticated) || errors.Is(err, targetgo.ErrBlocked) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
