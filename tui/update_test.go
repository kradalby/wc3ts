package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kradalby/wc3ts/tui"
	"github.com/kradalby/wc3ts/version"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// Regression test for the UI freeze (issue #6): callbacks that log must not run
// inline in Update. The TUI slog handler calls program.Send on an unbuffered
// channel; if that happens on the event-loop goroutine (inside Update), the loop
// can't drain the channel and deadlocks. Update must defer such work to a tea.Cmd.

func key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestRefreshDeferredToCmd(t *testing.T) {
	t.Parallel()

	called := false
	m := tui.NewModel(0, w3gs.GameVersion{}, version.Info{}, nil, func() { called = true })

	_, cmd := m.Update(key('r'))

	if called {
		t.Fatal("refresh callback ran inline in Update; must be deferred to a tea.Cmd")
	}

	if cmd == nil {
		t.Fatal("expected a tea.Cmd carrying the refresh callback")
	}

	cmd() // Bubble Tea runs Cmds off the event loop.

	if !called {
		t.Fatal("returned Cmd did not invoke the refresh callback")
	}
}

func TestVersionChangeDeferredToCmd(t *testing.T) {
	t.Parallel()

	called := false
	m := tui.NewModel(0, w3gs.GameVersion{}, version.Info{}, func(uint32) { called = true }, nil)

	_, cmd := m.Update(key(']'))

	if called {
		t.Fatal("version callback ran inline in Update; must be deferred to a tea.Cmd")
	}

	if cmd == nil {
		t.Fatal("expected a tea.Cmd carrying the version callback")
	}

	cmd()

	if !called {
		t.Fatal("returned Cmd did not invoke the version callback")
	}
}
