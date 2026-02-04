package extractor

import (
	"errors"
	"fmt"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
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
	metadataSink metadata.MetadataSink
}

func NewDomExtractor(
	metadataSink metadata.MetadataSink,
) DomExtractor {
	return DomExtractor{
		metadataSink: metadataSink,
	}
}

func (d *DomExtractor) Extract(
	fetchResult fetcher.FetchResult,
) (ExtractionResult, failure.ClassifiedError) {
	result, err := extract()
	if err != nil {
		var extractionError *ExtractionError
		errors.As(err, &extractionError)
		d.metadataSink.RecordError(
			time.Now(),
			"extractor",
			"DomExtractor.Extract",
			mapExtractionErrorToMetadataCause(extractionError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, fmt.Sprintf("%v", fetchResult.GetFetchURL())),
			},
		)
		return ExtractionResult{}, extractionError
	}
	return result, nil
}

func extract() (ExtractionResult, error) {
	return ExtractionResult{}, nil
}
