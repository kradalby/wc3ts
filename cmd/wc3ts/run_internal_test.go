package main

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

var errBusClosed = errors.New("bus closed")

// A failed IPN bus stream must be retried after a wait, until ctx ends.
func TestRunDiscoveryRetries(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		var starts []time.Time

		runDiscovery(ctx, func(context.Context) error {
			starts = append(starts, time.Now())

			switch len(starts) {
			case 3:
				cancel()
			case 4:
				t.Fatal("run called after ctx was cancelled")
			}

			return errBusClosed
		})

		if len(starts) != 3 {
			t.Fatalf("run called %d times, want 3", len(starts))
		}

		for i := 1; i < len(starts); i++ {
			if !starts[i].After(starts[i-1]) {
				t.Errorf("retry %d did not wait", i)
			}
		}
	})
}
