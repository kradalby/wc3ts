package main

import (
	"context"
	"flag"
	"log/slog"
	"math"
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
	"github.com/peterbourgon/ff/v3/ffcli"
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

func newRunCommand() *ffcli.Command {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	versionStr := fs.String("version", "26", "Game version (e.g., 26, 1.26, 27, 1.27, 28, 1.28)")

	return &ffcli.Command{
		Name:       "run",
		ShortUsage: "wc3ts run [flags]",
		ShortHelp:  "Run the WC3 LAN proxy with TUI",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			gameVersion, err := config.ParseVersion(*versionStr)
			if err != nil {
				return err
			}

			return runExec(ctx, args, gameVersion)
		},
	}
}

func runExec(ctx context.Context, _ []string, gameVersion uint32) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create app with default config
	a := &app{
		cfg: config.Default(),
	}

	// Override version if specified
	a.cfg.GameVersion.Version = gameVersion

	// Initialize services first (so we have peer manager for the callback)
	err := a.initServices(ctx)
	if err != nil {
		return err
	}

	// Create TUI model with version callback that updates peer manager
	versionCallback := func(v uint32) {
		newVersion := a.cfg.GameVersion
		newVersion.Version = v
		a.peerManager.SetVersion(newVersion)
		slog.Info("version changed", "version", config.FormatVersion(v))
	}

	// Create refresh callback that triggers peer probing
	refreshCallback := func() {
		a.peerManager.Refresh()
		slog.Debug("manual refresh triggered")
	}

	model := tui.NewModel(0, a.cfg.GameVersion, version.Get(), versionCallback, refreshCallback)
	a.program = tea.NewProgram(model, tea.WithAltScreen())

	// Set up logging to TUI (Debug level to see everything)
	handler := tui.NewHandler(a.program, slog.LevelDebug)
	slog.SetDefault(slog.New(handler))

	a.startServices(ctx)

	// Start TUI in goroutine
	tuiDone := make(chan error, 1)

	go func() {
		_, err := a.program.Run()
		tuiDone <- err
	}()

	// Mark handler ready once program is running
	handler.SetReady()

	// Update TUI model with actual proxy port
	a.program.Send(tui.PortMsg{Port: a.tcpProxy.Port()})

	// Log that we're ready
	slog.Info("wc3ts started", "proxyPort", a.tcpProxy.Port())

	// Wait for TUI to finish
	err = <-tuiDone
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

func (a *app) initServices(ctx context.Context) error {
	// Create game registry with callback
	a.registry = game.NewRegistry(a.onGamesChanged)

	// Create TCP proxy
	var err error

	a.tcpProxy, err = proxy.NewTCPProxy(ctx, a.registry)
	if err != nil {
		return err
	}

	// Create Tailscale discovery
	a.discovery = tailscale.NewDiscovery(a.onPeersChanged)

	// Create peer manager
	a.peerManager, err = peer.NewManager(a.discovery, a.registry, a.cfg.ProbeInterval)
	if err != nil {
		return err
	}

	// Create LAN broadcaster (uses ephemeral port, doesn't conflict with WC3)
	proxyPort := safeUint16(a.tcpProxy.Port())

	a.broadcaster, err = lan.NewBroadcaster(proxyPort)
	if err != nil {
		return err
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

	return nil
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
