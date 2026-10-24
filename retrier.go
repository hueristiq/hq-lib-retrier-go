package retrier

import (
	"context"
	"time"

	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
)

// options holds the settings for retry operations, defining the behavior of the retry
// mechanism.
//
// Fields:
//   - maxAttempts (int): The maximum number of attempts allowed before giving up, counting the
//     initial call.
//   - waitMin (time.Duration): The minimum allowable delay between retry attempts, serving as the
//     base delay for backoff calculations.
//   - waitMax (time.Duration): The maximum allowable delay between retry attempts, capping
//     the backoff duration.
//   - newBackoff (func(minDelay, maxDelay time.Duration) (backoff hqgoretrierbackoff.Backoff)): A constructor for the
//     strategy that computes the delay for each attempt. It is called once per retry loop with
//     the normalized wait bounds, so stateful strategies get fresh state.
//   - retryOn (func(err error) (retry bool)): A predicate deciding whether a failed attempt's error is
//     retryable, set by [WithRetryOn]. A nil predicate retries every error.
//   - onRetry (OnRetry): A callback function invoked after each failed attempt that will be
//     retried, receiving a RetryEvent with the attempt number, the triggering error, and the
//     computed delay.
type options struct {
	maxAttempts int
	waitMin     time.Duration
	waitMax     time.Duration
	newBackoff  func(minDelay, maxDelay time.Duration) (backoff hqgoretrierbackoff.Backoff)
	retryOn     func(err error) (retry bool)
	onRetry     OnRetry
}

// RetryEvent carries the data of a single retry notification, passed to the OnRetry callback
// after a failed attempt that will be followed by another attempt.
//
// Fields:
//   - Attempt (int): The number of the attempt that just failed, starting at 1.
//   - Err (error): The error the failed attempt returned. Will not be nil.
//   - Wait (time.Duration): The computed delay, after clamping to the retry loop's floor,
//     before the next attempt begins.
type RetryEvent struct {
	Attempt int
	Err     error
	Wait    time.Duration
}

// OnRetry is a callback function type used to observe retry attempts.
//
// It is invoked after each failed attempt that will be followed by another attempt, receiving
// a RetryEvent with the number of the attempt that failed, the error it returned, and the
// computed delay before the next attempt. It is not invoked after a successful attempt, an
// error rejected by the WithRetryOn predicate, or the final attempt allowed by WithMaxAttempts
// — in those cases the outcome is returned directly to the caller. This allows for custom
// logging, monitoring, or other side effects during retries.
//
// Invocation is synchronous, on the retry loop's goroutine: a slow callback delays the next
// attempt. If the context is canceled during the wait, the reported Wait may not fully elapse
// and no further attempt happens. A callback shared across concurrent retry loops must be safe
// for concurrent use.
//
// A panicking callback is not recovered: the panic propagates to the caller of Retry or
// RetryWithData and aborts the retry loop, the same as a panicking operation.
//
// Parameters:
//   - event (RetryEvent): The data of the failed attempt and the wait that follows it.
//
// Returns:
//
//	none.
type OnRetry func(event RetryEvent)

// OptionFunc is a function type used to modify the retry options in a declarative manner.
//
// It allows users to customize retry behavior by setting fields in the options struct,
// such as the maximum number of attempts, delay bounds, backoff strategy, retry predicate,
// or OnRetry callback. Multiple options can be combined to create a tailored retry policy.
// When the same option is supplied more than once, the last one wins.
//
// Parameters:
//   - opts (*options): A pointer to the options struct to be modified.
//
// Returns:
//
//	none.
type OptionFunc func(opts *options)

// Operation is a function type representing an operation that may be retried.
//
// It encapsulates a task that returns an error to indicate success (nil) or failure (non-nil).
// This type is used with the Retry function for operations that do not produce a result.
//
// The operation receives no context; capture the context passed to Retry if an attempt needs
// cancellation or a deadline. It may be called multiple times, so it should be prepared to run
// again after a failure. A panicking operation is not recovered — the panic propagates to the
// caller of Retry.
//
// Parameters:
//
//	none.
//
// Returns:
//   - err (error): The error from the operation, or nil if the operation succeeded.
type Operation func() (err error)

// withEmptyData wraps an Operation to convert it into an OperationWithData that returns an
// empty struct as its data.
//
// This method enables operations that do not produce a result to be used with RetryWithData,
// allowing a consistent interface for both result-producing and non-result-producing operations.
//
// Parameters:
//
//	none.
//
// Returns:
//   - operationWithData (OperationWithData[struct{}]): A function that executes the original
//     Operation and returns an empty struct alongside the operation's error.
func (o Operation) withEmptyData() (operationWithData OperationWithData[struct{}]) {
	operationWithData = func() (struct{}, error) {
		return struct{}{}, o()
	}

	return
}

// OperationWithData is a generic function type representing an operation that returns both
// a result of type T and an error.
//
// It is used with RetryWithData to handle operations that produce a result, allowing the retrier
// to return the successful result alongside a nil error when the operation succeeds.
//
// The operation receives no context; capture the context passed to RetryWithData if an attempt
// needs cancellation or a deadline. It may be called multiple times, so it should be prepared to
// run again after a failure. A panicking operation is not recovered — the panic propagates to
// the caller of RetryWithData.
//
// Type Parameters:
//   - T: The type of the data returned by the operation.
//
// Parameters:
//
//	none.
//
// Returns:
//   - data (T): The result of the operation if successful.
//   - err (error): The error from the operation, or nil if the operation succeeded.
type OperationWithData[T any] func() (data T, err error)

const (
	// DefaultMaxAttempts is the default maximum number of attempts, counting the initial call.
	//
	// It applies when WithMaxAttempts is not supplied, and when a supplied value less than or
	// equal to 0 is normalized back to the default.
	DefaultMaxAttempts = 3

	// DefaultWaitMin is the default minimum delay between retry attempts, serving as the base
	// delay for backoff calculations.
	//
	// It applies when WithWaitMin is not supplied, and when a supplied value less than or
	// equal to 0 is normalized back to the default.
	DefaultWaitMin = 1 * time.Second

	// DefaultWaitMax is the default maximum delay between retry attempts, capping the backoff
	// duration.
	//
	// It applies when WithWaitMax is not supplied, and when a supplied value less than or
	// equal to 0 is normalized back to the default.
	DefaultWaitMax = 30 * time.Second
)

// minRetryDelay is the floor applied to the computed delay between retry attempts.
//
// Backoff strategies that return a non-positive delay are clamped to it, so a misconfigured
// custom strategy cannot busy-spin the retry loop.
const minRetryDelay = time.Millisecond

// WithMaxAttempts returns an OptionFunc that sets the maximum number of attempts.
//
// The count includes the initial attempt, so a value of 3 means one initial call followed by up to
// two retries. Once the limit is reached, the retrier stops and returns the last error.
//
// Parameters:
//   - maxAttempts (int): The maximum number of attempts, including the initial one. Values less
//     than or equal to 0 fall back to DefaultMaxAttempts. To retry until the context is canceled,
//     pass a very large value such as math.MaxInt.
//
// Returns:
//   - f (OptionFunc): A functional option that sets the maxAttempts field in the options.
func WithMaxAttempts(maxAttempts int) (f OptionFunc) {
	return func(opts *options) {
		opts.maxAttempts = maxAttempts
	}
}

// WithWaitMin returns an OptionFunc that sets the minimum delay between retry attempts.
//
// It defines the base delay for backoff calculations, ensuring retries do not occur too rapidly.
// This is particularly important for preventing overwhelming a system with rapid retries.
//
// Parameters:
//   - retryWaitMin (time.Duration): The minimum delay duration. Values less than or equal to 0
//     fall back to DefaultWaitMin, so a misconfigured retrier cannot spin in a
//     zero-delay loop.
//
// Returns:
//   - f (OptionFunc): A functional option that sets the waitMin field in the options.
func WithWaitMin(retryWaitMin time.Duration) (f OptionFunc) {
	return func(opts *options) {
		opts.waitMin = retryWaitMin
	}
}

// WithWaitMax returns an OptionFunc that sets the maximum delay between retry attempts.
//
// It caps the backoff duration to prevent excessively long delays, ensuring retries occur within
// a reasonable timeframe. Typically, retryWaitMax should be greater than or equal to retryWaitMin.
//
// Parameters:
//   - retryWaitMax (time.Duration): The maximum delay duration. Values less than or equal to 0
//     fall back to DefaultWaitMax, and a value below retryWaitMin is raised to
//     retryWaitMin.
//
// Returns:
//   - f (OptionFunc): A functional option that sets the waitMax field in the options.
func WithWaitMax(retryWaitMax time.Duration) (f OptionFunc) {
	return func(opts *options) {
		opts.waitMax = retryWaitMax
	}
}

// WithBackoff returns an OptionFunc that selects the backoff strategy for computing
// retry delays.
//
// It takes a constructor rather than a ready-made strategy: the retrier calls newBackoff once per
// Retry or RetryWithData call, after normalizing the wait bounds, so stateful strategies such as
// the decorrelated jitter always run with fresh, per-loop state. The constructors in the backoff
// package match this signature and can be passed directly, e.g.
// WithBackoff(backoff.ExponentialWithFullJitter).
//
// Parameters:
//   - newBackoff (func(minDelay, maxDelay time.Duration) (backoff hqgoretrierbackoff.Backoff)): The backoff strategy
//     constructor. If nil, the default strategy (exponential backoff with decorrelated jitter)
//     is used.
//
// Returns:
//   - f (OptionFunc): A functional option that sets the newBackoff field in the options.
func WithBackoff(newBackoff func(minDelay, maxDelay time.Duration) (backoff hqgoretrierbackoff.Backoff)) (f OptionFunc) {
	return func(opts *options) {
		opts.newBackoff = newBackoff
	}
}

// WithRetryOn returns an OptionFunc that sets a predicate deciding whether a failed attempt's
// error is worth retrying.
//
// After each failed attempt the predicate is called, synchronously on the retry loop's goroutine,
// with the attempt's error; if it returns false, the retrier stops immediately and returns that
// error instead of scheduling another attempt. This is useful for error classes that retries
// cannot fix, such as validation or authorization failures. A predicate shared across concurrent
// retry loops must be safe for concurrent use.
//
// Parameters:
//   - retryOn (func(err error) bool): The predicate. If nil (the default), every error is
//     considered retryable.
//
// Returns:
//   - f (OptionFunc): A functional option that sets the retryOn field in the options.
func WithRetryOn(retryOn func(err error) bool) (f OptionFunc) {
	return func(opts *options) {
		opts.retryOn = retryOn
	}
}

// WithOnRetry returns an OptionFunc that sets a callback observing retry attempts.
//
// It configures a callback function that is invoked after each failed attempt that will be
// followed by another attempt, receiving a RetryEvent with the attempt number, the error, and
// the computed delay before the next attempt. This is useful for logging, monitoring, or other
// side effects during retries.
//
// The callback is invoked synchronously on the retry loop's goroutine — a slow callback delays
// the next attempt. A callback shared across concurrent retry loops must be safe for concurrent
// use.
//
// Parameters:
//   - onRetry (OnRetry): The callback function to be called before each retry wait. If nil, no
//     notification is performed.
//
// Returns:
//   - f (OptionFunc): A functional option that sets the onRetry field in the options.
func WithOnRetry(onRetry OnRetry) (f OptionFunc) {
	return func(opts *options) {
		opts.onRetry = onRetry
	}
}

// Retry executes an operation with retries, respecting the provided context and options.
//
// It attempts the operation up to maxAttempts times (as specified in the options), waiting
// between attempts according to the backoff strategy. If the operation succeeds (returns nil
// error), it returns immediately. If the operation fails with an error the WithRetryOn predicate
// rejects, it stops and returns that error. If the context is canceled or times out, it returns
// the context's error. If all attempts fail, it returns the last error from the operation.
//
// The backoff strategy and the OnRetry callback both receive the 1-based number of the failed
// attempt, and the callback is invoked synchronously — a slow callback delays the next attempt.
// Panics from the operation are not recovered; they propagate to the caller and abort the retry
// loop.
//
// Parameters:
//   - ctx (context.Context): The context controlling the retry lifecycle. Cancellation or timeout
//     aborts retries and returns context.Cause(ctx).
//   - operation (Operation): The operation to retry, which returns an error indicating success
//     or failure.
//   - ofs (...OptionFunc): Variadic functional options to customize retry behavior, such as
//     maximum attempts, delay bounds, backoff strategy, retry predicate, and OnRetry callback.
//
// Returns:
//   - err (error): The error from the last attempt if all attempts fail, or context.Cause(ctx) if
//     the context is canceled or times out. Returns nil if the operation succeeds.
func Retry(ctx context.Context, operation Operation, ofs ...OptionFunc) (err error) {
	_, err = RetryWithData(ctx, operation.withEmptyData(), ofs...)

	return
}

// RetryWithData executes a generic operation that returns data and an error, with retries.
//
// It attempts the operation up to maxAttempts times, using the configured backoff strategy to
// compute delays between attempts. If the operation succeeds (returns nil error), it returns
// the operation's result and nil. If the operation fails with an error the WithRetryOn predicate
// rejects, it stops and returns that error. If the context is canceled or times out, it returns
// the context's error; the result returned alongside it is whatever the most recent attempt
// produced (the zero value of T if none ran) and must not be treated as successful. If all
// attempts fail, it returns the last result and error from the operation.
//
// After the options are applied, invalid values are normalized: a nil backoff constructor falls
// back to exponential backoff with decorrelated jitter, a non-positive attempt limit or wait
// bound falls back to its default, and a waitMax below waitMin is raised to waitMin, so the
// built-in strategies always run with valid bounds. Any non-positive computed delay is clamped
// to minRetryDelay before the wait, so the retry loop never spins at full CPU. The backoff
// strategy is constructed once per call, after normalization, so stateful strategies always run
// with fresh state.
//
// The backoff strategy and the OnRetry callback both receive the 1-based number of the failed
// attempt, and the callback is invoked synchronously — a slow callback delays the next attempt.
// Panics from the operation are not recovered; they propagate to the caller and abort the retry
// loop.
//
// Parameters:
//   - ctx (context.Context): The context controlling the retry lifecycle. Cancellation or timeout
//     aborts retries and returns context.Cause(ctx).
//   - operation (OperationWithData[T]): The operation to retry, returning a result of type T
//     and an error.
//   - ofs (...OptionFunc): Variadic functional options to customize retry behavior.
//
// Returns:
//   - result (T): The result from the operation if it succeeds, or the result of the last
//     attempt if all attempts fail. If the context is canceled, it is the value the most recent
//     attempt produced — possibly partial or stale — or the zero value of T if no attempt ran;
//     treat it as successful only when err is nil.
//   - err (error): The error from the last attempt if all attempts fail, or context.Cause(ctx)
//     if the context is canceled or times out. Returns nil if the operation succeeds.
func RetryWithData[T any](ctx context.Context, operation OperationWithData[T], ofs ...OptionFunc) (result T, err error) {
	opts := &options{
		maxAttempts: DefaultMaxAttempts,
		waitMin:     DefaultWaitMin,
		waitMax:     DefaultWaitMax,
		newBackoff:  hqgoretrierbackoff.ExponentialWithDecorrelatedJitter,
	}

	for _, f := range ofs {
		f(opts)
	}

	if opts.maxAttempts <= 0 {
		opts.maxAttempts = DefaultMaxAttempts
	}

	if opts.waitMin <= 0 {
		opts.waitMin = DefaultWaitMin
	}

	if opts.waitMax <= 0 {
		opts.waitMax = DefaultWaitMax
	}

	if opts.waitMax < opts.waitMin {
		opts.waitMax = opts.waitMin
	}

	if opts.newBackoff == nil {
		opts.newBackoff = hqgoretrierbackoff.ExponentialWithDecorrelatedJitter
	}

	b := opts.newBackoff(opts.waitMin, opts.waitMax)

	// timer is created lazily on the first wait and reused across attempts via Reset, avoiding
	// one time.NewTimer allocation per failed attempt. Post-1.23 timer semantics make Reset
	// after expiry safe without the old Stop-and-drain dance.
	var timer *time.Timer

	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			return result, context.Cause(ctx)
		}

		result, err = operation()
		if err == nil {
			return result, nil
		}

		if opts.retryOn != nil && !opts.retryOn(err) {
			return result, err
		}

		if attempt >= opts.maxAttempts {
			return result, err
		}

		delay := b(attempt)

		// A custom strategy may compute a non-positive delay; clamp it to the floor so the
		// loop cannot busy-spin. The OnRetry event carries the clamped delay, matching the
		// wait that actually happens.
		if delay <= 0 {
			delay = minRetryDelay
		}

		if opts.onRetry != nil {
			opts.onRetry(RetryEvent{Attempt: attempt, Err: err, Wait: delay})
		}

		if timer == nil {
			timer = time.NewTimer(delay)
		} else {
			timer.Reset(delay)
		}

		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()

			return result, context.Cause(ctx)
		}
	}
}
