// Package tailscale provides Tailscale peer discovery via the local API.
package tailscale

import (
	"context"
	"net/netip"
	"strings"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
)

// mullvadExitNodeTag is the tag used by Mullvad exit nodes.
const mullvadExitNodeTag = "tag:mullvad-exit-node"

// Peer represents a Tailscale peer.
type Peer struct {
	// Name is the peer's hostname.
	Name string

	// IP is the peer's Tailscale IPv4 address.
	IP netip.Addr

	// Online indicates if the peer is currently connected.
	Online bool

	// OS is the peer's operating system (e.g., "windows", "macOS", "linux").
	OS string
}

// OnPeersChangedFunc is called when the peer list changes. peers is shared
// between subscribers and must not be modified.
type OnPeersChangedFunc func(peers []Peer)

// Discovery watches for Tailscale peer changes via the IPN bus.
type Discovery struct {
	client   *local.Client
	onChange OnPeersChangedFunc
}

// NewDiscovery creates a new Tailscale discovery instance.
func NewDiscovery(onChange OnPeersChangedFunc) *Discovery {
	return &Discovery{
		client:   &local.Client{},
		onChange: onChange,
	}
}

// Run starts watching for peer changes.
// It blocks until the context is cancelled or an error occurs.
func (d *Discovery) Run(ctx context.Context) error {
	// Subscribe to peer-set deltas, rate-limited. The bus is only used as a
	// change trigger; peer data is pulled from Status, since Notify.NetMap is
	// deprecated and not delivered after the initial notify on Linux.
	mask := ipn.NotifyPeerChanges | ipn.NotifyRateLimit

	watcher, err := d.client.WatchIPNBus(ctx, mask)
	if err != nil {
		return err
	}

	defer func() {
		_ = watcher.Close()
	}()

	// Populate initial state before waiting on the bus.
	err = d.refresh(ctx)
	if err != nil {
		return err
	}

	for {
		notify, err := watcher.Next()
		if err != nil {
			return err
		}

		if len(notify.PeersChanged) > 0 || len(notify.PeersRemoved) > 0 || notify.SelfChange != nil {
			err = d.refresh(ctx)
			if err != nil {
				return err
			}
		}
	}
}

// FetchSelfIP queries the Tailscale daemon for our IP address.
// This can be called before Run() to get the IP synchronously.
func (d *Discovery) FetchSelfIP(ctx context.Context) (netip.Addr, error) {
	status, err := d.client.Status(ctx)
	if err != nil {
		return netip.Addr{}, err
	}

	for _, ip := range status.TailscaleIPs {
		if ip.Is4() {
			return ip, nil
		}
	}

	return netip.Addr{}, nil
}

// refresh pulls the current status and rebuilds the peer list.
func (d *Discovery) refresh(ctx context.Context) error {
	status, err := d.client.Status(ctx)
	if err != nil {
		return err
	}

	if d.onChange != nil {
		d.onChange(extractPeers(status))
	}

	return nil
}

// extractPeers extracts online, non-exit, non-mobile peers from the status.
func extractPeers(status *ipnstate.Status) []Peer {
	var peers []Peer

	for _, ps := range status.Peer {
		if peer, ok := extractPeer(ps); ok {
			peers = append(peers, peer)
		}
	}

	return peers
}

// extractPeer extracts a single peer's information if it's a usable WC3 host.
func extractPeer(ps *ipnstate.PeerStatus) (Peer, bool) {
	if ps == nil || !ps.Online {
		return Peer{}, false
	}

	// Filter out Mullvad exit nodes.
	if ps.Tags != nil {
		for i := range ps.Tags.Len() {
			if ps.Tags.At(i) == mullvadExitNodeTag {
				return Peer{}, false
			}
		}
	}

	// Filter out mobile devices (iOS, Android) - they cannot run WC3.
	switch strings.ToLower(ps.OS) {
	case "ios", "android":
		return Peer{}, false
	}

	ip, ok := firstIPv4(ps)
	if !ok {
		return Peer{}, false
	}

	return Peer{
		Name:   peerName(ps),
		IP:     ip,
		Online: ps.Online,
		OS:     ps.OS,
	}, true
}

// peerName returns a short display name: the first DNS label, else the hostname.
func peerName(ps *ipnstate.PeerStatus) string {
	if ps.DNSName != "" {
		if name, _, _ := strings.Cut(ps.DNSName, "."); name != "" {
			return name
		}
	}

	return ps.HostName
}

// firstIPv4 returns the peer's first IPv4 Tailscale address.
func firstIPv4(ps *ipnstate.PeerStatus) (netip.Addr, bool) {
	if ps == nil {
		return netip.Addr{}, false
	}

	for _, ip := range ps.TailscaleIPs {
		if ip.Is4() {
			return ip, true
		}
	}

	return netip.Addr{}, false
}
