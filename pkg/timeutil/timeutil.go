package timeutil

import (
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
