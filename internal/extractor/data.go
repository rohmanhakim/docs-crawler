package extractor

import "golang.org/x/net/html"

// ExtractionResult holds the extraction outcome.
// DocumentRoot is the original parsed HTML document.
// ContentNode is the extracted meaningful content node (semantic container).
type ExtractionResult struct {
	DocumentRoot *html.Node
	ContentNode  *html.Node
}

// ExtractParam holds configurable parameters for content extraction heuristics.
// These parameters allow users to tune the extraction behavior for specific use cases.
type ExtractParam struct {
	// BodySpecificityBias is the threshold for preferring a child container over <body>.
	// If a child node's score is >= BodySpecificityBias * bodyScore, the child is preferred.
	// Default: 0.75 (75%)
	BodySpecificityBias float64

	// LinkDensityThreshold is the maximum ratio of link text to total text before
	// applying a penalty. Higher values allow more link-heavy content.
	// Default: 0.80 (80%)
	LinkDensityThreshold float64
}

// DefaultExtractParam returns an ExtractParam with sensible default values.
func DefaultExtractParam() ExtractParam {
	return ExtractParam{
		BodySpecificityBias:  0.75,
		LinkDensityThreshold: 0.80,
	}
}
