package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	targetgo "github.com/daslicious/target-go"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/eshaffer321/itemize/internal/adapters/providers/target"
	"github.com/eshaffer321/itemize/internal/infrastructure/config"
	"github.com/eshaffer321/itemize/internal/infrastructure/logging"
)

// TargetSessionPath returns where the Target session for account is saved:
// <ITEMIZE_TARGET_DIR>/session-<account>.json, with ITEMIZE_TARGET_DIR defaulting to ~/.itemize/target.
func TargetSessionPath(account string) (string, error) {
	dir := os.Getenv("ITEMIZE_TARGET_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		dir = filepath.Join(home, ".itemize", "target")
	}
	return targetgo.SessionPath(dir, account)
}

// targetRenewWindow: a token with less than this left is renewed before a run. Target only renews
// an expired token, so a token about to expire is waited out first (at most this long).
const targetRenewWindow = 15 * time.Minute

// NewTargetProvider creates a Target provider from the account's saved session, renewed first if
// its token wouldn't last through a run. Renewed tokens last 24 hours and runs are daily, so most
// runs renew; the client saves the renewed login at once (the refresh token rotates on renewal).
// ITEMIZE_TARGET_DUMP_DIR, if set, keeps a copy of every Target response there.
func NewTargetProvider(cfg *config.Config, verbose bool, account string) (*target.Provider, error) {
	loggingCfg := cfg.Observability.Logging
	if verbose {
		loggingCfg.Level = "debug"
	}
	targetLogger := logging.NewLoggerWithSystem(loggingCfg, "target")

	path, err := TargetSessionPath(account)
	if err != nil {
		return nil, err
	}
	opts := []targetgo.ClientOption{targetgo.WithLogger(targetLogger)}
	if dir := os.Getenv("ITEMIZE_TARGET_DUMP_DIR"); dir != "" {
		opts = append(opts, targetgo.WithDumpDir(dir))
	}
	client, err := targetgo.Open(path, opts...)
	if err != nil {
		return nil, err
	}
	if _, err := client.EnsureFresh(context.Background(), targetRenewWindow); err != nil {
		return nil, err
	}
	return target.NewProvider(client, targetLogger), nil
}

// RunTargetRefresh renews the account's login now and saves it.
func RunTargetRefresh(ctx context.Context, cfg *config.Config, verbose bool, account string, out io.Writer) error {
	loggingCfg := cfg.Observability.Logging
	if verbose {
		loggingCfg.Level = "debug"
	}
	path, err := TargetSessionPath(account)
	if err != nil {
		return err
	}
	client, err := targetgo.Open(path, targetgo.WithLogger(logging.NewLoggerWithSystem(loggingCfg, "target")))
	if err != nil {
		return err
	}
	if err := client.Refresh(ctx); err != nil {
		return err
	}
	exp, _ := client.Session().AccessTokenExpiry()
	_, _ = fmt.Fprintf(out, "Renewed and saved %s; the new access token expires %s.\n", path, exp.Local().Format("2006-01-02 15:04 MST"))
	return nil
}

// RunTargetSetSession reads a "Copy as cURL" (or a Cookie header) from in, checks that Target accepts
// it, and only then saves it as the account's session.
func RunTargetSetSession(ctx context.Context, account string, in io.Reader, out io.Writer) error {
	path, err := TargetSessionPath(account)
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(in, 1<<20))
	if err != nil {
		return err
	}
	session, err := targetgo.ParseSession(string(raw), time.Now().UTC())
	if err != nil {
		return err
	}

	// Test the login by replaying the request it was copied from, which target.com itself just made.
	client := targetgo.NewClient(session)
	if session.SourceURL != "" {
		err = client.Check(ctx, session.SourceURL)
	} else {
		_, err = client.OrderHistory(ctx, targetgo.PurchaseOnline, 1, 10)
	}
	if err != nil {
		return fmt.Errorf("not saved, Target refused the login: %w", err)
	}
	if err := targetgo.SaveSession(path, session); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Saved to %s. Target accepted the login.\n", path)

	// Report whether the order lists itemize reads still answer (their paths have changed before).
	for _, purchaseType := range []string{targetgo.PurchaseOnline, targetgo.PurchaseStore} {
		if history, err := client.OrderHistory(ctx, purchaseType, 1, 10); err != nil {
			_, _ = fmt.Fprintf(out, "Warning: the %s order list failed: %v\n", purchaseType, err)
		} else {
			_, _ = fmt.Fprintf(out, "%s order list: %d orders on record.\n", purchaseType, history.TotalOrders)
		}
	}
	if exp, ok := session.AccessTokenExpiry(); ok {
		_, _ = fmt.Fprintf(out, "The access token in these cookies expires %s (in %s).\n",
			exp.Local().Format("2006-01-02 15:04 MST"), time.Until(exp).Round(time.Minute))
	}
	return nil
}

// TargetOrderJSON is how `itemize target orders` prints an order.
type TargetOrderJSON struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Date     string          `json:"date"`
	Total    float64         `json:"total"`
	Subtotal float64         `json:"subtotal"`
	Tax      float64         `json:"tax"`
	Items    []TargetItemRow `json:"items"`
}

// TargetItemRow is one item of TargetOrderJSON.
type TargetItemRow struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Price    float64 `json:"price"`
	Category string  `json:"target_category,omitempty"`
}

// PrintTargetOrders writes orders as JSON, the way itemize would see them, without touching Monarch.
func PrintTargetOrders(w io.Writer, orders []providers.Order) error {
	rows := make([]TargetOrderJSON, 0, len(orders))
	for _, order := range orders {
		row := TargetOrderJSON{
			ID:       order.GetID(),
			Date:     order.GetDate().Format(time.RFC3339),
			Total:    order.GetTotal(),
			Subtotal: order.GetSubtotal(),
			Tax:      order.GetTax(),
			Items:    []TargetItemRow{},
		}
		if o, ok := order.(*target.Order); ok {
			row.Kind = o.Kind()
		}
		for _, item := range order.GetItems() {
			row.Items = append(row.Items, TargetItemRow{
				Name:     item.GetName(),
				Quantity: item.GetQuantity(),
				Price:    item.GetPrice(),
				Category: item.GetCategory(),
			})
		}
		rows = append(rows, row)
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(rows)
}
