package frontier

/*
 Frontier - manages crawl state & ordering
*/

import (
	"net/url"
	"time"
)

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

// CrawlAdmissionCandidate represents a URL that has already been
// admitted by the scheduler.
//
// Invariants:
// - Robots.txt checks have passed
// - Crawl scope and limits have been enforced
// - Frontier MUST treat this as an admitted URL
// - Frontier MUST NOT re-evaluate admission semantics
type CrawlAdmissionCandidate struct {
	// frontier MUST assume this URL is already admitted.
	targetURL url.URL

	// is it seed url or discovered during crawling?
	sourceContext SourceContext

	// additional information about the URL
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

type SourceContext string

const (
	SourceSeed  = "Seed"
	SourceCrawl = "Crawl"
)

type DiscoveryMetadata struct {
	// the depth of the path relative to hostname where the url is found
	// hostname/root -> depth = 0
	// TODO: implement delay overriding in both scheduler and frontier
	Depth         int
	delayOverride *time.Duration
}
