// Package amazon provides an OrderProvider implementation backed by the
// github.com/eshaffer321/amazon-go client library.
package amazon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	amazongo "github.com/eshaffer321/amazon-go"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

// validProfilePattern matches alphanumeric, dash, and underscore characters only.
var validProfilePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var chromeVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)

type amazonClient interface {
	FetchOrders(ctx context.Context, opts amazongo.FetchOptions) ([]*amazongo.Order, error)
	FetchOrderWithTransactions(ctx context.Context, orderID string) (*amazongo.Order, []*amazongo.Transaction, error)
	FetchTransactions(ctx context.Context, orderID string) ([]*amazongo.Transaction, error)
	HealthCheck() error
	SaveCookies() error
}

// isValidProfile checks if an account name is safe to use as an Amazon cookie account.
func isValidProfile(profile string) bool {
	if profile == "" {
		return true
	}
	return validProfilePattern.MatchString(profile)
}

// Provider implements the OrderProvider interface for Amazon.
type Provider struct {
	logger      *slog.Logger
	rateLimit   time.Duration
	profile     string
	cookieFile  string
	fingerprint *amazongo.BrowserFingerprint
	client      amazonClient
}

// ProviderConfig holds configuration for the Amazon provider.
type ProviderConfig struct {
	Profile            string // Profile/account name for multi-account support
	CookieFile         string // Optional explicit amazon-go cookie file
	BrowserFingerprint *amazongo.BrowserFingerprint
}

// NewProvider creates a new Amazon provider.
func NewProvider(logger *slog.Logger, cfg *ProviderConfig) *Provider {
	if logger == nil {
		logger = slog.Default()
	}

	profile := ""
	cookieFile := ""
	var fingerprint *amazongo.BrowserFingerprint
	if cfg != nil {
		if cfg.Profile != "" {
			if isValidProfile(cfg.Profile) {
				profile = cfg.Profile
			} else {
				logger.Warn("invalid Amazon account name ignored (must be alphanumeric, dash, or underscore)",
					slog.String("profile", cfg.Profile))
			}
		}
		cookieFile = cfg.CookieFile
		if cfg.BrowserFingerprint != nil {
			copied := *cfg.BrowserFingerprint
			fingerprint = &copied
		}
	}

	return &Provider{
		logger:      logger.With(slog.String("provider", "amazon")),
		rateLimit:   time.Second,
		profile:     profile,
		cookieFile:  cookieFile,
		fingerprint: fingerprint,
	}
}

func guidedSetupBrowserFingerprint(profile string) (*amazongo.BrowserFingerprint, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	versionPath := filepath.Join(homeDir, ".itemize", "amazon", profile, "Last Version")
	// #nosec G304 -- profile is restricted to a single safe path component,
	// and versionPath remains inside Itemize's owner-controlled profile root.
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return nil, err
	}
	version := strings.TrimSpace(string(data))
	if !chromeVersionPattern.MatchString(version) {
		return nil, fmt.Errorf("invalid Chromium version %q in %s", version, versionPath)
	}
	majorVersion := strings.SplitN(version, ".", 2)[0]
	return &amazongo.BrowserFingerprint{
		UserAgent:       fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version),
		SecCHUA:         fmt.Sprintf(`"Chromium";v="%s", "Not A(Brand";v="24"`, majorVersion),
		SecCHUAMobile:   "?0",
		SecCHUAPlatform: `"macOS"`,
	}, nil
}

// NewProviderWithClient creates a provider with an injected client for tests.
func NewProviderWithClient(logger *slog.Logger, cfg *ProviderConfig, client amazonClient) *Provider {
	provider := NewProvider(logger, cfg)
	provider.client = client
	return provider
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "amazon"
}

// DisplayName returns the human-readable provider name.
func (p *Provider) DisplayName() string {
	return "Amazon"
}

// FetchOrders fetches orders from Amazon within the specified date range.
func (p *Provider) FetchOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
	p.logger.Info("fetching orders",
		slog.Time("start_date", opts.StartDate),
		slog.Time("end_date", opts.EndDate),
		slog.Int("max_orders", opts.MaxOrders),
	)

	client, err := p.getClient()
	if err != nil {
		return nil, err
	}
	if err := client.HealthCheck(); err != nil {
		return nil, p.authCheckError(err)
	}

	amazonOrders, err := client.FetchOrders(ctx, amazongo.FetchOptions{
		StartDate:      opts.StartDate,
		EndDate:        opts.EndDate,
		MaxOrders:      opts.MaxOrders,
		IncludeDetails: opts.IncludeDetails,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Amazon orders: %w", err)
	}

	orders := make([]providers.Order, 0, len(amazonOrders))
	for _, amazonOrder := range amazonOrders {
		if amazonOrder == nil {
			continue
		}

		transactions, err := client.FetchTransactions(ctx, amazonOrder.ID)
		if err != nil {
			p.logger.Warn("failed to fetch Amazon transactions",
				slog.String("order_id", amazonOrder.ID),
				slog.String("error", err.Error()))
		}

		parsedOrder := convertGoOrder(amazonOrder, transactions)
		orders = append(orders, NewOrder(parsedOrder, p.logger))
	}

	p.logger.Info("processed orders", slog.Int("count", len(orders)))
	p.saveCookies(client)

	return orders, nil
}

// GetOrderDetails fetches details for a specific order.
func (p *Provider) GetOrderDetails(ctx context.Context, orderID string) (providers.Order, error) {
	client, err := p.getClient()
	if err != nil {
		return nil, err
	}

	amazonOrder, transactions, err := client.FetchOrderWithTransactions(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Amazon order %q: %w", orderID, err)
	}
	if amazonOrder == nil {
		return nil, fmt.Errorf("amazon order %q not found", orderID)
	}
	p.saveCookies(client)

	return NewOrder(convertGoOrder(amazonOrder, transactions), p.logger), nil
}

// SupportsDeliveryTips returns whether Amazon supports delivery tips.
func (p *Provider) SupportsDeliveryTips() bool {
	return false
}

// SupportsRefunds returns whether Amazon supports refund tracking.
func (p *Provider) SupportsRefunds() bool {
	return true
}

// SupportsBulkFetch returns whether Amazon supports bulk order fetching.
func (p *Provider) SupportsBulkFetch() bool {
	return true
}

// GetRateLimit returns the rate limit for API requests.
func (p *Provider) GetRateLimit() time.Duration {
	return p.rateLimit
}

// HealthCheck verifies the provider can connect and authenticate.
func (p *Provider) HealthCheck(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	client, err := p.getClient()
	if err != nil {
		return err
	}
	if err := client.HealthCheck(); err != nil {
		return p.authCheckError(err)
	}
	return nil
}

// MerchantSearchTerms returns the merchant names to search for in Monarch.
func (p *Provider) MerchantSearchTerms() []string {
	return []string{
		"Amazon",
		"AMZN",
		"Amzn Mktp",
		"AMZN Mktp US",
		"Amazon.com",
		"Amazon Prime",
		"Prime Video",
		"Whole Foods",
	}
}

func (p *Provider) getClient() (amazonClient, error) {
	if p.client != nil {
		return p.client, nil
	}

	cookieFile, err := p.resolvedCookieFile()
	if err != nil {
		return nil, err
	}
	fingerprint, err := p.resolveBrowserFingerprint(cookieFile)
	if err != nil {
		return nil, err
	}

	opts := []amazongo.Option{
		amazongo.WithLogger(p.logger),
		amazongo.WithRateLimit(p.rateLimit),
		amazongo.WithAutoSave(false),
	}
	if fingerprint != nil {
		opts = append(opts, amazongo.WithBrowserFingerprint(*fingerprint))
	}
	if p.profile != "" {
		opts = append(opts, amazongo.WithAccount(p.profile))
	}
	if p.cookieFile != "" {
		opts = append(opts, amazongo.WithCookieFile(p.cookieFile))
	}

	client, err := amazongo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Amazon client: %w", err)
	}
	p.client = client
	return client, nil
}

func (p *Provider) resolveBrowserFingerprint(cookieFile string) (*amazongo.BrowserFingerprint, error) {
	if p.fingerprint != nil {
		copied := *p.fingerprint
		return &copied, nil
	}

	store, err := amazongo.NewCookieStore(cookieFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load Amazon session: %w", err)
	}
	if saved := store.BrowserFingerprint(); saved != nil {
		return saved, nil
	}
	if p.profile == "" {
		return nil, nil
	}

	fingerprint, err := guidedSetupBrowserFingerprint(p.profile)
	if err == nil {
		return fingerprint, nil
	}
	if !os.IsNotExist(err) {
		p.logger.Warn("failed to read Amazon browser version; using amazon-go default fingerprint",
			slog.String("error", err.Error()))
	}
	return nil, nil
}

func (p *Provider) saveCookies(client amazonClient) {
	if err := client.SaveCookies(); err != nil {
		p.logger.Warn("failed to save Amazon cookies", slog.String("error", err.Error()))
	}
}

func (p *Provider) loginCommand() string {
	accountArg := ""
	if p.profile != "" {
		accountArg = " -account " + p.profile
	}
	if p.cookieFile != "" {
		return fmt.Sprintf("run 'itemize amazon setup -account <name> -cookie-file %q' to authenticate", p.cookieFile)
	}
	return fmt.Sprintf("run 'itemize amazon setup%s' to authenticate", accountArg)
}

func (p *Provider) authCheckError(err error) error {
	message := err.Error()
	if strings.Contains(strings.ToLower(message), "cookies are expired") {
		message = "Amazon rejected the saved cookies or opened a sign-in page"
	}
	return fmt.Errorf("amazon auth check failed: %s. %s", message, p.loginCommand())
}

func convertGoOrder(order *amazongo.Order, transactions []*amazongo.Transaction) *ParsedOrder {
	parsed := &ParsedOrder{
		ID:       order.ID,
		Date:     order.Date,
		Total:    order.Total,
		Subtotal: order.Subtotal,
		Tax:      order.Tax,
		Shipping: order.ShippingFees,
		Items:    make([]*ParsedOrderItem, 0, len(order.Items)),
	}

	for _, item := range order.Items {
		if item == nil {
			continue
		}
		price := item.Price
		if price == 0 && item.UnitPrice != 0 && item.Quantity != 0 {
			price = item.UnitPrice * item.Quantity
		}
		quantity := int(item.Quantity)
		if quantity == 0 {
			quantity = 1
		}
		parsed.Items = append(parsed.Items, &ParsedOrderItem{
			Name:     item.Name,
			Price:    price,
			Quantity: quantity,
		})
	}

	for _, tx := range transactions {
		if tx == nil {
			continue
		}
		parsed.Transactions = append(parsed.Transactions, convertGoTransaction(tx))
	}

	return parsed
}

func convertGoTransaction(tx *amazongo.Transaction) *ParsedTransaction {
	txType := "charge"
	status := strings.ToLower(tx.Status)
	if strings.Contains(status, "refund") || strings.Contains(status, "refunded") {
		txType = "refund"
	}
	if strings.Contains(status, "pending") {
		txType = "pending"
	}

	description := tx.PaymentMethod
	if description == "" {
		description = tx.CardType
	}

	return &ParsedTransaction{
		Date:        tx.Date,
		Amount:      tx.Amount,
		Type:        txType,
		Last4:       tx.LastFour,
		Description: description,
	}
}
