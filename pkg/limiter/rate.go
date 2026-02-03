package limiter

import (
	"math"
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
	mu          sync.RWMutex
	rngMu       sync.Mutex
	baseDelay   time.Duration
	jitter      time.Duration
	hostTimings map[string]hostTiming
	rng         *rand.Rand
}

func NewConcurrentRateLimiter() *ConcurrentRateLimiter {
	return &ConcurrentRateLimiter{
		hostTimings: make(map[string]hostTiming),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
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

// exponentialBackoffDelay computes exponential backoff based on count
// Does NOT take lock; caller must hold r.mu (RLock or Lock)
func (r *ConcurrentRateLimiter) exponentialBackoffDelay(backoffCount int) time.Duration {
	// Exponential backoff parameters
	initialBackoff := 1 * time.Second // Start with 1s
	multiplier := 2.0                 // Double each time
	maxBackoff := 30 * time.Second    // Cap at 30s

	// Compute exponential: initial * (multiplier ^ (count - 1))
	// First backoff (count=1): initialBackoff
	exponent := float64(backoffCount - 1)
	delay := float64(initialBackoff) * math.Pow(multiplier, exponent)
	if delay > float64(maxBackoff) {
		delay = float64(maxBackoff)
	}

	// Add jitter only if configured jitter > 0
	if r.jitter > 0 {
		jitterValue := r.computeJitter(r.jitter)
		delay += float64(jitterValue)
	}

	return time.Duration(delay)
}

// Backoff triggers exponential backoff for the given host.
// It increments the backoff counter and computes the delay.
func (r *ConcurrentRateLimiter) Backoff(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentHostTiming, exists := r.hostTimings[host]
	if exists {
		currentHostTiming.backoffCount++
		currentHostTiming.backoffDelay = r.exponentialBackoffDelay(currentHostTiming.backoffCount)
		r.hostTimings[host] = currentHostTiming
	} else {
		// Initialize with backoffCount=1
		r.hostTimings[host] = hostTiming{
			backoffCount: 1,
			backoffDelay: r.exponentialBackoffDelay(1),
		}
	}
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

// Compute jitter for the given max duration
// Returns a pseudo-random duration between 0 and max (inclusive)
func (r *ConcurrentRateLimiter) computeJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}

	r.rngMu.Lock()
	defer r.rngMu.Unlock()

	if r.rng == nil {
		r.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Safe to call Int63n under lock since we hold rngMu
	return time.Duration(r.rng.Int63n(int64(max)))
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

	// add jitter to the final delay (computeJitter protects rng)
	finalDelay += r.computeJitter(jitter)

	elapsed := time.Since(currentHostTiming.lastFetchAt)

	// return the remaining time since the host last been fetched,
	// else don't delay
	if elapsed < finalDelay {
		return finalDelay - elapsed
	}

	return time.Duration(0)
}

func (r *ConcurrentRateLimiter) GetBaseDelay() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.baseDelay
}

func (r *ConcurrentRateLimiter) GetJitter() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.jitter
}

func (r *ConcurrentRateLimiter) GetRng() *rand.Rand {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	return r.rng
}

func (r *ConcurrentRateLimiter) GetHostTimings() map[string]hostTiming {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// return a shallow copy to avoid exposing internal map for mutation
	copyMap := make(map[string]hostTiming, len(r.hostTimings))
	for k, v := range r.hostTimings {
		copyMap[k] = v
	}
	return copyMap
}

// timing-related data used to track when to fetch host during crawling
type hostTiming struct {
	lastFetchAt  time.Time
	backoffDelay time.Duration
	crawlDelay   time.Duration
	backoffCount int
}

func (h *hostTiming) GetCrawlDelay() time.Duration {
	return h.crawlDelay
}

func (h *hostTiming) GetBackOffDelay() time.Duration {
	return h.backoffDelay
}

func (h *hostTiming) GetLastFetchAt() time.Time {
	return h.lastFetchAt
}

func (h *hostTiming) GetBackoffCount() int {
	return h.backoffCount
}
