package frontier

import (
	"net/url"

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
	crawlTokenQueue  FIFOQueue[CrawlToken]
	visitedUrl       Set[url.URL]
	maxDepth         int
	currentDepth     int
	maxPages         int
	currentPageCount int
}

func NewFrontier() Frontier {
	return Frontier{
		crawlTokenQueue: NewFIFOQueue[CrawlToken](),
		visitedUrl:      NewSet[url.URL](),
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
	// return if the queue size has reached its allowed max page count from config
	// maxPages = 0 means unlimited
	if f.crawlTokenQueue.Size() == f.maxPages && f.maxPages != 0 {
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

	if f.visitedUrl.Contains(incomingToken.url) {
		return
	}

	if f.crawlTokenQueue.Size() == 0 {
		f.crawlTokenQueue.Enqueue(incomingToken)
		f.currentDepth = incomingToken.depth
	}

	if incomingToken.depth < f.currentDepth {
		higherDepthQueue := NewFIFOQueue[CrawlToken]()
		for i := 0; i < f.crawlTokenQueue.Size(); i++ {
			item, ok := f.crawlTokenQueue.Dequeue()
			if !ok {
				break
			}
			higherDepthQueue.Enqueue(item)
		}
		f.crawlTokenQueue.Enqueue(incomingToken)
		for i := 0; i < higherDepthQueue.Size(); i++ {
			item, ok := higherDepthQueue.Dequeue()
			if !ok {
				break
			}
			f.crawlTokenQueue.Enqueue(item)
		}
		f.currentDepth = incomingToken.depth
	} else {
		f.crawlTokenQueue.Enqueue(incomingToken)
		f.currentDepth = incomingToken.depth
	}
}

// Get next URL from the queue,
// returns false on the second returned values if empty
func (f *Frontier) Dequeue() (CrawlToken, bool) {
	if f.crawlTokenQueue.Size() == 0 {
		return CrawlToken{}, false
	}
	token, ok := f.crawlTokenQueue.Dequeue()
	if !ok {
		return CrawlToken{}, false
	}
	return token, true
}

// Check is canonicalized URL has been visited before
// return true if visited; false if has not been visited
func (f *Frontier) deduplicate(canonicalizedUrl url.URL, depth int) {
	// if already visited skip
	if f.visitedUrl.Contains(canonicalizedUrl) {
		return
	}
	f.visitedUrl.Add(canonicalizedUrl)
	token := CrawlToken{
		url:   canonicalizedUrl,
		depth: depth,
	}
	f.Enqueue(token)
}
