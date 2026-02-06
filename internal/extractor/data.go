package extractor

import "golang.org/x/net/html"

// ExtractionResult holds the extraction outcome.
// DocumentRoot is the original parsed HTML document.
// ContentNode is the extracted meaningful content node (semantic container).
type ExtractionResult struct {
	DocumentRoot *html.Node
	ContentNode  *html.Node
}
