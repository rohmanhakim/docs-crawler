package limiter

import (
	"math/rand"
	"sync"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// RateLimiter
// Specialized component to manage rate limiting during crawling
// Responsibilities:
// - Bookkeep each hostname's last fetch timestamp
// - Compute the final delay for each hostname given various factors
// - Make sure the crawling process respect the server's policy
type RateLimiter interface {
	SetBaseDelay(baseDelay time.Duration)
	SetJitter(jitter time.Duration)
	SetRandomSeed(randomSeed int64)
	SetCrawlDelay(host string, delay time.Duration)
	Backoff(host string)
	ResetBackoff(host string)
	MarkLastFetchAsNow(host string)
	SetRNG(rng interface{})
	ResolveDelay(host string) time.Duration
}

type ConcurrentRateLimiter struct {
	mu           sync.RWMutex
	rngMu        sync.Mutex
	baseDelay    time.Duration
	jitter       time.Duration
	hostTimings  map[string]hostTiming
	rng          *rand.Rand
	backoffParam timeutil.BackoffParam
}

func NewConcurrentRateLimiter() *ConcurrentRateLimiter {
	return &ConcurrentRateLimiter{
		hostTimings: make(map[string]hostTiming),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		backoffParam: timeutil.NewBackoffParam(
			1*time.Second,
			2.0,
			30*time.Second,
		),
	}
}

func (r *ConcurrentRateLimiter) SetBackoffParam(backoffParam timeutil.BackoffParam) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.backoffParam = backoffParam
}

func (r *ConcurrentRateLimiter) SetBaseDelay(baseDelay time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.baseDelay = baseDelay
}

func (r *ConcurrentRateLimiter) SetJitter(jitter time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.jitter = jitter
}

func (r *ConcurrentRateLimiter) SetRandomSeed(randomSeed int64) {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()

	r.rng = rand.New(rand.NewSource(randomSeed))
}

// Set delay to given host, separated from global base delay
func (r *ConcurrentRateLimiter) SetCrawlDelay(host string, delay time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentHostTiming, exists := r.hostTimings[host]
	if exists {
		currentHostTiming.crawlDelay = delay
		r.hostTimings[host] = currentHostTiming
	} else {
		r.hostTimings[host] = hostTiming{
			crawlDelay: delay,
		}
	}
}

// Backoff triggers exponential backoff for the given host.
// It increments the backoff counter and computes the delay.
func (r *ConcurrentRateLimiter) Backoff(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentHostTiming, exists := r.hostTimings[host]

	r.rngMu.Lock()
	if r.rng == nil {
		r.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if exists {
		currentHostTiming.backoffCount++
		currentHostTiming.backoffDelay = timeutil.ExponentialBackoffDelay(currentHostTiming.backoffCount, r.jitter, *r.rng, r.backoffParam)
		r.hostTimings[host] = currentHostTiming
	} else {
		// Initialize with backoffCount=1
		r.hostTimings[host] = hostTiming{
			backoffCount: 1,
			backoffDelay: timeutil.ExponentialBackoffDelay(1, r.jitter, *r.rng, r.backoffParam),
		}
	}
	r.rngMu.Unlock()
}

// ResetBackoff resets the backoff counter for the given host.
// Called after a successful request to clear backoff state.
func (r *ConcurrentRateLimiter) ResetBackoff(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentHostTiming, exists := r.hostTimings[host]
	if exists {
		currentHostTiming.backoffCount = 0
		currentHostTiming.backoffDelay = time.Duration(0)
		r.hostTimings[host] = currentHostTiming
	}
}

// Mark the given host lastFetch to time.Now()
func (r *ConcurrentRateLimiter) MarkLastFetchAsNow(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentHostTiming, exists := r.hostTimings[host]
	if exists {
		currentHostTiming.lastFetchAt = time.Now()
		r.hostTimings[host] = currentHostTiming
	} else {
		r.hostTimings[host] = hostTiming{
			lastFetchAt: time.Now(),
		}
	}
}

// SetRNG allows injecting a custom random number generator for testing
func (r *ConcurrentRateLimiter) SetRNG(rng interface{}) {
	if randImpl, ok := rng.(*rand.Rand); ok {
		r.rngMu.Lock()
		r.rng = randImpl
		r.rngMu.Unlock()
	}
}

// Compute the final delay resolution for given host
// FinalDelay = max(BaseDelay, crawlDelay, BackoffDelay) + Jitter
func (r *ConcurrentRateLimiter) ResolveDelay(host string) time.Duration {
	// copy needed state under read lock, then compute without holding r.mu
	r.mu.RLock()
	currentHostTiming, exists := r.hostTimings[host]
	base := r.baseDelay
	jitter := r.jitter
	r.mu.RUnlock()

	// return no delay if the host not registered yet
	if !exists {
		return time.Duration(0)
	}

	delays := []time.Duration{base, currentHostTiming.crawlDelay, currentHostTiming.backoffDelay}

	// compute the highest delay between BaseDelay, crawlDelay, and BackoffDelay
	finalDelay := timeutil.MaxDuration(delays)

	r.rngMu.Lock()
	// add jitter to the final delay (computeJitter protects rng)
	if r.rng == nil {
		r.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	finalDelay += timeutil.ComputeJitter(jitter, *r.rng)
	r.rngMu.Unlock()

	elapsed := time.Since(currentHostTiming.lastFetchAt)

	// return the remaining time since the host last been fetched,
	// else don't delay
	if elapsed < finalDelay {
		return finalDelay - elapsed
	}

	return time.Duration(0)
}

func (r *ConcurrentRateLimiter) BaseDelay() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.baseDelay
}

func (r *ConcurrentRateLimiter) Jitter() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.jitter
}

func (r *ConcurrentRateLimiter) RNG() *rand.Rand {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	return r.rng
}

func (r *ConcurrentRateLimiter) HostTimings() map[string]hostTiming {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// return a shallow copy to avoid exposing internal map for mutation
	copyMap := make(map[string]hostTiming, len(r.hostTimings))
	for k, v := range r.hostTimings {
		copyMap[k] = v
	}
	return copyMap
}
