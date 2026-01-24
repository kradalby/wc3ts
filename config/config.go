// Package config provides configuration for the wc3ts proxy.
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// Default configuration values.
const (
	DefaultProbeInterval   = 2 * time.Second
	DefaultRefreshInterval = 3 * time.Second
	DefaultGameTimeout     = 10 * time.Second

	// DefaultGameVersion is TFT 1.26 - common for classic WC3 LAN parties.
	// Classic WC3 versions: 26 (1.26), 27 (1.27), 28 (1.28).
	DefaultGameVersion = 26
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

// ParseVersion parses a version string like "1.26", "26", or "1.28" into uint32.
// Accepts formats: "1.26" -> 26, "26" -> 26, "1.28" -> 28.
func ParseVersion(s string) (uint32, error) {
	s = strings.TrimSpace(s)

	// Handle "1.XX" format
	if after, found := strings.CutPrefix(s, "1."); found {
		s = after
	}

	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", s, err)
	}

	return uint32(v), nil
}

// FormatVersion formats a version number as "1.XX".
func FormatVersion(v uint32) string {
	return fmt.Sprintf("1.%d", v)
}

// SupportedVersions returns the list of supported WC3 versions.
func SupportedVersions() []uint32 {
	return []uint32{26, 27, 28}
}
