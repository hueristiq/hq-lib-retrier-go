package jitter_test

import (
	"testing"
	"time"

	hqgoretrierjitter "github.com/hueristiq/hq-lib-retrier-go/jitter"
)

var sinkDuration time.Duration

func BenchmarkEqual(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		sinkDuration = hqgoretrierjitter.Equal(8 * time.Second)
	}
}

func BenchmarkFull(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		sinkDuration = hqgoretrierjitter.Full(8 * time.Second)
	}
}

func BenchmarkDecorrelated(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		sinkDuration = hqgoretrierjitter.Decorrelated(time.Second, time.Minute, 4*time.Second)
	}
}
