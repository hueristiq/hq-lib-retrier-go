// Package retrier runs operations that may fail transiently, retrying them with a configurable
// backoff until they succeed, a limit is reached, an error proves not worth retrying, or the
// context is canceled.
//
// It suits work whose failures are often temporary — network requests, database queries, calls to
// external APIs — where a brief pause and another attempt is more useful than failing on the first
// error.
//
// # Entry points
//
// [Retry] runs an [Operation] that returns only an error. [RetryWithData] runs an
// [OperationWithData] that returns a value alongside its error and hands that value back to the
// caller. Retry is a thin wrapper over RetryWithData, so both share identical retry, backoff, and
// cancellation behavior.
//
// Operations receive no context of their own; capture the context passed to the entry point if an
// attempt needs cancellation or a deadline. An operation may run several times, so it must be
// prepared to execute again after a failure.
//
// # Configuration
//
// Behavior is set through functional options passed to either entry point:
//
//   - [WithMaxAttempts] caps the number of attempts, counting the initial call.
//   - [WithWaitMin] and [WithWaitMax] bound the delay between attempts.
//   - [WithBackoff] selects the strategy that computes each delay. It takes a constructor,
//     which the retrier calls once per retry loop with the normalized bounds.
//   - [WithRetryOn] decides per error whether another attempt is worthwhile.
//   - [WithOnRetry] registers the callback invoked after every failed attempt that will be retried.
//
// Unset options fall back to defaults: [DefaultMaxAttempts] attempts, a minimum wait of
// [DefaultWaitMin] and a maximum wait of [DefaultWaitMax], and exponential backoff with
// decorrelated jitter. Invalid values are normalized the same way — a nil backoff constructor,
// a non-positive attempt limit, or non-positive wait bounds fall back to the defaults, and a
// maximum wait below the minimum is raised to the minimum — so the built-in strategies always
// run with valid bounds. Any non-positive computed delay is clamped to a one-millisecond floor,
// so the retry loop never spins at full CPU.
//
// # Non-retryable errors
//
// Not every failure deserves another attempt. The [WithRetryOn] predicate classifies errors after
// each failed attempt; when it rejects an error, the retry loop stops immediately and returns
// that error instead of scheduling the next one.
//
// # Backoff and jitter
//
// The delay between attempts comes from a [github.com/hueristiq/hq-lib-retrier-go/backoff.Backoff]
// function, called with the 1-based number of the attempt that just failed. The backoff package
// provides exponential strategies, optionally combined with the jitter strategies in
// [github.com/hueristiq/hq-lib-retrier-go/jitter] to spread retries across clients and avoid the
// thundering-herd problem.
//
// # Context
//
// Every attempt and every wait observes the supplied context. When the context is canceled or its
// deadline passes, the active call returns [context.Cause] for that context and abandons any
// further attempts. The result RetryWithData returns in that case is whatever the most recent
// attempt produced — the zero value of the result type if no attempt ran — so it must not be
// treated as successful; check the error first. On a clean context, the result of the final
// attempt — its value for RetryWithData, plus its error — is returned.
//
// # Concurrency
//
// The entry points are safe to call from multiple goroutines: every call is independent and
// constructs its own backoff state. Callbacks — the OnRetry callback and the retry predicate —
// are invoked synchronously on the retry loop's goroutine; a callback shared across concurrent
// loops must itself be safe for concurrent use.
//
// # Panics
//
// A panicking operation is not recovered: the panic propagates to the caller and aborts the
// retry loop. Recover inside the operation and convert the panic into an error if it should be
// retried. A panicking OnRetry callback likewise propagates, aborting the retry loop the same
// way.
package retrier
