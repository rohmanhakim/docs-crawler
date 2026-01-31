package frontier

import (
	"net/url"
	"time"
)

// Crawl state & ordering

// CrawlToken
// Frontier-issued, per-URL crawl Token
// It represents: “This URL, at this depth, in this deterministic order, is next”
// It contains no semantic policy decisions.
// It represents ordering + depth metadata only.
type CrawlToken struct {
	url   url.URL
	depth int
}

func (c *CrawlToken) URL() url.URL {
	return c.url
}

func (c *CrawlToken) Depth() int {
	return c.depth
}

// queueEntity - internal scheduling metadata for a single URL.
// It answers mechanical questions like:
// - Where is this URL in the queue?
// - Under what priority or ordering key?
// - Is it pending, dequeued, or dropped?
// It does not answer semantic questions about crawling.
type queueEntity struct {
}

type SourceContext string

const (
	SourceSeed  = "Seed"
	SourceCrawl = "Crawl"
)

// CrawlAdmissionCandidate represents a URL that has already been
// admitted by the scheduler.
//
// Invariants:
// - Robots.txt checks have passed
// - Crawl scope and limits have been enforced
// - Frontier MUST treat this as an admitted URL
// - Frontier MUST NOT re-evaluate admission semantics
// TODO:
// - Make CrawlAdmissionCandidate carry:
//   - resolved crawl delay
//   - discovery depth
type CrawlAdmissionCandidate struct {
	targetURL         url.URL // Frontier MUST assume this URL is already admitted.
	sourceContext     SourceContext
	discoveryMetadata DiscoveryMetadata
}

func NewCrawlAdmissionCandidate(
	targetUrl url.URL,
	sourceContext SourceContext,
	discoveryMetadata DiscoveryMetadata,
) CrawlAdmissionCandidate {
	return CrawlAdmissionCandidate{
		targetURL:         targetUrl,
		sourceContext:     sourceContext,
		discoveryMetadata: discoveryMetadata,
	}
}

type DiscoveryMetadata struct {
	// the depth of the path relative to hostname where the url is found
	// hostname/root -> depth = 0
	Depth         int
	delayOverride *time.Duration
}
