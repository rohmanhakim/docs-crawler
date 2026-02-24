/*
Responsibilities
- Normalize malformed markup
- Remove empty or duplicate nodes
- Stabilize heading hierarchy

This stage ensures downstream Markdown conversion is deterministic.
*/
package sanitizer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"golang.org/x/net/html"
)

type HtmlSanitizer struct {
	metadataSink metadata.MetadataSink
	debugLogger  debug.DebugLogger
}

func NewHTMLSanitizer(metadataSink metadata.MetadataSink) HtmlSanitizer {
	return HtmlSanitizer{
		metadataSink: metadataSink,
		debugLogger:  debug.NewNoOpLogger(),
	}
}

// SetDebugLogger sets the debug logger for the sanitizer.
// This is optional and defaults to NoOpLogger.
// If logger is nil, NoOpLogger is used as a safe default.
func (h *HtmlSanitizer) SetDebugLogger(logger debug.DebugLogger) {
	if logger == nil {
		h.debugLogger = debug.NewNoOpLogger()
		return
	}
	h.debugLogger = logger
}

// Sanitize is the exported entry point for HTML sanitization.
// It accepts an html.Node as the canonical data source for configuration.
// All sanitization errors are recorded via metadataSink before being returned.
func (h *HtmlSanitizer) Sanitize(
	inputContentNode *html.Node,
) (SanitizedHTMLDoc, failure.ClassifiedError) {
	sanitizedHtmlDoc, err := h.sanitize(inputContentNode)
	if err != nil {
		var sanitizationError *SanitizationError
		errors.As(err, &sanitizationError)

		// Build contextual attributes based on the error cause
		attrs := buildErrorAttributes(sanitizationError)

		h.metadataSink.RecordError(
			metadata.NewErrorRecord(
				time.Now(),
				"sanitizer",
				"HtmlSanitizer.Sanitize",
				mapSanitizationErrorToMetadataCause(*sanitizationError),
				err.Error(),
				attrs,
			),
		)
		return SanitizedHTMLDoc{}, sanitizationError
	}

	// TODO: PageURL is empty because Sanitize does not currently receive the page URL.
	// As a future improvement, extend the Sanitize interface method to accept a pageURL string
	// parameter so this event can carry full page context for per-page pipeline progress views.
	h.metadataSink.RecordPipelineStage(
		metadata.NewPipelineEvent(
			metadata.StageSanitize,
			"", // pageURL not available at this stage
			true,
			time.Now(),
			0, // linksFound is not applicable for the sanitize stage
		),
	)
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
func (h *HtmlSanitizer) sanitize(doc *html.Node) (SanitizedHTMLDoc, *SanitizationError) {
	// Step 1: Check if the document is parseable
	if !isParseable(doc) {
		if h.debugLogger.Enabled() {
			h.debugLogger.LogStep(context.TODO(), "sanitizer", "validate_input", debug.FieldMap{
				"has_content": false,
			})
		}
		return SanitizedHTMLDoc{}, NewSanitizationError(
			ErrCauseUnparseableHTML,
			"input HTML cannot be parsed: nil node or no content",
		)
	}

	// Log successful parseability check
	if h.debugLogger.Enabled() {
		h.debugLogger.LogStep(context.TODO(), "sanitizer", "validate_input", debug.FieldMap{
			"has_content": true,
		})
	}

	// Step 2: Check if the document is repairable
	result := isRepairable(doc)
	if !result.Repairable {
		if h.debugLogger.Enabled() {
			h.debugLogger.LogStep(context.TODO(), "sanitizer", "check_repairable", debug.FieldMap{
				"repairable": false,
				"reason":     string(result.Reason),
			})
		}
		cause := mapReasonToErrorCause(result.Reason)
		return SanitizedHTMLDoc{}, NewSanitizationError(
			cause,
			fmt.Sprintf("document is not repairable: %s", result.Reason),
		)
	}

	// Log successful repairability check
	if h.debugLogger.Enabled() {
		h.debugLogger.LogStep(context.TODO(), "sanitizer", "check_repairable", debug.FieldMap{
			"repairable": true,
		})
	}

	// Step 2.5: Linearize Tab Containers
	// Transforms tabbed UI components into linearized, deterministic document structures
	linearizedDoc := linearizeTabContainers(doc)

	// Step 3: Normalize heading levels (Invariant H1)
	// This renumbers headings to fix skipped levels without reordering nodes
	normalizedDoc, headingStats := normalizeHeadingLevelsWithStats(linearizedDoc)
	if h.debugLogger.Enabled() {
		h.debugLogger.LogStep(context.TODO(), "sanitizer", "normalize_headings", debug.FieldMap{
			"headings_count":   headingStats.totalCount,
			"renumbered_count": headingStats.renumberedCount,
		})
	}

	// Step 4: Remove pre-H1 chrome elements
	// This removes elements like "eyebrow" that precede the main H1 heading
	removedPreH1Count := removePreH1ChromeWithCount(normalizedDoc)
	if h.debugLogger.Enabled() {
		h.debugLogger.LogStep(context.TODO(), "sanitizer", "remove_pre_h1_chrome", debug.FieldMap{
			"removed_count": removedPreH1Count,
		})
	}

	// Step 4.5: Remove aria-hidden elements
	// This removes elements with aria-hidden="true" attribute (accessibility hidden content)
	removedAriaHiddenCount := removeAriaHiddenElementsWithCount(normalizedDoc)
	if h.debugLogger.Enabled() {
		h.debugLogger.LogStep(context.TODO(), "sanitizer", "remove_aria_hidden", debug.FieldMap{
			"removed_count": removedAriaHiddenCount,
		})
	}

	// Step 5: Remove duplicate and empty nodes (Invariant S4)
	// This performs structural cleanup: removes empty wrappers and deduplicates identical nodes
	cleanedDoc, removalStats := removeDuplicateAndEmptyNodeWithStats(normalizedDoc)
	if h.debugLogger.Enabled() {
		h.debugLogger.LogStep(context.TODO(), "sanitizer", "remove_empty_nodes", debug.FieldMap{
			"removed_count": removalStats.emptyRemoved,
		})
		h.debugLogger.LogStep(context.TODO(), "sanitizer", "remove_duplicates", debug.FieldMap{
			"removed_count": removalStats.duplicatesRemoved,
		})
	}

	// Step 6: Extract URLs from the document
	// Extracts hyperlinks exactly as authored, preserving relative URLs
	discoveredUrls, urlStats := extractUrlWithStats(cleanedDoc)
	if h.debugLogger.Enabled() {
		h.debugLogger.LogStep(context.TODO(), "sanitizer", "extract_urls", debug.FieldMap{
			"urls_found":       urlStats.found,
			"skipped_fragment": urlStats.skippedFragment,
			"skipped_invalid":  urlStats.skippedInvalid,
		})
	}

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

// headingStats tracks statistics for heading normalization.
type headingStats struct {
	totalCount      int
	renumberedCount int
}

// normalizeHeadingLevelsWithStats is like normalizeHeadingLevels but also returns stats.
func normalizeHeadingLevelsWithStats(doc *html.Node) (*html.Node, headingStats) {
	stats := headingStats{}

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

	stats.totalCount = len(headings)

	if len(headings) == 0 {
		return clonedDoc.Get(0), stats
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
					stats.renumberedCount++
				}
			}
		}
		// If going backward (currentLevel <= prevEffectiveLevel), keep as-is
		// This establishes a new section at a higher level

		prevEffectiveLevel = effectiveLevel
	}

	return clonedDoc.Get(0), stats
}

// removePreH1ChromeWithCount is like removePreH1Chrome but returns the count of removed elements.
func removePreH1ChromeWithCount(root *html.Node) int {
	if root == nil {
		return 0
	}

	// Find the first H1 element in the document
	firstH1 := findFirstH1(root)
	if firstH1 == nil {
		// No H1 found, nothing to remove
		return 0
	}

	// Collect all elements that precede the H1 and match chrome keywords
	var nodesToRemove []*html.Node

	var collectPreH1Chrome func(*html.Node, bool)
	collectPreH1Chrome = func(node *html.Node, beforeH1 bool) {
		if node == nil {
			return
		}

		// Check if this node is the H1 - stop collecting after this
		if node.Type == html.ElementNode && node.Data == "h1" && node == firstH1 {
			return // Don't process this node or its siblings
		}

		// If we're before the H1, check if this node matches chrome keywords
		if beforeH1 && node.Type == html.ElementNode {
			if hasPreH1ChromeKeyword(node) {
				nodesToRemove = append(nodesToRemove, node)
			}
		}

		// Recurse into children (still before H1)
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collectPreH1Chrome(child, beforeH1)
		}
	}

	// Start collection from root
	collectPreH1Chrome(root, true)

	// Remove collected nodes
	for _, node := range nodesToRemove {
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
		}
	}

	return len(nodesToRemove)
}

// removalStats tracks statistics for node removal operations.
type removalStats struct {
	emptyRemoved      int
	duplicatesRemoved int
}

// removeDuplicateAndEmptyNodeWithStats is like removeDuplicateAndEmptyNode but returns stats.
func removeDuplicateAndEmptyNodeWithStats(doc *html.Node) (*html.Node, removalStats) {
	stats := removalStats{}

	// Create a goquery document from the input for easier manipulation
	docQuery := goquery.NewDocumentFromNode(doc)

	// Clone the document to avoid mutating the original during iteration
	clonedDoc := goquery.CloneDocument(docQuery)
	rootNode := clonedDoc.Get(0)

	// Phase 1: Remove empty nodes (bottom-up traversal)
	// We traverse from leaves upward to handle nested empty containers
	stats.emptyRemoved = removeEmptyNodesBottomUpWithCount(rootNode)

	// Phase 2: Remove duplicate nodes
	// Keep track of seen node signatures to detect duplicates
	stats.duplicatesRemoved = removeDuplicateNodesWithCount(rootNode)

	return rootNode, stats
}

// urlStats tracks statistics for URL extraction.
type urlStats struct {
	found           int
	skippedFragment int
	skippedInvalid  int
}

// extractUrlWithStats is like extractUrl but also returns stats.
func extractUrlWithStats(doc *html.Node) ([]url.URL, urlStats) {
	stats := urlStats{}

	if doc == nil {
		return []url.URL{}, stats
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
			stats.skippedFragment++
			return
		}

		// Parse the URL to check scheme
		parsedURL, err := url.Parse(href)
		if err != nil {
			// Structurally invalid URL - skip
			stats.skippedInvalid++
			return
		}

		// Skip non-HTTP(S) schemes
		if parsedURL.Scheme != "" && parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			stats.skippedInvalid++
			return
		}

		// Deduplicate identical references
		if seen[href] {
			return
		}
		seen[href] = true

		urls = append(urls, *parsedURL)
		stats.found++
	})

	return urls, stats
}
