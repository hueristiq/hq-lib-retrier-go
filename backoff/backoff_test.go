package backoff_test

import (
	"testing"
	"time"

	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
)

func TestExponential(t *testing.T) {
	t.Parallel()

	backoff := hqgoretrierbackoff.Exponential(time.Second, 30*time.Second)

	if got := backoff(0); got != time.Second {
		t.Errorf("Exponential(0) = %v; want 1s", got)
	}

	if got := backoff(1); got != 2*time.Second {
		t.Errorf("Exponential(1) = %v; want 2s", got)
	}

	if got := backoff(20); got != 30*time.Second {
		t.Errorf("Exponential(20) = %v; want the 30s cap", got)
	}
}

func TestExponentialInvalidBounds(t *testing.T) {
	t.Parallel()

	backoff := hqgoretrierbackoff.Exponential(-time.Second, -time.Minute)

	if got := backoff(3); got != 0 {
		t.Errorf("Exponential with invalid bounds = %v; want 0", got)
	}
}

func TestConstant(t *testing.T) {
	t.Parallel()

	backoff := hqgoretrierbackoff.Constant(time.Second, 30*time.Second)

	for _, attempt := range []int{0, 1, 5} {
		if got := backoff(attempt); got != time.Second {
			t.Errorf("Constant(%d) = %v; want 1s", attempt, got)
		}
	}
}

func TestJitteredBounds(t *testing.T) {
	t.Parallel()

	for name, backoff := range map[string]hqgoretrierbackoff.Backoff{
		"equal":        hqgoretrierbackoff.ExponentialWithEqualJitter(time.Second, 30*time.Second),
		"full":         hqgoretrierbackoff.ExponentialWithFullJitter(time.Second, 30*time.Second),
		"decorrelated": hqgoretrierbackoff.ExponentialWithDecorrelatedJitter(time.Second, 30*time.Second),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for attempt := range 8 {
				if got := backoff(attempt); got < 0 || got > 30*time.Second {
					t.Errorf("backoff(%d) = %v; want within [0, 30s]", attempt, got)
				}
			}
		})
	}
}
