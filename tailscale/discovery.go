// Package tailscale provides Tailscale peer discovery via the local API.
package tailscale

import (
	"context"
	"net/netip"
	"slices"
	"strings"
	"sync"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/tailcfg"
	"tailscale.com/types/netmap"
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

// OnPeersChangedFunc is called when the peer list changes.
type OnPeersChangedFunc func(peers []Peer)

// Discovery watches for Tailscale peer changes via the IPN bus.
type Discovery struct {
	client   *local.Client
	watcher  *local.IPNBusWatcher
	peers    []Peer
	selfIP   netip.Addr
	onChange OnPeersChangedFunc
	mu       sync.RWMutex
}

// NewDiscovery creates a new Tailscale discovery instance.
func NewDiscovery(onChange OnPeersChangedFunc) *Discovery {
	return &Discovery{
		client:   &local.Client{},
		peers:    make([]Peer, 0),
		onChange: onChange,
	}
}

// Run starts watching for peer changes.
// It blocks until the context is cancelled or an error occurs.
func (d *Discovery) Run(ctx context.Context) error {
	// Subscribe with initial netmap and rate limiting
	mask := ipn.NotifyInitialNetMap | ipn.NotifyRateLimit

	watcher, err := d.client.WatchIPNBus(ctx, mask)
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.watcher = watcher
	d.mu.Unlock()

	defer func() {
		_ = watcher.Close()
	}()

	for {
		notify, err := watcher.Next()
		if err != nil {
			return err
		}

		// NetMap contains peer information when it changes
		if notify.NetMap != nil {
			d.updateFromNetMap(notify.NetMap)
		}
	}
}

// Peers returns a copy of the current peer list.
func (d *Discovery) Peers() []Peer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]Peer, len(d.peers))
	copy(result, d.peers)

	return result
}

// SelfIP returns this node's Tailscale IPv4 address.
// Returns zero addr if not yet known from netmap updates.
func (d *Discovery) SelfIP() netip.Addr {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.selfIP
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
			d.mu.Lock()
			d.selfIP = ip
			d.mu.Unlock()

			return ip, nil
		}
	}

	return netip.Addr{}, nil
}

// Close stops the discovery watcher.
func (d *Discovery) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.watcher != nil {
		return d.watcher.Close()
	}

	return nil
}

// updateFromNetMap extracts peer information from a network map.
func (d *Discovery) updateFromNetMap(nm *netmap.NetworkMap) {
	d.extractSelfIP(nm)
	peers := d.extractPeers(nm)

	d.mu.Lock()
	d.peers = peers
	d.mu.Unlock()

	if d.onChange != nil {
		d.onChange(peers)
	}
}

// extractSelfIP extracts our own IP from the SelfNode in the network map.
func (d *Discovery) extractSelfIP(nm *netmap.NetworkMap) {
	if !nm.SelfNode.Valid() {
		return
	}

	addrs := nm.SelfNode.Addresses()

	for i := range addrs.Len() {
		addr := addrs.At(i).Addr()
		if addr.Is4() {
			d.mu.Lock()
			d.selfIP = addr
			d.mu.Unlock()

			return
		}
	}
}

// extractPeers extracts peer information from the network map.
func (d *Discovery) extractPeers(nm *netmap.NetworkMap) []Peer {
	var peers []Peer

	for _, p := range nm.Peers {
		peer, ok := d.extractPeer(p)
		if ok {
			peers = append(peers, peer)
		}
	}

	return peers
}

// extractPeer extracts a single peer's information if valid.
func (d *Discovery) extractPeer(p tailcfg.NodeView) (Peer, bool) {
	if !p.Valid() {
		return Peer{}, false
	}

	// Check if peer is online
	online := p.Online().GetOr(false)
	if !online {
		return Peer{}, false
	}

	// Filter out Mullvad exit nodes
	tags := p.Tags()
	if slices.Contains(tags.AsSlice(), mullvadExitNodeTag) {
		return Peer{}, false
	}

	// Extract OS from hostinfo
	os := ""
	if hi := p.Hostinfo(); hi.Valid() {
		os = hi.OS()
	}

	// Filter out mobile devices (iOS, Android) - they cannot run WC3
	osLower := strings.ToLower(os)
	if osLower == "ios" || osLower == "android" {
		return Peer{}, false
	}

	// Find first IPv4 address
	addrs := p.Addresses()

	for i := range addrs.Len() {
		addr := addrs.At(i).Addr()
		if addr.Is4() {
			return Peer{
				Name:   p.ComputedName(),
				IP:     addr,
				Online: online,
				OS:     os,
			}, true
		}
	}

	return Peer{}, false
}
