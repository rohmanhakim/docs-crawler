package limiter_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/limiter"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// TestConcurrentAccessRateLimiter is a stress test for thread-safety of ConcurrentRateLimiter.
//
// Test Scenario:
// - Spawns 60 concurrent goroutines, each executing 800 random operations
// - Each goroutine independently performs setter, getter, and compute operations on a single shared RateLimiter
// - Operations are randomized across 12 different scenarios:
//   - Global setters (SetBaseDelay, SetJitter, SetRandomSeed)
//   - Host-specific setters (SetCrawlDelay, Backoff, MarkLastFetchAsNow)
//   - RNG injection (SetRNG)
//   - Global getters (GetBaseDelay, GetJitter, GetRng, GetHostTimings)
//   - Computation (ResolveDelay - reads multiple fields and computes with RNG)
//
// - Hosts are selected randomly from a fixed pool of 5 hostnames
//
// Expected Behavior:
// - All operations must be atomic and thread-safe; no data races
// - No deadlocks despite heavy concurrent load with many lock acquisitions
// - Final state must be valid (GetHostTimings returns non-nil map)
//
// Run with `-race` flag to detect data races:
//
//	go test -race ./pkg/limiter -run TestConcurrentAccessRateLimiter
func TestConcurrentAccessRateLimiter(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(100 * time.Millisecond)
	rl.SetJitter(50 * time.Millisecond)
	rl.SetRandomSeed(42)

	// Fixed pool of hosts to maximize contention on host-specific operations
	hosts := []string{"a.example", "b.example", "c.example", "d.example", "e.example"}

	var wg sync.WaitGroup
	workers := 60       // Number of concurrent goroutines
	opsPerWorker := 800 // Operations per goroutine (48,000 total ops)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Each goroutine has its own RNG to avoid contention on per-goroutine randomness
			r := rand.New(rand.NewSource(int64(id) + time.Now().UnixNano()))
			for j := 0; j < opsPerWorker; j++ {
				switch r.Intn(13) {
				case 0:
					// Setter: Modify global base delay
					rl.SetBaseDelay(time.Duration(r.Intn(300)) * time.Millisecond)
				case 1:
					// Setter: Modify global jitter configuration
					rl.SetJitter(time.Duration(r.Intn(200)) * time.Millisecond)
				case 2:
					// Setter: Replace the RNG with a new seeded instance (high contention point)
					rl.SetRandomSeed(int64(r.Intn(10000)))
				case 3:
					// Setter: Update crawl delay for a random host
					h := hosts[r.Intn(len(hosts))]
					rl.SetCrawlDelay(h, time.Duration(r.Intn(800))*time.Millisecond)
				case 4:
					// Setter: Trigger backoff for a random host
					h := hosts[r.Intn(len(hosts))]
					rl.Backoff(h)
				case 5:
					// Setter: Mark last fetch timestamp for a random host
					h := hosts[r.Intn(len(hosts))]
					rl.MarkLastFetchAsNow(h)
				case 6:
					// Setter: Inject a custom RNG (tests SetRNG under high contention)
					rl.SetRNG(rand.New(rand.NewSource(int64(r.Intn(1e6)))))
				case 7:
					// Setter: Update backoff parameters (tests SetBackoffParam under contention)
					customParam := timeutil.NewBackoffParam(
						time.Duration(r.Intn(2000))*time.Millisecond,
						2.0,
						30*time.Second,
					)
					rl.SetBackoffParam(customParam)
				case 8, 9:
					// Getters: Read global configuration (read lock operations)
					_ = rl.BaseDelay()
					_ = rl.Jitter()
				case 10:
					// Getter: Read the RNG instance (protected by rngMu)
					_ = rl.RNG()
				case 11:
					// Getter: Read the host timings map (read lock, returns copy)
					_ = rl.HostTimings()
				default:
					// Compute: Complex operation that reads multiple fields, calls computeJitter, and performs arithmetic
					// This tests coordination between r.mu (read) and rngMu locking patterns
					_ = rl.ResolveDelay(hosts[r.Intn(len(hosts))])
				}
			}
		}(i)
	}

	wg.Wait()

	// Sanity check: verify final state is valid

	if rl.HostTimings() == nil {
		t.Fatal("GetHostTimings returned nil map")
	}
}
