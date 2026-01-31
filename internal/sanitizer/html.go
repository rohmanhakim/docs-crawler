/*
Responsibilities
- Normalize malformed markup
- Remove empty or duplicate nodes
- Stabilize heading hierarchy

This stage ensures downstream Markdown conversion is deterministic.
*/
package sanitizer

import (
	"errors"
	"fmt"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal"
	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

type HtmlSanitizer struct {
	metadataSink metadata.MetadataSink
}

func NewHTMLSanitizer(metadataSink metadata.MetadataSink) HtmlSanitizer {
	return HtmlSanitizer{
		metadataSink: metadataSink,
	}
}

func (h *HtmlSanitizer) Sanitize(
	extractionResult extractor.ExtractionResult,
) (SanitizedHTMLDoc, internal.ClassifiedError) {
	sanitizedHtmlDoc, err := sanitize()
	if err != nil {
		var sanitizationError *SanitizationError
		errors.As(err, &sanitizationError)
		h.metadataSink.RecordError(
			time.Now(),
			"sanitizer",
			"HtmlSanitizer.Sanitize",
			mapSanitizationErrorToMetadataCause(*sanitizationError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrField, fmt.Sprintf("DOM: %v", "the-broken-dom-value")),
			},
		)
		return SanitizedHTMLDoc{}, sanitizationError
	}
	return sanitizedHtmlDoc, nil
}

func sanitize() (SanitizedHTMLDoc, error) {
	return SanitizedHTMLDoc{}, nil
}
