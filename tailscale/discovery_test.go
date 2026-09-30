package tailscale

import (
	"testing"

	"tailscale.com/ipn"
)

// TestWatchMaskValid guards against masks tailscaled rejects with 400.
func TestWatchMaskValid(t *testing.T) {
	err := ipn.ValidateNotifyWatchOpt(watchMask)
	if err != nil {
		t.Fatalf("watch mask %v rejected by tailscaled: %v", watchMask, err)
	}
}
