package timeutil

import "time"

// DurationPtr is a helper function to create a pointer to a time.Duration
func DurationPtr(d time.Duration) *time.Duration {
	return &d
}
