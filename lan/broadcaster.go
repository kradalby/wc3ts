package lan

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/kradalby/wc3ts/game"
	"github.com/nielsAD/gowarcraft3/network"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// DefaultPort is the standard WC3 LAN port.
const DefaultPort = 6112

// BroadcastInterval is how often to send game broadcasts.
const BroadcastInterval = 3 * time.Second

// writeBufferSize is the UDP write buffer size.
const writeBufferSize = 64 * 1024

// Broadcaster periodically broadcasts remote games to the local LAN.
// Unlike UDPAdvertiser, it doesn't bind to port 6112 - it just sends.
type Broadcaster struct {
	conn             *network.W3GSPacketConn
	games            []game.Game
	previousGameKeys map[string]uint32 // game key -> HostCounter for tracking removed games
	proxyPort        uint16
	showPeerNames    bool
	broadcastAddr    *net.UDPAddr
	mu               sync.RWMutex
}

// NewBroadcaster creates a new broadcaster.
func NewBroadcaster(proxyPort uint16, showPeerNames bool) (*Broadcaster, error) {
	// Bind to port 0 (ephemeral) - we only send, don't need to receive
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, err
	}

	// Enable broadcast
	err = conn.SetWriteBuffer(writeBufferSize)
	if err != nil {
		slog.Debug("failed to set write buffer", "error", err)
	}

	b := &Broadcaster{
		proxyPort:        proxyPort,
		showPeerNames:    showPeerNames,
		broadcastAddr:    &net.UDPAddr{IP: net.IPv4bcast, Port: DefaultPort},
		previousGameKeys: make(map[string]uint32),
	}

	// Wrap in W3GSPacketConn for proper packet encoding
	b.conn = &network.W3GSPacketConn{}
	b.conn.SetConn(conn, w3gs.NewFactoryCache(w3gs.DefaultFactory), w3gs.Encoding{})

	return b, nil
}

// Run starts the broadcast loop.
func (b *Broadcaster) Run(ctx context.Context) error {
	ticker := time.NewTicker(BroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			b.broadcastGames()
		}
	}
}

// OnGamesChanged updates the list of games to broadcast.
func (b *Broadcaster) OnGamesChanged(games []game.Game) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.games = games
}

// Close closes the broadcaster.
func (b *Broadcaster) Close() error {
	return b.conn.Close()
}

// broadcastGames sends GameInfo and RefreshGame for all remote games,
// and DecreateGame for any games that have been removed.
func (b *Broadcaster) broadcastGames() {
	b.mu.Lock()
	defer b.mu.Unlock()

	currentKeys := make(map[string]uint32)

	// Broadcast current remote games
	for i := range b.games {
		g := &b.games[i]

		if g.Source != game.SourceRemote {
			continue
		}

		key := g.Key()
		currentKeys[key] = g.Info.HostCounter

		info := b.modifyGameInfo(g)

		_, err := b.conn.Send(b.broadcastAddr, &info)
		if err != nil {
			slog.Debug("failed to broadcast game",
				"game", info.GameName,
				"error", err,
			)

			continue
		}

		// Send RefreshGame after GameInfo to update player counts
		b.sendRefreshGame(g)
	}

	// Send DecreateGame for any games that were removed
	for key, hostCounter := range b.previousGameKeys {
		if _, exists := currentKeys[key]; !exists {
			b.sendDecreateGame(hostCounter)

			slog.Debug("sent game cancellation",
				"key", key,
				"hostCounter", hostCounter,
			)
		}
	}

	// Update tracked games for next iteration
	b.previousGameKeys = currentKeys
}

// sendRefreshGame sends a RefreshGame packet to update player counts.
func (b *Broadcaster) sendRefreshGame(g *game.Game) {
	refresh := &w3gs.RefreshGame{
		HostCounter:    g.Info.HostCounter,
		SlotsUsed:      g.Info.SlotsUsed,
		SlotsAvailable: g.Info.SlotsAvailable,
	}

	_, err := b.conn.Send(b.broadcastAddr, refresh)
	if err != nil {
		slog.Debug("failed to send refresh",
			"game", g.Info.GameName,
			"error", err,
		)
	}
}

// sendDecreateGame sends a DecreateGame packet to notify game removal.
func (b *Broadcaster) sendDecreateGame(hostCounter uint32) {
	decreate := &w3gs.DecreateGame{
		HostCounter: hostCounter,
	}

	_, err := b.conn.Send(b.broadcastAddr, decreate)
	if err != nil {
		slog.Debug("failed to send decreate",
			"hostCounter", hostCounter,
			"error", err,
		)
	}
}

// modifyGameInfo creates a modified copy for local advertisement.
func (b *Broadcaster) modifyGameInfo(g *game.Game) w3gs.GameInfo {
	info := g.Info

	// Replace port with local TCP proxy port
	info.GamePort = b.proxyPort

	// Optionally prefix game name with peer hostname
	if b.showPeerNames && g.PeerName != "" {
		info.GameName = "[" + g.PeerName + "] " + info.GameName
	}

	return info
}
