package timeutil

import (
	"math"
	"math/rand"
	"slices"
	"time"
)

// durationPtr is a helper function to create a pointer to a time.Duration
func DurationPtr(d time.Duration) *time.Duration {
	return &d
}

// handy function to sort a slice of time.Duration and return the highest one
func MaxDuration(durations []time.Duration) time.Duration {
	// guard clause: return 0 for empty slice
	if len(durations) == 0 {
		return 0
	}

	// copy the inputs to not mutate it
	d := make([]time.Duration, len(durations))
	copy(d, durations)

	// comparison function for string time.Duration
	comparison := func(a, b time.Duration) int {
		// a > b returns -1
		// a < b returns 1
		// a == b returns 0
		if a > b {
			return -1
		} else if a < b {
			return 1
		}
		return 0
	}

	// sort descending, we don't care about sorting stability
	slices.SortFunc(d, comparison)

	// return the highest (first) one
	return d[0]
}

// Compute jitter for the given max duration
// Returns a pseudo-random duration between 0 and max (inclusive)
func ComputeJitter(max time.Duration, rng rand.Rand) time.Duration {
	if max <= 0 {
		return 0
	}

	return time.Duration(rng.Int63n(int64(max)))
}

// Computes exponential backoff based on count
func ExponentialBackoffDelay(
	backoffCount int,
	jitter time.Duration,
	rng rand.Rand,
	backOffParam BackoffParam,
) time.Duration {
	// Exponential backoff parameters
	initialBackoff := backOffParam.InitialDuration()
	multiplier := backOffParam.Multiplier()
	maxBackoff := backOffParam.MaxDuration()

	// Compute exponential: initial * (multiplier ^ (count - 1))
	// First backoff (count=1): initialBackoff
	exponent := float64(backoffCount - 1)
	delay := float64(initialBackoff) * math.Pow(multiplier, exponent)
	if delay > float64(maxBackoff) {
		delay = float64(maxBackoff)
	}

	// Add jitter only if jitter > 0
	if jitter > 0 {
		jitterValue := ComputeJitter(jitter, rng)
		delay += float64(jitterValue)
	}

	return time.Duration(delay)
}
