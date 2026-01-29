package frontier

import "github.com/rohmanhakim/docs-crawler/internal/config"

/*
Frontier Responsibilities
- Maintain BFS ordering
- Deduplicate URLs
- Track crawl depth
- Prevent infinite traversal
- Knows nothing about:
	- fetching
	- extraction
	- markdown
	- storage

It is a data structure + policy module, not a pipeline executor.
*/

func NewCrawlingPolicy(cfg config.Config) CrawlingPolicy {
	return CrawlingPolicy{}
}
