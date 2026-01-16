// Package main provides the entry point for wc3ts.
package main

import (
	"context"
	"flag"
	"fmt"
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
	"github.com/kradalby/wc3ts/version"
)

// app holds the application state and dependencies.
type app struct {
	cfg         *config.Config
	registry    *game.Registry
	tcpProxy    *proxy.TCPProxy
	discovery   *tailscale.Discovery
	peerManager *peer.Manager
	responder   *peer.Responder
	broadcaster *lan.Broadcaster
	program     *tea.Program
}

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")

	flag.Parse()

	if *showVersion {
		v := version.Get()
		_, _ = fmt.Fprintf(os.Stdout, "wc3ts %s\n", v.String())

		if v.GoVer != "" {
			_, _ = fmt.Fprintf(os.Stdout, "  go: %s\n", v.GoVer)
		}

		return
	}

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

	if a.broadcaster != nil {
		_ = a.broadcaster.Close()
	}

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

	// Create LAN broadcaster (uses ephemeral port, doesn't conflict with WC3)
	proxyPort := safeUint16(a.tcpProxy.Port())

	a.broadcaster, err = lan.NewBroadcaster(proxyPort, a.cfg.ShowPeerNames)
	if err != nil {
		return nil, err
	}

	// Update registry callback to also notify broadcaster
	a.registry = game.NewRegistry(a.onGamesChanged)

	// Recreate peer manager with new registry
	a.peerManager, err = peer.NewManager(a.discovery, a.registry, a.cfg.ProbeInterval)
	if err != nil {
		return nil, err
	}

	// Set default version for peer probing
	a.peerManager.SetVersion(a.cfg.GameVersion)

	// Create responder to answer queries from remote Tailscale peers
	// This requires our Tailscale IP, so we fetch it synchronously
	localIP, err := a.discovery.FetchSelfIP(ctx)
	if err != nil {
		slog.Warn("could not get Tailscale IP, remote discovery disabled", "error", err)
	} else if localIP.IsValid() {
		a.responder, err = peer.NewResponder(a.registry, localIP)
		if err != nil {
			slog.Warn("could not create responder, remote discovery disabled", "error", err)
		} else {
			slog.Info("responder listening for remote queries", "ip", localIP)
		}
	}

	return a, nil
}

func (a *app) onGamesChanged(games []game.Game) {
	if a.program != nil {
		a.program.Send(tui.GamesMsg{Games: games})
	}

	if a.broadcaster != nil {
		a.broadcaster.OnGamesChanged(games)
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

func (a *app) startServices(ctx context.Context) {
	go a.runDiscovery(ctx)
	go a.runPeerManager(ctx)
	go a.runBroadcaster(ctx)
	go a.runTCPProxy(ctx)

	if a.responder != nil {
		go a.runResponder(ctx)
	}
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

func (a *app) runBroadcaster(ctx context.Context) {
	err := a.broadcaster.Run(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Error("broadcaster error", "error", err)
	}
}

func (a *app) runTCPProxy(ctx context.Context) {
	err := a.tcpProxy.Run(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Error("TCP proxy error", "error", err)
	}
}

func (a *app) runResponder(ctx context.Context) {
	err := a.responder.Run(ctx)
	if err != nil && ctx.Err() == nil {
		slog.Error("responder error", "error", err)
	}
}

func (a *app) runTUI() error {
	model := tui.NewModel(a.tcpProxy.Port(), a.cfg.GameVersion, version.Get())
	a.program = tea.NewProgram(model, tea.WithAltScreen())

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
