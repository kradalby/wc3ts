package tailscale

import (
	"testing"

	"tailscale.com/ipn"
	"tailscale.com/tailcfg"
)

// TestWatchMaskValid guards against masks tailscaled rejects with 400.
func TestWatchMaskValid(t *testing.T) {
	t.Parallel()

	err := ipn.ValidateNotifyWatchOpt(watchMask)
	if err != nil {
		t.Fatalf("watch mask %v rejected by tailscaled: %v", watchMask, err)
	}
}

func TestPeersChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		notify ipn.Notify
		want   bool
	}{
		{name: "empty", notify: ipn.Notify{}, want: false},
		{name: "removed", notify: ipn.Notify{PeersRemoved: []tailcfg.NodeID{1}}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := peersChanged(tt.notify)
			if got != tt.want {
				t.Errorf("peersChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}
