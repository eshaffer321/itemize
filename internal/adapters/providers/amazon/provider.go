// Package amazon provides an OrderProvider implementation that fetches Amazon orders
// by shelling out to the amazon-order-scraper CLI (npm package).
//
// The CLI must be installed globally or available via npx:
//
//	npm install -g amazon-order-scraper
//
// Authentication is managed by the CLI - run `amazon-scraper --login` to authenticate.
package amazon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

// validProfilePattern matches alphanumeric, dash, and underscore characters only
var validProfilePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// allowedCLIArgsPattern limits arguments to the scraper flags and scalar values
// produced by buildCLIArgs.
var allowedCLIArgsPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const (
	amazonScraperCommand = "amazon-scraper"
	npxCommand           = "npx"
	scraperPackageName   = "amazon-order-scraper"
)

type cliCommand struct {
	name   string
	useNpx bool
}

// isValidProfile checks if a profile name is safe to pass to the CLI
func isValidProfile(profile string) bool {
	if profile == "" {
		return true
	}
	return validProfilePattern.MatchString(profile)
}

// Provider implements the OrderProvider interface for Amazon
// It shells out to the amazon-order-scraper CLI (npm package)
type Provider struct {
	logger         *slog.Logger
	rateLimit      time.Duration
	profile        string // Optional profile name for multi-account support
	headless       bool   // Run browser in headless mode
	browserDataDir string // Base directory for persistent Amazon browser profiles
}

// ProviderConfig holds configuration for the Amazon provider
type ProviderConfig struct {
	Profile        string // Profile name for multi-account support
	Headless       bool   // Run in headless mode (for automated/cron runs)
	BrowserDataDir string // Base directory for persistent browser profiles
}

// NewProvider creates a new Amazon provider
func NewProvider(logger *slog.Logger, cfg *ProviderConfig) *Provider {
	if logger == nil {
		logger = slog.Default()
	}

	profile := ""
	headless := false
	browserDataDir := ""
	if cfg != nil {
		// Validate profile name to prevent command injection
		if cfg.Profile != "" {
			if isValidProfile(cfg.Profile) {
				profile = cfg.Profile
			} else {
				logger.Warn("invalid profile name ignored (must be alphanumeric, dash, or underscore)",
					slog.String("profile", cfg.Profile))
			}
		}
		headless = cfg.Headless
		browserDataDir = cfg.BrowserDataDir
	}

	return &Provider{
		logger:         logger.With(slog.String("provider", "amazon")),
		rateLimit:      1 * time.Second,
		profile:        profile,
		headless:       headless,
		browserDataDir: browserDataDir,
	}
}

// Name returns the provider identifier
func (p *Provider) Name() string {
	return "amazon"
}

// DisplayName returns the human-readable provider name
func (p *Provider) DisplayName() string {
	return "Amazon"
}

// FetchOrders fetches orders from Amazon within the specified date range
// by shelling out to the amazon-order-scraper CLI
func (p *Provider) FetchOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
	p.logger.Info("fetching orders",
		slog.Time("start_date", opts.StartDate),
		slog.Time("end_date", opts.EndDate),
		slog.Int("max_orders", opts.MaxOrders),
	)

	// Build CLI arguments
	args := p.buildCLIArgs(opts)

	// Find and execute CLI
	output, err := p.executeCLI(ctx, args)
	if err != nil {
		return nil, err
	}

	// Parse CLI output
	cliOutput, err := ParseCLIOutputBytes(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CLI output: %w", err)
	}

	p.logger.Info("fetched orders from CLI", slog.Int("count", len(cliOutput.Orders)))

	// Convert to provider interface
	orders := make([]providers.Order, 0, len(cliOutput.Orders))
	for _, cliOrder := range cliOutput.Orders {
		parsedOrder, err := ConvertCLIOrder(cliOrder)
		if err != nil {
			p.logger.Warn("failed to parse order, skipping",
				slog.String("order_id", cliOrder.OrderID),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Filter by date range if specified
		if !opts.StartDate.IsZero() && parsedOrder.Date.Before(opts.StartDate) {
			continue
		}
		if !opts.EndDate.IsZero() && parsedOrder.Date.After(opts.EndDate) {
			continue
		}

		orders = append(orders, NewOrder(parsedOrder, p.logger))
	}

	// Apply max orders limit if specified
	if opts.MaxOrders > 0 && len(orders) > opts.MaxOrders {
		orders = orders[:opts.MaxOrders]
	}

	p.logger.Info("processed orders", slog.Int("count", len(orders)))

	return orders, nil
}

// buildCLIArgs builds the command line arguments for amazon-order-scraper
func (p *Provider) buildCLIArgs(opts providers.FetchOptions) []string {
	var args []string

	// Date range options
	if !opts.StartDate.IsZero() {
		args = append(args, "--since", opts.StartDate.Format("2006-01-02"))
	}
	if !opts.EndDate.IsZero() {
		args = append(args, "--until", opts.EndDate.Format("2006-01-02"))
	}

	// If no date range specified, calculate from lookback days
	// Default to last 14 days if nothing specified
	if opts.StartDate.IsZero() && opts.EndDate.IsZero() {
		args = append(args, "--days", "14")
	}

	// Profile for multi-account support
	if p.profile != "" {
		args = append(args, "--profile", p.profile)
	}

	// Headless mode for automated runs
	if p.headless {
		args = append(args, "--headless")
	}

	// Always output to stdout for parsing
	args = append(args, "--stdout")

	return args
}

// executeCLI executes the amazon-order-scraper CLI and returns the output
func (p *Provider) executeCLI(ctx context.Context, args []string) ([]byte, error) {
	// Try to find the CLI
	cli, err := p.findCLI()
	if err != nil {
		return nil, err
	}
	if err := validateCLIArgs(args); err != nil {
		return nil, err
	}

	var cmd *exec.Cmd
	// Keep executable names literal for static analysis; args are validated before assignment.
	if cli.useNpx {
		// Use npx to run the package
		cmd = exec.CommandContext(ctx, "npx")
		cmd.Args = append([]string{npxCommand, scraperPackageName}, args...)
		p.logger.Debug("executing CLI via npx", slog.String("args", fmt.Sprintf("%v", cmd.Args[1:])))
	} else {
		// Direct execution
		cmd = exec.CommandContext(ctx, "amazon-scraper")
		cmd.Args = append([]string{amazonScraperCommand}, args...)
		p.logger.Debug("executing CLI directly", slog.String("command", cli.name), slog.String("args", fmt.Sprintf("%v", args)))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.MultiWriter(&stderr, os.Stderr) // stream logs to terminal in real-time
	if p.browserDataDir != "" {
		cmd.Env = append(os.Environ(), "BROWSER_DATA_DIR="+p.browserDataDir)
	}

	err = cmd.Run()
	if err != nil {
		// Check exit code for specific errors
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			switch exitCode {
			case 2:
				return nil, fmt.Errorf("amazon login required: %s", p.loginCommand())
			default:
				return nil, fmt.Errorf("CLI failed (exit %d): %s", exitCode, stderr.String())
			}
		}
		return nil, fmt.Errorf("failed to execute CLI: %w", err)
	}

	return stdout.Bytes(), nil
}

func (p *Provider) loginCommand() string {
	profileArg := ""
	if p.profile != "" {
		profileArg = " --profile " + p.profile
	}
	if p.browserDataDir != "" {
		return fmt.Sprintf("run 'BROWSER_DATA_DIR=%q npx -y amazon-order-scraper --login%s' to authenticate", p.browserDataDir, profileArg)
	}
	return fmt.Sprintf("run 'amazon-scraper --login%s' to authenticate", profileArg)
}

// findCLI locates the amazon-order-scraper CLI
// Returns the path and whether to use npx
func (p *Provider) findCLI() (cliCommand, error) {
	// First, try to find globally installed CLI
	if _, err := exec.LookPath(amazonScraperCommand); err == nil {
		return cliCommand{name: amazonScraperCommand}, nil
	}

	// Fall back to npx
	if _, err := exec.LookPath(npxCommand); err == nil {
		return cliCommand{name: npxCommand, useNpx: true}, nil
	}

	return cliCommand{}, fmt.Errorf("amazon-order-scraper CLI not available: install %q or %q", amazonScraperCommand, npxCommand)
}

func validateCLIArgs(args []string) error {
	for _, arg := range args {
		switch arg {
		case "--since", "--until", "--days", "--profile", "--headless", "--stdout":
			continue
		}
		if strings.HasPrefix(arg, "--") {
			return fmt.Errorf("unsupported amazon CLI flag: %q", arg)
		}
		if !allowedCLIArgsPattern.MatchString(arg) {
			return fmt.Errorf("unsafe amazon CLI argument: %q", arg)
		}
	}

	return nil
}

// GetOrderDetails fetches details for a specific order
// Note: The CLI doesn't support fetching a single order by ID,
// so this is not implemented
func (p *Provider) GetOrderDetails(ctx context.Context, orderID string) (providers.Order, error) {
	return nil, fmt.Errorf("GetOrderDetails not supported for Amazon CLI provider")
}

// SupportsDeliveryTips returns whether Amazon supports delivery tips
func (p *Provider) SupportsDeliveryTips() bool {
	return false // Amazon doesn't have delivery tips like grocery services
}

// SupportsRefunds returns whether Amazon supports refund tracking
func (p *Provider) SupportsRefunds() bool {
	return true // Amazon has refund transactions
}

// SupportsBulkFetch returns whether Amazon supports bulk order fetching
func (p *Provider) SupportsBulkFetch() bool {
	return true // CLI fetches all orders at once
}

// GetRateLimit returns the rate limit for API requests
func (p *Provider) GetRateLimit() time.Duration {
	return p.rateLimit
}

// HealthCheck verifies the provider can connect and authenticate
func (p *Provider) HealthCheck(ctx context.Context) error {
	// Try to find the CLI
	cli, err := p.findCLI()
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	if cli.useNpx {
		cmd = exec.CommandContext(ctx, npxCommand, scraperPackageName, "--help")
	} else {
		cmd = exec.CommandContext(ctx, amazonScraperCommand, "--help")
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("amazon-order-scraper CLI not available: %w", err)
	}

	return nil
}

// MerchantSearchTerms returns the merchant names to search for in Monarch
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

// CalculateLookbackDays calculates the number of days to look back
func CalculateLookbackDays(startDate, endDate time.Time) int {
	if startDate.IsZero() || endDate.IsZero() {
		return 14 // Default
	}
	days := int(endDate.Sub(startDate).Hours() / 24)
	if days < 1 {
		return 1
	}
	return days
}

// FormatDaysArg formats the days argument for the CLI
func FormatDaysArg(days int) string {
	return strconv.Itoa(days)
}
