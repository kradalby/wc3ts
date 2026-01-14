package lan

import (
	"log/slog"
	"sync"

	"github.com/kradalby/wc3ts/game"
	"github.com/nielsAD/gowarcraft3/network/lan"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// Advertiser manages UDP advertisers for remote games.
type Advertiser struct {
	advertisers   map[string]*lan.UDPAdvertiser
	proxyPort     uint16
	showPeerNames bool
	mu            sync.Mutex
}

// NewAdvertiser creates a new advertiser manager.
func NewAdvertiser(proxyPort uint16, showPeerNames bool) *Advertiser {
	return &Advertiser{
		advertisers:   make(map[string]*lan.UDPAdvertiser),
		proxyPort:     proxyPort,
		showPeerNames: showPeerNames,
	}
}

// OnGamesChanged handles game list updates from the registry.
func (a *Advertiser) OnGamesChanged(games []game.Game) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Build set of current remote game keys
	currentGames := make(map[string]bool)

	for i := range games {
		g := &games[i]
		if g.Source == game.SourceRemote {
			currentGames[g.Key()] = true
		}
	}

	// Remove advertisers for games that no longer exist
	for key, adv := range a.advertisers {
		if !currentGames[key] {
			a.stopAdvertiser(key, adv)
		}
	}

	// Add advertisers for new remote games
	for i := range games {
		g := &games[i]
		if g.Source != game.SourceRemote {
			continue
		}

		key := g.Key()
		if _, exists := a.advertisers[key]; !exists {
			a.startAdvertiser(g)
		}
	}
}

// Close stops all advertisers.
func (a *Advertiser) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for key, adv := range a.advertisers {
		a.stopAdvertiser(key, adv)
	}
}

// startAdvertiser creates and starts a UDP advertiser for a remote game.
func (a *Advertiser) startAdvertiser(g *game.Game) {
	// Create modified game info with proxy port
	info := a.modifyGameInfo(g)

	adv, err := lan.NewUDPAdvertiser(&info, DefaultPort)
	if err != nil {
		slog.Error("failed to create advertiser",
			"game", g.Info.GameName,
			"error", err,
		)

		return
	}

	a.advertisers[g.Key()] = adv

	// Start advertising in background
	go func() {
		adv.InitDefaultHandlers()

		err := adv.Create()
		if err != nil {
			slog.Error("failed to create game advertisement",
				"game", g.Info.GameName,
				"error", err,
			)

			return
		}

		err = adv.Run()
		if err != nil {
			slog.Debug("advertiser stopped",
				"game", g.Info.GameName,
				"error", err,
			)
		}
	}()

	slog.Info("advertising remote game",
		"name", info.GameName,
		"peer", g.PeerName,
		"peerIP", g.PeerIP,
	)
}

// stopAdvertiser stops and removes a UDP advertiser.
func (a *Advertiser) stopAdvertiser(key string, adv *lan.UDPAdvertiser) {
	err := adv.Decreate()
	if err != nil {
		slog.Debug("failed to decreate game",
			"key", key,
			"error", err,
		)
	}

	err = adv.Close()
	if err != nil {
		slog.Debug("failed to close advertiser",
			"key", key,
			"error", err,
		)
	}

	delete(a.advertisers, key)

	slog.Debug("stopped advertising game",
		"key", key,
	)
}

// modifyGameInfo creates a modified copy of the game info for local advertisement.
func (a *Advertiser) modifyGameInfo(g *game.Game) w3gs.GameInfo {
	info := g.Info

	// Replace port with local TCP proxy port
	info.GamePort = a.proxyPort

	// Optionally prefix game name with peer hostname
	if a.showPeerNames && g.PeerName != "" {
		info.GameName = "[" + g.PeerName + "] " + info.GameName
	}

	return info
}
