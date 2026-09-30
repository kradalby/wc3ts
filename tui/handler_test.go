package tui_test

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kradalby/wc3ts/tui"
)

// A stalled event loop must not stall the goroutines that log.
func TestHandlerDoesNotBlockOnStalledTUI(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	delivered := make(chan tea.Msg, 1)

	log := slog.New(tui.NewHandler(func(msg tea.Msg) {
		<-release

		select {
		case delivered <- msg:
		default:
		}
	}, slog.LevelDebug))

	done := make(chan struct{})

	go func() {
		defer close(done)

		for i := range 1000 {
			log.Info("line", "i", i)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("logging blocked while the TUI was not draining")
	}

	close(release)

	msg, ok := (<-delivered).(tui.LogMsg)
	if !ok || !strings.HasSuffix(msg.Message, "line i=0") {
		t.Fatalf("first delivered = %#v, want LogMsg ending in %q", msg, "line i=0")
	}
}
