package jitter_test

import (
	"testing"
	"time"

	hqgoretrierjitter "github.com/hueristiq/hq-lib-retrier-go/jitter"
)

func TestEqual(t *testing.T) {
	t.Parallel()

	for range 64 {
		got := hqgoretrierjitter.Equal(10 * time.Second)

		if got < 5*time.Second || got > 10*time.Second {
			t.Fatalf("Equal(10s) = %v; want within [5s, 10s]", got)
		}
	}
}

func TestFull(t *testing.T) {
	t.Parallel()

	for range 64 {
		if got := hqgoretrierjitter.Full(10 * time.Second); got < 0 || got > 10*time.Second {
			t.Fatalf("Full(10s) = %v; want within [0, 10s]", got)
		}
	}
}

func TestDecorrelated(t *testing.T) {
	t.Parallel()

	for range 64 {
		got := hqgoretrierjitter.Decorrelated(time.Second, 30*time.Second, 4*time.Second)

		if got < time.Second || got > 30*time.Second {
			t.Fatalf("Decorrelated(1s, 30s, 4s) = %v; want within [1s, 30s]", got)
		}
	}
}
