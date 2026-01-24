package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Handler is a slog.Handler that sends logs to the TUI.
type Handler struct {
	program *tea.Program
	level   slog.Level
	attrs   []slog.Attr
	groups  []string
	ready   *atomic.Bool
}

// NewHandler creates a new TUI log handler.
func NewHandler(program *tea.Program, level slog.Level) *Handler {
	return &Handler{
		program: program,
		level:   level,
		ready:   &atomic.Bool{},
	}
}

// SetReady marks the handler as ready to send messages.
// Call this after program.Run() has started.
func (h *Handler) SetReady() {
	h.ready.Store(true)
}

// Enabled reports whether the handler handles records at the given level.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and sends the log record to the TUI.
func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	// Don't send if program isn't ready yet
	if !h.ready.Load() {
		return nil
	}

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
		b.WriteString(fmt.Sprintf("%v", a.Value.Any()))

		return true
	})

	// Add handler-level attributes
	for _, a := range h.attrs {
		b.WriteString(" ")
		b.WriteString(a.Key)
		b.WriteString("=")
		b.WriteString(fmt.Sprintf("%v", a.Value.Any()))
	}

	h.program.Send(LogMsg{Message: b.String()})

	return nil
}

// WithAttrs returns a new Handler with the given attributes added.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &Handler{
		program: h.program,
		level:   h.level,
		attrs:   newAttrs,
		groups:  h.groups,
		ready:   h.ready,
	}
}

// WithGroup returns a new Handler with the given group name added.
func (h *Handler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &Handler{
		program: h.program,
		level:   h.level,
		attrs:   h.attrs,
		groups:  newGroups,
		ready:   h.ready,
	}
}
