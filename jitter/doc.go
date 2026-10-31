// Package jitter adds randomness to backoff delays.
//
// Jitter spreads retry attempts across time so that clients which failed together do not retry in
// lockstep — the "thundering herd" problem that can keep a recovering service overwhelmed. Each
// function takes a computed backoff and returns a randomized duration:
//
//   - [Equal] keeps half the backoff fixed and randomizes the other half, returning a value in
//     [backoff/2, backoff).
//   - [Full] randomizes the entire backoff, returning a value in [0, backoff).
//   - [Decorrelated] draws from a range bounded by the previous delay, reducing correlation between
//     successive waits.
//
// The functions use math/rand/v2; cryptographic randomness is unnecessary for jitter. They are
// safe for concurrent use, since their randomness comes from the goroutine-safe top-level
// functions of math/rand/v2. They back the strategies in
// [github.com/hueristiq/hq-lib-retrier-go/backoff] but are exported for building custom ones.
//
// # Entry points
//
// [Decorrelated], [Equal], [Full]
package jitter
