package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// MavenHandler is a slog.Handler that formats logs in Maven-style:
// [LEVEL] [SYSTEM] [HH:MM:SS] message key=value key=value
type MavenHandler struct {
	w              io.Writer
	level          slog.Level
	mu             *sync.Mutex
	system         string // e.g., "sync", "costco", "walmart"
	showTimestamps bool
	useColors      bool
	groups         []string // For handling WithGroup
	attrs          []slog.Attr
}

// NewMavenHandler creates a new Maven-style handler
func NewMavenHandler(w io.Writer, opts *slog.HandlerOptions) *MavenHandler {
	h := &MavenHandler{
		w:              w,
		level:          slog.LevelInfo,
		mu:             &sync.Mutex{},
		showTimestamps: true,
		useColors:      isTerminal(w),
	}

	if opts != nil {
		if opts.Level != nil {
			h.level = opts.Level.Level()
		}
	}

	return h
}

// isTerminal checks if the writer is a terminal (for color output)
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// Enabled reports whether the handler handles records at the given level.
func (h *MavenHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and writes a log record
func (h *MavenHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var buf strings.Builder

	// Get level color
	levelColor := h.levelColor(r.Level)

	// [LEVEL] with color
	if h.useColors {
		buf.WriteString(levelColor)
	}
	buf.WriteString("[")
	buf.WriteString(levelString(r.Level))
	buf.WriteString("]")
	if h.useColors {
		buf.WriteString(colorReset)
	}

	// [SYSTEM]
	if h.system != "" {
		buf.WriteString(" [")
		buf.WriteString(h.system)
		buf.WriteString("]")
	}

	// [HH:MM:SS] in gray
	if h.showTimestamps {
		if h.useColors {
			buf.WriteString(colorGray)
		}
		buf.WriteString(" [")
		buf.WriteString(r.Time.Format("15:04:05"))
		buf.WriteString("]")
		if h.useColors {
			buf.WriteString(colorReset)
		}
	}

	// Message
	buf.WriteString(" ")
	buf.WriteString(r.Message)

	// Attributes from WithAttrs
	for _, attr := range h.attrs {
		if attr.Key != "system" { // Skip system attr, already shown in bracket
			h.appendAttr(&buf, attr)
		}
	}

	// Attributes from the log record
	r.Attrs(func(a slog.Attr) bool {
		if a.Key != "system" { // Skip system attr, already shown in bracket
			h.appendAttr(&buf, a)
		}
		return true
	})

	buf.WriteString("\n")

	_, err := h.w.Write([]byte(buf.String()))
	return err
}

// appendAttr appends a key=value pair to the buffer
func (h *MavenHandler) appendAttr(buf *strings.Builder, a slog.Attr) {
	buf.WriteString(" ")
	buf.WriteString(a.Key)
	buf.WriteString("=")
	buf.WriteString(fmt.Sprint(a.Value.Any()))
}

// WithAttrs returns a new handler with the given attributes added
func (h *MavenHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	// Extract "system" attribute if present
	system := h.system
	for _, attr := range attrs {
		if attr.Key == "system" {
			system = attr.Value.String()
		}
	}

	return &MavenHandler{
		w:              h.w,
		level:          h.level,
		mu:             h.mu,
		system:         system,
		showTimestamps: h.showTimestamps,
		useColors:      h.useColors,
		groups:         h.groups,
		attrs:          newAttrs,
	}
}

// WithGroup returns a new handler with the given group name added
func (h *MavenHandler) WithGroup(name string) slog.Handler {
	// For now, we'll just track groups but not use them in formatting
	// Could be extended to show nested structure if needed
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &MavenHandler{
		w:              h.w,
		level:          h.level,
		mu:             h.mu,
		system:         h.system,
		showTimestamps: h.showTimestamps,
		useColors:      h.useColors,
		groups:         newGroups,
		attrs:          h.attrs,
	}
}

// levelColor returns the ANSI color code for a log level (Maven-style)
func (h *MavenHandler) levelColor(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return colorGray
	case slog.LevelInfo:
		return colorCyan
	case slog.LevelWarn:
		return colorYellow
	case slog.LevelError:
		return colorRed
	default:
		return colorReset
	}
}

// levelString returns a short, uppercase string for the log level
func levelString(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		return fmt.Sprintf("LEVEL(%d)", level)
	}
}
