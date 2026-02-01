package frontier

import (
	"net/url"
	"sync"

	"github.com/rohmanhakim/docs-crawler/internal/config"
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

type Frontier struct {
	mu            sync.RWMutex
	queuesByDepth map[int]*FIFOQueue[CrawlToken]
	visitedUrl    Set[string]
	maxDepth      int
	currentDepth  int
	maxPages      int
}

func NewFrontier() Frontier {
	return Frontier{
		queuesByDepth: make(map[int]*FIFOQueue[CrawlToken]),
		visitedUrl:    NewSet[string](),
	}
}

func (f *Frontier) NewCrawlToken(url url.URL) CrawlToken {
	return CrawlToken{}
}

func (f *Frontier) Init(cfg config.Config) {
	f.maxDepth = cfg.MaxDepth()
	f.maxPages = cfg.MaxPages()
}

/*
Submit
- Assumes the URL is already admitted.
- It MUST NOT perform robots, scope, or policy checks.
*/
func (f *Frontier) Submit(admission CrawlAdmissionCandidate) {
	f.mu.Lock() // Lock for write
	defer f.mu.Unlock()

	// return if the visited URL size has reached its allowed max page count from config
	// maxPages = 0 means unlimited
	if f.visitedUrl.Size() == f.maxPages && f.maxPages != 0 {
		return
	}

	// return if new URL depth is higher than the allowed max depth from config
	// maxDepth = 0 means unlimited
	if admission.discoveryMetadata.Depth > f.maxDepth && f.maxDepth != 0 {
		return
	}

	// canonicalize the target URL before dedeuplication
	canonicalized := urlutil.Canonicalize(admission.targetURL)

	// deduplicate canonicalized URL
	f.deduplicate(canonicalized, admission.discoveryMetadata.Depth)
}

func (f *Frontier) Enqueue(incomingToken CrawlToken) {
	if f.queuesByDepth[incomingToken.depth] == nil {
		f.queuesByDepth[incomingToken.depth] = NewFIFOQueue[CrawlToken]()
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
func (f *Frontier) IsDepthExhausted(depth int) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	queue := f.queuesByDepth[depth]
	return queue == nil || queue.Size() == 0
}

// CurrentMinDepth returns the minimum depth that still has pending URLs.
// Returns -1 if the frontier is completely empty.
// This provides the scheduler with visibility into the current BFS progress.
func (f *Frontier) CurrentMinDepth() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for d := 0; d <= f.currentDepth; d++ {
		if q := f.queuesByDepth[d]; q != nil && q.Size() > 0 {
			return d
		}
	}
	return -1
}

// Get next URL from the queue,
// returns false on the second returned values if empty
func (f *Frontier) Dequeue() (CrawlToken, bool) {
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
func (f *Frontier) deduplicate(canonicalizedUrl url.URL, depth int) {
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
