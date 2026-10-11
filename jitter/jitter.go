package jitter

import (
	"math"
	"math/rand/v2"
	"time"
)

// Equal calculates a jitter duration using the equal jitter strategy based on the provided backoff.
//
// This strategy introduces moderate randomness by calculating a midpoint of the backoff duration
// and adding a random offset between 0 and that midpoint. The resulting jitter duration lies in
// the range [backoff/2, backoff), balancing predictability and randomization to avoid synchronized
// retry spikes in distributed systems.
//
//	jitter = (backoff / 2) + random(0, backoff / 2)
//
// Parameters:
//   - backoff (time.Duration): The base backoff duration to which jitter is applied.
//
// Returns:
//   - jitter (time.Duration): The calculated jitter duration, in [backoff/2, backoff) for
//     positive backoff. Returns 0 if backoff is 0 or negative.
func Equal(backoff time.Duration) (jitter time.Duration) {
	if backoff <= 0 {
		return 0
	}

	midpoint := backoff / 2

	return midpoint + getRandomDuration(midpoint)
}

// Full calculates a jitter duration using the full jitter strategy based on the provided backoff.
//
// This strategy generates a fully randomized delay between 0 and the backoff duration, providing
// maximum randomness to spread out retry attempts and mitigate the "thundering herd" problem.
//
//	jitter = random(0, backoff)
//
// Parameters:
//   - backoff (time.Duration): The base backoff duration to which jitter is applied.
//
// Returns:
//   - jitter (time.Duration): The calculated jitter duration, in [0, backoff) for positive backoff.
//     Returns 0 if backoff is 0 or negative.
func Full(backoff time.Duration) (jitter time.Duration) {
	if backoff <= 0 {
		return 0
	}

	return getRandomDuration(backoff)
}

// Decorrelated calculates a jitter duration using the decorrelated jitter strategy,
// incorporating the previous backoff to reduce correlation between successive retries.
//
// This strategy draws a delay from the range [minDelay, min(maxDelay, previous * 3)), where the
// upper bound grows with the previous backoff. This prevents exponential growth from becoming
// excessive while still providing randomness to avoid synchronized retries. The computation is:
//   - If previous is non-positive, it is set to minDelay.
//   - The upper bound is min(maxDelay, previous * 3), computed without overflowing.
//   - A random duration in [minDelay, upper) is returned, capped at maxDelay.
//
// Parameters:
//   - minDelay (time.Duration): The minimum allowable jitter duration.
//   - maxDelay (time.Duration): The maximum allowable jitter duration.
//   - previous (time.Duration): The previous backoff duration, influencing the jitter range.
//     If non-positive, defaults to minDelay.
//
// Returns:
//   - jitter (time.Duration): The calculated jitter duration, in [minDelay, maxDelay).
//     Returns 0 if minDelay or maxDelay is negative, or if minDelay exceeds maxDelay. Two
//     degenerate cases are worth noting: when minDelay equals maxDelay, every call returns
//     exactly maxDelay — a fixed delay, not a range; and when minDelay is 0, the range has no
//     floor — a non-positive previous yields 0, and once a positive previous exists the draw
//     is in [0, min(maxDelay, previous*3)) and can come arbitrarily close to zero.
func Decorrelated(minDelay, maxDelay, previous time.Duration) (jitter time.Duration) {
	if minDelay < 0 || maxDelay < 0 || minDelay > maxDelay {
		return 0
	}

	if previous <= 0 {
		previous = minDelay
	}

	upper := maxDelay

	if previous <= math.MaxInt64/3 {
		upper = min(upper, previous*3)
	}

	return min(minDelay+getRandomDuration(upper-minDelay), maxDelay)
}

// getRandomDuration generates a random duration in the range [0, maxDuration).
//
// It uses math/rand/v2, which is sufficient for spreading retry attempts; cryptographic randomness
// is unnecessary for jitter.
//
// Performance: 0 allocs/op (see BenchmarkEqual, BenchmarkFull, and BenchmarkDecorrelated).
//
// Parameters:
//   - maxDuration (time.Duration): The exclusive upper bound for the random duration.
//     If 0 or negative, returns 0, as no meaningful random duration can be generated.
//
// Returns:
//   - duration (time.Duration): A random duration in the range [0, maxDuration).
//     Returns 0 if maxDuration <= 0.
func getRandomDuration(maxDuration time.Duration) (duration time.Duration) {
	if maxDuration <= 0 {
		return 0
	}

	return time.Duration(rand.Int64N(int64(maxDuration))) //nolint:gosec // G404: cryptographic randomness is unnecessary for jitter; math/rand/v2 suffices to spread retry attempts.
}
