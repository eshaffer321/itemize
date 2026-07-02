// Package version holds build-time metadata that is injected via -ldflags
// during a GoReleaser (or manual) build. When built with plain `go build`
// (e.g. `go build -o itemize ./cmd/itemize/`), these stay at their zero
// values so it's obvious the binary is a local dev build rather than a
// tagged release.
package version

import "fmt"

// These are overridden at build time via:
//
//	-X github.com/eshaffer321/itemize/internal/version.Version={{.Version}}
//	-X github.com/eshaffer321/itemize/internal/version.Commit={{.Commit}}
//	-X github.com/eshaffer321/itemize/internal/version.Date={{.Date}}
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// String returns a human-readable version string suitable for printing from
// the `-version`/`version` CLI entry point. It intentionally distinguishes
// dev builds from tagged releases so a stale local binary is obvious at a
// glance instead of requiring a git log / file-timestamp investigation.
func String() string {
	if Version == "dev" {
		return fmt.Sprintf("itemize dev-build (commit %s, built %s)", Commit, Date)
	}
	return fmt.Sprintf("itemize %s (commit %s, built %s)", Version, Commit, Date)
}
