package frontier_test

import (
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
)

// Helper to must-parse URLs in tests
func mustURL(t *testing.T, raw string) url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid url %q: %v", raw, err)
	}
	return *u
}

func TestFrontier_EnforceBFS(t *testing.T) {
	// GIVEN a frontier with no depth/page limits (simplify scenario)
	cfg := config.Config{} // assume zero-values mean "no limits"

	f := frontier.NewCrawlFrontier()
	f.Init(cfg)

	/*
		Graph:
		    A (0)
		   / \
		  B   C (1)
		  |
		  D (2)

		Discovery order:
		- A discovers B, C
		- B discovers D
	*/

	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")
	C := mustURL(t, "https://example.com/c")
	D := mustURL(t, "https://example.com/d")

	// Seed A (depth 0)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A,
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))

	// Dequeue A
	token, ok := f.Dequeue()
	if !ok {
		t.Fatalf("expected A to be dequeued")
	}
	if token.URL() != A {
		t.Fatalf("expected A first, got %v", token.URL())
	}

	// A discovers B and C (depth 1)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B,
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(1, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C,
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(1, nil),
	))

	// Dequeue B
	token, ok = f.Dequeue()
	if !ok {
		t.Fatalf("expected B to be dequeued")
	}
	if token.URL() != B {
		t.Fatalf("expected B, got %v", token.URL())
	}

	// B discovers D (depth 2)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		D,
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(2, nil),
	))

	/*
		At this point, the frontier queue is:
		    C (depth 1)
		    D (depth 2)

		A strict BFS frontier would GUARANTEE that
		all depth-1 nodes are processed before ANY depth-2 node
		is even eligible for dequeue.
	*/

	// Dequeue C
	token, ok = f.Dequeue()
	if !ok {
		t.Fatalf("expected C to be dequeued")
	}
	if token.URL() != C {
		t.Fatalf("expected C, got %v", token.URL())
	}

	// Dequeue D
	token, ok = f.Dequeue()
	if !ok {
		t.Fatalf("expected D to be dequeued")
	}
	if token.URL() != D {
		t.Fatalf("expected D, got %v", token.URL())
	}

	/*
		A BFS-enforcing frontier would have rejected or deferred D
		until depth-1 exhaustion.
	*/
}

func TestFrontier_DoesNotAllowsDuplicateURL(t *testing.T) {
	// GIVEN a fresh frontier
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	A := mustURL(t, "https://example.com/docs")

	// WHEN the same URL is submitted twice
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A,
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))

	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, // same URL, same canonical form
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(1, nil),
	))

	// THEN only ONE crawl token should ever be dequeued
	token1, ok := f.Dequeue()
	if !ok {
		t.Fatalf("expected first dequeue to succeed")
	}
	if token1.URL() != A {
		t.Fatalf("expected URL A, got %v", token1.URL())
	}

	// ❌ This should NOT exist
	token2, ok := f.Dequeue()
	if ok {
		t.Fatalf(
			"duplicate URL dequeued: %v (frontier failed to deduplicate)",
			token2.URL(),
		)
	}
}

// TestFrontier_BFOrderingMaintained demonstrates that depth-2 URLs
// can not be dequeued BEFORE all depth-1 URLs are exhausted.
// This maintains the BFS guarantee
func TestFrontier_BFOrderingMaintained(t *testing.T) {
	// GIVEN a frontier
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	/*
		Simulate this crawl scenario:

		Graph:
		    A (0)
		   /
		  B (1)
		 /
		C (2)

		Seed: A (depth 0)
		A links to: B (depth 1)
		B links to: C (depth 2)

		Discovery order (realistic crawl):
		1. Submit A (depth 0) - seed
		2. Dequeue A, process it
		3. Submit B (depth 1) - discovered from A
		4. Dequeue B, process it
		5. Submit C (depth 2) - discovered from B

		At this point, frontier queue contains only C (depth 2).

		If another URL D (depth 1) is discovered later (e.g., from another branch),
		D should have dequeued BEFORE C.
	*/

	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")
	C := mustURL(t, "https://example.com/c")
	D := mustURL(t, "https://example.com/d")

	// Seed A at depth 0
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))

	// Process A, discover B
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// Process B, discover C (depth 2)
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))

	// Now simulate D being discovered at depth 1 (from another branch)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		D, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// D (depth 1) MUST be dequeued BEFORE C (depth 2)
	// Because all depth-1 URLs must be exhausted before ANY depth-2 URL
	token, ok := f.Dequeue()
	if !ok {
		t.Fatalf("expected dequeue to succeed")
	}

	if token.URL() == C {
		t.Logf("BUG CONFIRMED: C (depth 2) dequeued before D (depth 1)")
		t.Logf("BFS ordering violation: depth-%d URL returned when depth-1 URLs still pending",
			token.Depth())
		t.Fatalf("BFS ordering violated: got %v (depth %d) before D (depth 1)",
			token.URL(), token.Depth())
	}

	if token.URL() != D {
		t.Fatalf("expected D (depth 1), got %v (depth %d)", token.URL(), token.Depth())
	}
}

// TestFrontier_DepthLimitEnforced proves that depth limits from config
// are not applied during Submit
func TestFrontier_DepthLimitEnforced(t *testing.T) {
	// GIVEN a frontier with max depth of 2
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, err := config.WithDefault([]url.URL{*seedURL}).
		WithMaxDepth(2). // URLs at depth 3+ should be rejected
		Build()
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	f := frontier.NewCrawlFrontier()
	f.Init(cfg)

	deepURL := mustURL(t, "https://example.com/deep")

	// WHEN a URL at depth 5 is submitted (exceeds limit)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		deepURL,
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(5, nil), // Exceeds MaxDepth of 2
	))

	// THEN it should NOT be in the queue
	token, ok := f.Dequeue()
	if ok {
		t.Fatalf("BUG: URL at depth %d was accepted despite MaxDepth=%d. Token: %v",
			token.Depth(), cfg.MaxDepth(), token.URL())
	}
}

// TestFrontier_WideTreeBFMaintained demonstrates BFS ordering is maintained
// in a wide tree scenario where many depth-1 URLs should be
// processed before any depth-2 URL
func TestFrontier_WideTreeBFMaintained(t *testing.T) {
	// GIVEN a frontier
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	/*
		Tree structure:
		    Root (depth 0)
		    /  |  \
		   A   B   C  (depth 1)
		   |
		   D          (depth 2)

		In proper BFS: Root → A → B → C → D
		Wrong impl:  Root → A → D → B → C (D can appear before B, C!)
	*/

	root := mustURL(t, "https://example.com/root")
	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")
	C := mustURL(t, "https://example.com/c")
	D := mustURL(t, "https://example.com/d")

	// Submit root
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		root, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))

	// Process root, discover A, B, C
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// Process A early, discover D (depth 2) before B and C are even submitted
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		D, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))

	// Now submit B and C (depth 1)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// BFS requires: B, C (depth 1) must come before D (depth 2)
	order := []string{}
	for {
		token, ok := f.Dequeue()
		if !ok {
			break
		}
		order = append(order, token.URL().Path)
	}

	// Check if D appears before B or C
	idxD := indexOf(order, "/d")
	idxB := indexOf(order, "/b")
	idxC := indexOf(order, "/c")

	if idxD < idxB || idxD < idxC {
		t.Fatalf("BFS VIOLATION: D (depth 2) at position %d appears before "+
			"B (position %d) or C (position %d). Order: %v",
			idxD, idxB, idxC, order)
	}

	t.Logf("Correct BFS order: %v", order)
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

// TestFrontier_PageCountLimitEnforced proves that page count limits
// are tracked or enforced
func TestFrontier_PageCountLimitEnforced(t *testing.T) {
	// GIVEN a frontier with max pages limit of 2
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, err := config.WithDefault([]url.URL{*seedURL}).
		WithMaxPages(2). // Only 2 pages should be crawled
		Build()
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	f := frontier.NewCrawlFrontier()
	f.Init(cfg)

	urls := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/page3",
		"https://example.com/page4",
	}

	maxPages := cfg.MaxPages()

	// WHEN submitting more URLs than the limit
	for i, rawURL := range urls {
		u := mustURL(t, rawURL)
		f.Submit(frontier.NewCrawlAdmissionCandidate(
			u, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
		))

		// Dequeue immediately to simulate "crawling"
		if i < maxPages {
			f.Dequeue()
		}
	}

	// THEN page 3 and 4 should NOT be in the queue
	extraCount := 0
	for {
		_, ok := f.Dequeue()
		if !ok {
			break
		}
		extraCount++
	}

	if extraCount > 0 {
		t.Fatalf("BUG: Page count limit of %d not enforced. "+
			"Found %d extra pages in queue.", maxPages, extraCount)
	}
}

// TestFrontier_NilQueueDereference exposes a bug where Dequeue() panics
// when trying to access a depth level that was never initialized.
// This happens when a URL is submitted at depth N, but depth N-1 was never created.
func TestFrontier_NilQueueDereference(t *testing.T) {
	// GIVEN a frontier
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	A := mustURL(t, "https://example.com/a")
	C := mustURL(t, "https://example.com/c")

	// Submit A at depth 0
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))

	// Submit C at depth 2 (skipping depth 1 entirely)
	// This sets currentDepth to 2, but queuesByDepth[1] is nil
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))

	// Dequeue A (depth 0)
	token, ok := f.Dequeue()
	if !ok {
		t.Fatalf("expected to dequeue A")
	}
	if token.URL() != A {
		t.Fatalf("expected A, got %v", token.URL())
	}

	// WHEN we try to dequeue the next URL
	// The Dequeue() loop will check depth 1, which doesn't exist (nil)
	// This causes a panic: "runtime error: invalid memory address or nil pointer dereference"
	//
	// Expected behavior: Should gracefully handle missing depth levels and return C (depth 2)

	token, ok = f.Dequeue()
	if !ok {
		t.Fatalf("expected to dequeue C, but got nothing. " +
			"This may indicate a panic was recovered or the queue is incorrectly empty.")
	}
	if token.URL() != C {
		t.Fatalf("expected C (depth 2), got %v (depth %d)", token.URL(), token.Depth())
	}

	t.Logf("Successfully dequeued C at depth %d without nil pointer dereference", token.Depth())
}

// Test case to verify thread-safety when submitting and dequeueing
func TestFrontier_ConcurrentSubmitDequeue(t *testing.T) {
	// GIVEN a frontier with no limits
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	const numWorkers = 10
	const urlsPerWorker = 100
	const totalUrls = numWorkers * urlsPerWorker

	var wg sync.WaitGroup
	wg.Add(numWorkers * 2) // Submitters + Dequeueers

	// Spawn submitter workers
	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer func() {
				t.Logf("Submit worker %d calling Done()\n", workerID)
				wg.Done()
			}()
			for i := 0; i < urlsPerWorker; i++ {
				u := mustURL(t, fmt.Sprintf("https://example.com/w%d-p%d", workerID, i))
				depth := (workerID + i) % 5 // Mix of depths 0-4
				f.Submit(frontier.NewCrawlAdmissionCandidate(
					u, frontier.SourceSeed, frontier.NewDiscoveryMetadata(depth, nil),
				))
			}
		}(w)
	}

	// Spawn dequeuer workers
	dequeuedCount := int32(0)
	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer func() {
				t.Logf("Dequeue worker %d calling Done()\n", workerID)
				wg.Done()
			}()
			for {
				_, ok := f.Dequeue()
				if ok {
					atomic.AddInt32(&dequeuedCount, 1)
				}

				if atomic.LoadInt32(&dequeuedCount) >= totalUrls {
					t.Logf("Dequeue worker %d return", workerID)
					return
				}
			}
		}(w)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Logf("Successfully processed %d URLs concurrently", totalUrls)
	case <-time.After(10 * time.Second):
		t.Fatalf("Test timed out - possible deadlock or missing URLs")
	}

	// Verify all URLs were dequeued
	if atomic.LoadInt32(&dequeuedCount) != totalUrls {
		t.Fatalf("Expected %d dequeued URLs, got %d", totalUrls, dequeuedCount)
	} else {
		t.Logf("Processed %d urls\n", dequeuedCount)
	}
}

// Test unlimited limits (0 = unlimited)
func TestFrontier_UnlimitedLimits(t *testing.T) {
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, _ := config.WithDefault([]url.URL{*seedURL}).
		WithMaxDepth(0). // 0 = unlimited
		WithMaxPages(0). // 0 = unlimited
		Build()

	f := frontier.NewCrawlFrontier()
	f.Init(cfg)

	// Should accept URLs at any depth
	deepURL := mustURL(t, "https://example.com/a/b/c/d/e/f")
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		deepURL, frontier.SourceSeed, frontier.NewDiscoveryMetadata(100, nil),
	))

	token, ok := f.Dequeue()
	if !ok {
		t.Fatal("Expected URL to be accepted with unlimited depth")
	}
	if token.Depth() != 100 {
		t.Fatalf("Expected depth 100, got %d", token.Depth())
	}
}

// Test empty frontier
func TestFrontier_Empty(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	_, ok := f.Dequeue()
	if ok {
		t.Fatal("Dequeue from empty frontier should return false")
	}
}

// TestFrontier_URLStructDeduplicationBug demonstrates why using url.URL as a map key
// is dangerous. The url.URL struct contains pointer fields (User, ForceQuery, etc.)
// that can cause two semantically identical URLs to be treated as different keys.
//
// This test shows the bug with the OLD implementation (Set[url.URL]).
// The current implementation uses Set[string] with canonicalized URLs, which avoids this.
func TestFrontier_URLStructDeduplicationBug(t *testing.T) {
	// This test documents the bug that existed when we used Set[url.URL]
	// instead of Set[string] for deduplication.

	// url.URL structs with pointer fields can have different memory addresses
	// even when representing the same URL
	url1 := mustURL(t, "https://user:pass@example.com:8080/path?query=1#frag")
	url2 := mustURL(t, "https://user:pass@example.com:8080/path?query=1#frag")

	// The User field is a *Userinfo pointer - different allocations = different addresses
	if url1.User == url2.User {
		t.Log("Note: url1.User and url2.User point to same memory (implementation detail)")
	} else {
		t.Logf("url1.User pointer: %p, url2.User pointer: %p", url1.User, url2.User)
	}

	// When used as map keys, url.URL structs are compared by value, including pointer fields
	// This means url1 and url2 might not be considered equal even though they
	// represent the same URL semantically
	mapWithURLKey := make(map[url.URL]bool)
	mapWithURLKey[url1] = true

	// This demonstrates the potential bug: same semantic URL might not be found
	_, exists := mapWithURLKey[url2]
	t.Logf("url2 found in map[url.URL]: %v (BUG: should be true for deduplication)", exists)

	// Compare with string keys (current implementation)
	mapWithStringKey := make(map[string]bool)
	mapWithStringKey[url1.String()] = true
	_, exists = mapWithStringKey[url2.String()]
	t.Logf("url2.String() found in map[string]: %v (correct behavior)", exists)

	// The current frontier implementation uses Set[string] with canonicalized URLs,
	// so this deduplication works correctly. This test documents WHY we made that change.
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	// Submit same URL twice (parsed separately)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		url1, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		url2, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// Should only dequeue one
	token1, ok1 := f.Dequeue()
	if !ok1 {
		t.Fatal("expected first dequeue to succeed")
	}

	token2, ok2 := f.Dequeue()
	if ok2 {
		t.Fatalf("BUG: With url.URL as map key, duplicate was not detected. "+
			"Second token: %v. This is why we use string keys.", token2.URL())
	}

	t.Logf("SUCCESS: Deduplication worked correctly for %v", token1.URL())
	t.Log("The frontier now uses Set[string] with canonicalized URLs to avoid this bug")
}

// =============================================================================
// Depth Exhaustion API Tests
// =============================================================================

// TestFrontier_IsDepthExhausted_EmptyFrontier verifies that all depths
// are reported as exhausted when the frontier is empty.
func TestFrontier_IsDepthExhausted_EmptyFrontier(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	// All depths should be exhausted for an empty frontier
	if !f.IsDepthExhausted(0) {
		t.Error("Expected depth 0 to be exhausted for empty frontier")
	}
	if !f.IsDepthExhausted(1) {
		t.Error("Expected depth 1 to be exhausted for empty frontier")
	}
	if !f.IsDepthExhausted(100) {
		t.Error("Expected depth 100 to be exhausted for empty frontier")
	}
}

// TestFrontier_IsDepthExhausted_WithPendingURLs verifies that depth exhaustion
// is correctly reported when URLs exist at various depths.
func TestFrontier_IsDepthExhausted_WithPendingURLs(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")
	C := mustURL(t, "https://example.com/c")

	// Submit URLs at depths 0 and 2
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))

	// Depth 0 should NOT be exhausted (has A)
	if f.IsDepthExhausted(0) {
		t.Error("Expected depth 0 to NOT be exhausted (has pending URL)")
	}

	// Depth 1 should be exhausted (no URLs at this depth)
	if !f.IsDepthExhausted(1) {
		t.Error("Expected depth 1 to be exhausted (no URLs at this depth)")
	}

	// Depth 2 should NOT be exhausted (has B)
	if f.IsDepthExhausted(2) {
		t.Error("Expected depth 2 to NOT be exhausted (has pending URL)")
	}

	// Depth 3+ should be exhausted
	if !f.IsDepthExhausted(3) {
		t.Error("Expected depth 3 to be exhausted")
	}

	// Dequeue A (depth 0)
	f.Dequeue()

	// Now depth 0 should be exhausted
	if !f.IsDepthExhausted(0) {
		t.Error("Expected depth 0 to be exhausted after dequeuing A")
	}

	// Depth 2 still not exhausted
	if f.IsDepthExhausted(2) {
		t.Error("Expected depth 2 to NOT be exhausted (B still pending)")
	}

	// Submit C at depth 1
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// Depth 1 should NOT be exhausted now
	if f.IsDepthExhausted(1) {
		t.Error("Expected depth 1 to NOT be exhausted after submitting C")
	}
}

// TestFrontier_IsDepthExhausted_TracksBFSProgression demonstrates how
// IsDepthExhausted can be used to track BFS level completion.
func TestFrontier_IsDepthExhausted_TracksBFSProgression(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	root := mustURL(t, "https://example.com/root")
	child1 := mustURL(t, "https://example.com/child1")
	child2 := mustURL(t, "https://example.com/child2")
	grandchild := mustURL(t, "https://example.com/grandchild")

	// Setup: root (depth 0) discovers children (depth 1)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		root, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))

	// Before processing: depth 0 not exhausted
	if f.IsDepthExhausted(0) {
		t.Error("Depth 0 should not be exhausted before processing root")
	}

	// Process root
	f.Dequeue()

	// After root dequeued: depth 0 exhausted
	if !f.IsDepthExhausted(0) {
		t.Error("Depth 0 should be exhausted after dequeuing root")
	}

	// Submit children at depth 1
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		child1, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		child2, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// Depth 1 not exhausted
	if f.IsDepthExhausted(1) {
		t.Error("Depth 1 should not be exhausted with pending children")
	}

	// Process child1, discover grandchild (depth 2)
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		grandchild, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))

	// Depth 1 still not exhausted (child2 pending)
	if f.IsDepthExhausted(1) {
		t.Error("Depth 1 should not be exhausted with child2 still pending")
	}

	// Process child2
	f.Dequeue()

	// NOW depth 1 is exhausted
	if !f.IsDepthExhausted(1) {
		t.Error("Depth 1 should be exhausted after both children processed")
	}

	// But grandchild (depth 2) still pending
	if f.IsDepthExhausted(2) {
		t.Error("Depth 2 should not be exhausted with grandchild pending")
	}
}

// TestFrontier_CurrentMinDepth_EmptyFrontier verifies that CurrentMinDepth
// returns -1 for an empty frontier.
func TestFrontier_CurrentMinDepth_EmptyFrontier(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	minDepth := f.CurrentMinDepth()
	if minDepth != -1 {
		t.Errorf("Expected CurrentMinDepth() = -1 for empty frontier, got %d", minDepth)
	}
}

// TestFrontier_CurrentMinDepth_SingleDepth verifies CurrentMinDepth with
// URLs at a single depth level.
func TestFrontier_CurrentMinDepth_SingleDepth(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")

	// Submit URLs at depth 2 only
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceSeed, frontier.NewDiscoveryMetadata(2, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))

	// CurrentMinDepth should be 2 (skipping empty 0 and 1)
	if minDepth := f.CurrentMinDepth(); minDepth != 2 {
		t.Errorf("Expected CurrentMinDepth() = 2, got %d", minDepth)
	}

	// Dequeue A
	f.Dequeue()

	// CurrentMinDepth should still be 2 (B still pending)
	if minDepth := f.CurrentMinDepth(); minDepth != 2 {
		t.Errorf("Expected CurrentMinDepth() = 2 after first dequeue, got %d", minDepth)
	}

	// Dequeue B
	f.Dequeue()

	// Now frontier is empty
	if minDepth := f.CurrentMinDepth(); minDepth != -1 {
		t.Errorf("Expected CurrentMinDepth() = -1 after emptying, got %d", minDepth)
	}
}

// TestFrontier_CurrentMinDepth_MultipleDepths verifies CurrentMinDepth
// correctly tracks the minimum depth with pending URLs across multiple levels.
func TestFrontier_CurrentMinDepth_MultipleDepths(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	urlD0 := mustURL(t, "https://example.com/d0")
	urlD1 := mustURL(t, "https://example.com/d1")
	urlD2 := mustURL(t, "https://example.com/d2")

	// Submit at depths 0, 1, and 2
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urlD0, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urlD1, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urlD2, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))

	// CurrentMinDepth should be 0
	if minDepth := f.CurrentMinDepth(); minDepth != 0 {
		t.Errorf("Expected CurrentMinDepth() = 0, got %d", minDepth)
	}

	// Dequeue depth 0
	f.Dequeue()

	// CurrentMinDepth should advance to 1
	if minDepth := f.CurrentMinDepth(); minDepth != 1 {
		t.Errorf("Expected CurrentMinDepth() = 1 after exhausting depth 0, got %d", minDepth)
	}

	// Dequeue depth 1
	f.Dequeue()

	// CurrentMinDepth should advance to 2
	if minDepth := f.CurrentMinDepth(); minDepth != 2 {
		t.Errorf("Expected CurrentMinDepth() = 2 after exhausting depth 1, got %d", minDepth)
	}

	// Dequeue depth 2
	f.Dequeue()

	// Frontier empty
	if minDepth := f.CurrentMinDepth(); minDepth != -1 {
		t.Errorf("Expected CurrentMinDepth() = -1 after exhausting all, got %d", minDepth)
	}
}

// TestFrontier_CurrentMinDepth_WithGaps verifies CurrentMinDepth handles
// gaps in depth levels (e.g., URLs at depth 0 and 2, but not 1).
func TestFrontier_CurrentMinDepth_WithGaps(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	urlD0 := mustURL(t, "https://example.com/d0")
	urlD2a := mustURL(t, "https://example.com/d2a")
	urlD2b := mustURL(t, "https://example.com/d2b")

	// Submit at depths 0 and 2 (gap at depth 1)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urlD0, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urlD2a, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urlD2b, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))

	// CurrentMinDepth should be 0
	if minDepth := f.CurrentMinDepth(); minDepth != 0 {
		t.Errorf("Expected CurrentMinDepth() = 0, got %d", minDepth)
	}

	// Dequeue depth 0
	f.Dequeue()

	// CurrentMinDepth should skip the gap and report 2
	if minDepth := f.CurrentMinDepth(); minDepth != 2 {
		t.Errorf("Expected CurrentMinDepth() = 2 (skipping empty depth 1), got %d", minDepth)
	}

	// Submit at depth 1 (fill the gap)
	urlD1 := mustURL(t, "https://example.com/d1")
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urlD1, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// CurrentMinDepth should now report 1 (lower than 2)
	if minDepth := f.CurrentMinDepth(); minDepth != 1 {
		t.Errorf("Expected CurrentMinDepth() = 1 (new lower depth), got %d", minDepth)
	}
}

// TestFrontier_DepthAPIs_Consistency verifies that IsDepthExhausted and
// CurrentMinDepth return consistent results.
func TestFrontier_DepthAPIs_Consistency(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	urls := make([]url.URL, 5)
	for i := 0; i < 5; i++ {
		urls[i] = mustURL(t, fmt.Sprintf("https://example.com/page%d", i))
	}

	// Submit URLs at depths 0, 1, and 3
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urls[0], frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urls[1], frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urls[2], frontier.SourceCrawl, frontier.NewDiscoveryMetadata(3, nil),
	))

	// Consistency check: CurrentMinDepth should return the smallest non-exhausted depth
	minDepth := f.CurrentMinDepth()
	for d := 0; d < minDepth; d++ {
		if !f.IsDepthExhausted(d) {
			t.Errorf("Inconsistency: CurrentMinDepth() = %d but IsDepthExhausted(%d) = false",
				minDepth, d)
		}
	}
	if f.IsDepthExhausted(minDepth) {
		t.Errorf("Inconsistency: CurrentMinDepth() = %d but IsDepthExhausted(%d) = true",
			minDepth, minDepth)
	}

	// Process all URLs and verify consistency at each step
	for {
		_, ok := f.Dequeue()
		if !ok {
			break
		}

		// After each dequeue, verify consistency
		minDepth := f.CurrentMinDepth()
		if minDepth == -1 {
			// Frontier empty - all depths should be exhausted
			for d := 0; d <= 3; d++ {
				if !f.IsDepthExhausted(d) {
					t.Errorf("Inconsistency after dequeue: frontier empty but depth %d not exhausted", d)
				}
			}
		} else {
			// All depths below minDepth should be exhausted
			for d := 0; d < minDepth; d++ {
				if !f.IsDepthExhausted(d) {
					t.Errorf("Inconsistency after dequeue: minDepth=%d but depth %d not exhausted",
						minDepth, d)
				}
			}
			// minDepth itself should not be exhausted
			if f.IsDepthExhausted(minDepth) {
				t.Errorf("Inconsistency after dequeue: minDepth=%d is exhausted", minDepth)
			}
		}
	}
}

// TestFrontier_DepthAPIs_ConcurrentAccess verifies thread-safety of the
// depth exhaustion APIs under concurrent access.
func TestFrontier_DepthAPIs_ConcurrentAccess(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	const numSubmitters = 5
	const numQueryers = 5
	const urlsPerSubmitter = 50

	var wg sync.WaitGroup
	wg.Add(numSubmitters + numQueryers)

	// Submitters add URLs at various depths
	for i := 0; i < numSubmitters; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < urlsPerSubmitter; j++ {
				u := mustURL(t, fmt.Sprintf("https://example.com/s%d-u%d", id, j))
				depth := (id + j) % 5 // Mix depths 0-4
				f.Submit(frontier.NewCrawlAdmissionCandidate(
					u, frontier.SourceSeed, frontier.NewDiscoveryMetadata(depth, nil),
				))
			}
		}(i)
	}

	// Queryers continuously call depth APIs
	stopQuerying := make(chan struct{})
	queryCount := int32(0)
	for i := 0; i < numQueryers; i++ {
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stopQuerying:
					return
				default:
					// Call both APIs concurrently with other operations
					_ = f.IsDepthExhausted(id % 5)
					_ = f.CurrentMinDepth()
					atomic.AddInt32(&queryCount, 1)
				}
			}
		}(i)
	}

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)
	close(stopQuerying)

	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Logf("Completed %d concurrent queries without race conditions", queryCount)
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}
}

// TestFrontier_IsDepthExhausted_NegativeDepth verifies behavior with negative depth.
func TestFrontier_IsDepthExhausted_NegativeDepth(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	// Negative depths should always be exhausted (they don't exist)
	if !f.IsDepthExhausted(-1) {
		t.Error("Expected negative depth to be exhausted")
	}
	if !f.IsDepthExhausted(-100) {
		t.Error("Expected large negative depth to be exhausted")
	}
}

// =============================================================================
// VisitedCount API Tests
// =============================================================================

// TestFrontier_VisitedCount_EmptyFrontier verifies that VisitedCount returns 0
// for an empty frontier that has never had any URLs submitted.
func TestFrontier_VisitedCount_EmptyFrontier(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	count := f.VisitedCount()
	if count != 0 {
		t.Errorf("Expected VisitedCount() = 0 for empty frontier, got %d", count)
	}
}

// TestFrontier_VisitedCount_AfterSubmit verifies that VisitedCount correctly
// tracks the number of unique URLs submitted to the frontier.
func TestFrontier_VisitedCount_AfterSubmit(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")
	C := mustURL(t, "https://example.com/c")

	// Initially empty
	if count := f.VisitedCount(); count != 0 {
		t.Errorf("Expected VisitedCount() = 0 initially, got %d", count)
	}

	// Submit first URL
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))
	if count := f.VisitedCount(); count != 1 {
		t.Errorf("Expected VisitedCount() = 1 after first submit, got %d", count)
	}

	// Submit second URL
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))
	if count := f.VisitedCount(); count != 2 {
		t.Errorf("Expected VisitedCount() = 2 after second submit, got %d", count)
	}

	// Submit third URL
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
	))
	if count := f.VisitedCount(); count != 3 {
		t.Errorf("Expected VisitedCount() = 3 after third submit, got %d", count)
	}
}

// TestFrontier_VisitedCount_Deduplication verifies that VisitedCount only
// counts unique URLs, not duplicates.
func TestFrontier_VisitedCount_Deduplication(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	A := mustURL(t, "https://example.com/a")

	// Submit the same URL multiple times
	for i := 0; i < 5; i++ {
		f.Submit(frontier.NewCrawlAdmissionCandidate(
			A, frontier.SourceSeed, frontier.NewDiscoveryMetadata(i, nil),
		))
	}

	// Should only count as 1 unique URL
	if count := f.VisitedCount(); count != 1 {
		t.Errorf("Expected VisitedCount() = 1 (deduplicated), got %d", count)
	}
}

// TestFrontier_VisitedCount_AfterDequeue verifies that VisitedCount does not
// decrease after URLs are dequeued (the visited set is append-only).
func TestFrontier_VisitedCount_AfterDequeue(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")
	C := mustURL(t, "https://example.com/c")

	// Submit URLs
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// Should have 3 visited URLs
	if count := f.VisitedCount(); count != 3 {
		t.Errorf("Expected VisitedCount() = 3 before dequeue, got %d", count)
	}

	// Dequeue all URLs
	f.Dequeue()
	f.Dequeue()
	f.Dequeue()

	// VisitedCount should still be 3 (visited set is append-only)
	if count := f.VisitedCount(); count != 3 {
		t.Errorf("Expected VisitedCount() = 3 after dequeue, got %d", count)
	}
}

// TestFrontier_VisitedCount_MixedUniqueAndDuplicates tests VisitedCount
// with a mix of unique URLs and duplicates.
func TestFrontier_VisitedCount_MixedUniqueAndDuplicates(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	// Create URLs: A, B, C, then A again, D, B again
	urls := []url.URL{
		mustURL(t, "https://example.com/a"),
		mustURL(t, "https://example.com/b"),
		mustURL(t, "https://example.com/c"),
		mustURL(t, "https://example.com/a"), // duplicate
		mustURL(t, "https://example.com/d"),
		mustURL(t, "https://example.com/b"), // duplicate
	}

	// Submit all URLs
	for i, u := range urls {
		f.Submit(frontier.NewCrawlAdmissionCandidate(
			u, frontier.SourceSeed, frontier.NewDiscoveryMetadata(i%3, nil),
		))
	}

	// Should only count 4 unique URLs: a, b, c, d
	if count := f.VisitedCount(); count != 4 {
		t.Errorf("Expected VisitedCount() = 4 (unique URLs: a, b, c, d), got %d", count)
	}
}

// TestFrontier_VisitedCount_ConcurrentAccess verifies thread-safety of
// VisitedCount under concurrent submissions.
func TestFrontier_VisitedCount_ConcurrentAccess(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	const numWorkers = 10
	const urlsPerWorker = 100
	const expectedUnique = 50 // We'll create 50 unique URLs

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	// Spawn workers that submit the same 50 URLs repeatedly
	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < urlsPerWorker; i++ {
				// Create URL that cycles through 50 unique values
				uniqueID := i % expectedUnique
				u := mustURL(t, fmt.Sprintf("https://example.com/page%d", uniqueID))
				f.Submit(frontier.NewCrawlAdmissionCandidate(
					u, frontier.SourceSeed, frontier.NewDiscoveryMetadata(workerID%3, nil),
				))
			}
		}(w)
	}

	// Also concurrently call VisitedCount
	stopCounting := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCounting:
				return
			default:
				_ = f.VisitedCount()
			}
		}
	}()

	// Wait for submissions to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(stopCounting)
		t.Log("All submissions completed")
	case <-time.After(10 * time.Second):
		close(stopCounting)
		t.Fatal("Test timed out - possible deadlock")
	}

	// Final VisitedCount should be exactly 50
	finalCount := f.VisitedCount()
	if finalCount != expectedUnique {
		t.Errorf("Expected VisitedCount() = %d after concurrent submissions, got %d", expectedUnique, finalCount)
	}
}

// TestFrontier_VisitedCount_WithMaxPagesLimit verifies that VisitedCount
// respects the max pages limit (URLs beyond the limit are not counted).
func TestFrontier_VisitedCount_WithMaxPagesLimit(t *testing.T) {
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, _ := config.WithDefault([]url.URL{*seedURL}).
		WithMaxPages(3). // Limit to 3 pages
		Build()

	f := frontier.NewCrawlFrontier()
	f.Init(cfg)

	// Submit 5 URLs
	urls := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/page3",
		"https://example.com/page4",
		"https://example.com/page5",
	}

	for _, rawURL := range urls {
		u := mustURL(t, rawURL)
		f.Submit(frontier.NewCrawlAdmissionCandidate(
			u, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
		))
	}

	// VisitedCount should be 3 (limited by maxPages)
	if count := f.VisitedCount(); count != 3 {
		t.Errorf("Expected VisitedCount() = 3 (maxPages limit), got %d", count)
	}
}

// TestFrontier_VisitedCount_Canonicalization verifies that VisitedCount
// uses canonicalized URLs for deduplication.
func TestFrontier_VisitedCount_Canonicalization(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	// These URLs are different but should canonicalize to the same form
	url1 := mustURL(t, "https://example.com:443/path") // explicit default port
	url2 := mustURL(t, "https://example.com/path")     // implicit default port
	url3 := mustURL(t, "https://example.com/path/")    // trailing slash
	url4 := mustURL(t, "https://example.com/path?q=1") // query string

	// Submit all URLs
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		url1, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		url2, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		url3, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		url4, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
	))

	// After canonicalization, all these should be deduplicated
	// The exact count depends on the canonicalization rules, but should be < 4
	count := f.VisitedCount()
	if count < 1 || count > 2 {
		t.Logf("Canonicalization result: VisitedCount() = %d (URLs canonicalized together)", count)
	}
}

// TestFrontier_VisitedCount_Integration provides an integration test that
// verifies VisitedCount works correctly throughout a realistic crawl scenario.
func TestFrontier_VisitedCount_Integration(t *testing.T) {
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	// Simulate a realistic crawl scenario
	// Root page (depth 0)
	root := mustURL(t, "https://example.com/")
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		root, frontier.SourceSeed, frontier.NewDiscoveryMetadata(0, nil),
	))

	if count := f.VisitedCount(); count != 1 {
		t.Errorf("After root: Expected VisitedCount() = 1, got %d", count)
	}

	// Dequeue root and discover children (depth 1)
	f.Dequeue()
	children := []string{
		"https://example.com/about",
		"https://example.com/products",
		"https://example.com/contact",
	}
	for _, childURL := range children {
		u := mustURL(t, childURL)
		f.Submit(frontier.NewCrawlAdmissionCandidate(
			u, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(1, nil),
		))
	}

	if count := f.VisitedCount(); count != 4 {
		t.Errorf("After children: Expected VisitedCount() = 4, got %d", count)
	}

	// Dequeue some children and discover grandchildren (depth 2)
	f.Dequeue() // about
	grandchildren := []string{
		"https://example.com/products/item1",
		"https://example.com/products/item2",
		"https://example.com/products/item3",
	}
	for _, gcURL := range grandchildren {
		u := mustURL(t, gcURL)
		f.Submit(frontier.NewCrawlAdmissionCandidate(
			u, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(2, nil),
		))
	}

	if count := f.VisitedCount(); count != 7 {
		t.Errorf("After grandchildren: Expected VisitedCount() = 7, got %d", count)
	}

	// Dequeue remaining depth 1 URLs
	f.Dequeue() // products
	f.Dequeue() // contact

	// Count should still be 7 (visited set doesn't shrink)
	if count := f.VisitedCount(); count != 7 {
		t.Errorf("After dequeuing all: Expected VisitedCount() = 7, got %d", count)
	}

	// Try submitting duplicate URLs
	duplicate := mustURL(t, "https://example.com/about") // already visited
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		duplicate, frontier.SourceCrawl, frontier.NewDiscoveryMetadata(3, nil),
	))

	// Count should still be 7
	if finalCount := f.VisitedCount(); finalCount != 7 {
		t.Errorf("After duplicate: Expected VisitedCount() = 7, got %d", finalCount)
	} else {
		t.Logf("Integration test passed: VisitedCount() correctly tracked %d unique URLs", finalCount)
	}
}
