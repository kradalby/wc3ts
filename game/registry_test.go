package game_test

import (
	"bytes"
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

	go func() { _ = r.Run(t.Context(), time.Hour) }()

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

	go func() { _ = r.Run(t.Context(), time.Hour) }()

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

// Callers share no memory with the registry, so one editing what it passed in
// or got back cannot change what the TUI, proxy and broadcaster read.
func TestRegistryDoesNotShareMemory(t *testing.T) {
	t.Parallel()

	r := game.NewRegistry(nil)

	in := remote("r", 1)
	in.RawData = []byte{1}
	r.Add(in)
	in.RawData[0] = 9

	r.Add(game.Game{Info: w3gs.GameInfo{GameName: "l"}, RawData: []byte{1}, Source: game.SourceLocal})

	for _, g := range [][]game.Game{r.Games(), r.LocalGames(), {*r.FindByHostCounter(1)}} {
		g[0].Info.GameName = "x"
		g[0].RawData[0] = 9
	}

	for _, g := range r.Games() {
		if g.Info.GameName == "x" || !bytes.Equal(g.RawData, []byte{1}) {
			t.Fatalf("registry game changed by caller: %+v", g)
		}
	}
}

func TestRegistryAddKeepsFirstSeen(t *testing.T) {
	t.Parallel()

	r := game.NewRegistry(nil)
	r.Add(remote("a", 1))
	first := r.Games()[0].FirstSeen

	r.Add(remote("a", 1))

	if got := r.Games()[0].FirstSeen; got.IsZero() || !got.Equal(first) {
		t.Fatalf("FirstSeen after refresh = %v, want %v", got, first)
	}
}

func TestRegistryExpire(t *testing.T) {
	t.Parallel()

	r := game.NewRegistry(nil)
	r.Add(remote("a", 1))

	if n := r.Expire(time.Now().Add(-time.Hour)); n != 0 || len(r.Games()) != 1 {
		t.Fatalf("Expire(past) removed %d, left %d games; want 0, 1", n, len(r.Games()))
	}

	if n := r.Expire(time.Now().Add(time.Hour)); n != 1 || len(r.Games()) != 0 {
		t.Fatalf("Expire(future) removed %d, left %d games; want 1, 0", n, len(r.Games()))
	}
}

// Ended games must stop being advertised once probes stop answering.
func TestRegistryRunExpiresUnseenGames(t *testing.T) {
	t.Parallel()

	got := make(chan []game.Game, 16)
	r := game.NewRegistry(func(games []game.Game) { got <- games })
	r.Add(remote("a", 1))

	go func() { _ = r.Run(t.Context(), 20*time.Millisecond) }()

	timeout := time.After(5 * time.Second)

	for {
		select {
		case games := <-got:
			if len(games) == 0 {
				return
			}
		case <-timeout:
			t.Fatal("game never expired")
		}
	}
}
