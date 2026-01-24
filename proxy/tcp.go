// Package proxy provides TCP proxying for game connections.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/kradalby/wc3ts/game"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// Number of goroutines for bidirectional relay.
const relayGoroutines = 2

// Default timeout for connecting to remote hosts.
const dialTimeout = 10 * time.Second

// maxJoinPacketSize is the maximum expected size of a Join packet.
const maxJoinPacketSize = 512

// readTimeout is the timeout for reading the initial Join packet.
const readTimeout = 5 * time.Second

// ErrNoRemoteGame is returned when no remote game is found for a connection.
var ErrNoRemoteGame = errors.New("no remote game found for connection")

// ErrUnexpectedListenerType is returned when the listener address is not a TCP address.
var ErrUnexpectedListenerType = errors.New("unexpected listener address type")

// ErrUnexpectedPacketType is returned when the first packet is not a Join packet.
var ErrUnexpectedPacketType = errors.New("expected Join packet")

// TCPProxy proxies TCP connections to remote game hosts.
type TCPProxy struct {
	listener net.Listener
	registry *game.Registry
	port     int
}

// NewTCPProxy creates a new TCP proxy.
func NewTCPProxy(ctx context.Context, registry *game.Registry) (*TCPProxy, error) {
	// Listen on all interfaces with a random available port.
	// This is required because WC3 connects to the source IP of the UDP broadcast,
	// which is the LAN interface, not localhost.
	lc := &net.ListenConfig{}

	listener, err := lc.Listen(ctx, "tcp", "0.0.0.0:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP listener: %w", err)
	}

	// Extract the port from the listener address
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()

		return nil, ErrUnexpectedListenerType
	}

	return &TCPProxy{
		listener: listener,
		registry: registry,
		port:     addr.Port,
	}, nil
}

// Port returns the port the proxy is listening on.
func (p *TCPProxy) Port() int {
	return p.port
}

// Run starts accepting connections and proxying them.
// It blocks until the context is cancelled.
func (p *TCPProxy) Run(ctx context.Context) error {
	// Accept connections in background
	go p.acceptLoop(ctx)

	<-ctx.Done()

	return p.Close()
}

// Close stops the proxy and closes all connections.
func (p *TCPProxy) Close() error {
	return p.listener.Close()
}

// acceptLoop accepts incoming connections.
func (p *TCPProxy) acceptLoop(ctx context.Context) {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			// Check if listener was closed
			if errors.Is(err, net.ErrClosed) {
				return
			}

			slog.Error("failed to accept connection",
				"error", err,
			)

			continue
		}

		go p.handleConnection(ctx, conn)
	}
}

// handleConnection handles a single client connection.
func (p *TCPProxy) handleConnection(ctx context.Context, clientConn net.Conn) {
	defer func() {
		err := clientConn.Close()
		if err != nil {
			slog.Debug("error closing client connection", "error", err)
		}
	}()

	slog.Info("received TCP connection",
		"client", clientConn.RemoteAddr(),
	)

	// Read and parse the initial Join packet
	joinPkt, initialPacket, err := p.readJoinPacket(clientConn)
	if err != nil {
		slog.Error("failed to read Join packet",
			"client", clientConn.RemoteAddr(),
			"error", err,
		)

		return
	}

	slog.Info("parsed Join packet",
		"client", clientConn.RemoteAddr(),
		"hostCounter", joinPkt.HostCounter,
		"playerName", joinPkt.PlayerName,
	)

	// Find the game by HostCounter
	remoteGame := p.registry.FindByHostCounter(joinPkt.HostCounter)
	if remoteGame == nil {
		// Log all remote games for debugging
		allGames := p.registry.Games()
		for _, g := range allGames {
			slog.Info("registry game",
				"name", g.Info.GameName,
				"hostCounter", g.Info.HostCounter,
				"source", g.Source,
				"peerIP", g.PeerIP,
			)
		}

		slog.Warn("no remote game found for HostCounter",
			"client", clientConn.RemoteAddr(),
			"hostCounter", joinPkt.HostCounter,
		)

		return
	}

	slog.Info("found remote game",
		"game", remoteGame.Info.GameName,
		"hostCounter", remoteGame.Info.HostCounter,
		"peerIP", remoteGame.PeerIP,
		"gamePort", remoteGame.Info.GamePort,
	)

	// Connect to the remote host
	remoteConn, err := p.connectToRemote(ctx, remoteGame)
	if err != nil {
		slog.Error("failed to connect to remote game",
			"game", remoteGame.Info.GameName,
			"error", err,
		)

		return
	}

	defer func() {
		err := remoteConn.Close()
		if err != nil {
			slog.Debug("error closing remote connection", "error", err)
		}
	}()

	slog.Info("proxying connection",
		"client", clientConn.RemoteAddr(),
		"game", remoteGame.Info.GameName,
		"hostCounter", joinPkt.HostCounter,
		"player", joinPkt.PlayerName,
	)

	// Forward the initial Join packet to the remote host
	_, err = remoteConn.Write(initialPacket)
	if err != nil {
		slog.Error("failed to forward Join packet", "error", err)

		return
	}

	// Bidirectional relay for the rest of the traffic
	p.relay(clientConn, remoteConn)
}

// readJoinPacket reads and parses the initial Join packet from the client.
func (p *TCPProxy) readJoinPacket(conn net.Conn) (*w3gs.Join, []byte, error) {
	// Set read deadline for the initial packet
	err := conn.SetReadDeadline(time.Now().Add(readTimeout))
	if err != nil {
		return nil, nil, fmt.Errorf("set read deadline: %w", err)
	}

	// Read the first packet
	buf := make([]byte, maxJoinPacketSize)

	n, err := conn.Read(buf)
	if err != nil {
		return nil, nil, fmt.Errorf("read packet: %w", err)
	}

	// Clear the read deadline for future reads
	err = conn.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, nil, fmt.Errorf("clear read deadline: %w", err)
	}

	initialPacket := buf[:n]

	// Parse the packet
	pkt, _, err := w3gs.Deserialize(initialPacket, w3gs.Encoding{})
	if err != nil {
		return nil, nil, fmt.Errorf("deserialize packet: %w", err)
	}

	// Expect a Join packet
	joinPkt, ok := pkt.(*w3gs.Join)
	if !ok {
		return nil, nil, ErrUnexpectedPacketType
	}

	slog.Debug("received Join packet",
		"hostCounter", joinPkt.HostCounter,
		"playerName", joinPkt.PlayerName,
	)

	return joinPkt, initialPacket, nil
}

// connectToRemote establishes a connection to the remote game host.
func (p *TCPProxy) connectToRemote(ctx context.Context, g *game.Game) (net.Conn, error) {
	remoteAddr := net.JoinHostPort(
		g.PeerIP.String(),
		strconv.Itoa(int(g.Info.GamePort)),
	)

	dialer := &net.Dialer{
		Timeout: dialTimeout,
	}

	return dialer.DialContext(ctx, "tcp", remoteAddr)
}

// relay copies data bidirectionally between two connections.
func (p *TCPProxy) relay(conn1, conn2 net.Conn) {
	var wg sync.WaitGroup

	wg.Add(relayGoroutines)

	// Copy conn1 -> conn2
	go func() {
		defer wg.Done()

		_, err := io.Copy(conn2, conn1)
		if err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Debug("relay error (client -> remote)",
				"error", err,
			)
		}

		// Close the write side when done reading
		if tc, ok := conn2.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	// Copy conn2 -> conn1
	go func() {
		defer wg.Done()

		_, err := io.Copy(conn1, conn2)
		if err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Debug("relay error (remote -> client)",
				"error", err,
			)
		}

		// Close the write side when done reading
		if tc, ok := conn1.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	wg.Wait()
}
