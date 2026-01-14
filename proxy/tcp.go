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
)

// Number of goroutines for bidirectional relay.
const relayGoroutines = 2

// Default timeout for connecting to remote hosts.
const dialTimeout = 10 * time.Second

// ErrNoRemoteGame is returned when no remote game is found for a connection.
var ErrNoRemoteGame = errors.New("no remote game found for connection")

// ErrUnexpectedListenerType is returned when the listener address is not a TCP address.
var ErrUnexpectedListenerType = errors.New("unexpected listener address type")

// TCPProxy proxies TCP connections to remote game hosts.
type TCPProxy struct {
	listener net.Listener
	registry *game.Registry
	port     int
}

// NewTCPProxy creates a new TCP proxy.
func NewTCPProxy(ctx context.Context, registry *game.Registry) (*TCPProxy, error) {
	// Listen on localhost with a random available port.
	// Using localhost is more secure than binding to all interfaces.
	lc := &net.ListenConfig{}

	listener, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
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
			slog.Debug("error closing client connection",
				"error", err,
			)
		}
	}()

	// Find a remote game to connect to
	remoteGame := p.findRemoteGame()
	if remoteGame == nil {
		slog.Warn("no remote game found for incoming connection",
			"client", clientConn.RemoteAddr(),
		)

		return
	}

	// Connect to the remote host using JoinHostPort for IPv6 compatibility
	remoteAddr := net.JoinHostPort(
		remoteGame.PeerIP.String(),
		strconv.Itoa(int(remoteGame.Info.GamePort)),
	)

	dialer := &net.Dialer{
		Timeout: dialTimeout,
	}

	remoteConn, err := dialer.DialContext(ctx, "tcp", remoteAddr)
	if err != nil {
		slog.Error("failed to connect to remote game",
			"game", remoteGame.Info.GameName,
			"remote", remoteAddr,
			"error", err,
		)

		return
	}

	defer func() {
		err := remoteConn.Close()
		if err != nil {
			slog.Debug("error closing remote connection",
				"error", err,
			)
		}
	}()

	slog.Info("proxying connection",
		"client", clientConn.RemoteAddr(),
		"game", remoteGame.Info.GameName,
		"remote", remoteAddr,
	)

	// Bidirectional relay
	p.relay(clientConn, remoteConn)
}

// findRemoteGame finds a remote game to proxy to.
// Currently returns the first remote game found.
func (p *TCPProxy) findRemoteGame() *game.Game {
	remoteGames := p.registry.RemoteGames()
	if len(remoteGames) == 0 {
		return nil
	}

	return &remoteGames[0]
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
