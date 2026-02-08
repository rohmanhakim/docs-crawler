package sanitizer

import (
	"net/url"

	"golang.org/x/net/html"
)

type SanitizedHTMLDoc struct {
	contentNode    *html.Node
	discoveredUrls []url.URL
}

func (s *SanitizedHTMLDoc) GetContentNode() *html.Node {
	return s.contentNode
}

func (s *SanitizedHTMLDoc) GetDiscoveredURLs() []url.URL {
	return s.discoveredUrls
}

// NewSanitizedHTMLDoc creates a SanitizedHTMLDoc for testing purposes.
// The fields remain private to maintain immutability.
func NewSanitizedHTMLDoc(contentNode *html.Node, discoveredUrls []url.URL) SanitizedHTMLDoc {
	return SanitizedHTMLDoc{
		contentNode:    contentNode,
		discoveredUrls: discoveredUrls,
	}
}

// SanitizeParam holds configuration parameters for the sanitization process.
// This allows external configuration without hardcoding magic values.
type SanitizeParam struct {
	// MinimumHeadingLevel is the minimum heading level considered valid for document structure.
	// Defaults to 1 (h1) if zero.
	MinimumHeadingLevel int
}

func DefaultSanitizeParam() SanitizeParam {
	return SanitizeParam{
		MinimumHeadingLevel: 1,
	}
}

// headingInfo represents a heading element with its level and position
type headingInfo struct {
	level int
	node  *html.Node
	text  string
}

// RepairableResult contains the outcome of the repairability check.
// If Repairable is false, Reason contains the specific violation type.
type RepairableResult struct {
	Repairable bool
	Reason     UnrepairabilityReason // empty when Repairable is true
}
