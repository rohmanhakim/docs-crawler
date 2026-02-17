package frontier

import (
	"net/url"
	"sync"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/pkg/collections"
	"github.com/rohmanhakim/docs-crawler/pkg/urlutil"
)

/*
 Frontier is a deterministic URL ordering and deduplication structure.

 Frontier invariants:
 - Frontier MUST NOT perform semantic admission checks.
 - It MUST assume that every submitted URL has already been admitted
   by the scheduler.
 - Frontier MUST NOT consult robots.txt, scope rules, metadata, or
   pipeline state.
 - Frontier decisions are mechanical only (ordering, deduplication,
   depth tracking).

 Frontier MUST NOT influence crawl control flow and MUST NOT reject
 URLs for policy reasons.

 Frontier Responsibilities:
 - Maintain BFS ordering
 - Deduplicate URLs
 - Track crawl depth
 - Prevent infinite traversal
 - Knows nothing about:
	- fetching
	- extraction
	- markdown
	- storage

 Frontier MUST:
 - lives for the entire crawl run (hostname scope)
 - it is a data structure + policy module, not a pipeline executor
 - owns crawl identity, ordering, and admission rules, but it never drives the pipeline
*/

type Frontier interface {
	Init(cfg config.Config)
	Submit(admission CrawlAdmissionCandidate)
	Enqueue(incomingToken CrawlToken)
	IsDepthExhausted(depth int) bool
	CurrentMinDepth() int
	VisitedCount() int
	Dequeue() (CrawlToken, bool)
	BookKeepForRetry(url url.URL, reason error, stage Stage, retryCount int)
	GetRetryCandidates() []url.URL
	RetryQueueSize() int
	ClearRetryQueue(processed []url.URL)
}

type CrawlFrontier struct {
	mu            sync.RWMutex
	queuesByDepth map[int]*collections.FIFOQueue[CrawlToken]
	visitedUrl    collections.Set[string]
	maxDepth      int
	currentDepth  int
	maxPages      int

	// Retry queue for manual retry - URLs that exhausted auto-retry
	retryQueue collections.FIFOQueue[RetryEntry]
	retrySet   collections.Set[string]
}

func NewCrawlFrontier() CrawlFrontier {
	return CrawlFrontier{
		queuesByDepth: make(map[int]*collections.FIFOQueue[CrawlToken]),
		visitedUrl:    collections.NewSet[string](),
	}
}

func (f *CrawlFrontier) Init(cfg config.Config) {
	f.maxDepth = cfg.MaxDepth()
	f.maxPages = cfg.MaxPages()
}

/*
Submit
- Assumes the URL is already admitted.
- It MUST NOT perform robots, scope, or policy checks.
*/
func (f *CrawlFrontier) Submit(admission CrawlAdmissionCandidate) {
	f.mu.Lock() // Lock for write
	defer f.mu.Unlock()

	// return if the visited URL size has reached its allowed max page count from config
	// maxPages = 0 means unlimited
	if f.visitedUrl.Size() == f.maxPages && f.maxPages != 0 {
		return
	}

	// return if new URL depth is higher than the allowed max depth from config
	// maxDepth = 0 means unlimited
	if admission.discoveryMetadata.depth > f.maxDepth && f.maxDepth != 0 {
		return
	}

	// canonicalize the target URL before dedeuplication
	canonicalized := urlutil.Canonicalize(admission.targetURL)

	// deduplicate canonicalized URL
	f.deduplicate(canonicalized, admission.discoveryMetadata.depth)
}

func (f *CrawlFrontier) Enqueue(incomingToken CrawlToken) {
	if f.queuesByDepth[incomingToken.depth] == nil {
		f.queuesByDepth[incomingToken.depth] = collections.NewFIFOQueue[CrawlToken]()
	}
	f.queuesByDepth[incomingToken.depth].Enqueue(incomingToken)
	if incomingToken.depth > f.currentDepth {
		f.currentDepth = incomingToken.depth
	}
}

// IsDepthExhausted reports whether all URLs at the given depth have been
// dequeued (i.e., the depth level is empty).
// Returns true if the depth level has no pending URLs or does not exist.
// This allows the scheduler to enforce strict BFS by detecting when it's
// safe to advance to the next depth level.
func (f *CrawlFrontier) IsDepthExhausted(depth int) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	queue := f.queuesByDepth[depth]
	return queue == nil || queue.Size() == 0
}

// CurrentMinDepth returns the minimum depth that still has pending URLs.
// Returns -1 if the frontier is completely empty.
// This provides the scheduler with visibility into the current BFS progress.
func (f *CrawlFrontier) CurrentMinDepth() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for d := 0; d <= f.currentDepth; d++ {
		if q := f.queuesByDepth[d]; q != nil && q.Size() > 0 {
			return d
		}
	}
	return -1
}

// VisitedCount returns the total number of unique URLs that have been
// submitted to the frontier (i.e., the size of the visited URL set).
// This represents the total unique URLs admitted for crawling.
func (f *CrawlFrontier) VisitedCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.visitedUrl.Size()
}

// Get next URL from the queue,
// returns false on the second returned values if empty
func (f *CrawlFrontier) Dequeue() (CrawlToken, bool) {
	f.mu.Lock() // Lock for write (modifies queues)
	defer f.mu.Unlock()

	// Always exhaust current depth before advancing
	for i := 0; i < f.currentDepth; i++ {
		// prevent nil dereference when a URL is submitted at depth N, but depth N-1 was never created
		if queue := f.queuesByDepth[i]; queue != nil && queue.Size() > 0 {
			return queue.Dequeue()
		}
	}
	// prevention if no url has been submitted yet
	if f.queuesByDepth[f.currentDepth] != nil {
		// only return token at current depth if all lower depth queues are empty
		return f.queuesByDepth[f.currentDepth].Dequeue()
	}
	// the queue is empty
	return CrawlToken{}, false
}

// Check is canonicalized URL has been visited before
// return true if visited; false if has not been visited
func (f *CrawlFrontier) deduplicate(canonicalizedUrl url.URL, depth int) {
	// if already visited skip
	if f.visitedUrl.Contains(canonicalizedUrl.String()) {
		return
	}
	f.visitedUrl.Add(canonicalizedUrl.String())
	token := CrawlToken{
		url:   canonicalizedUrl,
		depth: depth,
	}
	f.Enqueue(token)
}

// BookKeepForRetry adds a URL to the manual retry queue.
// It deduplicates URLs - if a URL is already in the queue, it won't be added again.
// The error is stored as the reason for debugging/display purposes.
// stage and retryCount are recorded for future Failure Journal persistence.
func (f *CrawlFrontier) BookKeepForRetry(url url.URL, reason error, stage Stage, retryCount int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Initialize retry set if needed
	if f.retrySet == nil {
		f.retrySet = collections.NewSet[string]()
	}
	// Initialize retry queue if needed
	if f.retryQueue == nil {
		f.retryQueue = *collections.NewFIFOQueue[RetryEntry]()
	}

	key := url.String()
	if f.retrySet.Contains(key) {
		return // Already tracked
	}

	f.retrySet.Add(key)
	f.retryQueue.Enqueue(RetryEntry{
		URL:        url,
		Reason:     reason.Error(),
		Timestamp:  time.Now(),
		Stage:      stage,
		RetryCount: retryCount,
	})
}

// GetRetryCandidates returns all URLs that are eligible for manual retry.
// This should be called after a crawl completes to get URLs that can be retried.
func (f *CrawlFrontier) GetRetryCandidates() []url.URL {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.retryQueue == nil {
		return nil
	}

	candidates := make([]url.URL, 0, f.retryQueue.Size())
	for _, entry := range f.retryQueue {
		candidates = append(candidates, entry.URL)
	}
	return candidates
}

// RetryQueueSize returns the number of URLs in the retry queue.
func (f *CrawlFrontier) RetryQueueSize() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.retryQueue == nil {
		return 0
	}
	return f.retryQueue.Size()
}

// ClearRetryQueue removes URLs from the retry queue that have been successfully processed.
// The processed parameter is a list of URLs that should be removed.
func (f *CrawlFrontier) ClearRetryQueue(processed []url.URL) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.retryQueue == nil || f.retrySet == nil {
		return
	}

	// Build a set of processed URLs for efficient lookup
	processedSet := make(map[string]bool)
	for _, url := range processed {
		processedSet[url.String()] = true
	}

	// Rebuild retry queue and set, excluding processed URLs
	newQueue := *collections.NewFIFOQueue[RetryEntry]()
	newSet := collections.NewSet[string]()

	for _, entry := range f.retryQueue {
		key := entry.URL.String()
		if !processedSet[key] {
			newQueue.Enqueue(entry)
			newSet.Add(key)
		}
		// If processed, don't add to new queue (effectively removing it)
	}

	f.retryQueue = newQueue
	f.retrySet = newSet
}
