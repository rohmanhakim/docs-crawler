package fetcher

import (
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
)

/*
Responsibilities

- Perform HTTP requests
- Apply headers and timeouts
- Handle redirects safely
- Classify responses

Fetch Semantics

- Only successful HTML responses are processed
- Non-HTML content is discarded
- Redirect chains are bounded
- All responses are logged with metadata

The fetcher never parses content; it only returns bytes and metadata.
*/

type HtmlFetcher struct {
	cfg            config.Config
	crawlingPolicy frontier.CrawlingPolicy
}

func NewHtmlFetcher(
	cfg config.Config,
	crawlingPolicy frontier.CrawlingPolicy,
) HtmlFetcher {
	return HtmlFetcher{}
}
