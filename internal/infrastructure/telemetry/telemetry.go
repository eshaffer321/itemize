package telemetry

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/eshaffer321/itemize/internal/application/sync"
	"github.com/eshaffer321/itemize/internal/cli"
	"github.com/getsentry/sentry-go"
)

// dsn is the Sentry project DSN. It is intentionally public — Sentry DSNs are
// client-side keys rate-limited at the project level.
const dsn = "https://dd8d38ecea079b123faf44cbf4d9d3bf@o4509941888122880.ingest.us.sentry.io/4511661709197312"

// tokenPattern matches whole values (extras, tags) that look like API keys.
var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_\-]{20,}$`)

// tokenSubstring finds token-like sequences embedded within longer strings
// (e.g. error messages that include a credential in the middle).
var tokenSubstring = regexp.MustCompile(`[A-Za-z0-9_\-]{20,}`)

// IsEnabled reports whether telemetry is active for this run.
func IsEnabled() bool {
	if os.Getenv("ITEMIZE_NO_TELEMETRY") != "" {
		return false
	}
	if os.Getenv("DO_NOT_TRACK") == "1" {
		return false
	}
	return dsn != "SENTRY_DSN_PLACEHOLDER" && dsn != ""
}

// Init initialises Sentry and returns a flush function that must be deferred by
// the caller. If telemetry is disabled it returns a no-op and prints nothing.
func Init() func() {
	if !IsEnabled() {
		return func() {}
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		BeforeSend:       scrubEvent,
		TracesSampleRate: 0,
		// Explicitly opt out of attaching PII like IP addresses.
		SendDefaultPII: false,
	})
	if err != nil {
		// Non-fatal: telemetry failure must never break the CLI.
		return func() {}
	}

	fmt.Fprintln(os.Stderr, "Telemetry enabled. Set ITEMIZE_NO_TELEMETRY=1 to opt out. See https://github.com/eshaffer321/itemize#telemetry")

	return func() {
		sentry.Flush(2 * time.Second)
	}
}

// CaptureSync records a successful (or dry-run) sync as a Sentry message event
// so the dashboard shows usage broken down by provider and flag combination.
func CaptureSync(provider string, flags cli.SyncFlags, result *sync.Result) {
	if !IsEnabled() {
		return
	}
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("provider", provider)
		scope.SetTag("dry_run", fmt.Sprintf("%t", flags.DryRun))
		scope.SetTag("event_type", "sync_complete")
		scope.SetExtra("processed", result.ProcessedCount)
		scope.SetExtra("skipped", result.SkippedCount)
		scope.SetExtra("errors", result.ErrorCount)
		scope.SetExtra("lookback_days", flags.LookbackDays)
		scope.SetExtra("max_orders", flags.MaxOrders)
		scope.SetExtra("force", flags.Force)
		sentry.CaptureMessage("sync_complete")
	})
}

// CaptureError records a fatal error with the provider and stage it occurred in.
func CaptureError(err error, provider, stage string) {
	if !IsEnabled() || err == nil {
		return
	}
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("provider", provider)
		scope.SetTag("stage", stage)
		sentry.CaptureException(err)
	})
}

// scrubEvent is the Sentry BeforeSend hook. It is the last line of defence
// before an event leaves the process. Multiple independent passes are applied
// so that a gap in one layer is caught by another.
func scrubEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	// Layer 1: strip identity and request context entirely.
	event.User = sentry.User{}
	event.Request = nil

	// Layer 2: scrub Extras — drop any value that looks like a token.
	for k, v := range event.Extra {
		if s, ok := v.(string); ok && tokenPattern.MatchString(s) {
			delete(event.Extra, k)
		}
	}

	// Layer 3: scrub Tags — same rule.
	for k, v := range event.Tags {
		if tokenPattern.MatchString(v) {
			delete(event.Tags, k)
		}
	}

	// Layer 4: scrub exception values — an error message could contain a token
	// embedded in a longer string (e.g. "request failed: invalid token abc123xyz...").
	for i := range event.Exception {
		event.Exception[i].Value = tokenSubstring.ReplaceAllString(event.Exception[i].Value, "[redacted]")
	}

	// Layer 5: wipe breadcrumb data — breadcrumbs can carry arbitrary key/value
	// pairs from anywhere in the call stack.
	for i := range event.Breadcrumbs {
		event.Breadcrumbs[i].Data = nil
	}

	return event
}
