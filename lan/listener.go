// Package lan provides WC3 LAN protocol handling.
package lan

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"

	"github.com/kradalby/wc3ts/game"
	"github.com/nielsAD/gowarcraft3/network"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// DefaultPort is the standard WC3 LAN port.
const DefaultPort = 6112

// OnVersionDetectedFunc is called when the WC3 version is detected.
type OnVersionDetectedFunc func(version w3gs.GameVersion)

// Listener intercepts local WC3 LAN broadcasts.
type Listener struct {
	network.EventEmitter
	network.W3GSPacketConn

	registry          *game.Registry
	version           w3gs.GameVersion
	versionDetected   bool
	onVersionDetected OnVersionDetectedFunc
	mu                sync.RWMutex
}

// NewListener creates a new LAN listener.
func NewListener(
	ctx context.Context,
	registry *game.Registry,
	onVersionDetected OnVersionDetectedFunc,
) (*Listener, error) {
	// Use reusable port so WC3 can also bind to 6112
	conn, err := listenUDPReusable(ctx, DefaultPort)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on UDP port %d: %w", DefaultPort, err)
	}

	listener := &Listener{
		registry:          registry,
		onVersionDetected: onVersionDetected,
	}

	// Initialize with a default encoding, will be updated when version is detected
	listener.SetConn(conn, w3gs.NewFactoryCache(w3gs.DefaultFactory), w3gs.Encoding{})

	return listener, nil
}

// Run starts listening for WC3 LAN packets.
// It blocks until the context is cancelled.
func (l *Listener) Run(ctx context.Context) error {
	l.initHandlers()

	// Run packet processing in a goroutine
	errChan := make(chan error, 1)

	go func() {
		errChan <- l.W3GSPacketConn.Run(&l.EventEmitter, 0)
	}()

	select {
	case <-ctx.Done():
		_ = l.Close()

		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

// SetVersion sets the game version for the listener.
func (l *Listener) SetVersion(version w3gs.GameVersion) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.version = version
	l.versionDetected = true
}

// Version returns the detected game version.
func (l *Listener) Version() w3gs.GameVersion {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.version
}

// initHandlers sets up packet handlers.
func (l *Listener) initHandlers() {
	// Handle SearchGame packets - auto-detect version and respond with remote games
	l.On(&w3gs.SearchGame{}, l.onSearchGame)

	// Handle CreateGame packets - track locally hosted games
	l.On(&w3gs.CreateGame{}, l.onCreateGame)

	// Handle GameInfo packets - track local game details
	l.On(&w3gs.GameInfo{}, l.onGameInfo)

	// Handle DecreateGame packets - remove local games
	l.On(&w3gs.DecreateGame{}, l.onDecreateGame)
}

// onSearchGame handles SearchGame packets from local WC3 clients.
func (l *Listener) onSearchGame(ev *network.Event) {
	pkt, ok := ev.Arg.(*w3gs.SearchGame)
	if !ok {
		return
	}

	// Auto-detect version from first SearchGame packet
	l.mu.Lock()

	if !l.versionDetected {
		l.version = pkt.GameVersion
		l.versionDetected = true

		slog.Info("detected WC3 version",
			"product", pkt.Product.String(),
			"version", pkt.Version,
		)

		if l.onVersionDetected != nil {
			l.onVersionDetected(l.version)
		}
	}

	l.mu.Unlock()

	// Respond with remote games
	l.respondWithRemoteGames(ev)
}

// respondWithRemoteGames sends GameInfo packets for remote games to the requesting client.
func (l *Listener) respondWithRemoteGames(ev *network.Event) {
	addr, ok := ev.Opt[0].(net.Addr)
	if !ok {
		return
	}

	remoteGames := l.registry.RemoteGames()

	for i := range remoteGames {
		g := &remoteGames[i]

		// Create a copy of the game info to send
		info := g.Info

		// Send the game info to the requesting client
		_, err := l.Send(addr, &info)
		if err != nil {
			slog.Debug("failed to send game info",
				"game", info.GameName,
				"error", err,
			)
		}
	}
}

// onCreateGame handles CreateGame packets from local WC3.
func (l *Listener) onCreateGame(ev *network.Event) {
	pkt, ok := ev.Arg.(*w3gs.CreateGame)
	if !ok {
		return
	}

	slog.Debug("local game created",
		"hostCounter", pkt.HostCounter,
		"product", pkt.Product.String(),
		"version", pkt.Version,
	)

	// Update version if not yet detected
	l.mu.Lock()

	if !l.versionDetected {
		l.version = pkt.GameVersion
		l.versionDetected = true

		if l.onVersionDetected != nil {
			l.onVersionDetected(l.version)
		}
	}

	l.mu.Unlock()
}

// onGameInfo handles GameInfo packets from local WC3.
func (l *Listener) onGameInfo(ev *network.Event) {
	pkt, ok := ev.Arg.(*w3gs.GameInfo)
	if !ok {
		return
	}

	// Get source address to determine local IP
	var sourceIP netip.Addr

	if addr, ok := ev.Opt[0].(net.Addr); ok {
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			sourceIP, _ = netip.AddrFromSlice(udpAddr.IP)
		}
	}

	slog.Debug("local game info",
		"name", pkt.GameName,
		"hostCounter", pkt.HostCounter,
		"slots", pkt.SlotsUsed, "/", pkt.SlotsTotal,
		"source", sourceIP,
	)

	// Add to registry as local game
	l.registry.Add(game.Game{
		Info:   *pkt,
		Source: game.SourceLocal,
	})
}

// onDecreateGame handles DecreateGame packets from local WC3.
func (l *Listener) onDecreateGame(ev *network.Event) {
	pkt, ok := ev.Arg.(*w3gs.DecreateGame)
	if !ok {
		return
	}

	slog.Debug("local game removed",
		"hostCounter", pkt.HostCounter,
	)

	// Find and remove the local game with this host counter
	localGames := l.registry.LocalGames()

	for i := range localGames {
		g := &localGames[i]
		if g.Info.HostCounter == pkt.HostCounter {
			l.registry.Remove(g.Key())

			break
		}
	}
}
