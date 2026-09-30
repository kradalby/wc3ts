package game_test

import (
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/nielsAD/gowarcraft3/protocol/w3gs"

	"github.com/kradalby/wc3ts/game"
)

func remote(name string, hostCounter uint32) game.Game {
	return game.Game{
		Info:   w3gs.GameInfo{GameName: name, HostCounter: hostCounter},
		Source: game.SourceRemote,
		PeerIP: netip.MustParseAddr("100.64.0.1"),
	}
}

// A subscriber stuck in onChange (e.g. a stalled TUI) must not stall
// writers or the proxy's lookups.
func TestRegistryDoesNotWaitForSubscriber(t *testing.T) {
	t.Parallel()

	stuck := make(chan struct{})

	t.Cleanup(func() { close(stuck) })

	r := game.NewRegistry(func([]game.Game) { <-stuck })

	go func() { _ = r.Run(t.Context()) }()

	found := make(chan bool, 1)

	go func() {
		for i := range uint32(10) {
			r.Add(remote(strconv.FormatUint(uint64(i), 10), i))
		}

		found <- r.FindByHostCounter(9) != nil
	}()

	select {
	case ok := <-found:
		if !ok {
			t.Fatal("FindByHostCounter(9) = nil after Add")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("registry blocked behind a stuck subscriber")
	}
}

func TestRegistryNotifiesLatest(t *testing.T) {
	t.Parallel()

	got := make(chan []game.Game, 16)
	r := game.NewRegistry(func(games []game.Game) { got <- games })

	go func() { _ = r.Run(t.Context()) }()

	r.Add(remote("a", 1))

	select {
	case games := <-got:
		if len(games) != 1 || games[0].Info.GameName != "a" {
			t.Fatalf("onChange games = %v, want [a]", games)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("onChange not called after Add")
	}
}

// Map order would reshuffle the TUI table and move its cursor on every probe.
func TestRegistryGamesSortedByKey(t *testing.T) {
	t.Parallel()

	r := game.NewRegistry(nil)
	for i := range uint32(32) {
		r.Add(remote(fmt.Sprintf("g%02d", (i*7)%32), i))
	}

	keys := make([]string, 0, 32)
	for _, g := range r.Games() {
		keys = append(keys, g.Key())
	}

	if !slices.IsSorted(keys) {
		t.Fatalf("Games() not sorted by Key: %v", keys)
	}
}
