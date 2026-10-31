// Package backoff computes the delay between retry attempts.
//
// A strategy is any function matching the [Backoff] type. Constructors take the delay bounds and
// return a ready-to-use Backoff bound to them, so each call needs only the attempt number. The
// package implements exponential backoff, where the base delay grows as minDelay * 2^attempt and
// is capped at maxDelay, in four variants that differ only in how they apply randomness (jitter)
// drawn from the [github.com/hueristiq/hq-lib-retrier-go/jitter] package:
//
//   - [Exponential] applies no jitter; the delay is fully deterministic.
//   - [Constant] yields a fixed delay, regardless of the attempt number.
//   - [ExponentialWithEqualJitter] keeps half the base fixed and randomizes the rest, yielding a
//     delay in [base/2, base).
//   - [ExponentialWithFullJitter] randomizes the entire base, yielding a delay in [0, base). This
//     spreads retries most aggressively.
//   - [ExponentialWithDecorrelatedJitter] draws from a range bounded by the previous delay it
//     produced, decoupling successive waits. It is the only stateful strategy.
//
// Jittered strategies reduce the "thundering herd" effect, where many clients that failed together
// retry in lockstep and overwhelm a recovering service. The constructors match the signature
// expected by [github.com/hueristiq/hq-lib-retrier-go.WithBackoff], which builds a fresh
// Backoff for every retry loop — pass the constructor itself, e.g.
// WithBackoff(backoff.Exponential).
//
// All strategies guard against integer overflow and produce a zero duration for invalid input —
// a constructor called with non-positive bounds or a minimum above the maximum returns a Backoff
// that always yields zero, and a negative attempt yields zero.
//
// # Entry points
//
// [Constant], [Exponential], [ExponentialWithDecorrelatedJitter], [ExponentialWithEqualJitter], [ExponentialWithFullJitter], [Backoff]
package backoff
