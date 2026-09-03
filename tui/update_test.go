package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"

	"github.com/kradalby/wc3ts/tui"
	"github.com/kradalby/wc3ts/version"
)

func key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func modelFrom(t *testing.T, model tea.Model) tui.Model {
	t.Helper()

	m, ok := model.(tui.Model)
	if !ok {
		t.Fatalf("model type = %T, want tui.Model", model)
	}

	return m
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

	done := cmd() // Bubble Tea runs Cmds off the event loop.

	if !called {
		t.Fatal("returned Cmd did not invoke the refresh callback")
	}

	if done == nil {
		t.Fatal("refresh command did not return a completion message")
	}
}

func TestDetailRefreshDeferredToCmd(t *testing.T) {
	t.Parallel()

	called := false
	m := tui.NewModel(0, w3gs.GameVersion{}, version.Info{}, nil, func() { called = true })

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if called {
		t.Fatal("detail refresh callback ran inline in Update")
	}

	if cmd == nil {
		t.Fatal("expected detail view to return a refresh command")
	}

	cmd()

	if !called {
		t.Fatal("detail refresh command did not invoke callback")
	}
}

func TestRefreshRequestsAreCoalesced(t *testing.T) {
	t.Parallel()

	calls := 0
	m := tui.NewModel(0, w3gs.GameVersion{}, version.Info{}, nil, func() { calls++ })

	model, first := m.Update(key('r'))
	m = modelFrom(t, model)

	_, second := m.Update(key('r'))
	if second != nil {
		t.Fatal("second refresh was not coalesced")
	}

	done := first()
	model, _ = m.Update(done)
	m = modelFrom(t, model)

	_, third := m.Update(key('r'))
	if third == nil {
		t.Fatal("refresh remained blocked after completion")
	}

	third()

	if calls != 2 {
		t.Fatalf("refresh callback called %d times, want 2", calls)
	}
}

func TestVersionChangesPreserveOrder(t *testing.T) {
	t.Parallel()

	versions := make([]uint32, 0, 2)
	m := tui.NewModel(
		0,
		w3gs.GameVersion{Version: 26},
		version.Info{},
		func(v uint32) { versions = append(versions, v) },
		nil,
	)

	model, first := m.Update(key(']'))
	m = modelFrom(t, model)
	_, second := m.Update(key(']'))

	if first != nil || second != nil {
		t.Fatal("ordered version changes must not create concurrent commands")
	}

	want := []uint32{27, 28}
	if len(versions) != len(want) || versions[0] != want[0] || versions[1] != want[1] {
		t.Fatalf("version callbacks = %v, want %v", versions, want)
	}
}
