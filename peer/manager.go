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

// udpBufferSize is the size of the UDP receive buffer.
const udpBufferSize = 512

// Manager probes Tailscale peers to discover remote WC3 games.
type Manager struct {
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
	// Start packet receiving in background (captures raw bytes)
	go m.receiveLoop()

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

// Refresh triggers an immediate probe of all peers.
func (m *Manager) Refresh() {
	m.probeAllPeers()
}

// OnPeersChanged handles peer list updates from Tailscale discovery.
func (m *Manager) OnPeersChanged(peers []tailscale.Peer) {
	m.mu.Lock()
	m.peers = peers
	m.mu.Unlock()

	// Probe new peers immediately
	m.probeAllPeers()
}

// receiveLoop reads raw UDP packets and processes them.
func (m *Manager) receiveLoop() {
	buf := make([]byte, udpBufferSize)

	for {
		n, addr, err := m.Conn().ReadFrom(buf)
		if err != nil {
			return
		}

		// Copy raw bytes before any processing
		rawData := make([]byte, n)
		copy(rawData, buf[:n])

		// Deserialize using gowarcraft3 for display/debug purposes
		pkt, _, err := w3gs.Deserialize(rawData, w3gs.Encoding{})
		if err != nil {
			continue
		}

		// Only handle GameInfo packets
		info, ok := pkt.(*w3gs.GameInfo)
		if !ok {
			continue
		}

		m.handleGameInfo(info, rawData, addr)
	}
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

// handleGameInfo processes a GameInfo packet with its raw bytes.
func (m *Manager) handleGameInfo(pkt *w3gs.GameInfo, rawData []byte, addr net.Addr) {
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

	// Always store raw data - needed for responder to send exact packets
	gameRawData := rawData

	slog.Debug("discovered game",
		"name", pkt.GameName,
		"hostCounter", pkt.HostCounter,
		"source", source,
		"peer", peerName,
		"peerIP", peerIP,
		"slots", pkt.SlotsUsed, "/", pkt.SlotsTotal,
	)

	m.registry.Add(game.Game{
		Info:     *pkt,
		RawData:  gameRawData,
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
