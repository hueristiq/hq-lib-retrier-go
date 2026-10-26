package retrier

import (
	"context"
	"fmt"
	"time"

	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
)

// Options configures a retry loop. [Retry] and [RetryWithData] apply the
// options and then normalize invalid values.
//
// Fields:
//   - MaxAttempts (int): maximum attempts, counting the initial call.
//   - WaitMin (time.Duration): base delay between attempts.
//   - WaitMax (time.Duration): maximum delay between attempts.
//   - NewBackoff (func(minDelay, maxDelay time.Duration) (backoff hqgoretrierbackoff.Backoff)):
//     computes each delay; called once per loop.
//   - RetryOn (func(err error) (retry bool)): decides whether an error is
//     retryable; nil retries all.
//   - OnRetry (OnRetry): invoked after each failed attempt that will be retried.
type Options struct {
	MaxAttempts int
	WaitMin     time.Duration
	WaitMax     time.Duration
	NewBackoff  func(minDelay, maxDelay time.Duration) (backoff hqgoretrierbackoff.Backoff)
	RetryOn     func(err error) (retry bool)
	OnRetry     OnRetry
}

// RetryEvent carries the data of a single retry notification, passed to the OnRetry callback
// after a failed attempt that will be followed by another attempt.
//
// Fields:
//   - Attempt (int): number of the attempt that just failed, starting at 1.
//   - Err (error): error the failed attempt returned; never nil.
//   - Wait (time.Duration): computed delay, after clamping to the retry loop's floor,
//     before the next attempt.
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
//   - event (RetryEvent): the data of the failed attempt and the wait that follows it.
type OnRetry func(event RetryEvent)

// OptionFunc configures an [Options] value. A nil OptionFunc is ignored.
// Multiple options can be combined; when the same setting is supplied more than
// once, the last value wins. The With* helpers cover common settings, and
// callers may write custom options because [Options] is exported.
//
// Parameters:
//   - opts (*Options): the options to modify.
type OptionFunc func(opts *Options)

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
// Returns:
//   - err (error): the error from the operation, or nil if the operation succeeded.
type Operation func() (err error)

// withEmptyData wraps an Operation to convert it into an OperationWithData that returns an
// empty struct as its data.
//
// This method enables operations that do not produce a result to be used with RetryWithData,
// allowing a consistent interface for both result-producing and non-result-producing operations.
//
// Returns:
//   - operationWithData (OperationWithData[struct{}]): a function that executes the original
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
//   - T: the type of the data returned by the operation.
//
// Returns:
//   - data (T): the result of the operation if successful.
//   - err (error): the error from the operation, or nil if the operation succeeded.
type OperationWithData[T any] func() (data T, err error)

const (
	// DefaultMaxAttempts is the default maximum number of attempts, counting the initial call.
	//
	// It applies when WithMaxAttempts is not supplied, and when a supplied value less than or
	// equal to 0 is normalized back to the default.
	DefaultMaxAttempts = 3

	// DefaultWaitMin is the default base delay for retry attempts, used by the built-in
	// backoff calculations.
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
//   - maxAttempts (int): the maximum number of attempts, including the initial one. Values less
//     than or equal to 0 fall back to DefaultMaxAttempts. To retry until the context is canceled,
//     pass a very large value such as math.MaxInt.
//
// Returns:
//   - fn (OptionFunc): a functional option that sets the MaxAttempts field in the options.
func WithMaxAttempts(maxAttempts int) (fn OptionFunc) {
	return func(opts *Options) {
		opts.MaxAttempts = maxAttempts
	}
}

// WithWaitMin returns an OptionFunc that sets the base delay between retry attempts.
//
// It defines the base delay for backoff calculations, ensuring retries do not occur too rapidly.
// This is particularly important for preventing overwhelming a system with rapid retries.
// The built-in exponential strategies compute base * 2^attempt with the attempt counted
// from 1, so the first retry waits twice this value; jitter can reduce it further.
//
// Parameters:
//   - retryWaitMin (time.Duration): the base delay duration. Values less than or equal to 0
//     fall back to DefaultWaitMin, so a misconfigured retrier cannot spin in a
//     zero-delay loop.
//
// Returns:
//   - fn (OptionFunc): a functional option that sets the WaitMin field in the options.
func WithWaitMin(retryWaitMin time.Duration) (fn OptionFunc) {
	return func(opts *Options) {
		opts.WaitMin = retryWaitMin
	}
}

// WithWaitMax returns an OptionFunc that sets the maximum delay between retry attempts.
//
// It caps the backoff duration to prevent excessively long delays, ensuring retries occur within
// a reasonable timeframe. Typically, retryWaitMax should be greater than or equal to retryWaitMin.
//
// Parameters:
//   - retryWaitMax (time.Duration): the maximum delay duration. Values less than or equal to 0
//     fall back to DefaultWaitMax, and a value below retryWaitMin is raised to
//     retryWaitMin.
//
// Returns:
//   - fn (OptionFunc): a functional option that sets the WaitMax field in the options.
func WithWaitMax(retryWaitMax time.Duration) (fn OptionFunc) {
	return func(opts *Options) {
		opts.WaitMax = retryWaitMax
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
// The constructor must not return a nil Backoff; a nil result is a programmer
// error and panics.
//
// Parameters:
//   - newBackoff (func(minDelay, maxDelay time.Duration) (backoff hqgoretrierbackoff.Backoff)): the backoff strategy
//     constructor. If nil, the default strategy (exponential backoff with decorrelated jitter)
//     is used.
//
// Returns:
//   - fn (OptionFunc): a functional option that sets the NewBackoff field in the options.
func WithBackoff(newBackoff func(minDelay, maxDelay time.Duration) (backoff hqgoretrierbackoff.Backoff)) (fn OptionFunc) {
	return func(opts *Options) {
		opts.NewBackoff = newBackoff
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
//   - retryOn (func(err error) bool): the predicate. If nil (the default), every error is
//     considered retryable.
//
// Returns:
//   - fn (OptionFunc): a functional option that sets the RetryOn field in the options.
func WithRetryOn(retryOn func(err error) bool) (fn OptionFunc) {
	return func(opts *Options) {
		opts.RetryOn = retryOn
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
//   - onRetry (OnRetry): the callback function to be called before each retry wait. If nil, no
//     notification is performed.
//
// Returns:
//   - fn (OptionFunc): a functional option that sets the OnRetry field in the options.
func WithOnRetry(onRetry OnRetry) (fn OptionFunc) {
	return func(opts *Options) {
		opts.OnRetry = onRetry
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
//   - ctx (context.Context): the context controlling the retry lifecycle. Cancellation or timeout
//     aborts retries and returns context.Cause(ctx) wrapped with the entry point's name.
//   - operation (Operation): the operation to retry, which returns an error indicating success
//     or failure.
//   - ofs (...OptionFunc): Variadic functional options to customize retry behavior, such as
//     maximum attempts, delay bounds, backoff strategy, retry predicate, and OnRetry callback.
//
// Returns:
//   - err (error): the error from the last attempt if all attempts fail, or context.Cause(ctx)
//     wrapped with the entry point's name if the context is canceled or times out. Returns nil if
//     the operation succeeds.
func Retry(ctx context.Context, operation Operation, ofs ...OptionFunc) (err error) {
	_, err = retry(ctx, operation.withEmptyData(), "retrier.Retry", ofs...)

	return
}

// RetryWithData executes a generic operation that returns data and an error, with retries.
//
// It attempts the operation up to maxAttempts times, using the configured backoff strategy to
// compute delays between attempts. If the operation succeeds (returns nil error), it returns
// the operation's result and nil. If the operation fails with an error the WithRetryOn predicate
// rejects, it stops and returns that error. If the context is canceled or times out, it returns
// the context's cause wrapped with the entry point's name; the result returned alongside it is
// whatever the most recent attempt produced (the zero value of T if none ran) and must not be
// treated as successful. If all attempts fail, it returns the last result and error from the
// operation.
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
//   - ctx (context.Context): the context controlling the retry lifecycle. Cancellation or timeout
//     aborts retries and returns context.Cause(ctx) wrapped with the entry point's name.
//   - operation (OperationWithData[T]): the operation to retry, returning a result of type T
//     and an error.
//   - ofs (...OptionFunc): Variadic functional options to customize retry behavior.
//
// Returns:
//   - result (T): the result from the operation if it succeeds, or the result of the last
//     attempt if all attempts fail. If the context is canceled, it is the value the most recent
//     attempt produced — possibly partial or stale — or the zero value of T if no attempt ran;
//     treat it as successful only when err is nil.
//   - err (error): the error from the last attempt if all attempts fail, or context.Cause(ctx)
//     wrapped with the entry point's name if the context is canceled or times out. Returns nil
//     if the operation succeeds.
func RetryWithData[T any](ctx context.Context, operation OperationWithData[T], ofs ...OptionFunc) (result T, err error) {
	return retry(ctx, operation, "retrier.RetryWithData", ofs...)
}

// retry is the shared implementation for [Retry] and [RetryWithData]. name is
// the entry point's name, used to wrap context errors.
//
// Type Parameters:
//   - T: type of the data returned by the operation.
//
// Parameters:
//   - ctx (context.Context): context controlling the retry lifecycle.
//   - operation (OperationWithData[T]): operation to retry.
//   - name (string): entry point's name, used to wrap context errors.
//   - ofs (...OptionFunc): functional options configuring the retry loop.
//
// Returns:
//   - result (T): result of the successful or final attempt.
//   - err (error): nil on success, the last attempt's error, or the wrapped
//     context cause.
func retry[T any](ctx context.Context, operation OperationWithData[T], name string, ofs ...OptionFunc) (result T, err error) {
	opts := &Options{
		MaxAttempts: DefaultMaxAttempts,
		WaitMin:     DefaultWaitMin,
		WaitMax:     DefaultWaitMax,
		NewBackoff:  hqgoretrierbackoff.ExponentialWithDecorrelatedJitter,
	}

	for _, f := range ofs {
		if f != nil {
			f(opts)
		}
	}

	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = DefaultMaxAttempts
	}

	if opts.WaitMin <= 0 {
		opts.WaitMin = DefaultWaitMin
	}

	if opts.WaitMax <= 0 {
		opts.WaitMax = DefaultWaitMax
	}

	if opts.WaitMax < opts.WaitMin {
		opts.WaitMax = opts.WaitMin
	}

	if opts.NewBackoff == nil {
		opts.NewBackoff = hqgoretrierbackoff.ExponentialWithDecorrelatedJitter
	}

	b := opts.NewBackoff(opts.WaitMin, opts.WaitMax)
	if b == nil {
		panic("retrier: backoff constructor returned nil")
	}

	// timer is created lazily on the first wait and reused across attempts via Reset, avoiding
	// one time.NewTimer allocation per failed attempt. Post-1.23 timer semantics make Reset
	// after expiry safe without the old Stop-and-drain dance.
	var timer *time.Timer

	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			return result, fmt.Errorf("%s: %w", name, context.Cause(ctx))
		}

		result, err = operation()
		if err == nil {
			return result, nil
		}

		if opts.RetryOn != nil && !opts.RetryOn(err) {
			return result, err
		}

		if attempt >= opts.MaxAttempts {
			return result, err
		}

		delay := b(attempt)

		// A custom strategy may compute a non-positive delay; clamp it to the floor so the
		// loop cannot busy-spin. The OnRetry event carries the clamped delay, matching the
		// wait that actually happens.
		if delay <= 0 {
			delay = minRetryDelay
		}

		if opts.OnRetry != nil {
			opts.OnRetry(RetryEvent{Attempt: attempt, Err: err, Wait: delay})
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

			return result, fmt.Errorf("%s: %w", name, context.Cause(ctx))
		}
	}
}
