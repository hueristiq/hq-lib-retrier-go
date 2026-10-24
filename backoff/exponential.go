package backoff

import (
	"math"
	"sync"
	"time"

	hqgoretrierjitter "github.com/hueristiq/hq-lib-retrier-go/jitter"
)

// validBounds reports whether the delay bounds handed to a constructor are usable: both
// positive and ordered.
//
// Parameters:
//   - minDelay (time.Duration): The base (minimum) delay duration.
//   - maxDelay (time.Duration): The maximum allowable delay duration.
//
// Returns:
//   - valid (bool): True when minDelay and maxDelay are positive and minDelay does not exceed
//     maxDelay.
func validBounds(minDelay, maxDelay time.Duration) (valid bool) {
	return minDelay > 0 && maxDelay > 0 && minDelay <= maxDelay
}

// zeroBackoff returns a Backoff that always yields a zero delay. Constructors return it when
// called with invalid bounds, preserving the package contract that invalid input produces a
// zero duration.
//
// Parameters:
//
//	none.
//
// Returns:
//   - backoff (Backoff): A function that returns a zero duration for every attempt.
func zeroBackoff() (backoff Backoff) {
	return func(int) time.Duration {
		return 0
	}
}

// exponential computes the base exponential backoff for a given attempt.
//
// The base delay grows as minDelay * 2^attempt and is capped at maxDelay. The growth is computed
// with a single guarded shift instead of repeated doubling: an attempt whose product would
// overflow an int64 clamps to maxDelay, exactly as the doubling loop's overflow path did. It is
// the shared core used by all exponential strategies in this package.
//
// Callers must guarantee valid input — positive, ordered bounds and a non-negative attempt; the
// constructors in this package enforce this before calling it.
//
// Parameters:
//   - minDelay (time.Duration): The base (minimum) delay duration.
//   - maxDelay (time.Duration): The maximum allowable delay duration.
//   - attempt (int): The current retry attempt number (typically starting at 1).
//
// Returns:
//   - base (time.Duration): The exponential base delay, capped at maxDelay.
func exponential(minDelay, maxDelay time.Duration, attempt int) (base time.Duration) {
	// A shift of 63 or more bits overflows every positive int64; below that, a minDelay above
	// math.MaxInt64>>attempt would overflow when shifted. Both clamp to maxDelay, as does any
	// product that grows past maxDelay itself.
	if attempt > 62 || minDelay > math.MaxInt64>>attempt {
		return maxDelay
	}

	return min(minDelay<<attempt, maxDelay)
}

// Exponential returns a Backoff function that implements a basic exponential backoff strategy.
//
// This strategy calculates the delay by exponentially increasing the base delay (minDelay) based on
// the retry attempt number, using the formula:
//
//	delay = min(maxDelay, minDelay * 2^attempt)
//
// The delay is capped at maxDelay to ensure reasonable retry intervals. If minDelay or maxDelay is
// less than or equal to 0 or minDelay exceeds maxDelay, the returned Backoff always yields a zero
// duration. A negative attempt also yields zero.
//
// Parameters:
//   - minDelay (time.Duration): The base (minimum) delay duration.
//   - maxDelay (time.Duration): The maximum allowable delay duration.
//
// Returns:
//   - backoff (Backoff): A function that computes the exponential backoff delay for a given attempt,
//     capped at maxDelay.
func Exponential(minDelay, maxDelay time.Duration) (backoff Backoff) {
	if !validBounds(minDelay, maxDelay) {
		return zeroBackoff()
	}

	return func(attempt int) (delay time.Duration) {
		if attempt < 0 {
			return 0
		}

		return exponential(minDelay, maxDelay, attempt)
	}
}

// ExponentialWithEqualJitter returns a Backoff function that implements exponential backoff with
// equal jitter to add moderate randomness to retry delays.
//
// The exponential base is computed as min(maxDelay, minDelay * 2^attempt), then equal jitter is
// applied via jitter.Equal, yielding a delay in the range [base/2, base):
//
//	delay = jitter.Equal(min(maxDelay, minDelay * 2^attempt))
//
// If minDelay or maxDelay is less than or equal to 0 or minDelay exceeds maxDelay, the returned
// Backoff always yields a zero duration. A negative attempt also yields zero.
//
// Parameters:
//   - minDelay (time.Duration): The base (minimum) delay duration.
//   - maxDelay (time.Duration): The maximum allowable delay duration.
//
// Returns:
//   - backoff (Backoff): A function that computes the exponential backoff delay with equal jitter for a
//     given attempt, never exceeding maxDelay.
func ExponentialWithEqualJitter(minDelay, maxDelay time.Duration) (backoff Backoff) {
	if !validBounds(minDelay, maxDelay) {
		return zeroBackoff()
	}

	return func(attempt int) (delay time.Duration) {
		if attempt < 0 {
			return 0
		}

		return hqgoretrierjitter.Equal(exponential(minDelay, maxDelay, attempt))
	}
}

// ExponentialWithFullJitter returns a Backoff function that implements exponential backoff with
// full jitter to add maximum randomness to retry delays.
//
// The exponential base is computed as min(maxDelay, minDelay * 2^attempt), then full jitter is
// applied via jitter.Full, yielding a delay in the range [0, base):
//
//	delay = jitter.Full(min(maxDelay, minDelay * 2^attempt))
//
// Allowing delays near zero maximally spreads retries, mitigating the "thundering herd" problem.
// If minDelay or maxDelay is less than or equal to 0 or minDelay exceeds maxDelay, the returned
// Backoff always yields a zero duration. A negative attempt also yields zero.
//
// Parameters:
//   - minDelay (time.Duration): The base (minimum) delay duration.
//   - maxDelay (time.Duration): The maximum allowable delay duration.
//
// Returns:
//   - backoff (Backoff): A function that computes the exponential backoff delay with full jitter for a
//     given attempt, never exceeding maxDelay.
func ExponentialWithFullJitter(minDelay, maxDelay time.Duration) (backoff Backoff) {
	if !validBounds(minDelay, maxDelay) {
		return zeroBackoff()
	}

	return func(attempt int) (delay time.Duration) {
		if attempt < 0 {
			return 0
		}

		return hqgoretrierjitter.Full(exponential(minDelay, maxDelay, attempt))
	}
}

// ExponentialWithDecorrelatedJitter returns a Backoff function that implements exponential backoff
// with decorrelated jitter, reducing correlation between successive retry delays.
//
// Each delay is drawn from the range [minDelay, min(maxDelay, previous * 3)], where previous is
// the delay produced by the preceding call — not a fixed function of the attempt number:
//
//	delay = jitter.Decorrelated(minDelay, maxDelay, previousDelay)
//
// Because every draw depends on the previous random draw, successive waits are decorrelated in
// the sense of the AWS Architecture Blog's "decorrelated jitter": a run of short waits is not
// followed predictably by another short wait, which spreads retries of many clients over time.
//
// The returned function is stateful: it remembers the last delay it produced and starts from
// minDelay. It is safe for concurrent use, but all callers of a shared instance contribute to
// one delay sequence — construct one instance per retry loop for independent decorrelation, which
// Retry and RetryWithData do automatically when this constructor is passed to WithBackoff.
// The attempt parameter is validated but otherwise unused; growth is driven by the previous delay.
//
// If minDelay or maxDelay is less than or equal to 0 or minDelay exceeds maxDelay, the returned
// Backoff always yields a zero duration. A negative attempt also yields zero.
//
// Parameters:
//   - minDelay (time.Duration): The base (minimum) delay duration.
//   - maxDelay (time.Duration): The maximum allowable delay duration.
//
// Returns:
//   - backoff (Backoff): A function that computes the exponential backoff delay with decorrelated jitter
//     for a given attempt, never exceeding maxDelay.
func ExponentialWithDecorrelatedJitter(minDelay, maxDelay time.Duration) (backoff Backoff) {
	if !validBounds(minDelay, maxDelay) {
		return zeroBackoff()
	}

	var (
		mu       sync.Mutex
		previous time.Duration
	)

	return func(attempt int) (delay time.Duration) {
		if attempt < 0 {
			return 0
		}

		mu.Lock()
		defer mu.Unlock()

		delay = hqgoretrierjitter.Decorrelated(minDelay, maxDelay, previous)

		previous = delay

		return
	}
}
