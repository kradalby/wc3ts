package peer

import (
	"context"
	"log/slog"
	"net"
	"net/netip"

	"github.com/kradalby/wc3ts/game"
	"github.com/kradalby/wc3ts/lan"
	"github.com/nielsAD/gowarcraft3/network"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// Responder listens for SearchGame queries from remote Tailscale peers
// and responds with local game information.
type Responder struct {
	network.EventEmitter
	network.W3GSPacketConn

	registry *game.Registry
	localIP  netip.Addr
}

// NewResponder creates a new responder that listens on the given Tailscale IP.
func NewResponder(registry *game.Registry, localIP netip.Addr) (*Responder, error) {
	// Listen on Tailscale IP, port 6112
	addr := &net.UDPAddr{
		IP:   localIP.AsSlice(),
		Port: lan.DefaultPort,
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}

	r := &Responder{
		registry: registry,
		localIP:  localIP,
	}

	r.SetConn(conn, w3gs.NewFactoryCache(w3gs.DefaultFactory), w3gs.Encoding{})

	return r, nil
}

// Run starts listening for SearchGame queries and responding with local games.
// It blocks until the context is cancelled.
func (r *Responder) Run(ctx context.Context) error {
	r.On(&w3gs.SearchGame{}, r.onSearchGame)

	// Start packet receiving in background
	go func() {
		_ = r.W3GSPacketConn.Run(&r.EventEmitter, 0)
	}()

	<-ctx.Done()

	_ = r.Close()

	return ctx.Err()
}

// onSearchGame handles SearchGame queries from remote peers.
func (r *Responder) onSearchGame(ev *network.Event) {
	// Get requester address
	addr, ok := ev.Opt[0].(net.Addr)
	if !ok {
		return
	}

	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return
	}

	// Get local games and respond with each
	games := r.registry.LocalGames()

	slog.Debug("received SearchGame query",
		"from", addr,
		"localGames", len(games),
	)

	for i := range games {
		g := &games[i]

		// Send raw packet data (preserves exact HostCounter)
		if len(g.RawData) == 0 {
			slog.Warn("game has no RawData, skipping",
				"game", g.Info.GameName,
			)

			continue
		}

		_, err := r.Conn().WriteTo(g.RawData, udpAddr)
		if err != nil {
			slog.Debug("failed to send raw GameInfo response",
				"game", g.Info.GameName,
				"to", addr,
				"error", err,
			)
		}
	}
}
