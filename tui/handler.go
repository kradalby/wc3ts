package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// logBuffer bounds log lines queued for the TUI; beyond it lines are dropped.
const logBuffer = 256

// Handler is a slog.Handler that sends logs to the TUI.
type Handler struct {
	lines  chan<- string
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

// NewHandler creates a TUI log handler that passes each line to send as a
// LogMsg, typically tea.Program.Send.
//
// send runs on a single goroutine fed by a bounded queue. Handle never waits
// for it: logging happens under locks and on network paths, and a stalled
// event loop must not stall those, so lines are dropped once the queue fills.
func NewHandler(send func(tea.Msg), level slog.Level) *Handler {
	lines := make(chan string, logBuffer)

	go func() {
		for line := range lines {
			send(LogMsg{Message: line})
		}
	}()

	return &Handler{
		lines: lines,
		level: level,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and sends the log record to the TUI.
func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	// Format: HH:MM:SS LEVEL message key=value ...
	b.WriteString(r.Time.Format(time.TimeOnly))
	b.WriteString(" ")
	b.WriteString(r.Level.String())
	b.WriteString(" ")
	b.WriteString(r.Message)

	// Add attributes
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" ")
		b.WriteString(a.Key)
		b.WriteString("=")
		fmt.Fprintf(&b, "%v", a.Value.Any())

		return true
	})

	// Add handler-level attributes
	for _, a := range h.attrs {
		b.WriteString(" ")
		b.WriteString(a.Key)
		b.WriteString("=")
		fmt.Fprintf(&b, "%v", a.Value.Any())
	}

	select {
	case h.lines <- b.String():
	default:
	}

	return nil
}

// WithAttrs returns a new Handler with the given attributes added.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &Handler{
		lines:  h.lines,
		level:  h.level,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

// WithGroup returns a new Handler with the given group name added.
func (h *Handler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &Handler{
		lines:  h.lines,
		level:  h.level,
		attrs:  h.attrs,
		groups: newGroups,
	}
}
