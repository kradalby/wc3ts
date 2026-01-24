// Package game provides game state management for the wc3ts proxy.
package game

import (
	"net/netip"
	"time"

	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// Source indicates where a game was discovered.
type Source string

// Game sources.
const (
	SourceLocal  Source = "local"  // Hosted on this machine
	SourceRemote Source = "remote" // From another Tailscale peer
)

// Game represents a discovered WC3 game.
type Game struct {
	// Info contains the WC3 game information (parsed for display).
	Info w3gs.GameInfo

	// RawData contains the original packet bytes for forwarding.
	// Set for all games to preserve exact HostCounter values.
	RawData []byte

	// Source indicates where this game was discovered.
	Source Source

	// PeerIP is the Tailscale IP of the peer hosting this game.
	// Only set for remote games.
	PeerIP netip.Addr

	// PeerName is the hostname of the peer hosting this game.
	// Only set for remote games.
	PeerName string

	// FirstSeen is when this game was first discovered.
	FirstSeen time.Time

	// LastSeen is when this game was last seen/refreshed.
	LastSeen time.Time
}

// Key returns a unique identifier for this game.
func (g *Game) Key() string {
	if g.Source == SourceLocal {
		return "local:" + g.Info.GameName
	}

	return g.PeerIP.String() + ":" + g.Info.GameName
}

// IsStale returns true if the game hasn't been seen recently.
func (g *Game) IsStale(timeout time.Duration) bool {
	return time.Since(g.LastSeen) > timeout
}
