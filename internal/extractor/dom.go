package extractor

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"golang.org/x/net/html"
)

/*
Responsibilities
- Parse HTML into a DOM tree
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
	sourceUrl url.URL,
	htmlByte []byte,
) (ExtractionResult, failure.ClassifiedError) {
	result, err := d.extract(htmlByte)
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
				metadata.NewAttr(metadata.AttrURL, fmt.Sprintf("%v", sourceUrl)),
			},
		)
		return ExtractionResult{}, extractionError
	}
	return result, nil
}

func (d *DomExtractor) extract(htmlByte []byte) (ExtractionResult, error) {
	// Parse HTML
	doc, err := html.Parse(bytes.NewReader(htmlByte))
	if err != nil {
		return ExtractionResult{}, &ExtractionError{
			Message:   fmt.Sprintf("failed to parse HTML: %v", err),
			Retryable: false,
			Cause:     ErrCauseNotHTML,
		}
	}

	// Validate that this is actually HTML (has <html> element)
	if !isValidHTML(doc) {
		return ExtractionResult{}, &ExtractionError{
			Message:   "input is not valid HTML document",
			Retryable: false,
			Cause:     ErrCauseNotHTML,
		}
	}

	// Extract semantic container (first heuristic layer)
	contentNode := extractSemanticContainer(doc)
	if contentNode == nil {
		return ExtractionResult{}, &ExtractionError{
			Message:   "no meaningful semantic container found",
			Retryable: false,
			Cause:     ErrCauseNoContent,
		}
	}

	return ExtractionResult{
		DocumentRoot: doc,
		ContentNode:  contentNode,
	}, nil
}

// isValidHTML checks if the parsed document has a proper HTML structure
func isValidHTML(doc *html.Node) bool {
	// Walk the tree to find <html> element
	var findHTML func(*html.Node) bool
	findHTML = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "html" {
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if findHTML(c) {
				return true
			}
		}
		return false
	}
	return findHTML(doc)
}

// extractSemanticContainer applies the first heuristic layer:
// Priority: <main> -> <article> -> [role="main"]
// Returns the first meaningful match, or nil if none found
func extractSemanticContainer(doc *html.Node) *html.Node {
	// Use goquery as convenience wrapper
	gqDoc := goquery.NewDocumentFromNode(doc)

	// Priority 1: <main>
	if main := gqDoc.Find("main").First(); main.Length() > 0 {
		if node := main.Nodes[0]; isMeaningful(node) {
			return node
		}
	}

	// Priority 2: <article>
	if article := gqDoc.Find("article").First(); article.Length() > 0 {
		if node := article.Nodes[0]; isMeaningful(node) {
			return node
		}
	}

	// Priority 3: [role="main"]
	if roleMain := gqDoc.Find("[role='main']").First(); roleMain.Length() > 0 {
		if node := roleMain.Nodes[0]; isMeaningful(node) {
			return node
		}
	}

	return nil
}

// isMeaningful checks if a node contains meaningful content.
// This function will be reused by every heuristic layer.
// A node is meaningful if it contains:
//   - Substantive text content (not just whitespace)
//   - Headings (h1-h6)
//   - Paragraphs with text
//   - Code blocks (important for documentation)
//
// It rejects nodes with only navigation links.
func isMeaningful(node *html.Node) bool {
	if node == nil {
		return false
	}

	var stats struct {
		textLength     int
		nonWhitespace  int
		headings       int
		paragraphs     int
		codeBlocks     int
		links          int
		linkTextLength int
	}

	// Walk the subtree to collect statistics
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}

		switch n.Type {
		case html.TextNode:
			text := n.Data
			stats.textLength += len(text)
			for _, r := range text {
				if !unicode.IsSpace(r) {
					stats.nonWhitespace++
				}
			}

		case html.ElementNode:
			switch n.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				stats.headings++
			case "p":
				stats.paragraphs++
			case "pre":
				// Check if contains <code>
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "code" {
						stats.codeBlocks++
						break
					}
				}
			case "code":
				// Inline code or code block without pre
				stats.codeBlocks++
			case "a":
				stats.links++
				// Count text within the link
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						stats.linkTextLength += len(strings.TrimSpace(c.Data))
					}
				}
			}
		}

		// Recurse into children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(node)

	// Minimum thresholds for meaningful content
	const minNonWhitespace = 50   // At least 50 non-whitespace characters
	const minHeadings = 0         // Headings are optional but valuable
	const minParagraphsOrCode = 1 // At least 1 paragraph OR code block
	const maxLinkDensity = 0.8    // Max 80% of text can be links (avoid nav)

	// Check basic text presence
	if stats.nonWhitespace < minNonWhitespace {
		return false
	}

	// Check for navigation-only content (high link density)
	if stats.textLength > 0 {
		linkDensity := float64(stats.linkTextLength) / float64(stats.textLength)
		if linkDensity > maxLinkDensity && stats.links > 2 {
			return false
		}
	}

	// Must have at least paragraphs or code blocks
	hasContent := stats.paragraphs >= minParagraphsOrCode || stats.codeBlocks >= minParagraphsOrCode

	// Or must have headings with some text
	hasHeadingsWithText := stats.headings > minHeadings && stats.nonWhitespace >= 20

	return hasContent || hasHeadingsWithText
}
