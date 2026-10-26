package backoff

import (
	"time"
)

// Backoff is a function type that calculates the delay duration between successive retry attempts.
//
// It serves as an abstraction for implementing various retry strategies (e.g., constant, linear,
// exponential, or jittered backoff) by defining a standardized function signature. Implementations
// of this type determine the wait time before a retry attempt, allowing for flexible and customizable
// retry logic in fault-tolerant systems.
//
// A Backoff is bound to its minimum and maximum delays when it is created — the constructors in
// this package take those bounds and return the ready-to-use function — so each call needs only
// the attempt number.
//
// Implementations must be safe for concurrent use by multiple goroutines, since a single Backoff
// value may be shared across independent retry loops.
//
// Parameters:
//   - attempt (int): the current retry attempt number, starting at 1. When driven by the
//     retrier, it is the number of the attempt that just failed. Implementations use this value
//     to adjust the delay (e.g., increasing it for subsequent retries in exponential backoff).
//
// Returns:
//   - delay (time.Duration): the calculated delay duration to wait before the next retry attempt,
//     computed based on the implemented strategy (e.g., constant, linear, or exponential). For a
//     valid attempt the delay lies in [0, the configured maximum]; jittered strategies may
//     legitimately return less than the configured minimum. A negative attempt yields a zero
//     duration, and a Backoff created with invalid bounds — non-positive bounds or a minimum
//     above the maximum — always returns zero.
type Backoff func(attempt int) (delay time.Duration)
