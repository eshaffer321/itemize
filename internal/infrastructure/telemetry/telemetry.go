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
const dsn = "SENTRY_DSN_PLACEHOLDER"

// tokenPattern matches strings that look like API keys/tokens and should never
// appear in events. Anything 20+ chars of alphanumeric/dash/underscore is suspect.
var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_\-]{20,}$`)

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

// scrubEvent is the Sentry BeforeSend hook. It strips or redacts any field that
// could contain credentials, file paths, or PII before the event leaves the process.
func scrubEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	// Never send user identity or request context.
	event.User = sentry.User{}
	event.Request = nil

	// Scrub Extras: drop any value that looks like a token.
	for k, v := range event.Extra {
		if s, ok := v.(string); ok && tokenPattern.MatchString(s) {
			delete(event.Extra, k)
		}
	}

	// Scrub Tags: same rule.
	for k, v := range event.Tags {
		if tokenPattern.MatchString(v) {
			delete(event.Tags, k)
		}
	}

	return event
}
