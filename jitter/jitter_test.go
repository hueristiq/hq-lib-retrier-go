package jitter

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEqualJitter(t *testing.T) {
	t.Parallel()

	t.Run("negative backoff", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, time.Duration(0), Equal(-time.Second))
	})

	t.Run("zero backoff", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, time.Duration(0), Equal(0))
	})

	t.Run("positive backoff", func(t *testing.T) {
		t.Parallel()

		backoff := 10 * time.Second

		for range 100 {
			jittered := Equal(backoff)
			midpoint := backoff / 2

			assert.GreaterOrEqual(t, jittered, midpoint, "Jittered duration should be at least the midpoint")
			assert.Less(t, jittered, backoff, "Jittered duration should be below the original backoff, the range is [backoff/2, backoff)")
		}
	})

	t.Run("small backoff", func(t *testing.T) {
		t.Parallel()

		backoff := 1 * time.Nanosecond
		jittered := Equal(backoff)

		assert.Equal(t, backoff/2, jittered, "For very small backoffs, should return midpoint")
	})

	t.Run("odd backoff truncates midpoint", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 1*time.Nanosecond, Equal(3*time.Nanosecond), "Expected the truncated midpoint for an odd backoff")
	})
}

func TestFullJitter(t *testing.T) {
	t.Parallel()

	t.Run("negative backoff", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, time.Duration(0), Full(-time.Second))
	})

	t.Run("zero backoff", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, time.Duration(0), Full(0))
	})

	t.Run("positive backoff", func(t *testing.T) {
		t.Parallel()

		backoff := 10 * time.Second

		for range 100 {
			jittered := Full(backoff)

			assert.GreaterOrEqual(t, jittered, 0*time.Second, "Jittered duration should be at least 0")
			assert.Less(t, jittered, backoff, "Jittered duration should be less than the original backoff")
		}
	})

	t.Run("small backoff", func(t *testing.T) {
		t.Parallel()

		backoff := 1 * time.Nanosecond
		jittered := Full(backoff)

		assert.Equal(t, time.Duration(0), jittered, "For very small backoffs, should return 0")
	})
}

func TestDecorrelatedJitter(t *testing.T) {
	t.Parallel()

	t.Run("negative min/max", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, time.Duration(0), Decorrelated(-1, 10, 0))
		assert.Equal(t, time.Duration(0), Decorrelated(1, -10, 0))
		assert.Equal(t, time.Duration(0), Decorrelated(-1, -10, 0))
	})

	t.Run("min > max", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, time.Duration(0), Decorrelated(10, 5, 0))
	})

	t.Run("zero bounds", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, time.Duration(0), Decorrelated(0, 0, 0))
		assert.Equal(t, time.Duration(0), Decorrelated(0, 0, 5*time.Second))
	})

	t.Run("zero minDelay degenerates", func(t *testing.T) {
		t.Parallel()

		maxDelay := 10 * time.Second

		assert.Equal(t, time.Duration(0), Decorrelated(0, maxDelay, 0), "Expected the first call with a non-positive previous to return 0")

		for range 100 {
			jittered := Decorrelated(0, maxDelay, 5*time.Second)

			assert.GreaterOrEqual(t, jittered, 0*time.Second, "Jittered duration should be at least 0")
			assert.LessOrEqual(t, jittered, maxDelay, "Jittered duration should not exceed the maximum")
		}
	})

	t.Run("min equals max", func(t *testing.T) {
		t.Parallel()

		delay := 5 * time.Second
		jittered := Decorrelated(delay, delay, 0)
		assert.Equal(t, delay, jittered)
	})

	t.Run("first call (previous=0)", func(t *testing.T) {
		t.Parallel()

		minDelay := 2 * time.Second
		maxDelay := 10 * time.Second
		previous := 0 * time.Second

		jittered := Decorrelated(minDelay, maxDelay, previous)

		assert.GreaterOrEqual(t, jittered, minDelay, "Jittered duration should be at least the minimum")
		assert.LessOrEqual(t, jittered, maxDelay, "Jittered duration should not exceed the maximum")
	})

	t.Run("negative previous defaults to minDelay", func(t *testing.T) {
		t.Parallel()

		minDelay := 2 * time.Second
		maxDelay := 10 * time.Second

		for range 100 {
			jittered := Decorrelated(minDelay, maxDelay, -5*time.Second)

			assert.GreaterOrEqual(t, jittered, minDelay, "Jittered duration should be at least the minimum")
			assert.LessOrEqual(t, jittered, maxDelay, "Jittered duration should not exceed the maximum")
		}
	})

	t.Run("previous smaller than minDelay clamps to minDelay", func(t *testing.T) {
		t.Parallel()

		minDelay := 5 * time.Second
		maxDelay := 10 * time.Second
		previous := 1 * time.Second

		jittered := Decorrelated(minDelay, maxDelay, previous)

		assert.Equal(t, minDelay, jittered, "Jittered duration should clamp to minDelay")
	})

	t.Run("subsequent calls", func(t *testing.T) {
		t.Parallel()

		minDelay := 2 * time.Second
		maxDelay := 10 * time.Second
		previous := 4 * time.Second

		for range 100 {
			jittered := Decorrelated(minDelay, maxDelay, previous)

			assert.GreaterOrEqual(t, jittered, minDelay, "Jittered duration should be at least the minimum")
			assert.LessOrEqual(t, jittered, maxDelay, "Jittered duration should not exceed the maximum")
			assert.LessOrEqual(t, jittered, previous*3+minDelay, "Jittered duration should not exceed three times the previous duration plus minDelay")
		}
	})

	t.Run("max boundary", func(t *testing.T) {
		t.Parallel()

		minDelay := 1 * time.Second
		maxDelay := 10 * time.Second
		previous := 5 * time.Second

		for range 100 {
			jittered := Decorrelated(minDelay, maxDelay, previous)

			assert.GreaterOrEqual(t, jittered, minDelay, "Jittered duration should be at least the minimum")
			assert.LessOrEqual(t, jittered, maxDelay, "Jittered duration should not exceed the maximum")
		}
	})

	t.Run("previous causes overflow", func(t *testing.T) {
		t.Parallel()

		minDelay := 1 * time.Second
		maxDelay := 10 * time.Second
		previous := time.Duration(1<<63 - 1)

		jittered := Decorrelated(minDelay, maxDelay, previous)

		assert.GreaterOrEqual(t, jittered, minDelay, "Jittered duration should be at least the minimum")
		assert.LessOrEqual(t, jittered, maxDelay, "Jittered duration should not exceed the maximum even on overflow")
	})
}

func TestJitterRandomness(t *testing.T) {
	t.Parallel()

	const samples = 50

	base := 8 * time.Second

	t.Run("equal varies", func(t *testing.T) {
		t.Parallel()

		seen := make(map[time.Duration]struct{})

		for range samples {
			seen[Equal(base)] = struct{}{}
		}

		assert.Greater(t, len(seen), 1, "Equal jitter should produce varied durations")
	})

	t.Run("full varies", func(t *testing.T) {
		t.Parallel()

		seen := make(map[time.Duration]struct{})

		for range samples {
			seen[Full(base)] = struct{}{}
		}

		assert.Greater(t, len(seen), 1, "Full jitter should produce varied durations")
	})

	t.Run("decorrelated varies", func(t *testing.T) {
		t.Parallel()

		seen := make(map[time.Duration]struct{})

		for range samples {
			seen[Decorrelated(time.Second, time.Minute, 4*time.Second)] = struct{}{}
		}

		assert.Greater(t, len(seen), 1, "Decorrelated jitter should produce varied durations")
	})
}

func TestJitterConcurrentUse(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 8
		draws      = 100
	)

	base := 8 * time.Second

	var (
		wg       sync.WaitGroup
		failures atomic.Int32
	)

	for range goroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range draws {
				equal := Equal(base)
				full := Full(base)
				decorrelated := Decorrelated(time.Second, time.Minute, base)

				if equal < base/2 || equal >= base || full < 0 || full >= base || decorrelated < time.Second || decorrelated > time.Minute {
					failures.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	assert.Zero(t, failures.Load(), "Expected every concurrent jitter draw to stay within its documented range")
}

var sinkDuration time.Duration

func BenchmarkEqual(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		sinkDuration = Equal(8 * time.Second)
	}
}

func BenchmarkFull(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		sinkDuration = Full(8 * time.Second)
	}
}

func BenchmarkDecorrelated(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		sinkDuration = Decorrelated(time.Second, time.Minute, 4*time.Second)
	}
}
