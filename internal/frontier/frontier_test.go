package frontier_test

import (
	"net/url"
	"testing"

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

func TestFrontier_DoesNotEnforceBFS(t *testing.T) {
	// GIVEN a frontier with no depth/page limits (simplify scenario)
	cfg := config.Config{} // assume zero-values mean "no limits"

	f := frontier.NewFrontier()
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
		frontier.DiscoveryMetadata{Depth: 0},
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
		frontier.DiscoveryMetadata{Depth: 1},
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C,
		frontier.SourceCrawl,
		frontier.DiscoveryMetadata{Depth: 1},
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
		frontier.DiscoveryMetadata{Depth: 2},
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
		ASSERTION OF FAILURE (conceptual):

		This test PASSES today, but it exposes the bug:
		D (depth 2) was allowed into the queue BEFORE
		the frontier had finished processing depth-1 URLs.

		A BFS-enforcing frontier would have rejected or deferred D
		until depth-1 exhaustion.
	*/
}

func TestFrontier_AllowsDuplicateURL(t *testing.T) {
	// GIVEN a fresh frontier
	f := frontier.NewFrontier()
	f.Init(config.Config{})

	A := mustURL(t, "https://example.com/docs")

	// WHEN the same URL is submitted twice
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A,
		frontier.SourceSeed,
		frontier.DiscoveryMetadata{Depth: 0},
	))

	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, // same URL, same canonical form
		frontier.SourceCrawl,
		frontier.DiscoveryMetadata{Depth: 1},
	))

	// THEN only ONE crawl token should ever be dequeued
	token1, ok := f.Dequeue()
	if !ok {
		t.Fatalf("expected first dequeue to succeed")
	}
	if token1.URL() != A {
		t.Fatalf("expected URL A, got %v", token1.URL())
	}

	// ❌ This should NOT exist in a correct frontier
	token2, ok := f.Dequeue()
	if ok {
		t.Fatalf(
			"duplicate URL dequeued: %v (frontier failed to deduplicate)",
			token2.URL(),
		)
	}
}

// TestFrontier_BFOrderingViolation demonstrates that depth-2 URLs
// can be dequeued BEFORE all depth-1 URLs are exhausted.
// This violates the BFS guarantee required by Task 1.2.
func TestFrontier_BFOrderingViolation(t *testing.T) {
	// GIVEN a frontier
	f := frontier.NewFrontier()
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
		a proper BFS frontier would still have D dequeued BEFORE C.
		But with a single FIFO queue, C is already in front.
	*/

	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")
	C := mustURL(t, "https://example.com/c")
	D := mustURL(t, "https://example.com/d")

	// Seed A at depth 0
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceSeed, frontier.DiscoveryMetadata{Depth: 0},
	))

	// Process A, discover B
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 1},
	))

	// Process B, discover C (depth 2)
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 2},
	))

	// Now simulate D being discovered at depth 1 (from another branch)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		D, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 1},
	))

	// In proper BFS: D (depth 1) MUST be dequeued BEFORE C (depth 2)
	// Because all depth-1 URLs must be exhausted before ANY depth-2 URL
	token, ok := f.Dequeue()
	if !ok {
		t.Fatalf("expected dequeue to succeed")
	}

	// ❌ BUG: Current implementation returns C (depth 2) before D (depth 1)
	// This proves BFS ordering is NOT enforced
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

// TestFrontier_DepthLimitNotEnforced proves that depth limits from config
// are not applied during Submit
func TestFrontier_DepthLimitNotEnforced(t *testing.T) {
	// GIVEN a frontier with max depth of 2
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, err := config.WithDefault([]url.URL{*seedURL}).
		WithMaxDepth(2). // URLs at depth 3+ should be rejected
		Build()
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	f := frontier.NewFrontier()
	f.Init(cfg)

	deepURL := mustURL(t, "https://example.com/deep")

	// WHEN a URL at depth 5 is submitted (exceeds limit)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		deepURL,
		frontier.SourceCrawl,
		frontier.DiscoveryMetadata{Depth: 5}, // Exceeds MaxDepth of 2
	))

	// THEN it should NOT be in the queue
	// ❌ BUG: Current implementation accepts it anyway
	token, ok := f.Dequeue()
	if ok {
		t.Fatalf("BUG: URL at depth %d was accepted despite MaxDepth=%d. Token: %v",
			token.Depth(), cfg.MaxDepth(), token.URL())
	}
}

// TestFrontier_DeduplicationUsesStructEquality exposes the subtle bug
// where url.URL struct comparison fails due to pointer fields
func TestFrontier_DeduplicationUsesStructEquality(t *testing.T) {
	// GIVEN two URLs that are semantically identical but may differ in memory
	f := frontier.NewFrontier()
	f.Init(config.Config{})

	// These URLs are semantically identical
	url1 := mustURL(t, "https://example.com/docs")
	url2 := mustURL(t, "https://example.com/docs")

	// WHEN both are submitted
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		url1, frontier.SourceSeed, frontier.DiscoveryMetadata{Depth: 0},
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		url2, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 1},
	))

	// THEN only one should be dequeued
	token1, ok1 := f.Dequeue()
	if !ok1 {
		t.Fatalf("expected first dequeue to succeed")
	}

	token2, ok2 := f.Dequeue()
	if ok2 {
		// ❌ BUG: The Set[url.URL] treats url1 and url2 as different keys
		// even though they represent the same URL
		t.Fatalf("BUG: Deduplication failed - url.URL struct comparison issue. "+
			"First: %v (ptr: %p), Second: %v (ptr: %p)",
			token1.URL(), &token1, token2.URL(), &token2)
	}

	t.Logf("Deduplication worked for: %v", token1.URL())
}

// TestFrontier_WideTreeBFViolation demonstrates BFS violation
// in a wide tree scenario where many depth-1 URLs should be
// processed before any depth-2 URL
func TestFrontier_WideTreeBFViolation(t *testing.T) {
	// GIVEN a frontier
	f := frontier.NewFrontier()
	f.Init(config.Config{})

	/*
		Tree structure:
		    Root (depth 0)
		    /  |  \
		   A   B   C  (depth 1)
		   |
		   D          (depth 2)

		In proper BFS: Root → A → B → C → D
		Current impl:  Root → A → D → B → C (D can appear before B, C!)
	*/

	root := mustURL(t, "https://example.com/root")
	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")
	C := mustURL(t, "https://example.com/c")
	D := mustURL(t, "https://example.com/d")

	// Submit root
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		root, frontier.SourceSeed, frontier.DiscoveryMetadata{Depth: 0},
	))

	// Process root, discover A, B, C
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 1},
	))

	// Process A early, discover D (depth 2) before B and C are even submitted
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		D, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 2},
	))

	// Now submit B and C (depth 1)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 1},
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 1},
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

// TestFrontier_PageCountLimitNotEnforced proves that page count limits
// are not tracked or enforced
func TestFrontier_PageCountLimitNotEnforced(t *testing.T) {
	// GIVEN a frontier with max pages limit of 2
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, err := config.WithDefault([]url.URL{*seedURL}).
		WithMaxPages(2). // Only 2 pages should be crawled
		Build()
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	f := frontier.NewFrontier()
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
			u, frontier.SourceSeed, frontier.DiscoveryMetadata{Depth: 0},
		))

		// Dequeue immediately to simulate "crawling"
		if i < maxPages {
			f.Dequeue()
		}
	}

	// THEN page 3 and 4 should NOT be in the queue
	// ❌ BUG: Current implementation doesn't track or enforce page limits
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

// TestFrontier_NoDeterministicDepthExhaustion proves that the frontier
// cannot guarantee all URLs at depth N are exhausted before depth N+1
func TestFrontier_NoDeterministicDepthExhaustion(t *testing.T) {
	// This test proves a critical BFS property is missing:
	// The frontier provides no way to know when a depth level is exhausted.

	f := frontier.NewFrontier()
	f.Init(config.Config{})

	A := mustURL(t, "https://example.com/a")
	B := mustURL(t, "https://example.com/b")
	C := mustURL(t, "https://example.com/c")

	// Submit A at depth 0
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		A, frontier.SourceSeed, frontier.DiscoveryMetadata{Depth: 0},
	))

	// Dequeue A, discover B at depth 1
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		B, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 1},
	))

	// Dequeue B, discover C at depth 2
	f.Dequeue()
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		C, frontier.SourceCrawl, frontier.DiscoveryMetadata{Depth: 2},
	))

	// At this point, C (depth 2) is the ONLY item in queue.
	// In a proper BFS frontier, we should be able to determine:
	// "All depth-1 URLs have been exhausted" - but we CANNOT.

	// There's no API to check depth-level exhaustion
	// The frontier has no concept of "current processing depth"
	// This makes true BFS coordination impossible.

	token, _ := f.Dequeue()
	if token.Depth() != 2 {
		t.Fatalf("unexpected depth: %d", token.Depth())
	}

	t.Log("CONFIRMED: Frontier has no depth-level exhaustion tracking.")
	t.Log("BFS property 'process all depth-N before depth-(N+1)' cannot be guaranteed.")
}
