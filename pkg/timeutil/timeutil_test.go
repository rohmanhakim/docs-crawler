package timeutil

import (
	"math/rand"
	"testing"
	"time"
)

func TestMaxDuration(t *testing.T) {
	tests := []struct {
		name      string
		durations []time.Duration
		want      time.Duration
	}{
		{
			name:      "multiple values returns maximum",
			durations: []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 200 * time.Millisecond},
			want:      500 * time.Millisecond,
		},
		{
			name:      "single value returns that value",
			durations: []time.Duration{300 * time.Millisecond},
			want:      300 * time.Millisecond,
		},
		{
			name:      "empty slice returns zero",
			durations: []time.Duration{},
			want:      0,
		},
		{
			name:      "all same values returns that value",
			durations: []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond},
			want:      100 * time.Millisecond,
		},
		{
			name:      "negative durations handled correctly",
			durations: []time.Duration{-100 * time.Millisecond, 50 * time.Millisecond, -200 * time.Millisecond},
			want:      50 * time.Millisecond,
		},
		{
			name:      "all negative returns least negative",
			durations: []time.Duration{-100 * time.Millisecond, -50 * time.Millisecond, -200 * time.Millisecond},
			want:      -50 * time.Millisecond,
		},
		{
			name:      "zero in mix returns positive max",
			durations: []time.Duration{0, 100 * time.Millisecond, 0},
			want:      100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxDuration(tt.durations)
			if got != tt.want {
				t.Errorf("MaxDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaxDurationDoesNotMutateInput(t *testing.T) {
	original := []time.Duration{300 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}
	expected := []time.Duration{300 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}

	_ = MaxDuration(original)

	for i := range original {
		if original[i] != expected[i] {
			t.Errorf("MaxDuration mutated input slice: got %v at index %d, want %v", original[i], i, expected[i])
		}
	}
}

func TestDurationPtr(t *testing.T) {
	d := 5 * time.Second
	ptr := DurationPtr(d)

	if ptr == nil {
		t.Fatal("DurationPtr returned nil")
	}

	if *ptr != d {
		t.Errorf("DurationPtr() = %v, want %v", *ptr, d)
	}
}

func TestComputeJitter(t *testing.T) {
	tests := []struct {
		name string
		max  time.Duration
		rng  rand.Rand
	}{
		{
			name: "max=0 returns 0",
			max:  0,
			rng:  *rand.New(rand.NewSource(1)),
		},
		{
			name: "negative max returns 0",
			max:  -100 * time.Millisecond,
			rng:  *rand.New(rand.NewSource(1)),
		},
		{
			name: "positive max returns value within range",
			max:  1000 * time.Millisecond,
			rng:  *rand.New(rand.NewSource(42)),
		},
		{
			name: "large max works correctly",
			max:  10 * time.Second,
			rng:  *rand.New(rand.NewSource(123)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeJitter(tt.max, tt.rng)

			if tt.max <= 0 {
				if got != 0 {
					t.Errorf("ComputeJitter() = %v, want 0", got)
				}
				return
			}

			if got < 0 || got > tt.max {
				t.Errorf("ComputeJitter() = %v, want between 0 and %v", got, tt.max)
			}
		})
	}
}

func TestComputeJitterDistribution(t *testing.T) {
	const max = 100 * time.Millisecond
	const iterations = 10000
	rng := rand.New(rand.NewSource(42))

	min := max
	maxObserved := time.Duration(0)
	sum := int64(0)

	for i := 0; i < iterations; i++ {
		val := ComputeJitter(max, *rng)
		sum += int64(val)
		if val < min {
			min = val
		}
		if val > maxObserved {
			maxObserved = val
		}
	}

	avg := time.Duration(sum / int64(iterations))

	// rand.Int63n(n) returns values in [0, n), so maxObserved should be < max
	// But it should be very close to max (within about 1ms or 1%)
	maxTolerance := time.Duration(1 * time.Millisecond)
	if maxObserved < max-maxTolerance {
		t.Errorf("Expected maximum jitter to be at least %v (within tolerance), got %v", max-maxTolerance, maxObserved)
	}

	// Minimum should be very close to 0 (within 1ms)
	minTolerance := time.Duration(1 * time.Millisecond)
	if min > minTolerance {
		t.Errorf("Expected minimum jitter to be near 0 (within 1ms), got %v", min)
	}

	// For uniform distribution, average should be approximately max/2
	expectedAvg := max / 2
	tolerance := max / 10 // 10% tolerance
	if avg < expectedAvg-tolerance || avg > expectedAvg+tolerance {
		t.Errorf("Average jitter = %v, expected approximately %v (±%v)", avg, expectedAvg, tolerance)
	}
}

func TestExponentialBackoffDelay(t *testing.T) {
	tests := []struct {
		name          string
		backoffCount  int
		jitter        time.Duration
		backoffParam  BackoffParam
		rng           rand.Rand
		wantMin       time.Duration
		wantMax       time.Duration
		verifyExact   bool
		expectedExact time.Duration
	}{
		{
			name:          "first backoff (count=1) with no jitter",
			backoffCount:  1,
			jitter:        0,
			backoffParam:  NewBackoffParam(1*time.Second, 2.0, 30*time.Second),
			rng:           *rand.New(rand.NewSource(1)),
			wantMin:       1 * time.Second,
			wantMax:       1 * time.Second,
			verifyExact:   true,
			expectedExact: 1 * time.Second,
		},
		{
			name:          "second backoff (count=2) doubles",
			backoffCount:  2,
			jitter:        0,
			backoffParam:  NewBackoffParam(1*time.Second, 2.0, 30*time.Second),
			rng:           *rand.New(rand.NewSource(1)),
			wantMin:       2 * time.Second,
			wantMax:       2 * time.Second,
			verifyExact:   true,
			expectedExact: 2 * time.Second,
		},
		{
			name:          "third backoff (count=3) quadruples",
			backoffCount:  3,
			jitter:        0,
			backoffParam:  NewBackoffParam(1*time.Second, 2.0, 30*time.Second),
			rng:           *rand.New(rand.NewSource(1)),
			wantMin:       4 * time.Second,
			wantMax:       4 * time.Second,
			verifyExact:   true,
			expectedExact: 4 * time.Second,
		},
		{
			name:          "backoff hits max cap",
			backoffCount:  10,
			jitter:        0,
			backoffParam:  NewBackoffParam(1*time.Second, 2.0, 10*time.Second),
			rng:           *rand.New(rand.NewSource(1)),
			wantMin:       10 * time.Second,
			wantMax:       10 * time.Second,
			verifyExact:   true,
			expectedExact: 10 * time.Second,
		},
		{
			name:         "jitter adds positive variance",
			backoffCount: 2,
			jitter:       100 * time.Millisecond,
			backoffParam: NewBackoffParam(1*time.Second, 2.0, 30*time.Second),
			rng:          *rand.New(rand.NewSource(42)),
			wantMin:      2 * time.Second,
			wantMax:      2*time.Second + 100*time.Millisecond,
		},
		{
			name:          "zero initial duration",
			backoffCount:  5,
			jitter:        0,
			backoffParam:  NewBackoffParam(0, 2.0, 30*time.Second),
			rng:           *rand.New(rand.NewSource(1)),
			wantMin:       0,
			wantMax:       0,
			verifyExact:   true,
			expectedExact: 0,
		},
		{
			name:          "multiplier of 1 (no growth)",
			backoffCount:  5,
			jitter:        0,
			backoffParam:  NewBackoffParam(1*time.Second, 1.0, 30*time.Second),
			rng:           *rand.New(rand.NewSource(1)),
			wantMin:       1 * time.Second,
			wantMax:       1 * time.Second,
			verifyExact:   true,
			expectedExact: 1 * time.Second,
		},
		{
			name:          "fractional multiplier",
			backoffCount:  2,
			jitter:        0,
			backoffParam:  NewBackoffParam(1*time.Second, 1.5, 30*time.Second),
			rng:           *rand.New(rand.NewSource(1)),
			wantMin:       time.Duration(float64(1*time.Second) * 1.5),
			wantMax:       time.Duration(float64(1*time.Second) * 1.5),
			verifyExact:   true,
			expectedExact: time.Duration(float64(1*time.Second) * 1.5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExponentialBackoffDelay(tt.backoffCount, tt.jitter, tt.rng, tt.backoffParam)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("ExponentialBackoffDelay() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}

			if tt.verifyExact && got != tt.expectedExact {
				t.Errorf("ExponentialBackoffDelay() = %v, want %v", got, tt.expectedExact)
			}
		})
	}
}

func TestExponentialBackoffDelay_JitterDistribution(t *testing.T) {
	backoffCount := 3
	jitter := 50 * time.Millisecond
	backoffParam := NewBackoffParam(1*time.Second, 2.0, 30*time.Second)
	rng := rand.New(rand.NewSource(42))

	baseDelay := 4 * time.Second // count=3: 1 * 2^(3-1) = 4 seconds

	iterations := 1000
	min := baseDelay + jitter
	max := baseDelay
	sum := int64(0)

	for i := 0; i < iterations; i++ {
		val := ExponentialBackoffDelay(backoffCount, jitter, *rng, backoffParam)
		sum += int64(val)
		if val < min {
			min = val
		}
		if val > max {
			max = val
		}
	}

	avg := time.Duration(sum / int64(iterations))

	// With jitter, values should be in range [baseDelay, baseDelay + jitter]
	if min < baseDelay {
		t.Errorf("Expected minimum delay >= %v, got %v", baseDelay, min)
	}
	if max > baseDelay+jitter {
		t.Errorf("Expected maximum delay <= %v, got %v", baseDelay+jitter, max)
	}

	// For uniform jitter distribution, average should be approximately base + jitter/2
	expectedAvg := baseDelay + jitter/2
	tolerance := jitter / 10
	if avg < expectedAvg-tolerance || avg > expectedAvg+tolerance {
		t.Errorf("Average delay = %v, expected approximately %v (±%v)", avg, expectedAvg, tolerance)
	}
}

func TestExponentialBackoffDelay_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		backoffCount int
		jitter       time.Duration
		backoffParam BackoffParam
		rng          rand.Rand
	}{
		{
			name:         "zero backoff count",
			backoffCount: 0,
			jitter:       0,
			backoffParam: NewBackoffParam(1*time.Second, 2.0, 30*time.Second),
			rng:          *rand.New(rand.NewSource(1)),
		},
		{
			name:         "negative backoff count",
			backoffCount: -1,
			jitter:       0,
			backoffParam: NewBackoffParam(1*time.Second, 2.0, 30*time.Second),
			rng:          *rand.New(rand.NewSource(1)),
		},
		{
			name:         "zero jitter",
			backoffCount: 1,
			jitter:       0,
			backoffParam: NewBackoffParam(1*time.Second, 2.0, 30*time.Second),
			rng:          *rand.New(rand.NewSource(1)),
		},
		{
			name:         "negative jitter",
			backoffCount: 1,
			jitter:       -100 * time.Millisecond,
			backoffParam: NewBackoffParam(1*time.Second, 2.0, 30*time.Second),
			rng:          *rand.New(rand.NewSource(1)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should not panic and return reasonable values
			got := ExponentialBackoffDelay(tt.backoffCount, tt.jitter, tt.rng, tt.backoffParam)

			if got < 0 {
				t.Errorf("ExponentialBackoffDelay() returned negative duration: %v", got)
			}
		})
	}
}
