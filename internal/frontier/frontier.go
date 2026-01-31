package frontier

import (
	"net/url"

	"github.com/rohmanhakim/docs-crawler/internal/config"
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
}

func NewFrontier() Frontier {
	return Frontier{}
}

func (f *Frontier) NewCrawlingPolicy(url url.URL) CrawlingPolicy {
	return CrawlingPolicy{}
}

func (f *Frontier) Init(cfg config.Config) {

}

/*
Submit
- Assumes the URL is already admitted.
- It MUST NOT perform robots, scope, or policy checks.
*/
func (f *Frontier) Submit(admission CrawlAdmissionCandidate) {

}

func (f *Frontier) Dequeue() (CrawlingPolicy, bool) {
	return CrawlingPolicy{}, true
}
