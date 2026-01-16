// Package main provides the entry point for wc3ts.
package main

import (
	"context"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kradalby/wc3ts/config"
	"github.com/kradalby/wc3ts/game"
	"github.com/kradalby/wc3ts/lan"
	"github.com/kradalby/wc3ts/peer"
	"github.com/kradalby/wc3ts/proxy"
	"github.com/kradalby/wc3ts/tailscale"
	"github.com/kradalby/wc3ts/tui"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// app holds the application state and dependencies.
type app struct {
	cfg            *config.Config
	registry       *game.Registry
	tcpProxy       *proxy.TCPProxy
	discovery      *tailscale.Discovery
	peerManager    *peer.Manager
	advertiser     *lan.Advertiser
	lanListener    *lan.Listener
	lanListenerErr error
	program        *tea.Program
}

func main() {
	// Set up logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	err := run()
	if err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	a, err := newApp(ctx)
	if err != nil {
		return err
	}

	a.startServices(ctx)

	err = a.runTUI()
	if err != nil {
		return err
	}

	// Clean up
	cancel()
	a.advertiser.Close()

	return nil
}

func newApp(ctx context.Context) (*app, error) {
	a := &app{
		cfg: config.Default(),
	}

	// Create game registry with callback
	a.registry = game.NewRegistry(a.onGamesChanged)

	// Create TCP proxy
	var err error

	a.tcpProxy, err = proxy.NewTCPProxy(ctx, a.registry)
	if err != nil {
		return nil, err
	}

	// Create Tailscale discovery
	a.discovery = tailscale.NewDiscovery(a.onPeersChanged)

	// Create peer manager
	a.peerManager, err = peer.NewManager(a.discovery, a.registry, a.cfg.ProbeInterval)
	if err != nil {
		return nil, err
	}

	// Create LAN advertiser
	proxyPort := safeUint16(a.tcpProxy.Port())
	a.advertiser = lan.NewAdvertiser(proxyPort, a.cfg.ShowPeerNames)

	// Update registry callback to also notify advertiser
	a.registry = game.NewRegistry(a.onGamesChanged)

	// Recreate peer manager with new registry
	a.peerManager, err = peer.NewManager(a.discovery, a.registry, a.cfg.ProbeInterval)
	if err != nil {
		return nil, err
	}

	// Create LAN listener (non-fatal if it fails - WC3 may have the port)
	a.lanListener, err = lan.NewListener(ctx, a.registry, a.onVersionDetected)
	if err != nil {
		slog.Warn("LAN listener failed to start", "error", err)
		a.lanListenerErr = err
		// Continue without LAN listener - will show warning in TUI
	}

	return a, nil
}

func (a *app) onGamesChanged(games []game.Game) {
	if a.program != nil {
		a.program.Send(tui.GamesMsg{Games: games})
	}

	if a.advertiser != nil {
		a.advertiser.OnGamesChanged(games)
	}
}

func (a *app) onPeersChanged(peers []tailscale.Peer) {
	if a.program != nil {
		a.program.Send(tui.PeersMsg{Peers: peers})
	}

	if a.peerManager != nil {
		a.peerManager.OnPeersChanged(peers)
	}
}

func (a *app) onVersionDetected(version w3gs.GameVersion) {
	if a.program != nil {
		a.program.Send(tui.VersionMsg{Version: version})
	}

	if a.peerManager != nil {
		a.peerManager.SetVersion(version)
	}
}

func (a *app) startServices(ctx context.Context) {
	go a.runDiscovery(ctx)
	go a.runPeerManager(ctx)
	go a.runLANListener(ctx)
	go a.runTCPProxy(ctx)
}

func (a *app) runDiscovery(ctx context.Context) {
	err := a.discovery.Run(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Error("tailscale discovery error", "error", err)
	}
}

func (a *app) runPeerManager(ctx context.Context) {
	err := a.peerManager.Run(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Error("peer manager error", "error", err)
	}
}

func (a *app) runLANListener(ctx context.Context) {
	if a.lanListener == nil {
		return
	}

	err := a.lanListener.Run(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Error("LAN listener error", "error", err)
	}
}

func (a *app) runTCPProxy(ctx context.Context) {
	err := a.tcpProxy.Run(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Error("TCP proxy error", "error", err)
	}
}

func (a *app) runTUI() error {
	model := tui.NewModel(a.tcpProxy.Port())
	a.program = tea.NewProgram(model, tea.WithAltScreen())

	// Send warning if LAN listener failed to start
	if a.lanListenerErr != nil {
		go func() {
			a.program.Send(tui.WarningMsg{
				Message: "LAN listener unavailable - start wc3ts before Warcraft III",
			})
		}()
	}

	_, err := a.program.Run()

	return err
}

// safeUint16 safely converts an int to uint16, clamping to max value.
func safeUint16(n int) uint16 {
	if n < 0 {
		return 0
	}

	if n > math.MaxUint16 {
		return math.MaxUint16
	}

	return uint16(n)
}
