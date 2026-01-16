// Package tailscale provides Tailscale peer discovery via the local API.
package tailscale

import (
	"context"
	"net/netip"
	"sync"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/types/netmap"
)

// Peer represents a Tailscale peer.
type Peer struct {
	// Name is the peer's hostname.
	Name string

	// IP is the peer's Tailscale IPv4 address.
	IP netip.Addr

	// Online indicates if the peer is currently connected.
	Online bool
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
	var peers []Peer

	// Extract self IP from SelfNode
	if nm.SelfNode.Valid() {
		addrs := nm.SelfNode.Addresses()

		for i := range addrs.Len() {
			addr := addrs.At(i).Addr()
			if addr.Is4() {
				d.mu.Lock()
				d.selfIP = addr
				d.mu.Unlock()

				break
			}
		}
	}

	// Extract peer information
	for _, p := range nm.Peers {
		if !p.Valid() {
			continue
		}

		// Check if peer is online
		online := p.Online().GetOr(false)
		if !online {
			continue
		}

		// Find first IPv4 address
		addrs := p.Addresses()

		for i := range addrs.Len() {
			addr := addrs.At(i).Addr()
			if addr.Is4() {
				peers = append(peers, Peer{
					Name:   p.ComputedName(),
					IP:     addr,
					Online: online,
				})

				break // Only first IPv4
			}
		}
	}

	d.mu.Lock()
	d.peers = peers
	d.mu.Unlock()

	if d.onChange != nil {
		d.onChange(peers)
	}
}
