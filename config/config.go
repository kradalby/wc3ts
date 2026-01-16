// Package config provides configuration for the wc3ts proxy.
package config

import (
	"time"

	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// Default configuration values.
const (
	DefaultProbeInterval   = 2 * time.Second
	DefaultRefreshInterval = 3 * time.Second
	DefaultGameTimeout     = 10 * time.Second

	// DefaultGameVersion is TFT 1.28 - common for LAN parties.
	DefaultGameVersion = 10028
)

// Config holds the configuration for the WC3 Tailscale proxy.
type Config struct {
	// GameVersion specifies the WC3 version to use.
	// If zero, version will be auto-detected from local WC3 traffic.
	GameVersion w3gs.GameVersion

	// ProbeInterval is how often to probe peers for games.
	ProbeInterval time.Duration

	// RefreshInterval is how often to refresh game advertisements.
	RefreshInterval time.Duration

	// GameTimeout is how long before a game is considered stale.
	GameTimeout time.Duration

	// ShowPeerNames prefixes game names with peer hostname.
	ShowPeerNames bool
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		GameVersion: w3gs.GameVersion{
			Product: w3gs.ProductTFT,
			Version: DefaultGameVersion,
		},
		ProbeInterval:   DefaultProbeInterval,
		RefreshInterval: DefaultRefreshInterval,
		GameTimeout:     DefaultGameTimeout,
		ShowPeerNames:   true,
	}
}
