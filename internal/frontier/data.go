package frontier

import (
	"net/url"
	"time"
)

// Crawl state & ordering

// Per-URL Crawling policy
type CrawlingPolicy struct {
	urlNode URLNode
	depth   Depth
}

func (c *CrawlingPolicy) GetURL() url.URL {
	return c.urlNode.url
}

type URLNode struct {
	url url.URL
}

type Depth struct{}

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
	depth         int
	delayOverride *time.Duration
}
