package timeutil

import "time"

// durationPtr is a helper function to create a pointer to a time.Duration
func DurationPtr(d time.Duration) *time.Duration {
	return &d
}
