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
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"golang.org/x/net/html"
)

type HtmlSanitizer struct {
	metadataSink metadata.MetadataSink
}

func NewHTMLSanitizer(metadataSink metadata.MetadataSink) HtmlSanitizer {
	return HtmlSanitizer{
		metadataSink: metadataSink,
	}
}

// Sanitize is the exported entry point for HTML sanitization.
// It accepts an html.Node as the canonical data source for configuration.
// All sanitization errors are recorded via metadataSink before being returned.
func (h *HtmlSanitizer) Sanitize(
	inputContentNode *html.Node,
) (SanitizedHTMLDoc, failure.ClassifiedError) {
	sanitizedHtmlDoc, err := sanitize(inputContentNode)
	if err != nil {
		var sanitizationError *SanitizationError
		errors.As(err, &sanitizationError)

		// Build contextual attributes based on the error cause
		attrs := buildErrorAttributes(sanitizationError)

		h.metadataSink.RecordError(
			time.Now(),
			"sanitizer",
			"HtmlSanitizer.Sanitize",
			mapSanitizationErrorToMetadataCause(*sanitizationError),
			err.Error(),
			attrs,
		)
		return SanitizedHTMLDoc{}, sanitizationError
	}
	return sanitizedHtmlDoc, nil
}

// buildErrorAttributes creates metadata attributes based on the sanitization error cause.
// This provides contextual information for observability and debugging.
func buildErrorAttributes(err *SanitizationError) []metadata.Attribute {
	var attrs []metadata.Attribute

	// Add the error cause as an attribute
	attrs = append(attrs, metadata.NewAttr(metadata.AttrField, string(err.Cause)))

	// Add human-readable message based on cause
	switch err.Cause {
	case ErrCauseUnparseableHTML:
		attrs = append(attrs, metadata.NewAttr(metadata.AttrMessage, "HTML cannot be parsed: nil node or no content"))
	case ErrCauseCompetingRoots:
		attrs = append(attrs, metadata.NewAttr(metadata.AttrMessage, "Multiple competing document roots found"))
	case ErrCauseNoStructuralAnchor:
		attrs = append(attrs, metadata.NewAttr(metadata.AttrMessage, "No headings and no structural anchors like article/main"))
	case ErrCauseMultipleH1NoRoot:
		attrs = append(attrs, metadata.NewAttr(metadata.AttrMessage, "Multiple H1 elements without provable primary root"))
	case ErrCauseImpliedMultipleDocs:
		attrs = append(attrs, metadata.NewAttr(metadata.AttrMessage, "Document structure implies multiple documents"))
	case ErrCauseAmbiguousDOM:
		attrs = append(attrs, metadata.NewAttr(metadata.AttrMessage, "Structurally ambiguous DOM with overlapping contexts"))
	default:
		attrs = append(attrs, metadata.NewAttr(metadata.AttrMessage, "Unknown sanitization error"))
	}

	return attrs
}

// sanitize is the private orchestration method that coordinates all sanitization steps.
// It first checks if the document is parseable, then proceeds with structural repairs.
func sanitize(doc *html.Node) (SanitizedHTMLDoc, *SanitizationError) {
	// Step 1: Check if the document is parseable
	if !isParseable(doc) {
		return SanitizedHTMLDoc{}, &SanitizationError{
			Message:   "input HTML cannot be parsed: nil node or no content",
			Retryable: false,
			Cause:     ErrCauseUnparseableHTML,
		}
	}

	// Step 2: Check if the document is repairable
	result := isRepairable(doc)
	if !result.Repairable {
		cause := mapReasonToErrorCause(result.Reason)
		return SanitizedHTMLDoc{}, &SanitizationError{
			Message:   fmt.Sprintf("document is not repairable: %s", result.Reason),
			Retryable: false,
			Cause:     cause,
		}
	}

	// Step 3: Normalize heading levels (Invariant H1)
	// This renumbers headings to fix skipped levels without reordering nodes
	normalizedDoc := normalizeHeadingLevels(doc)

	// Step 4: Remove duplicate and empty nodes (Invariant S4)
	// This performs structural cleanup: removes empty wrappers and deduplicates identical nodes
	cleanedDoc := removeDuplicateAndEmptyNode(normalizedDoc)

	// Step 5: Extract URLs from the document
	// Extracts hyperlinks exactly as authored, preserving relative URLs
	discoveredUrls := extractUrl(cleanedDoc)

	return SanitizedHTMLDoc{
		contentNode:    cleanedDoc,
		discoveredUrls: discoveredUrls,
	}, nil
}

// mapReasonToErrorCause maps UnrepairabilityReason to SanitizationErrorCause.
// This translation occurs at the sanitize() level to keep isRepairable() independent
// of error cause types.
func mapReasonToErrorCause(reason UnrepairabilityReason) SanitizationErrorCause {
	switch reason {
	case ReasonCompetingRoots:
		return ErrCauseCompetingRoots
	case ReasonNoStructuralAnchor:
		return ErrCauseNoStructuralAnchor
	case ReasonMultipleH1NoRoot:
		return ErrCauseMultipleH1NoRoot
	case ReasonImpliedMultipleDocs:
		return ErrCauseImpliedMultipleDocs
	case ReasonAmbiguousDOM:
		return ErrCauseAmbiguousDOM
	default:
		return ""
	}
}

// isParseable determines if the input html.Node can be parsed according to the sanitizer invariants.
// It returns false if:
//   - The input node is nil
//   - The node has no children (FirstChild is nil)
//   - The node cannot be wrapped by goquery for traversal
//
// This method uses goquery as a convenience wrapper while treating html.Node as the canonical data source.
func isParseable(doc *html.Node) bool {
	// Check for nil node
	if doc == nil {
		return false
	}

	// Check for nil children - a parseable document must have some structure
	if doc.FirstChild == nil {
		return false
	}

	// Use goquery as convenience wrapper to verify the node can be traversed
	// This validates that the DOM structure is readable
	docQuery := goquery.NewDocumentFromNode(doc)
	if docQuery == nil {
		return false
	}

	// Additional check: ensure we can at least access the root element
	// This catches cases where the node exists but has no usable structure
	selection := docQuery.Find("*")
	if selection == nil {
		return false
	}

	return true
}

// normalizeHeadingLevels renumbers heading levels to fix skipped levels.
// Headings should not skip more than one level.
// For example: h1 -> h3 becomes h1 -> h2 (h3 is renumbered to h2).
// Going backward (e.g., h4 -> h2) is allowed as it establishes a new section.
// This function creates a copy of the input document and modifies the copy,
// leaving the original input unchanged.
func normalizeHeadingLevels(doc *html.Node) *html.Node {
	// Create a goquery document from the input
	docQuery := goquery.NewDocumentFromNode(doc)

	// Clone the document to avoid mutating the original
	clonedDoc := goquery.CloneDocument(docQuery)

	// Find all headings in DOM order using a single selector
	// This ensures we process headings in their actual document order
	var headings []*html.Node
	clonedDoc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		if node := s.Get(0); node != nil {
			headings = append(headings, node)
		}
	})

	if len(headings) == 0 {
		return clonedDoc.Get(0)
	}

	// Track the previous heading level (effective level after renumbering)
	prevEffectiveLevel := 0

	for _, node := range headings {
		// Get current level from the node tag name
		currentLevel := 0
		if len(node.Data) == 2 && node.Data[0] == 'h' {
			currentLevel = int(node.Data[1] - '0')
		}
		if currentLevel < 1 || currentLevel > 6 {
			continue
		}

		// Determine effective level after potential renumbering
		effectiveLevel := currentLevel

		// If this is the first heading or we're going deeper
		if prevEffectiveLevel == 0 || currentLevel > prevEffectiveLevel {
			// Check if we're skipping more than one level
			if currentLevel > prevEffectiveLevel+1 {
				// Renumber to prevEffectiveLevel + 1
				newLevel := prevEffectiveLevel + 1
				if newLevel >= 1 && newLevel <= 6 {
					node.Data = fmt.Sprintf("h%d", newLevel)
					effectiveLevel = newLevel
				}
			}
		}
		// If going backward (currentLevel <= prevEffectiveLevel), keep as-is
		// This establishes a new section at a higher level

		prevEffectiveLevel = effectiveLevel
	}

	return clonedDoc.Get(0)
}

// removeDuplicateAndEmptyNode removes empty containers and duplicate structural nodes.
// It performs two passes:
// 1. Remove empty nodes (bottom-up to handle nested empty containers)
// 2. Remove duplicate nodes (keeping the first occurrence)
//
// This is structural repair only:
// - Empty wrappers like <div></div> or <section></section> are removed
// - Duplicate nodes with identical tag, attributes, and content are deduplicated
// - Headings and structural anchors are preserved (not deduplicated)
func removeDuplicateAndEmptyNode(doc *html.Node) *html.Node {
	// Create a goquery document from the input for easier manipulation
	docQuery := goquery.NewDocumentFromNode(doc)

	// Clone the document to avoid mutating the original during iteration
	clonedDoc := goquery.CloneDocument(docQuery)
	rootNode := clonedDoc.Get(0)

	// Phase 1: Remove empty nodes (bottom-up traversal)
	// We traverse from leaves upward to handle nested empty containers
	removeEmptyNodesBottomUp(rootNode)

	// Phase 2: Remove duplicate nodes
	// Keep track of seen node signatures to detect duplicates
	removeDuplicateNodes(rootNode)

	return rootNode
}

// extractUrl extracts all hyperlinks from the document.
// It extracts URLs exactly as authored in the DOM without resolution.
// Design:
//   - Preserves relative URLs as-is (no resolution)
//   - Extracts only HTTP(S) schemes
//   - Skips empty and fragment-only hrefs
//   - Deduplicates identical references
//
// This is called after removeDuplicateAndEmptyNode() returns a valid html.Node.
func extractUrl(doc *html.Node) []url.URL {
	if doc == nil {
		return []url.URL{}
	}

	// Use goquery as convenience wrapper
	docQuery := goquery.NewDocumentFromNode(doc)

	// Track seen URLs for deduplication
	seen := make(map[string]bool)
	var urls []url.URL

	// Find all anchor elements with href attributes
	docQuery.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Skip empty hrefs
		if strings.TrimSpace(href) == "" {
			return
		}

		// Skip fragment-only links
		if strings.HasPrefix(href, "#") {
			return
		}

		// Parse the URL to check scheme
		parsedURL, err := url.Parse(href)
		if err != nil {
			// Structurally invalid URL - skip
			return
		}

		// Skip non-HTTP(S) schemes
		if parsedURL.Scheme != "" && parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return
		}

		// Deduplicate identical references
		if seen[href] {
			return
		}
		seen[href] = true

		urls = append(urls, *parsedURL)
	})

	return urls
}
