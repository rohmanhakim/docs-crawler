package limiter_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/limiter"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

func TestNewConcurrentRateLimiter(t *testing.T) {
	baseDelay := 1 * time.Second
	jitter := 100 * time.Millisecond
	seed := int64(42)

	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(baseDelay)
	rl.SetJitter(jitter)
	rl.SetRandomSeed(seed)

	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	if rl.BaseDelay() != baseDelay {
		t.Errorf("baseDelay = %v, want %v", rl.BaseDelay(), baseDelay)
	}

	if rl.Jitter() != jitter {
		t.Errorf("jitter = %v, want %v", rl.Jitter(), jitter)
	}

	if rl.HostTimings() == nil {
		t.Error("hostTimings map not initialized")
	}

	if rl.RNG() == nil {
		t.Error("rng not initialized")
	}
}

func TestRateLimiter_SetCrawlDelay(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(1 * time.Second)
	rl.SetJitter(100 * time.Millisecond)
	rl.SetRandomSeed(42)
	host := "example.com"
	newDelay := 2 * time.Second

	rl.SetCrawlDelay(host, newDelay)

	timing := rl.HostTimings()[host]
	if timing.CrawlDelay() != newDelay {
		t.Errorf("crawlDelay = %v, want %v", timing.CrawlDelay(), newDelay)
	}
}

func TestRateLimiter_Backoff(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(1 * time.Second)
	rl.SetJitter(0) // Disable jitter for predictable tests
	rl.SetRandomSeed(42)
	host := "example.com"

	// First backoff
	rl.Backoff(host)
	timing1 := rl.HostTimings()[host]
	if timing1.BackoffCount() != 1 {
		t.Errorf("backoffCount after first Backoff = %d, want 1", timing1.BackoffCount())
	}
	if timing1.BackOffDelay() != 1*time.Second {
		t.Errorf("backoffDelay after first Backoff = %v, want 1s", timing1.BackOffDelay())
	}

	// Second backoff
	rl.Backoff(host)
	timing2 := rl.HostTimings()[host]
	if timing2.BackoffCount() != 2 {
		t.Errorf("backoffCount after second Backoff = %d, want 2", timing2.BackoffCount())
	}
	// 1s * 2^1 = 2s
	expected2 := 2 * time.Second
	if timing2.BackOffDelay() != expected2 {
		t.Errorf("backoffDelay after second Backoff = %v, want %v", timing2.BackOffDelay(), expected2)
	}

	// Third backoff
	rl.Backoff(host)
	timing3 := rl.HostTimings()[host]
	if timing3.BackoffCount() != 3 {
		t.Errorf("backoffCount after third Backoff = %d, want 3", timing3.BackoffCount())
	}
	// 1s * 2^2 = 4s
	expected3 := 4 * time.Second
	if timing3.BackOffDelay() != expected3 {
		t.Errorf("backoffDelay after third Backoff = %v, want %v", timing3.BackOffDelay(), expected3)
	}
}

func TestRateLimiter_Backoff_MaxCap(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(1 * time.Second)
	rl.SetJitter(0)
	rl.SetRandomSeed(42)
	host := "example.com"

	// Trigger many backoffs to reach max cap (30s)
	// 1s * 2^(n-1) >= 30s => n >= 5 (2^4 = 16, 2^5 = 32)
	for i := 0; i < 10; i++ {
		rl.Backoff(host)
	}

	timing := rl.HostTimings()[host]
	// After enough backoffs, should be capped at 30s
	if timing.BackOffDelay() > 30*time.Second+100*time.Millisecond {
		t.Errorf("backoffDelay after many backoffs = %v, want capped at ~30s", timing.BackOffDelay())
	}
}

func TestRateLimiter_ResetBackoff(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(1 * time.Second)
	rl.SetJitter(0)
	host := "example.com"

	// Trigger backoff
	rl.Backoff(host)
	rl.Backoff(host)
	timing1 := rl.HostTimings()[host]
	if timing1.BackoffCount() != 2 {
		t.Fatalf("setup: backoffCount = %d, want 2", timing1.BackoffCount())
	}

	// Reset backoff
	rl.ResetBackoff(host)
	timing2 := rl.HostTimings()[host]
	if timing2.BackoffCount() != 0 {
		t.Errorf("backoffCount after ResetBackoff = %d, want 0", timing2.BackoffCount())
	}
	if timing2.BackOffDelay() != time.Duration(0) {
		t.Errorf("backoffDelay after ResetBackoff = %v, want 0", timing2.BackOffDelay())
	}

	// After reset, next Backoff should start from count=1 again
	rl.Backoff(host)
	timing3 := rl.HostTimings()[host]
	if timing3.BackoffCount() != 1 {
		t.Errorf("backoffCount after reset and new Backoff = %d, want 1", timing3.BackoffCount())
	}
}

func TestRateLimiter_ResolveDelay_UnregisteredHost(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(1 * time.Second)
	rl.SetJitter(100 * time.Millisecond)
	rl.SetRandomSeed(42)

	delay := rl.ResolveDelay("unregistered.com")

	if delay != 0 {
		t.Errorf("ResolveDelay for unregistered host = %v, want 0", delay)
	}
}

func TestRateLimiter_ResolveDelay_BaseDelayOnly(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(500 * time.Millisecond)
	rl.SetJitter(0)
	rl.SetRandomSeed(42)
	host := "example.com"

	rl.MarkLastFetchAsNow(host)

	// Immediately after marking, should return base delay (minus tiny elapsed time)
	delay := rl.ResolveDelay(host)

	// Allow small margin for elapsed time
	if delay < 490*time.Millisecond || delay > 500*time.Millisecond {
		t.Errorf("ResolveDelay = %v, want approximately 500ms", delay)
	}
}

func TestNewConcurrentRateLimiter_Defaults(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()

	if rl == nil {
		t.Fatal("NewConcurrentRateLimiter returned nil")
	}

	// Check default values
	if rl.BaseDelay() != 0 {
		t.Errorf("default baseDelay = %v, want 0", rl.BaseDelay())
	}
	if rl.Jitter() != 0 {
		t.Errorf("default jitter = %v, want 0", rl.Jitter())
	}
	if rl.RNG() == nil {
		t.Error("default rng not initialized")
	}
	if rl.HostTimings() == nil {
		t.Error("hostTimings map not initialized")
	}

	// Verify backoffParam default: initial backoff should be 1s
	host := "example.com"
	rl.SetJitter(0)             // disable jitter for deterministic backoff
	rl.SetRandomSeed(42)        // deterministic RNG
	rl.MarkLastFetchAsNow(host) // set last fetch to now
	rl.Backoff(host)
	timing := rl.HostTimings()[host]
	if timing.BackOffDelay() != 1*time.Second {
		t.Errorf("default backoff initial delay = %v, want 1s", timing.BackOffDelay())
	}
}

func TestConcurrentRateLimiter_SetBackoffParam(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetJitter(0)
	rl.SetRandomSeed(42)
	host := "example.com"

	// Set custom backoff parameters: initial=2s, multiplier=3.0, max=60s
	customParam := timeutil.NewBackoffParam(2*time.Second, 3.0, 60*time.Second)
	rl.SetBackoffParam(customParam)

	// Verify exponential growth with custom parameters
	rl.MarkLastFetchAsNow(host)

	rl.Backoff(host)
	timing1 := rl.HostTimings()[host]
	expected1 := 2 * time.Second
	if timing1.BackOffDelay() != expected1 {
		t.Errorf("backoff after SetBackoffParam: count=1, got %v, want %v", timing1.BackOffDelay(), expected1)
	}

	rl.Backoff(host)
	timing2 := rl.HostTimings()[host]
	expected2 := 6 * time.Second // 2 * 3^1
	if timing2.BackOffDelay() != expected2 {
		t.Errorf("backoff after SetBackoffParam: count=2, got %v, want %v", timing2.BackOffDelay(), expected2)
	}

	rl.Backoff(host)
	timing3 := rl.HostTimings()[host]
	expected3 := 18 * time.Second // 2 * 3^2
	if timing3.BackOffDelay() != expected3 {
		t.Errorf("backoff after SetBackoffParam: count=3, got %v, want %v", timing3.BackOffDelay(), expected3)
	}

	rl.Backoff(host)
	timing4 := rl.HostTimings()[host]
	expected4 := 54 * time.Second // 2 * 3^3
	if timing4.BackOffDelay() != expected4 {
		t.Errorf("backoff after SetBackoffParam: count=4, got %v, want %v", timing4.BackOffDelay(), expected4)
	}

	rl.Backoff(host)
	timing5 := rl.HostTimings()[host]
	expected5 := 60 * time.Second // capped at max
	if timing5.BackOffDelay() != expected5 {
		t.Errorf("backoff after SetBackoffParam: count=5 (capped), got %v, want %v", timing5.BackOffDelay(), expected5)
	}
}

func TestRateLimiter_ResolveDelay_ElapsedTime(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(100 * time.Millisecond)
	rl.SetJitter(0)
	rl.SetRandomSeed(42)
	host := "example.com"

	rl.MarkLastFetchAsNow(host)

	// Wait for delay to pass
	time.Sleep(150 * time.Millisecond)

	delay := rl.ResolveDelay(host)

	if delay != 0 {
		t.Errorf("ResolveDelay after elapsed time = %v, want 0", delay)
	}
}

func TestRateLimiter_ResolveDelay_CrawlDelayOverridesBase(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(100 * time.Millisecond)
	rl.SetJitter(0)
	rl.SetRandomSeed(42)
	host := "example.com"

	rl.SetCrawlDelay(host, 500*time.Millisecond)
	rl.MarkLastFetchAsNow(host)

	delay := rl.ResolveDelay(host)

	// Should use crawlDelay (500ms) instead of baseDelay (100ms)
	if delay < 490*time.Millisecond {
		t.Errorf("ResolveDelay = %v, want at least 490ms (crawlDelay should override)", delay)
	}
}

func TestRateLimiter_ResolveDelay_BackoffDelayTakesPrecedence(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(100 * time.Millisecond)
	rl.SetJitter(0)
	rl.SetRandomSeed(42)
	host := "example.com"

	rl.SetCrawlDelay(host, 200*time.Millisecond)

	rl.Backoff(host) // This sets backoffDelay to 1s (count=1)
	rl.MarkLastFetchAsNow(host)

	delay := rl.ResolveDelay(host)

	// Should use backoffDelay (1s) as it's the maximum
	if delay < 990*time.Millisecond {
		t.Errorf("ResolveDelay = %v, want at least 990ms (backoffDelay should take precedence)", delay)
	}
}

func TestRateLimiter_ResolveDelay_Jitter(t *testing.T) {
	host := "testhost.com"
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(1 * time.Second)
	rl.SetJitter(100 * time.Millisecond)
	rl.SetRandomSeed(42)

	tests := []struct {
		name string
		max  time.Duration
		want time.Duration
	}{
		{
			name: "positive base returns value within range",
			max:  100 * time.Millisecond,
			want: 100 * time.Millisecond,
		},
		{
			name: "zero base returns zero",
			max:  0,
			want: 0,
		},
		{
			name: "negative base returns zero",
			max:  -100 * time.Millisecond,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// set configured jitter base for this scenario and derive jitter value
			rl.SetJitter(tt.max)
			rl.MarkLastFetchAsNow(host)
			got := rl.ResolveDelay(host)

			// derive computed jitter by subtracting baseDelay (ResolveDelay returns base + jitter - elapsed)
			derived := got - rl.BaseDelay()

			const tolerance = 5 * time.Millisecond
			if tt.max <= 0 {
				// allow tiny negative/positive differences due to elapsed time
				if derived < -tolerance || derived > tolerance {
					t.Errorf("Jitter() = %v, want ~0", derived)
				}
			} else {
				if derived < 0 || derived > tt.max+tolerance {
					t.Errorf("Jitter() = %v, want value between 0 and %v", derived, tt.max)
				}
			}
		})
	}
}

func TestRateLimiter_ResolveDelay_JitterIsDeterministic(t *testing.T) {
	seed := int64(12345)
	rl1 := limiter.NewConcurrentRateLimiter()
	rl1.SetBaseDelay(1 * time.Second)
	rl1.SetJitter(100 * time.Millisecond)
	rl1.SetRandomSeed(seed)
	rl2 := limiter.NewConcurrentRateLimiter()
	rl2.SetBaseDelay(1 * time.Second)
	rl2.SetJitter(100 * time.Millisecond)
	rl2.SetRandomSeed(seed)

	host := "deterministic.example"

	// With same seed, ResolveDelay should produce the same jitter-derived result
	// Allow tiny timing differences by using a small tolerance
	const tolerance = 5 * time.Millisecond

	for i := 0; i < 10; i++ {
		rl1.MarkLastFetchAsNow(host)
		rl2.MarkLastFetchAsNow(host)

		d1 := rl1.ResolveDelay(host)
		d2 := rl2.ResolveDelay(host)

		if d1 < d2-tolerance || d1 > d2+tolerance {
			t.Errorf("ResolveDelay not deterministic: iteration %d, got %v and %v", i, d1, d2)
		}
	}
}

func TestRateLimiter_ResolveDelay_NoJitter(t *testing.T) {
	// Use zero jitter to isolate the test
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(100 * time.Millisecond)
	rl.SetJitter(0)
	rl.SetRandomSeed(42)
	host := "example.com"

	rl.MarkLastFetchAsNow(host)

	delay := rl.ResolveDelay(host)

	// Should be exactly baseDelay (no jitter with zero jitter config)
	if delay < 95*time.Millisecond || delay > 105*time.Millisecond {
		t.Errorf("ResolveDelay with zero jitter = %v, want approximately 100ms", delay)
	}
}

func TestRateLimiter_SetRNG(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(1 * time.Second)
	rl.SetJitter(100 * time.Millisecond)
	rl.SetRandomSeed(42)
	newRng := rand.New(rand.NewSource(99999))

	rl.SetRNG(newRng)

	if rl.RNG() != newRng {
		t.Error("SetRNG did not set rng correctly")
	}
}

func TestRateLimiter_ResolveDelay_WithJitter(t *testing.T) {
	// Use non-zero jitter
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(100 * time.Millisecond)
	rl.SetJitter(50 * time.Millisecond)
	rl.SetRandomSeed(42)
	host := "example.com"

	rl.MarkLastFetchAsNow(host)

	delay := rl.ResolveDelay(host)

	// Should be at least baseDelay, possibly more due to jitter
	if delay < 95*time.Millisecond {
		t.Errorf("ResolveDelay = %v, want at least ~100ms (base + jitter)", delay)
	}

	// With 50ms jitter, max should be around 150ms (allowing for elapsed time)
	if delay > 160*time.Millisecond {
		t.Errorf("ResolveDelay = %v, want at most ~160ms", delay)
	}
}

func TestRateLimiter_CompleteFlow(t *testing.T) {
	// Integration test for complete flow
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(100 * time.Millisecond)
	rl.SetJitter(10 * time.Millisecond)
	rl.SetRandomSeed(42)
	host := "api.example.com"

	// Step 1: first fetch - should get baseDelay
	rl.MarkLastFetchAsNow(host)

	firstDelay := rl.ResolveDelay(host)
	if firstDelay < 90*time.Millisecond {
		t.Errorf("First delay = %v, want at least 90ms", firstDelay)
	}

	// Step 2: Set crawl delay - should override base
	rl.SetCrawlDelay(host, 200*time.Millisecond)
	rl.MarkLastFetchAsNow(host)

	secondDelay := rl.ResolveDelay(host)
	if secondDelay < 190*time.Millisecond {
		t.Errorf("Second delay with crawlDelay = %v, want at least 190ms", secondDelay)
	}

	// Step 3: Trigger backoff - should take precedence
	rl.Backoff(host) // backoffDelay = 1s (count=1)
	rl.MarkLastFetchAsNow(host)

	thirdDelay := rl.ResolveDelay(host)
	if thirdDelay < 990*time.Millisecond {
		t.Errorf("Third delay with backoff = %v, want at least 990ms", thirdDelay)
	}

	// Step 4: Wait past delay - should return 0
	// Sleep longer than backoff delay (1s) to ensure elapsed time exceeds the delay
	time.Sleep(1200 * time.Millisecond)

	finalDelay := rl.ResolveDelay(host)
	if finalDelay != 0 {
		t.Errorf("Final delay after elapsed time = %v, want 0", finalDelay)
	}
}

func TestRateLimiter_BackoffExponentialGrowth(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetJitter(0) // Disable jitter for predictable testing
	rl.SetRandomSeed(42)
	host := "example.com"

	expectedDelays := []time.Duration{
		1 * time.Second,  // 1st backoff
		2 * time.Second,  // 2nd backoff
		4 * time.Second,  // 3rd backoff
		8 * time.Second,  // 4th backoff
		16 * time.Second, // 5th backoff
		30 * time.Second, // 6th backoff (capped)
		30 * time.Second, // 7th backoff (capped)
	}

	for i, expected := range expectedDelays {
		rl.Backoff(host)
		timing := rl.HostTimings()[host]
		actual := timing.BackOffDelay()
		if actual != expected {
			t.Errorf("Backoff %d: got %v, want %v", i+1, actual, expected)
		}
	}
}

func TestRateLimiter_ResetBackoffClearsState(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetJitter(0)
	host := "example.com"

	// Create backoff state
	for i := 0; i < 3; i++ {
		rl.Backoff(host)
	}

	// Verify state exists
	timingBefore := rl.HostTimings()[host]
	if timingBefore.BackoffCount() != 3 {
		t.Fatalf("setup: backoffCount = %d, want 3", timingBefore.BackoffCount())
	}
	if timingBefore.BackOffDelay() != 4*time.Second {
		t.Fatalf("setup: backoffDelay = %v, want 4s", timingBefore.BackOffDelay())
	}

	// Reset
	rl.ResetBackoff(host)

	// Verify state cleared
	timingAfter := rl.HostTimings()[host]
	if timingAfter.BackoffCount() != 0 {
		t.Errorf("After reset: backoffCount = %d, want 0", timingAfter.BackoffCount())
	}
	if timingAfter.BackOffDelay() != 0 {
		t.Errorf("After reset: backoffDelay = %v, want 0", timingAfter.BackOffDelay())
	}
}

func TestRateLimiter_BackoffWithJitter(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetJitter(50 * time.Millisecond) // Fixed jitter
	rl.SetRandomSeed(12345)
	host := "example.com"

	rl.Backoff(host)
	timing := rl.HostTimings()[host]

	// backoffDelay should be 1s + jitter (0-50ms)
	baseExpected := 1 * time.Second
	if timing.BackOffDelay() < baseExpected || timing.BackOffDelay() > baseExpected+60*time.Millisecond {
		t.Errorf("Backoff with jitter = %v, want between %v and %v", timing.BackOffDelay(), baseExpected, baseExpected+60*time.Millisecond)
	}
}

func TestRateLimiter_ResolveDelay_WithBackoff(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(100 * time.Millisecond)
	rl.SetJitter(0)
	rl.SetRandomSeed(42)
	host := "example.com"

	// Mark last fetch
	rl.MarkLastFetchAsNow(host)

	// Trigger backoff (should set delay to 1s)
	rl.Backoff(host)

	// Resolve should return backoff-based delay
	delay := rl.ResolveDelay(host)
	if delay < 990*time.Millisecond {
		t.Errorf("ResolveDelay after backoff = %v, want at least 990ms", delay)
	}
}

func TestRateLimiter_BackoffOnNewHost(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetJitter(0)
	host := "newhost.example"

	// Backoff on a host that doesn't exist yet
	rl.Backoff(host)

	timing := rl.HostTimings()[host]
	if timing.BackoffCount() != 1 {
		t.Errorf("backoffCount for new host = %d, want 1", timing.BackoffCount())
	}
	if timing.BackOffDelay() != 1*time.Second {
		t.Errorf("backoffDelay for new host = %v, want 1s", timing.BackOffDelay())
	}
	// lastFetchAt should be zero value since we didn't mark it
	if !timing.LastFetchAt().IsZero() {
		t.Errorf("lastFetchAt for new host should be zero, got %v", timing.LastFetchAt())
	}
}

func TestRateLimiter_Backoff_WithNilRng(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(1 * time.Second)
	rl.SetJitter(0)

	// Set r.rng to nil using SetRNG with a nil *rand.Rand
	var nilRng *rand.Rand = nil
	rl.SetRNG(nilRng)

	host := "example.com"

	// This should not panic and should initialize r.rng
	rl.Backoff(host)

	// After Backoff, r.rng should be initialized (non-nil)
	if rl.RNG() == nil {
		t.Error("rng should be initialized after Backoff with nil rng")
	}

	timing := rl.HostTimings()[host]
	if timing.BackoffCount() != 1 {
		t.Errorf("backoffCount = %d, want 1", timing.BackoffCount())
	}
	if timing.BackOffDelay() != 1*time.Second {
		t.Errorf("backoffDelay = %v, want 1s", timing.BackOffDelay())
	}
}

func TestRateLimiter_ResolveDelay_WithNilRng(t *testing.T) {
	rl := limiter.NewConcurrentRateLimiter()
	rl.SetBaseDelay(500 * time.Millisecond)
	rl.SetJitter(0)

	// Set r.rng to nil
	var nilRng *rand.Rand = nil
	rl.SetRNG(nilRng)

	host := "example.com"
	rl.MarkLastFetchAsNow(host)

	// This should not panic and should initialize r.rng
	delay := rl.ResolveDelay(host)

	// After ResolveDelay, r.rng should be initialized
	if rl.RNG() == nil {
		t.Error("rng should be initialized after ResolveDelay with nil rng")
	}

	// Should return baseDelay approximately (since no crawlDelay/backoff)
	if delay < 490*time.Millisecond || delay > 500*time.Millisecond {
		t.Errorf("ResolveDelay = %v, want approximately 500ms", delay)
	}
}
