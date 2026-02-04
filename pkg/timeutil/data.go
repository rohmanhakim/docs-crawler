package timeutil

import "time"

// Exponential Backoff parameters
// example:
//
//	initialDuration := 1 * time.Second // Start with 1s
//	multiplier := 2.0                 // Double each time
//	maxDuration := 30 * time.Second    // Cap at 30s

type BackoffParam struct {
	initialDuration time.Duration
	multiplier      float64
	maxDuration     time.Duration
}

func NewBackoffParam(
	initialDuration time.Duration,
	multiplier float64,
	maxDuration time.Duration,
) BackoffParam {
	return BackoffParam{
		initialDuration: initialDuration,
		multiplier:      multiplier,
		maxDuration:     maxDuration,
	}
}

func (b *BackoffParam) InitialDuration() time.Duration {
	return b.initialDuration
}

func (b *BackoffParam) Multiplier() float64 {
	return b.multiplier
}

func (b *BackoffParam) MaxDuration() time.Duration {
	return b.maxDuration
}
