package extractor

import "golang.org/x/net/html"

// ExtractionResult holds the extraction outcome.
// DocumentRoot is the original parsed HTML document.
// ContentNode is the extracted meaningful content node (semantic container).
type ExtractionResult struct {
	DocumentRoot *html.Node
	ContentNode  *html.Node
}

// ContentScoreMultiplier holds the scoring weights for content elements.
// These multipliers are used in calculateContentScore to determine the
// relevance of a content container based on its structural elements.
type ContentScoreMultiplier struct {
	// NonWhitespaceDivisor is the divisor for calculating text score.
	// Score gets +1 point per NonWhitespaceDivisor characters.
	// Default: 50.0
	NonWhitespaceDivisor float64

	// Paragraphs is the score multiplier for each paragraph element.
	// Default: 5.0
	Paragraphs float64

	// Headings is the score multiplier for each heading element (h1-h3).
	// Default: 10.0
	Headings float64

	// CodeBlocks is the score multiplier for each code block.
	// Default: 15.0
	CodeBlocks float64

	// ListItems is the score multiplier for each list item.
	// Default: 2.0
	ListItems float64
}

// MeaningfulThreshold holds minimum thresholds for determining if content
// is meaningful enough to be considered the main documentation content.
type MeaningfulThreshold struct {
	// MinNonWhitespace is the minimum number of non-whitespace characters
	// required for content to be considered meaningful.
	// Default: 50
	MinNonWhitespace int

	// MinHeadings is the minimum number of headings required.
	// Headings are optional but valuable.
	// Default: 0
	MinHeadings int

	// MinParagraphsOrCode is the minimum number of paragraphs OR code blocks
	// required for content to be considered meaningful.
	// Default: 1
	MinParagraphsOrCode int

	// MaxLinkDensity is the maximum ratio of link text to total text before
	// content is considered navigation-only and rejected.
	// Default: 0.8 (80%)
	MaxLinkDensity float64
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

	// ScoreMultiplier holds the scoring weights for different content elements.
	// Used by calculateContentScore to determine container relevance.
	ScoreMultiplier ContentScoreMultiplier

	// Threshold holds the minimum thresholds for meaningful content detection.
	// Used by isMeaningful to validate extracted content.
	Threshold MeaningfulThreshold
}

// DefaultExtractParam returns an ExtractParam with sensible default values.
func DefaultExtractParam() ExtractParam {
	return ExtractParam{
		BodySpecificityBias:  0.75,
		LinkDensityThreshold: 0.80,
		ScoreMultiplier: ContentScoreMultiplier{
			NonWhitespaceDivisor: 50.0,
			Paragraphs:           5.0,
			Headings:             10.0,
			CodeBlocks:           15.0,
			ListItems:            2.0,
		},
		Threshold: MeaningfulThreshold{
			MinNonWhitespace:    50,
			MinHeadings:         0,
			MinParagraphsOrCode: 1,
			MaxLinkDensity:      0.8,
		},
	}
}
