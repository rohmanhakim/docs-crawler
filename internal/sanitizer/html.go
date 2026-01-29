/*
Responsibilities
- Normalize malformed markup
- Remove empty or duplicate nodes
- Stabilize heading hierarchy

This stage ensures downstream Markdown conversion is deterministic.
*/
package sanitizer

import (
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/extractor"
)

type HtmlSanitizer struct {
	cfg config.Config
	extractor.ExtractionResult
}

func NewContentSanitizer(
	cfg config.Config,
	extractionResult extractor.ExtractionResult,
) SanitizedHTMLDoc {
	return SanitizedHTMLDoc{}
}
