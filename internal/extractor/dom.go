package extractor

import (
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
)

/*
Responsibilities
- Parse HTML into a DOM tre
- Isolate main documentation content
- Remove site chrome and noise

Extraction Strategy
- Priority order:
	- Semantic containers (main, article)
    - Configured selectors
    - Heuristic fallback (largest coherent text block)
Removal Rules
- Strip:
    - Navigation menus
    - Headers and footers
    - Sidebars
    - Cookie banners
    - Version selectors
    - Edit links

Only content relevant to the document body may pass through.
*/

type DomExtractor struct {
	config      config.Config
	fetchResult fetcher.FetchResult
}

func NewDomExtractor(
	config config.Config,
	fetchResult fetcher.FetchResult,
) DomExtractor {
	return DomExtractor{}
}
