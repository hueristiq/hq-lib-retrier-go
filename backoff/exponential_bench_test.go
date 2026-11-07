package backoff_test

import (
	"testing"
	"time"

	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
)

var sinkDuration time.Duration

func BenchmarkConstant(b *testing.B) {
	backoff := hqgoretrierbackoff.Constant(time.Millisecond, 30*time.Second)

	b.ReportAllocs()

	for range b.N {
		sinkDuration = backoff(5)
	}
}

func BenchmarkExponential(b *testing.B) {
	backoff := hqgoretrierbackoff.Exponential(time.Millisecond, 30*time.Second)

	b.ReportAllocs()

	for range b.N {
		sinkDuration = backoff(5)
	}
}

func BenchmarkExponentialWithDecorrelatedJitter(b *testing.B) {
	backoff := hqgoretrierbackoff.ExponentialWithDecorrelatedJitter(time.Millisecond, 30*time.Second)

	b.ReportAllocs()

	for range b.N {
		sinkDuration = backoff(5)
	}
}
