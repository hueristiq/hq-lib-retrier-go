package backoff

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExponentialBackoff(t *testing.T) {
	t.Parallel()

	t.Run("standard progression", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			attempt  int
			expected time.Duration
		}{
			{name: "attempt 1", attempt: 1, expected: 2 * time.Millisecond},
			{name: "attempt 2", attempt: 2, expected: 4 * time.Millisecond},
			{name: "attempt 3", attempt: 3, expected: 8 * time.Millisecond},
			{name: "attempt 4", attempt: 4, expected: 16 * time.Millisecond},
			{name: "attempt 5", attempt: 5, expected: 32 * time.Millisecond},
			{name: "attempt 6", attempt: 6, expected: 64 * time.Millisecond},
			{name: "attempt 7", attempt: 7, expected: 128 * time.Millisecond},
			{name: "attempt 8", attempt: 8, expected: 256 * time.Millisecond},
			{name: "attempt 9", attempt: 9, expected: 512 * time.Millisecond},
			{name: "attempt 10 (capped)", attempt: 10, expected: time.Second},
			{name: "attempt 20 (capped)", attempt: 20, expected: time.Second},
			{name: "attempt 60 (capped)", attempt: 60, expected: time.Second},
		}

		b := Exponential(time.Millisecond, time.Second)

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				delay := b(tt.attempt)

				assert.Equal(t, tt.expected, delay, "Unexpected backoff duration for attempt %d", tt.attempt)
			})
		}
	})

	t.Run("edge cases", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name               string
			minDelay, maxDelay time.Duration
			attempt            int
			expected           time.Duration
		}{
			{
				name:     "negative minDelay",
				minDelay: -time.Millisecond,
				maxDelay: time.Second,
				attempt:  1,
				expected: 0,
			},
			{
				name:     "negative maxDelay",
				minDelay: time.Millisecond,
				maxDelay: -time.Second,
				attempt:  1,
				expected: 0,
			},
			{
				name:     "minDelay = maxDelay",
				minDelay: time.Second,
				maxDelay: time.Second,
				attempt:  5,
				expected: time.Second,
			},
			{
				name:     "minDelay > maxDelay",
				minDelay: 2 * time.Second,
				maxDelay: time.Second,
				attempt:  0,
				expected: 0,
			},
			{
				name:     "negative attempt",
				minDelay: time.Millisecond,
				maxDelay: time.Second,
				attempt:  -1,
				expected: 0,
			},
			{
				name:     "zero attempt",
				minDelay: time.Millisecond,
				maxDelay: time.Second,
				attempt:  0,
				expected: time.Millisecond,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b := Exponential(tt.minDelay, tt.maxDelay)

				delay := b(tt.attempt)

				assert.Equal(t, tt.expected, delay)
			})
		}
	})

	t.Run("overflow protection", func(t *testing.T) {
		t.Parallel()

		maxDelay := time.Duration(math.MaxInt64)
		b := Exponential(time.Duration(math.MaxInt64/2), maxDelay)

		delay := b(2)

		assert.Equal(t, maxDelay, delay, "Should cap at maxDelay when overflow would occur")
	})
}

func expBase(minDelay, maxDelay time.Duration, attempt int) time.Duration {
	return min(minDelay<<attempt, maxDelay)
}

func TestExponentialDeterminism(t *testing.T) {
	t.Parallel()

	b := Exponential(time.Millisecond, time.Second)
	want := b(4)

	for range 20 {
		assert.Equal(t, want, b(4), "Exponential should be deterministic")
	}
}

func TestExponentialJitterVaries(t *testing.T) {
	t.Parallel()

	const samples = 50

	strategies := []struct {
		name string
		b    Backoff
	}{
		{"equal jitter", ExponentialWithEqualJitter(time.Second, time.Minute)},
		{"full jitter", ExponentialWithFullJitter(time.Second, time.Minute)},
		{"decorrelated jitter", ExponentialWithDecorrelatedJitter(time.Second, time.Minute)},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			seen := make(map[time.Duration]struct{})

			for range samples {
				seen[s.b(3)] = struct{}{}
			}

			assert.Greater(t, len(seen), 1, "Jittered strategy should produce varied delays")
		})
	}
}

func TestInvalidBoundsAlwaysReturnZero(t *testing.T) {
	t.Parallel()

	constructors := []struct {
		name string
		new  func(minDelay, maxDelay time.Duration) Backoff
	}{
		{"exponential", Exponential},
		{"constant", Constant},
		{"equal jitter", ExponentialWithEqualJitter},
		{"full jitter", ExponentialWithFullJitter},
		{"decorrelated jitter", ExponentialWithDecorrelatedJitter},
	}

	bounds := []struct {
		name               string
		minDelay, maxDelay time.Duration
	}{
		{"zero minDelay", 0, time.Second},
		{"negative minDelay", -time.Second, time.Second},
		{"zero maxDelay", time.Second, 0},
		{"negative maxDelay", time.Second, -time.Second},
		{"minDelay > maxDelay", 2 * time.Second, time.Second},
	}

	for _, c := range constructors {
		for _, b := range bounds {
			t.Run(c.name+"/"+b.name, func(t *testing.T) {
				t.Parallel()

				strategy := c.new(b.minDelay, b.maxDelay)

				assert.Zero(t, strategy(0), "Expected a zero delay from invalid bounds")
				assert.Zero(t, strategy(5), "Expected a zero delay from invalid bounds")
			})
		}
	}
}

func TestExponentialWithEqualJitterBackoff(t *testing.T) {
	t.Parallel()

	t.Run("jitter range validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			maxDelay time.Duration
			attempt  int
		}{
			{"attempt 1", time.Second, 1},
			{"attempt 2", time.Second, 2},
			{"attempt 3", time.Second, 3},
			{"attempt 4", time.Second, 4},
			{"attempt 5", time.Second, 5},
			{"attempt 6", time.Second, 6},
			{"attempt 7", time.Second, 7},
			{"attempt 8", time.Second, 8},
			{"attempt 9", time.Second, 9},
			{"attempt 10 (capped)", 2 * time.Second, 10},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				minDelay := time.Millisecond
				base := expBase(minDelay, tt.maxDelay, tt.attempt)
				b := ExponentialWithEqualJitter(minDelay, tt.maxDelay)

				for range 10 {
					delay := b(tt.attempt)

					assert.GreaterOrEqual(t, delay, base/2, "Delay should be at least base/2")
					assert.Less(t, delay, base, "Delay should be below base, the range is [base/2, base)")
					assert.LessOrEqual(t, delay, tt.maxDelay, "Delay should not exceed maxDelay")
				}
			})
		}
	})

	t.Run("edge cases", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name               string
			minDelay, maxDelay time.Duration
			attempt            int
			expectZero         bool
		}{
			{"negative minDelay", -time.Millisecond, time.Second, 1, true},
			{"negative maxDelay", time.Millisecond, -time.Second, 1, true},
			{"negative attempt", time.Millisecond, time.Second, -1, true},
			{"minDelay = maxDelay", time.Second, time.Second, 5, false},
			{"minDelay > maxDelay", 2 * time.Second, time.Second, 0, true},
			{"zero attempt", time.Millisecond, time.Second, 0, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b := ExponentialWithEqualJitter(tt.minDelay, tt.maxDelay)

				delay := b(tt.attempt)

				if tt.expectZero {
					assert.Equal(t, time.Duration(0), delay)

					return
				}

				assert.GreaterOrEqual(t, delay, time.Duration(0))
				assert.LessOrEqual(t, delay, tt.maxDelay)
			})
		}
	})

	t.Run("overflow protection", func(t *testing.T) {
		t.Parallel()

		maxDelay := time.Duration(math.MaxInt64)
		b := ExponentialWithEqualJitter(time.Duration(math.MaxInt64/2), maxDelay)

		delay := b(2)

		assert.GreaterOrEqual(t, delay, time.Duration(0))
		assert.LessOrEqual(t, delay, maxDelay, "Should not exceed maxDelay when overflow would occur")
	})
}

func TestExponentialWithFullJitterBackoff(t *testing.T) {
	t.Parallel()

	t.Run("jitter range validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			attempt int
		}{
			{"attempt 1", 1},
			{"attempt 2", 2},
			{"attempt 3", 3},
			{"attempt 4", 4},
			{"attempt 5", 5},
			{"attempt 6", 6},
			{"attempt 7", 7},
			{"attempt 8", 8},
			{"attempt 9", 9},
			{"attempt 10 (capped)", 10},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				minDelay := time.Millisecond
				maxDelay := time.Second
				base := expBase(minDelay, maxDelay, tt.attempt)
				b := ExponentialWithFullJitter(minDelay, maxDelay)

				for range 10 {
					delay := b(tt.attempt)

					assert.GreaterOrEqual(t, delay, time.Duration(0), "Delay should be at least 0")
					assert.Less(t, delay, base, "Delay should be below base, the range is [0, base)")
					assert.LessOrEqual(t, delay, maxDelay, "Delay should not exceed maxDelay")
				}
			})
		}
	})

	t.Run("edge cases", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name               string
			minDelay, maxDelay time.Duration
			attempt            int
			expectZero         bool
		}{
			{"negative minDelay", -time.Millisecond, time.Second, 1, true},
			{"negative maxDelay", time.Millisecond, -time.Second, 1, true},
			{"negative attempt", time.Millisecond, time.Second, -1, true},
			{"minDelay = maxDelay", time.Second, time.Second, 5, false},
			{"minDelay > maxDelay", 2 * time.Second, time.Second, 0, true},
			{"zero attempt", time.Millisecond, time.Second, 0, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b := ExponentialWithFullJitter(tt.minDelay, tt.maxDelay)

				delay := b(tt.attempt)

				if tt.expectZero {
					assert.Equal(t, time.Duration(0), delay)

					return
				}

				assert.GreaterOrEqual(t, delay, time.Duration(0))
				assert.LessOrEqual(t, delay, tt.maxDelay)
			})
		}
	})

	t.Run("overflow protection", func(t *testing.T) {
		t.Parallel()

		maxDelay := time.Duration(math.MaxInt64)
		b := ExponentialWithFullJitter(time.Duration(math.MaxInt64/2), maxDelay)

		delay := b(2)

		assert.GreaterOrEqual(t, delay, time.Duration(0))
		assert.LessOrEqual(t, delay, maxDelay, "Should not exceed maxDelay when overflow would occur")
	})
}

func TestExponentialWithDecorrelatedJitterBackoff(t *testing.T) {
	t.Parallel()

	t.Run("jitter progression", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			attempt int
		}{
			{"attempt 1", 1},
			{"attempt 2", 2},
			{"attempt 3", 3},
			{"attempt 4", 4},
			{"attempt 5", 5},
			{"attempt 6", 6},
			{"attempt 7", 7},
			{"attempt 8", 8},
			{"attempt 9", 9},
			{"attempt 10 (capped)", 10},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				minDelay := time.Millisecond
				maxDelay := time.Second
				b := ExponentialWithDecorrelatedJitter(minDelay, maxDelay)

				for range 10 {
					delay := b(tt.attempt)

					assert.GreaterOrEqual(t, delay, minDelay, "Delay should be at least minDelay")
					assert.LessOrEqual(t, delay, maxDelay, "Delay should not exceed maxDelay")
				}
			})
		}
	})

	t.Run("edge cases", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name               string
			minDelay, maxDelay time.Duration
			attempt            int
			expectZero         bool
		}{
			{"negative minDelay", -time.Millisecond, time.Second, 1, true},
			{"negative maxDelay", time.Millisecond, -time.Second, 1, true},
			{"negative attempt", time.Millisecond, time.Second, -1, true},
			{"minDelay > maxDelay", 2 * time.Second, time.Second, 0, true},
			{"minDelay = maxDelay", time.Second, time.Second, 5, false},
			{"zero attempt", time.Millisecond, time.Second, 0, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b := ExponentialWithDecorrelatedJitter(tt.minDelay, tt.maxDelay)

				delay := b(tt.attempt)

				if tt.expectZero {
					assert.Equal(t, time.Duration(0), delay)

					return
				}

				assert.GreaterOrEqual(t, delay, time.Duration(0))
				assert.LessOrEqual(t, delay, tt.maxDelay)
			})
		}
	})

	t.Run("overflow protection", func(t *testing.T) {
		t.Parallel()

		maxDelay := time.Duration(math.MaxInt64)
		b := ExponentialWithDecorrelatedJitter(time.Duration(math.MaxInt64/2), maxDelay)

		delay := b(2)

		assert.GreaterOrEqual(t, delay, time.Duration(0))
		assert.LessOrEqual(t, delay, maxDelay, "Should not exceed maxDelay when overflow would occur")
	})

	t.Run("instances are independent", func(t *testing.T) {
		t.Parallel()

		minDelay := time.Millisecond
		maxDelay := time.Second

		first := ExponentialWithDecorrelatedJitter(minDelay, maxDelay)
		second := ExponentialWithDecorrelatedJitter(minDelay, maxDelay)

		for range 20 {
			first(1)
		}

		delay := second(1)

		assert.GreaterOrEqual(t, delay, minDelay, "A fresh instance should start within its initial range")
		assert.LessOrEqual(t, delay, minDelay*3, "A fresh instance should start from minDelay, unaffected by another instance")
	})
}

func TestExponentialWithDecorrelatedJitterGrowthBound(t *testing.T) {
	t.Parallel()

	minDelay := time.Millisecond
	maxDelay := time.Second

	b := ExponentialWithDecorrelatedJitter(minDelay, maxDelay)

	previous := b(1)

	require.GreaterOrEqual(t, previous, minDelay, "Expected the first draw to be at least minDelay")
	require.LessOrEqual(t, previous, minDelay*3, "Expected the first draw to start from minDelay")

	for range 100 {
		delay := b(1)

		upper := min(maxDelay, previous*3)

		assert.GreaterOrEqual(t, delay, minDelay, "Delay should be at least minDelay")
		assert.LessOrEqual(t, delay, upper, "Delay should not exceed three times the previous delay")

		previous = delay
	}
}

func TestExponentialWithDecorrelatedJitterConcurrentUse(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 8
		draws      = 100
	)

	minDelay := time.Millisecond
	maxDelay := time.Second

	b := ExponentialWithDecorrelatedJitter(minDelay, maxDelay)

	var (
		wg       sync.WaitGroup
		failures atomic.Int32
	)

	for range goroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range draws {
				delay := b(1)

				if delay < minDelay || delay > maxDelay {
					failures.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	assert.Zero(t, failures.Load(), "Expected every concurrent draw to stay within bounds")
}

var sinkDuration time.Duration

func TestConstantBackoff(t *testing.T) {
	t.Parallel()

	t.Run("fixed delay for every attempt", func(t *testing.T) {
		t.Parallel()

		b := Constant(100*time.Millisecond, time.Second)

		for attempt := 1; attempt <= 10; attempt++ {
			assert.Equal(t, 100*time.Millisecond, b(attempt), "Expected a fixed delay for attempt %d", attempt)
		}
	})

	t.Run("edge cases", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name               string
			minDelay, maxDelay time.Duration
			attempt            int
			expected           time.Duration
		}{
			{"negative minDelay", -time.Millisecond, time.Second, 1, 0},
			{"negative maxDelay", time.Millisecond, -time.Second, 1, 0},
			{"minDelay > maxDelay", 2 * time.Second, time.Second, 1, 0},
			{"negative attempt", time.Millisecond, time.Second, -1, 0},
			{"zero attempt", time.Millisecond, time.Second, 0, time.Millisecond},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b := Constant(tt.minDelay, tt.maxDelay)

				assert.Equal(t, tt.expected, b(tt.attempt))
			})
		}
	})
}

func BenchmarkConstant(b *testing.B) {
	b.ReportAllocs()

	bo := Constant(time.Millisecond, 30*time.Second)

	for range b.N {
		sinkDuration = bo(5)
	}
}

func BenchmarkExponential(b *testing.B) {
	b.ReportAllocs()

	bo := Exponential(time.Millisecond, 30*time.Second)

	for range b.N {
		sinkDuration = bo(5)
	}
}

func BenchmarkExponentialWithDecorrelatedJitter(b *testing.B) {
	b.ReportAllocs()

	bo := ExponentialWithDecorrelatedJitter(time.Millisecond, 30*time.Second)

	for range b.N {
		sinkDuration = bo(5)
	}
}
