package backoff_test

import (
	"testing"
	"time"

	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
)

func TestBackoffType(t *testing.T) {
	t.Parallel()

	backoff := hqgoretrierbackoff.Constant(time.Second, time.Second)

	if got := backoff(1); got != time.Second {
		t.Fatalf("Backoff(1) = %v, want 1s", got)
	}
}
