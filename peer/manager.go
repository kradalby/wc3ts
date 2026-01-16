// Package peer provides Tailscale peer game discovery.
package peer

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/kradalby/wc3ts/game"
	"github.com/kradalby/wc3ts/lan"
	"github.com/kradalby/wc3ts/tailscale"
	"github.com/nielsAD/gowarcraft3/network"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// DefaultProbeInterval is how often to probe peers for games.
const DefaultProbeInterval = 5 * time.Second

// Manager probes Tailscale peers to discover remote WC3 games.
type Manager struct {
	network.EventEmitter
	network.W3GSPacketConn

	discovery     *tailscale.Discovery
	registry      *game.Registry
	version       w3gs.GameVersion
	probeInterval time.Duration
	peers         []tailscale.Peer
	mu            sync.RWMutex
}

// NewManager creates a new peer manager.
func NewManager(
	discovery *tailscale.Discovery,
	registry *game.Registry,
	probeInterval time.Duration,
) (*Manager, error) {
	conn, err := net.ListenUDP("udp4", nil) // Random port for sending
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		discovery:     discovery,
		registry:      registry,
		probeInterval: probeInterval,
		peers:         make([]tailscale.Peer, 0),
	}

	mgr.SetConn(conn, w3gs.NewFactoryCache(w3gs.DefaultFactory), w3gs.Encoding{})

	return mgr, nil
}

// Run starts probing peers for games.
// It blocks until the context is cancelled.
func (m *Manager) Run(ctx context.Context) error {
	m.initHandlers()

	// Start packet receiving in background
	go func() {
		_ = m.W3GSPacketConn.Run(&m.EventEmitter, 0)
	}()

	// Probe peers periodically
	ticker := time.NewTicker(m.probeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = m.Close()

			return ctx.Err()
		case <-ticker.C:
			m.probeAllPeers()
		}
	}
}

// SetVersion sets the game version to use for probing.
func (m *Manager) SetVersion(version w3gs.GameVersion) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.version = version
}

// OnPeersChanged handles peer list updates from Tailscale discovery.
func (m *Manager) OnPeersChanged(peers []tailscale.Peer) {
	m.mu.Lock()
	m.peers = peers
	m.mu.Unlock()

	// Probe new peers immediately
	m.probeAllPeers()
}

// initHandlers sets up packet handlers.
func (m *Manager) initHandlers() {
	m.On(&w3gs.GameInfo{}, m.onGameInfo)
}

// probeAllPeers sends SearchGame to all known peers and localhost.
func (m *Manager) probeAllPeers() {
	m.mu.RLock()
	peers := make([]tailscale.Peer, len(m.peers))
	copy(peers, m.peers)
	version := m.version
	m.mu.RUnlock()

	// Skip if version not yet detected
	if version.Version == 0 {
		return
	}

	// Probe localhost for local games
	m.probeLocal(version)

	// Probe remote Tailscale peers
	for i := range peers {
		peer := &peers[i]
		if peer.Online {
			m.probePeer(peer.IP, version)
		}
	}
}

// probeLocal sends a SearchGame packet to localhost to discover local games.
func (m *Manager) probeLocal(version w3gs.GameVersion) {
	addr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: lan.DefaultPort,
	}

	pkt := &w3gs.SearchGame{
		GameVersion: version,
		HostCounter: 0,
	}

	_, err := m.Send(addr, pkt)
	if err != nil {
		slog.Debug("failed to probe localhost", "error", err)
	}
}

// probePeer sends a SearchGame packet to a specific peer.
func (m *Manager) probePeer(peerIP netip.Addr, version w3gs.GameVersion) {
	addr := &net.UDPAddr{
		IP:   peerIP.AsSlice(),
		Port: lan.DefaultPort,
	}

	pkt := &w3gs.SearchGame{
		GameVersion: version,
		HostCounter: 0,
	}

	_, err := m.Send(addr, pkt)
	if err != nil {
		slog.Debug("failed to probe peer",
			"peer", peerIP,
			"error", err,
		)
	}
}

// onGameInfo handles GameInfo packets from local WC3 or remote peers.
func (m *Manager) onGameInfo(ev *network.Event) {
	pkt, ok := ev.Arg.(*w3gs.GameInfo)
	if !ok {
		return
	}

	// Get source address
	addr, ok := ev.Opt[0].(net.Addr)
	if !ok {
		return
	}

	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return
	}

	peerIP, ok := netip.AddrFromSlice(udpAddr.IP)
	if !ok {
		return
	}

	// Determine if this is a local or remote game
	var source game.Source

	var peerName string

	if peerIP.IsLoopback() {
		source = game.SourceLocal
		peerName = "local"
	} else {
		source = game.SourceRemote
		peerName = m.findPeerName(peerIP)
	}

	slog.Debug("discovered game",
		"name", pkt.GameName,
		"source", source,
		"peer", peerName,
		"peerIP", peerIP,
		"slots", pkt.SlotsUsed, "/", pkt.SlotsTotal,
	)

	m.registry.Add(game.Game{
		Info:     *pkt,
		Source:   source,
		PeerIP:   peerIP,
		PeerName: peerName,
	})
}

// findPeerName looks up the hostname for a peer IP.
func (m *Manager) findPeerName(ip netip.Addr) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.peers {
		peer := &m.peers[i]
		if peer.IP == ip {
			return peer.Name
		}
	}

	return ""
}
