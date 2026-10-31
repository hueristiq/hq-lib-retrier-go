package retrier_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
)

var (
	errTestOperation = errors.New("operation failed")
	errAlternate     = errors.New("alternate failure")
)

type flakyOperation struct {
	failures int
	calls    int
}

func (o *flakyOperation) run() error {
	o.calls++

	if o.calls <= o.failures {
		return errTestOperation
	}

	return nil
}

func noWaitBackoff(_, _ time.Duration) hqgoretrierbackoff.Backoff {
	return func(int) time.Duration {
		return 0
	}
}

func TestRetry_ImmediateSuccess(t *testing.T) {
	t.Parallel()

	op := &flakyOperation{failures: 0}

	err := hqgoretrier.Retry(
		context.Background(),
		op.run,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.NoError(t, err, "Expected the operation to succeed on the first attempt")
	assert.Equal(t, 1, op.calls, "Expected the operation to be called exactly once")
}

func TestRetry_SuccessAfterFailures(t *testing.T) {
	t.Parallel()

	op := &flakyOperation{failures: 2}

	err := hqgoretrier.Retry(
		context.Background(),
		op.run,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.NoError(t, err, "Expected the operation to succeed after retries")
	assert.Equal(t, 3, op.calls, "Expected the operation to be called 3 times")
}

func TestRetry_AcrossBackoffStrategies(t *testing.T) {
	t.Parallel()

	strategies := []struct {
		name       string
		newBackoff func(minDelay, maxDelay time.Duration) hqgoretrierbackoff.Backoff
	}{
		{"exponential", hqgoretrierbackoff.Exponential},
		{"equal jitter", hqgoretrierbackoff.ExponentialWithEqualJitter},
		{"full jitter", hqgoretrierbackoff.ExponentialWithFullJitter},
		{"decorrelated jitter", hqgoretrierbackoff.ExponentialWithDecorrelatedJitter},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			op := &flakyOperation{failures: 2}

			err := hqgoretrier.Retry(
				context.Background(),
				op.run,
				hqgoretrier.WithMaxAttempts(5),
				hqgoretrier.WithWaitMin(time.Millisecond),
				hqgoretrier.WithWaitMax(5*time.Millisecond),
				hqgoretrier.WithBackoff(s.newBackoff),
			)

			require.NoError(t, err, "Expected the operation to succeed after retries")
			assert.Equal(t, 3, op.calls, "Expected the operation to be called 3 times")
		})
	}
}

func TestRetry_MaxAttempts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		maxAttempts int
		wantCalls   int
	}{
		{"single attempt", 1, 1},
		{"two attempts", 2, 2},
		{"three attempts", 3, 3},
		{"five attempts", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			op := &flakyOperation{failures: 1000}

			err := hqgoretrier.Retry(
				context.Background(),
				op.run,
				hqgoretrier.WithMaxAttempts(tt.maxAttempts),
				hqgoretrier.WithBackoff(noWaitBackoff),
			)

			require.ErrorIs(t, err, errTestOperation, "Expected the last operation error")
			assert.Equal(t, tt.wantCalls, op.calls, "Expected attempts to equal maxAttempts")
		})
	}
}

func TestRetry_MaxAttemptsFallsBackToDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		maxAttempts int
	}{
		{"zero", 0},
		{"negative", -2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			op := &flakyOperation{failures: 1000}

			err := hqgoretrier.Retry(
				context.Background(),
				op.run,
				hqgoretrier.WithMaxAttempts(tt.maxAttempts),
				hqgoretrier.WithBackoff(noWaitBackoff),
			)

			require.ErrorIs(t, err, errTestOperation, "Expected the last operation error")
			assert.Equal(t, hqgoretrier.DefaultMaxAttempts, op.calls, "Expected a non-positive maxAttempts to fall back to DefaultMaxAttempts")
		})
	}
}

func TestRetry_OptionsLastOneWins(t *testing.T) {
	t.Parallel()

	op := &flakyOperation{failures: 1000}

	err := hqgoretrier.Retry(
		context.Background(),
		op.run,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithMaxAttempts(2),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.ErrorIs(t, err, errTestOperation, "Expected the last operation error")
	assert.Equal(t, 2, op.calls, "Expected the later option to override the earlier one")
}

func TestDefaultConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 3, hqgoretrier.DefaultMaxAttempts, "Expected the default maximum attempts to be 3")
	assert.Equal(t, time.Second, hqgoretrier.DefaultWaitMin, "Expected the default minimum wait to be 1 second")
	assert.Equal(t, 30*time.Second, hqgoretrier.DefaultWaitMax, "Expected the default maximum wait to be 30 seconds")
}

func TestRetry_DefaultMaxAttempts(t *testing.T) {
	t.Parallel()

	op := &flakyOperation{failures: 1000}

	err := hqgoretrier.Retry(
		context.Background(),
		op.run,
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.Error(t, err, "Expected the operation to fail after the default number of attempts")
	assert.Equal(t, hqgoretrier.DefaultMaxAttempts, op.calls, "Expected the default of DefaultMaxAttempts attempts")
}

func TestRetry_ReturnsLastError(t *testing.T) {
	t.Parallel()

	calls := 0

	op := func() error {
		calls++

		if calls == 1 {
			return errTestOperation
		}

		return errAlternate
	}

	err := hqgoretrier.Retry(
		context.Background(),
		op,
		hqgoretrier.WithMaxAttempts(2),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.ErrorIs(t, err, errAlternate, "Expected the error from the final attempt")
	assert.NotErrorIs(t, err, errTestOperation, "Expected the earlier error to be discarded")
}

func TestRetry_RetryIf(t *testing.T) {
	t.Parallel()

	t.Run("rejected error stops retries", func(t *testing.T) {
		t.Parallel()

		op := &flakyOperation{failures: 1000}

		err := hqgoretrier.Retry(
			context.Background(),
			op.run,
			hqgoretrier.WithMaxAttempts(5),
			hqgoretrier.WithBackoff(noWaitBackoff),
			hqgoretrier.WithRetryOn(func(err error) bool {
				return errors.Is(err, errAlternate)
			}),
		)

		require.ErrorIs(t, err, errTestOperation, "Expected the rejected error")
		assert.Equal(t, 1, op.calls, "Expected no retries once the predicate rejects the error")
	})

	t.Run("accepted error is retried", func(t *testing.T) {
		t.Parallel()

		op := &flakyOperation{failures: 2}

		err := hqgoretrier.Retry(
			context.Background(),
			op.run,
			hqgoretrier.WithMaxAttempts(5),
			hqgoretrier.WithBackoff(noWaitBackoff),
			hqgoretrier.WithRetryOn(func(err error) bool {
				return errors.Is(err, errTestOperation)
			}),
		)

		require.NoError(t, err, "Expected the operation to succeed after retries")
		assert.Equal(t, 3, op.calls, "Expected the operation to be called 3 times")
	})

	t.Run("nil predicate retries every error", func(t *testing.T) {
		t.Parallel()

		op := &flakyOperation{failures: 2}

		err := hqgoretrier.Retry(
			context.Background(),
			op.run,
			hqgoretrier.WithMaxAttempts(5),
			hqgoretrier.WithBackoff(noWaitBackoff),
			hqgoretrier.WithRetryOn(nil),
		)

		require.NoError(t, err, "Expected the operation to succeed after retries")
		assert.Equal(t, 3, op.calls, "Expected the operation to be called 3 times")
	})
}

func TestRetry_OnRetryReceivesAttemptErrorAndWait(t *testing.T) {
	t.Parallel()

	var got []hqgoretrier.RetryEvent

	perAttempt := func(_, _ time.Duration) hqgoretrierbackoff.Backoff {
		return func(attempt int) time.Duration {
			return time.Duration(attempt) * time.Millisecond
		}
	}

	op := &flakyOperation{failures: 2}

	err := hqgoretrier.Retry(
		context.Background(),
		op.run,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(perAttempt),
		hqgoretrier.WithOnRetry(func(ev hqgoretrier.RetryEvent) {
			got = append(got, ev)
		}),
	)

	require.NoError(t, err, "Expected the operation to succeed after retries")
	require.Len(t, got, 2, "Expected the OnRetry callback to fire once per failed attempt that is retried")

	for i, ev := range got {
		assert.Equal(t, i+1, ev.Attempt, "Expected the 1-based number of the failed attempt")
		require.ErrorIs(t, ev.Err, errTestOperation, "Expected the OnRetry callback to receive the failure")
		assert.Equal(t, time.Duration(i+1)*time.Millisecond, ev.Wait, "Expected the computed delay")
	}
}

func TestRetry_OnRetrySkipsFinalAttempt(t *testing.T) {
	t.Parallel()

	calls := 0

	op := &flakyOperation{failures: 1000}

	err := hqgoretrier.Retry(
		context.Background(),
		op.run,
		hqgoretrier.WithMaxAttempts(3),
		hqgoretrier.WithBackoff(noWaitBackoff),
		hqgoretrier.WithOnRetry(func(hqgoretrier.RetryEvent) {
			calls++
		}),
	)

	require.ErrorIs(t, err, errTestOperation, "Expected the last operation error")
	assert.Equal(t, 2, calls, "Expected the OnRetry callback to fire only before the waits, not after the final failure")
}

func TestRetry_OnRetrySilentWithoutUpcomingRetry(t *testing.T) {
	t.Parallel()

	notified := 0

	op := &flakyOperation{failures: 1000}

	err := hqgoretrier.Retry(
		context.Background(),
		op.run,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(noWaitBackoff),
		hqgoretrier.WithRetryOn(func(error) bool { return false }),
		hqgoretrier.WithOnRetry(func(hqgoretrier.RetryEvent) {
			notified++
		}),
	)

	require.ErrorIs(t, err, errTestOperation, "Expected the rejected error")
	assert.Zero(t, notified, "Expected no notification when no retry follows a rejected error")
}

func TestRetry_OnRetryPanicPropagates(t *testing.T) {
	t.Parallel()

	op := &flakyOperation{failures: 1}

	require.Panics(t, func() {
		_ = hqgoretrier.Retry(
			context.Background(),
			op.run,
			hqgoretrier.WithMaxAttempts(5),
			hqgoretrier.WithBackoff(noWaitBackoff),
			hqgoretrier.WithOnRetry(func(hqgoretrier.RetryEvent) {
				panic("OnRetry observer failed")
			}),
		)
	}, "Expected a panicking OnRetry callback to propagate its panic")
	assert.Equal(t, 1, op.calls, "Expected the panic to abort the retry loop after the first attempt")
}

func TestRetry_ZeroDelayBackoffIsClampedToFloor(t *testing.T) {
	t.Parallel()

	var waits []time.Duration

	op := &flakyOperation{failures: 1000}

	start := time.Now()

	err := hqgoretrier.Retry(
		context.Background(),
		op.run,
		hqgoretrier.WithMaxAttempts(3),
		hqgoretrier.WithBackoff(noWaitBackoff), // computes a zero delay on every attempt
		hqgoretrier.WithOnRetry(func(ev hqgoretrier.RetryEvent) {
			waits = append(waits, ev.Wait)
		}),
	)

	elapsed := time.Since(start)

	require.ErrorIs(t, err, errTestOperation, "Expected the last operation error")
	assert.Equal(t, 3, op.calls, "Expected the loop to stop at maxAttempts rather than spin")
	require.Len(t, waits, 2, "Expected the OnRetry callback to fire once per wait between attempts")

	for _, wait := range waits {
		assert.Equal(t, time.Millisecond, wait, "Expected the OnRetry callback to observe the zero delay clamped to the 1 ms floor")
	}

	assert.Less(t, elapsed, 5*time.Second, "Expected the clamped loop to finish near the floor delay per wait")
}

func TestRetry_BackoffConstructedOnceWithConfiguredBounds(t *testing.T) {
	t.Parallel()

	var (
		constructs     int
		gotMin, gotMax time.Duration //nolint:revive // Min/Max name the backoff bounds.
		attempts       []int
	)

	waitMin := 7 * time.Millisecond
	waitMax := 11 * time.Millisecond

	factory := func(minDelay, maxDelay time.Duration) hqgoretrierbackoff.Backoff {
		constructs++
		gotMin, gotMax = minDelay, maxDelay

		return func(attempt int) time.Duration {
			attempts = append(attempts, attempt)

			return 0
		}
	}

	op := &flakyOperation{failures: 3}

	err := hqgoretrier.Retry(
		context.Background(),
		op.run,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithWaitMin(waitMin),
		hqgoretrier.WithWaitMax(waitMax),
		hqgoretrier.WithBackoff(factory),
	)

	require.NoError(t, err, "Expected the operation to succeed after retries")
	assert.Equal(t, 1, constructs, "Expected the backoff to be constructed once per retry loop")
	assert.Equal(t, waitMin, gotMin, "Expected the configured minimum wait")
	assert.Equal(t, waitMax, gotMax, "Expected the configured maximum wait")
	assert.Equal(t, []int{1, 2, 3}, attempts, "Expected increasing attempt numbers")
}

func TestRetry_NilBackoffFallsBackToDefault(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

	defer cancel()

	op := &flakyOperation{failures: 1000}

	err := hqgoretrier.Retry(
		ctx,
		op.run,
		hqgoretrier.WithBackoff(nil),
	)

	require.ErrorIs(t, err, context.DeadlineExceeded, "Expected the default backoff's wait to outlast the context")
	assert.Equal(t, 1, op.calls, "Expected a single attempt before the default backoff's wait begins")
}

func TestRetry_WaitBoundsAreNormalized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		waitMin time.Duration
		waitMax time.Duration
		wantMin time.Duration
		wantMax time.Duration
	}{
		{"zero bounds", 0, 0, hqgoretrier.DefaultWaitMin, hqgoretrier.DefaultWaitMax},
		{"negative bounds", -time.Second, -time.Second, hqgoretrier.DefaultWaitMin, hqgoretrier.DefaultWaitMax},
		{"zero min", 0, 5 * time.Second, hqgoretrier.DefaultWaitMin, 5 * time.Second},
		{"zero max", 5 * time.Second, 0, 5 * time.Second, hqgoretrier.DefaultWaitMax},
		{"max below min", 10 * time.Second, 5 * time.Second, 10 * time.Second, 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotMin, gotMax time.Duration //nolint:revive // Min/Max name the backoff bounds.

			factory := func(minDelay, maxDelay time.Duration) hqgoretrierbackoff.Backoff {
				gotMin, gotMax = minDelay, maxDelay

				return func(int) time.Duration {
					return 0
				}
			}

			op := &flakyOperation{failures: 1000}

			err := hqgoretrier.Retry(
				context.Background(),
				op.run,
				hqgoretrier.WithMaxAttempts(2),
				hqgoretrier.WithWaitMin(tt.waitMin),
				hqgoretrier.WithWaitMax(tt.waitMax),
				hqgoretrier.WithBackoff(factory),
			)

			require.ErrorIs(t, err, errTestOperation, "Expected the last operation error")
			assert.Equal(t, tt.wantMin, gotMin, "Expected the normalized minimum wait")
			assert.Equal(t, tt.wantMax, gotMax, "Expected the normalized maximum wait")
		})
	}
}

func TestRetry_ContextCanceledBeforeStart(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	op := &flakyOperation{failures: 2}

	err := hqgoretrier.Retry(
		ctx,
		op.run,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.ErrorIs(t, err, context.Canceled, "Expected a cancellation error")
	assert.Zero(t, op.calls, "Expected the operation never to run once the context is already canceled")
}

func TestRetry_ContextCanceledBeforeStartWithCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("shutting down")

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)

	op := &flakyOperation{failures: 2}

	err := hqgoretrier.Retry(
		ctx,
		op.run,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.ErrorIs(t, err, cause, "Expected the cancellation cause from the pre-attempt branch")
	assert.Zero(t, op.calls, "Expected the operation never to run once the context is already canceled")
}

func TestRetry_ContextCanceledDuringWait(t *testing.T) {
	t.Parallel()

	cause := errors.New("canceled during wait")

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	waiting := make(chan struct{})

	var once sync.Once

	longBackoff := func(_, _ time.Duration) hqgoretrierbackoff.Backoff {
		return func(int) time.Duration {
			once.Do(func() { close(waiting) })

			return time.Hour
		}
	}

	op := func() error {
		return errTestOperation
	}

	go func() {
		<-waiting

		cancel(cause)
	}()

	err := hqgoretrier.Retry(
		ctx,
		op,
		hqgoretrier.WithMaxAttempts(10),
		hqgoretrier.WithBackoff(longBackoff),
	)

	require.ErrorIs(t, err, cause, "Expected the cancellation cause from the wait branch")
}

func TestRetry_ContextTimeout(t *testing.T) {
	t.Parallel()

	op := &flakyOperation{failures: 1000}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)

	defer cancel()

	err := hqgoretrier.Retry(
		ctx,
		op.run,
		hqgoretrier.WithMaxAttempts(1000),
		hqgoretrier.WithWaitMin(30*time.Millisecond),
		hqgoretrier.WithWaitMax(100*time.Millisecond),
		hqgoretrier.WithBackoff(hqgoretrierbackoff.Exponential),
	)

	require.ErrorIs(t, err, context.DeadlineExceeded, "Expected a deadline-exceeded error")
	assert.LessOrEqual(t, op.calls, 2, "Expected few attempts before the deadline elapses")
}

func TestRetry_UnboundedAttemptsStopOnContext(t *testing.T) {
	t.Parallel()

	op := &flakyOperation{failures: 1_000_000}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)

	defer cancel()

	err := hqgoretrier.Retry(
		ctx,
		op.run,
		hqgoretrier.WithMaxAttempts(math.MaxInt),
		hqgoretrier.WithWaitMin(time.Millisecond),
		hqgoretrier.WithWaitMax(2*time.Millisecond),
		hqgoretrier.WithBackoff(hqgoretrierbackoff.Exponential),
	)

	require.ErrorIs(t, err, context.DeadlineExceeded, "Expected the context deadline to stop the retries")
	assert.Positive(t, op.calls, "Expected the operation to be attempted at least once")
}

func TestRetry_ConcurrentLoops(t *testing.T) {
	t.Parallel()

	const loops = 8

	var (
		wg       sync.WaitGroup
		failures atomic.Int32
	)

	for range loops {
		wg.Add(1)

		go func() {
			defer wg.Done()

			op := &flakyOperation{failures: 2}

			err := hqgoretrier.Retry(
				context.Background(),
				op.run,
				hqgoretrier.WithMaxAttempts(5),
				hqgoretrier.WithWaitMin(time.Nanosecond),
				hqgoretrier.WithWaitMax(time.Microsecond),
				hqgoretrier.WithBackoff(hqgoretrierbackoff.ExponentialWithDecorrelatedJitter),
			)
			if err != nil {
				failures.Add(1)
			}
		}()
	}

	wg.Wait()

	assert.Zero(t, failures.Load(), "Expected all concurrent retry loops to succeed with per-loop backoff state")
}

func TestRetryWithData_ImmediateSuccess(t *testing.T) {
	t.Parallel()

	calls := 0

	op := func() (string, error) {
		calls++

		return "ok", nil
	}

	result, err := hqgoretrier.RetryWithData(
		context.Background(),
		op,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.NoError(t, err, "Expected the operation to succeed immediately")
	assert.Equal(t, "ok", result, "Expected the successful result")
	assert.Equal(t, 1, calls, "Expected the operation to be called exactly once")
}

func TestRetryWithData_SuccessAfterFailures(t *testing.T) {
	t.Parallel()

	calls := 0

	op := func() (int, error) {
		calls++

		if calls <= 2 {
			return 0, errTestOperation
		}

		return 42, nil
	}

	result, err := hqgoretrier.RetryWithData(
		context.Background(),
		op,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.NoError(t, err, "Expected the operation to succeed after retries")
	assert.Equal(t, 42, result, "Expected the successful result")
	assert.Equal(t, 3, calls, "Expected the operation to be called 3 times")
}

func TestRetryWithData_ReturnsLastResultOnFailure(t *testing.T) {
	t.Parallel()

	calls := 0

	op := func() (int, error) {
		calls++

		return calls, errTestOperation
	}

	result, err := hqgoretrier.RetryWithData(
		context.Background(),
		op,
		hqgoretrier.WithMaxAttempts(3),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.ErrorIs(t, err, errTestOperation, "Expected the last operation error")
	assert.Equal(t, 3, result, "Expected the data from the final attempt")
	assert.Equal(t, 3, calls, "Expected three attempts")
}

func TestRetryWithData_RetryIfStopsRetries(t *testing.T) {
	t.Parallel()

	calls := 0

	op := func() (int, error) {
		calls++

		return calls, errTestOperation
	}

	result, err := hqgoretrier.RetryWithData(
		context.Background(),
		op,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(noWaitBackoff),
		hqgoretrier.WithRetryOn(func(err error) bool {
			return errors.Is(err, errAlternate)
		}),
	)

	require.ErrorIs(t, err, errTestOperation, "Expected the rejected error")
	assert.Equal(t, 1, result, "Expected the data from the only attempt")
	assert.Equal(t, 1, calls, "Expected no retries once the predicate rejects the error")
}

func TestRetryWithData_ContextCanceledBeforeStart(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0

	op := func() (int, error) {
		calls++

		return 99, errTestOperation
	}

	result, err := hqgoretrier.RetryWithData(
		ctx,
		op,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithBackoff(noWaitBackoff),
	)

	require.ErrorIs(t, err, context.Canceled, "Expected a cancellation error")
	assert.Zero(t, result, "Expected the zero value when canceled before any attempt")
	assert.Zero(t, calls, "Expected the operation never to run")
}

func TestRetryWithData_ContextCanceledDuringWait(t *testing.T) {
	t.Parallel()

	cause := errors.New("canceled during wait")

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	waiting := make(chan struct{})

	var once sync.Once

	longBackoff := func(_, _ time.Duration) hqgoretrierbackoff.Backoff {
		return func(int) time.Duration {
			once.Do(func() { close(waiting) })

			return time.Hour
		}
	}

	op := func() (int, error) {
		return 0, errTestOperation
	}

	go func() {
		<-waiting

		cancel(cause)
	}()

	result, err := hqgoretrier.RetryWithData(
		ctx,
		op,
		hqgoretrier.WithMaxAttempts(10),
		hqgoretrier.WithBackoff(longBackoff),
	)

	require.ErrorIs(t, err, cause, "Expected the cancellation cause from the wait branch")
	assert.Zero(t, result, "Expected the zero value when canceled during the wait")
}

func TestRetryWithData_ContextCanceledDuringWaitReturnsStaleResult(t *testing.T) {
	t.Parallel()

	cause := errors.New("canceled during wait")

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	waiting := make(chan struct{})

	var once sync.Once

	longBackoff := func(_, _ time.Duration) hqgoretrierbackoff.Backoff {
		return func(int) time.Duration {
			once.Do(func() { close(waiting) })

			return time.Hour
		}
	}

	op := func() (int, error) {
		return 7, errTestOperation
	}

	go func() {
		<-waiting

		cancel(cause)
	}()

	result, err := hqgoretrier.RetryWithData(
		ctx,
		op,
		hqgoretrier.WithMaxAttempts(10),
		hqgoretrier.WithBackoff(longBackoff),
	)

	require.ErrorIs(t, err, cause, "Expected the cancellation cause from the wait branch")
	assert.Equal(t, 7, result, "Expected the stale result from the most recent attempt when canceled during the wait")
}

var (
	sinkResult int
	errSink    error
)

func BenchmarkRetryWithData(b *testing.B) {
	b.ReportAllocs()

	constantBackoff := func(_, _ time.Duration) hqgoretrierbackoff.Backoff {
		return func(int) time.Duration {
			return time.Nanosecond
		}
	}

	ctx := context.Background()

	for range b.N {
		calls := 0

		operation := func() (int, error) {
			calls++

			if calls <= 2 {
				return 0, errTestOperation
			}

			return 42, nil
		}

		sinkResult, errSink = hqgoretrier.RetryWithData(
			ctx,
			operation,
			hqgoretrier.WithMaxAttempts(5),
			hqgoretrier.WithBackoff(constantBackoff),
		)
	}
}

// ExampleRetry demonstrates retrying an operation that returns only an error.
func ExampleRetry() {
	attempts := 0

	operation := func() error {
		attempts++

		if attempts < 3 {
			return errors.New("temporary failure")
		}

		return nil
	}

	err := hqgoretrier.Retry(
		context.Background(),
		operation,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithWaitMin(time.Millisecond),
		hqgoretrier.WithWaitMax(time.Millisecond),
	)

	fmt.Printf("err: %v\n", err)
	// Output:
	// err: <nil>
}

// ExampleRetryWithData demonstrates retrying an operation that returns a value.
func ExampleRetryWithData() {
	attempts := 0

	fetch := func() (string, error) {
		attempts++

		if attempts < 2 {
			return "", errors.New("temporary failure")
		}

		return "payload", nil
	}

	result, err := hqgoretrier.RetryWithData(
		context.Background(),
		fetch,
		hqgoretrier.WithMaxAttempts(5),
		hqgoretrier.WithWaitMin(time.Millisecond),
		hqgoretrier.WithWaitMax(time.Millisecond),
	)
	if err != nil {
		fmt.Println(err)

		return
	}

	fmt.Println(result)
	// Output:
	// payload
}

// FuzzRetryWithData exercises the retry loop over arbitrary failure counts and
// attempt limits with pinned nanosecond wait bounds, so runs stay fast: the
// loop must succeed exactly when failures do not exhaust the attempt budget,
// and otherwise return the operation error with a zero result.
func FuzzRetryWithData(f *testing.F) {
	f.Add(0, 5)
	f.Add(3, 5)
	f.Add(10, 1)

	f.Fuzz(func(t *testing.T, failures int, maxAttempts int) {
		if failures < 0 {
			failures = -failures
		}

		failures %= 100

		if maxAttempts < 0 {
			maxAttempts = -maxAttempts
		}

		maxAttempts %= 10

		if maxAttempts == 0 {
			maxAttempts = hqgoretrier.DefaultMaxAttempts
		}

		calls := 0

		op := func() (int, error) {
			calls++

			if calls <= failures {
				return 0, errTestOperation
			}

			return calls, nil
		}

		result, err := hqgoretrier.RetryWithData(
			context.Background(),
			op,
			hqgoretrier.WithMaxAttempts(maxAttempts),
			hqgoretrier.WithWaitMin(time.Nanosecond),
			hqgoretrier.WithWaitMax(time.Nanosecond),
		)

		if failures >= maxAttempts {
			if !errors.Is(err, errTestOperation) {
				t.Fatalf("Expected the operation error after %d attempts, got %v", maxAttempts, err)
			}

			if result != 0 {
				t.Fatalf("Expected a zero result on failure, got %d", result)
			}

			return
		}

		if err != nil {
			t.Fatalf("Expected success with %d failures and %d attempts, got %v", failures, maxAttempts, err)
		}

		if result != failures+1 {
			t.Fatalf("Expected the successful attempt's result %d, got %d", failures+1, result)
		}
	})
}
