package sanitizer

import (
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"golang.org/x/net/html"
)

// Sanitizer defines the interface for HTML sanitization.
// Implementations must ensure the DOM is structurally valid and deterministic
type Sanitizer interface {
	// Sanitize processes the input HTML node and returns a sanitized document.
	// It returns a SanitizedHTMLDoc containing the cleaned content and discovered URLs,
	// or a ClassifiedError if the document cannot be sanitized.
	Sanitize(inputContentNode *html.Node) (SanitizedHTMLDoc, failure.ClassifiedError)
}

// Compile-time interface check
var _ Sanitizer = (*HtmlSanitizer)(nil)
