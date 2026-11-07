package retrier_test

import (
	"context"
	"testing"
	"time"

	hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
)

var (
	sinkResult int
	errSink    error
)

func BenchmarkRetryWithData(b *testing.B) {
	b.ReportAllocs()

	constantBackoff := func(_, _ time.Duration) hqgoretrierbackoff.Backoff {
		return func(int) time.Duration {
			return time.Nanosecond
		}
	}

	ctx := context.Background()

	for range b.N {
		calls := 0

		operation := func(context.Context) (int, error) {
			calls++

			if calls <= 2 {
				return 0, errTestOperation
			}

			return 42, nil
		}

		sinkResult, errSink = hqgoretrier.RetryWithData(
			ctx,
			operation,
			hqgoretrier.WithMaxAttempts(5),
			hqgoretrier.WithBackoff(constantBackoff),
		)
	}
}
