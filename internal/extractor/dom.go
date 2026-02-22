package extractor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
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
type Extractor interface {
	// Extract processes the HTML bytes and returns the extracted content.
	// It returns an ExtractionResult containing the document root and content node,
	// or a ClassifiedError if extraction fails.
	Extract(sourceUrl url.URL, htmlByte []byte) (ExtractionResult, failure.ClassifiedError)

	// SetExtractParam allows callers to override the default extraction parameters.
	SetExtractParam(params ExtractParam)
}

type DomExtractor struct {
	metadataSink    metadata.MetadataSink
	customSelectors []string
	params          ExtractParam
	debugLogger     debug.DebugLogger
}

// NewDomExtractor creates a new DomExtractor with default parameters.
// Use SetExtractParam to override default parameters after construction.
func NewDomExtractor(
	metadataSink metadata.MetadataSink,
	customSelectors ...string,
) DomExtractor {
	return DomExtractor{
		metadataSink:    metadataSink,
		customSelectors: customSelectors,
		params:          DefaultExtractParam(),
		debugLogger:     debug.NewNoOpLogger(),
	}
}

// SetExtractParam allows callers to override the default extraction parameters.
// This enables runtime configuration of extraction behavior.
func (d *DomExtractor) SetExtractParam(params ExtractParam) {
	d.params = params
}

// SetDebugLogger sets the debug logger for the extractor.
// This is optional and defaults to NoOpLogger.
// If logger is nil, NoOpLogger is used as a safe default.
func (d *DomExtractor) SetDebugLogger(logger debug.DebugLogger) {
	if logger == nil {
		d.debugLogger = debug.NewNoOpLogger()
		return
	}
	d.debugLogger = logger
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
			metadata.NewErrorRecord(
				time.Now(),
				"extractor",
				"DomExtractor.Extract",
				mapExtractionErrorToMetadataCause(extractionError),
				err.Error(),
				[]metadata.Attribute{
					metadata.NewAttr(metadata.AttrURL, fmt.Sprintf("%v", sourceUrl)),
				},
			),
		)
		return ExtractionResult{}, extractionError
	}
	// ExtractionResult does not carry a discovered-URL count;
	// link extraction is a downstream concern. LinksFound is 0.
	d.metadataSink.RecordPipelineStage(
		metadata.NewPipelineEvent(
			metadata.StageExtract,
			sourceUrl.String(),
			true,
			time.Now(),
			0,
		),
	)

	return result, nil
}

func (d *DomExtractor) extract(htmlByte []byte) (ExtractionResult, error) {
	// Log input size at the start
	if d.debugLogger.Enabled() {
		d.debugLogger.LogStep(context.TODO(), "extractor", "parse_html", debug.FieldMap{
			"input_size_bytes": len(htmlByte),
		})
	}

	// Parse HTML
	doc, err := html.Parse(bytes.NewReader(htmlByte))
	if err != nil {
		return ExtractionResult{}, NewExtractionError(
			ErrCauseNotHTML,
			fmt.Sprintf("failed to parse HTML: %v", err),
		)
	}

	// Validate that this is actually HTML (has <html> element)
	if !isValidHTML(doc) {
		return ExtractionResult{}, NewExtractionError(
			ErrCauseNotHTML,
			"input is not valid HTML document",
		)
	}

	// Layer 0: Remove blacklisted elements before any extraction
	// This ensures noise elements are removed regardless of which layer finds content
	if len(d.params.SelectorBlacklist) > 0 {
		removedCount := removeBlacklistedElementsWithCount(doc, d.params.SelectorBlacklist)
		if d.debugLogger.Enabled() {
			d.debugLogger.LogStep(context.TODO(), "extractor", "layer_0_blacklist", debug.FieldMap{
				"selectors_count": len(d.params.SelectorBlacklist),
				"removed_count":   removedCount,
			})
		}
	}

	// Layer 1: Extract semantic container (main, article, [role="main"])
	contentNode, selector := extractSemanticContainerWithSelector(doc, d.params.Threshold)
	if contentNode != nil {
		if d.debugLogger.Enabled() {
			d.debugLogger.LogStep(context.TODO(), "extractor", "layer_1_semantic", debug.FieldMap{
				"found":    true,
				"selector": selector,
			})
			d.debugLogger.LogStep(context.TODO(), "extractor", "content_selected", debug.FieldMap{
				"final_layer": 1,
				"node_tag":    contentNode.Data,
			})
		}
		return ExtractionResult{
			DocumentRoot: doc,
			ContentNode:  contentNode,
		}, nil
	}

	// Log that layer 1 didn't find content
	if d.debugLogger.Enabled() {
		d.debugLogger.LogStep(context.TODO(), "extractor", "layer_1_semantic", debug.FieldMap{
			"found": false,
		})
	}

	// Layer 2: Try known documentation container selectors
	contentNode, selector = d.extractKnownDocContainerWithSelector(doc)
	if contentNode != nil {
		if d.debugLogger.Enabled() {
			d.debugLogger.LogStep(context.TODO(), "extractor", "layer_2_known", debug.FieldMap{
				"found":    true,
				"selector": selector,
			})
			d.debugLogger.LogStep(context.TODO(), "extractor", "content_selected", debug.FieldMap{
				"final_layer": 2,
				"node_tag":    contentNode.Data,
			})
		}
		return ExtractionResult{
			DocumentRoot: doc,
			ContentNode:  contentNode,
		}, nil
	}

	// Log that layer 2 didn't find content
	if d.debugLogger.Enabled() {
		d.debugLogger.LogStep(context.TODO(), "extractor", "layer_2_known", debug.FieldMap{
			"found": false,
		})
	}

	// Layer 3: Explicit chrome removal + text-density scoring
	contentNode = d.extractContainerAfterExplicitChromesRemoval(*doc)
	if contentNode != nil {
		if d.debugLogger.Enabled() {
			d.debugLogger.LogStep(context.TODO(), "extractor", "layer_3_heuristic", debug.FieldMap{
				"found":    true,
				"node_tag": contentNode.Data,
			})
			d.debugLogger.LogStep(context.TODO(), "extractor", "content_selected", debug.FieldMap{
				"final_layer": 3,
				"node_tag":    contentNode.Data,
			})
		}
		return ExtractionResult{
			DocumentRoot: doc,
			ContentNode:  contentNode,
		}, nil
	}

	// Log that layer 3 didn't find content
	if d.debugLogger.Enabled() {
		d.debugLogger.LogStep(context.TODO(), "extractor", "layer_3_heuristic", debug.FieldMap{
			"found": false,
		})
	}

	// All layers failed to find meaningful content
	if d.debugLogger.Enabled() {
		d.debugLogger.LogStep(context.TODO(), "extractor", "extraction_failed", debug.FieldMap{
			"error_cause": string(ErrCauseNoContent),
		})
	}
	return ExtractionResult{}, NewExtractionError(
		ErrCauseNoContent,
		"no meaningful content container found",
	)
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
func extractSemanticContainer(doc *html.Node, threshold MeaningfulThreshold) *html.Node {
	node, _ := extractSemanticContainerWithSelector(doc, threshold)
	return node
}

// extractSemanticContainerWithSelector is like extractSemanticContainer but also returns
// the selector that matched the content node for debug logging.
func extractSemanticContainerWithSelector(doc *html.Node, threshold MeaningfulThreshold) (*html.Node, string) {
	// Use goquery as convenience wrapper
	gqDoc := goquery.NewDocumentFromNode(doc)

	// Priority 1: <main>
	if main := gqDoc.Find("main").First(); main.Length() > 0 {
		if node := main.Nodes[0]; isMeaningful(node, threshold) {
			return node, "main"
		}
	}

	// Priority 2: <article>
	if article := gqDoc.Find("article").First(); article.Length() > 0 {
		if node := article.Nodes[0]; isMeaningful(node, threshold) {
			return node, "article"
		}
	}

	// Priority 3: [role="main"]
	if roleMain := gqDoc.Find("[role='main']").First(); roleMain.Length() > 0 {
		if node := roleMain.Nodes[0]; isMeaningful(node, threshold) {
			return node, "[role='main']"
		}
	}

	return nil, ""
}

// extractKnownDocContainer applies the second heuristic layer:
// Known documentation container selectors from popular frameworks.
// Combines default selectors with user-provided custom selectors (deduplicated).
// Returns the first meaningful match, or nil if none found.
// Skips matches that would miss an orphan header with H1 (important page title).
func (d *DomExtractor) extractKnownDocContainer(doc *html.Node) *html.Node {
	node, _ := d.extractKnownDocContainerWithSelector(doc)
	return node
}

// extractKnownDocContainerWithSelector is like extractKnownDocContainer but also returns
// the selector that matched the content node for debug logging.
func (d *DomExtractor) extractKnownDocContainerWithSelector(doc *html.Node) (*html.Node, string) {
	// Get all default selectors
	defaultSelectors := getAllSelectors()

	// Merge with custom selectors, deduplicating
	allSelectors := mergeSelectors(defaultSelectors, d.customSelectors)

	// Use goquery as convenience wrapper
	gqDoc := goquery.NewDocumentFromNode(doc)

	// Try each selector in priority order
	for _, selector := range allSelectors {
		if elem := gqDoc.Find(selector).First(); elem.Length() > 0 {
			if node := elem.Nodes[0]; isMeaningful(node, d.params.Threshold) {
				// Check if there's an orphan header with H1 outside this container
				// If so, skip this match and let Layer 3 handle it (which may return <body>)
				if hasOrphanHeaderWithH1(doc, node) {
					continue
				}
				return node, selector
			}
		}
	}

	return nil, ""
}

// hasOrphanHeaderWithH1 checks if there's a header element with H1 that is NOT
// inside the candidate container AND the candidate doesn't have its own H1.
// This detects cases where extracting just the candidate would miss the only
// important page title.
// Returns true if an orphan header with H1 exists that would cause H1 loss.
func hasOrphanHeaderWithH1(doc *html.Node, candidate *html.Node) bool {
	// Find all header elements with H1 in the document
	var headersWithH1 []*html.Node

	var findHeaders func(*html.Node)
	findHeaders = func(n *html.Node) {
		if n == nil {
			return
		}

		if n.Type == html.ElementNode && n.Data == "header" {
			if hasMeaningfulContentInHeader(n) {
				headersWithH1 = append(headersWithH1, n)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findHeaders(c)
		}
	}

	findHeaders(doc)

	// Check if candidate already has an H1 inside it
	candidateHasH1 := hasH1Element(candidate)

	// If candidate already has H1, orphan header is not a problem
	if candidateHasH1 {
		return false
	}

	// Check if any header with H1 is outside the candidate container
	for _, header := range headersWithH1 {
		if !isDescendant(header, candidate) {
			return true // Found orphan header with H1 that would be lost
		}
	}

	return false
}

// hasH1Element checks if a node contains an H1 element
func hasH1Element(node *html.Node) bool {
	if node == nil {
		return false
	}

	var check func(*html.Node) bool
	check = func(n *html.Node) bool {
		if n == nil {
			return false
		}
		if n.Type == html.ElementNode && n.Data == "h1" {
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if check(c) {
				return true
			}
		}
		return false
	}

	return check(node)
}

// isDescendant checks if a node is a descendant of (or equal to) another node
func isDescendant(node, potentialAncestor *html.Node) bool {
	if node == nil || potentialAncestor == nil {
		return false
	}

	// Walk up the tree from node
	for current := node; current != nil; current = current.Parent {
		if current == potentialAncestor {
			return true
		}
	}

	return false
}

// extractContainerAfterExplicitChromesRemoval applies the third heuristic layer:
// 1. Remove explicit chrome elements (nav, header, footer, aside)
// 2. Remove elements with chrome-related class/id names
// 3. Apply text-density scoring to find the best content container
// 4. Apply specificity bias to prefer child containers over <body>
// Returns the best content node, or nil if none found.
func (d *DomExtractor) extractContainerAfterExplicitChromesRemoval(doc html.Node) *html.Node {
	// Step 1: Remove explicit chromes and get cleaned DOM
	cleanedDoc := removeExplicitChromes(&doc)
	if cleanedDoc == nil {
		return nil
	}

	// Step 2: Find the best content container using weighted scoring
	contentNode := d.findBestContentContainer(cleanedDoc)
	if contentNode == nil {
		return nil
	}

	// Step 3: Validate that the selected node is meaningful
	if !isMeaningful(contentNode, d.params.Threshold) {
		return nil
	}

	return contentNode
}

// removeExplicitChromes creates a deep clone of the document and removes:
// 1. Explicit chrome elements: <nav>, <header>, <footer>, <aside>
// 2. Elements with class/id containing chrome keywords
// Returns the cleaned document root.
func removeExplicitChromes(doc *html.Node) *html.Node {
	// Deep clone the document to avoid modifying the original
	clonedDoc := deepCloneNode(doc)
	if clonedDoc == nil {
		return nil
	}

	// Find and remove chrome elements
	removeChromeElements(clonedDoc)

	// Remove elements with chrome-related classes/ids
	removeElementsWithChromeAttributes(clonedDoc)

	return clonedDoc
}

// deepCloneNode creates a deep copy of an html.Node
func deepCloneNode(node *html.Node) *html.Node {
	if node == nil {
		return nil
	}

	// Create new node with same properties
	cloned := &html.Node{
		Type:      node.Type,
		DataAtom:  node.DataAtom,
		Data:      node.Data,
		Namespace: node.Namespace,
	}

	// Clone attributes
	if len(node.Attr) > 0 {
		cloned.Attr = make([]html.Attribute, len(node.Attr))
		copy(cloned.Attr, node.Attr)
	}

	// Clone children recursively
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		clonedChild := deepCloneNode(child)
		if clonedChild != nil {
			cloned.AppendChild(clonedChild)
		}
	}

	return cloned
}

// chromeElementNames contains element names that are always chrome
var chromeElementNames = map[string]bool{
	"nav":    true,
	"header": true,
	"footer": true,
	"aside":  true,
}

// chromeAttributeKeywords contains keywords that indicate chrome when found in class/id
var chromeAttributeKeywords = []string{
	"nav", "sidebar", "menu", "breadcrumb",
	"search", "footer", "header", "cookie",
	"consent", "version", "language", "theme",
	"edit", "github",
}

// hasMeaningfulContentInHeader checks if a header element contains meaningful content
// Returns true if header contains H1 elements or substantial text content (>50 chars)
func hasMeaningfulContentInHeader(header *html.Node) bool {
	var hasH1 bool
	var textContent int

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}

		switch n.Type {
		case html.ElementNode:
			if n.Data == "h1" {
				hasH1 = true
			}
			// Skip navigation elements in content calculation
			if n.Data != "nav" && n.Data != "ul" && n.Data != "ol" {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
			}
		case html.TextNode:
			text := strings.TrimSpace(n.Data)
			if text != "" {
				textContent += len(text)
			}
		}
	}

	walk(header)

	// Consider meaningful if has H1 OR substantial text content (>50 chars)
	return hasH1 || textContent > 50
}

// removeChromeElements removes elements that are always chrome (nav, header, footer, aside)
// But preserves headers with meaningful content (H1s, substantial text)
func removeChromeElements(root *html.Node) {
	var nodesToRemove []*html.Node

	// First pass: collect chrome elements, but be selective about headers
	var collectChromeElements func(*html.Node)
	collectChromeElements = func(n *html.Node) {
		if n == nil {
			return
		}

		if n.Type == html.ElementNode && chromeElementNames[n.Data] {
			if n.Data == "header" {
				// Only remove header if it doesn't contain meaningful content
				if !hasMeaningfulContentInHeader(n) {
					nodesToRemove = append(nodesToRemove, n)
				}
				// Preserve headers with H1s or substantial text
			} else {
				// Remove other chrome elements unconditionally (nav, footer, aside)
				nodesToRemove = append(nodesToRemove, n)
			}
		}

		// Recurse into children (but not into already marked chrome elements)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectChromeElements(c)
		}
	}
	collectChromeElements(root)

	// Second pass: remove collected nodes
	for _, node := range nodesToRemove {
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
		}
	}
}

// removeElementsWithChromeAttributes removes elements with class/id containing chrome keywords
// BUT: preserves <header> elements and their descendants if the header has meaningful content
func removeElementsWithChromeAttributes(root *html.Node) {
	var nodesToRemove []*html.Node

	// First, find all headers with meaningful content to preserve their descendants
	var meaningfulHeaders []*html.Node
	var findMeaningfulHeaders func(*html.Node)
	findMeaningfulHeaders = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "header" && hasMeaningfulContentInHeader(n) {
			meaningfulHeaders = append(meaningfulHeaders, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findMeaningfulHeaders(c)
		}
	}
	findMeaningfulHeaders(root)

	// Collect elements with chrome-related attributes
	var collectChromeAttributedElements func(*html.Node)
	collectChromeAttributedElements = func(n *html.Node) {
		if n == nil {
			return
		}

		if n.Type == html.ElementNode && hasChromeAttribute(n) {
			// Preserve <header> elements with meaningful content
			if n.Data == "header" && hasMeaningfulContentInHeader(n) {
				// Don't remove this header - it has important content
			} else if isDescendantOfMeaningfulHeader(n, meaningfulHeaders) {
				// Don't remove elements inside a meaningful header
			} else {
				nodesToRemove = append(nodesToRemove, n)
			}
		}

		// Recurse into children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectChromeAttributedElements(c)
		}
	}
	collectChromeAttributedElements(root)

	// Second pass: remove collected nodes
	for _, node := range nodesToRemove {
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
		}
	}
}

// isDescendantOfMeaningfulHeader checks if a node is inside any of the meaningful headers
func isDescendantOfMeaningfulHeader(node *html.Node, meaningfulHeaders []*html.Node) bool {
	for _, header := range meaningfulHeaders {
		if isDescendant(node, header) {
			return true
		}
	}
	return false
}

// hasChromeAttribute checks if an element has class or id containing chrome keywords
func hasChromeAttribute(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" || attr.Key == "id" {
			lowerValue := strings.ToLower(attr.Val)
			for _, keyword := range chromeAttributeKeywords {
				if strings.Contains(lowerValue, keyword) {
					return true
				}
			}
		}
	}
	return false
}

// findBestContentContainer finds the best content container using weighted scoring
// It applies specificity bias: prefers child containers over <body>
// BUT: if there's an orphan header with H1, prefer <body> to capture all content
func (d *DomExtractor) findBestContentContainer(doc *html.Node) *html.Node {
	candidates := collectCandidateNodes(doc)
	if len(candidates) == 0 {
		return nil
	}

	// Score all candidates
	scores := make(map[*html.Node]float64)
	var bodyNode *html.Node
	var bodyScore float64

	for _, candidate := range candidates {
		score := calculateContentScore(candidate, d.params.LinkDensityThreshold, d.params.ScoreMultiplier)
		scores[candidate] = score

		if candidate.Data == "body" {
			bodyNode = candidate
			bodyScore = score
		}
	}

	// Find highest scoring node
	var bestNode *html.Node
	var bestScore float64

	for node, score := range scores {
		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}

	// Handle case when no candidates found
	if bestNode == nil {
		return nil
	}

	// Apply specificity bias: if <body> is best, check if a child is close enough
	// BUT: if child would miss an orphan header with H1, prefer <body>
	if bestNode == bodyNode && bodyNode != nil {
		for node, score := range scores {
			if node == bodyNode {
				continue
			}
			// If child score is >= bias * bodyScore, check if it would miss orphan H1
			if score >= d.params.BodySpecificityBias*bodyScore {
				if score > bestScore*0.9 { // Must also be reasonably close to best
					// Check for orphan header with H1
					if hasOrphanHeaderWithH1(doc, node) {
						continue // Skip this child, prefer body
					}
					bestNode = node
					bestScore = score
					break
				}
			}
		}
	}

	// If best is not body but there's an orphan header, prefer body
	if bestNode != bodyNode && bodyNode != nil {
		if hasOrphanHeaderWithH1(doc, bestNode) {
			bestNode = bodyNode
		}
	}

	return bestNode
}

// collectCandidateNodes collects potential content container nodes
func collectCandidateNodes(root *html.Node) []*html.Node {
	var candidates []*html.Node

	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n == nil {
			return
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "div", "section", "body":
				candidates = append(candidates, n)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}

	collect(root)
	return candidates
}

// calculateContentScore calculates a weighted content score for a node
// Recommendations:
// - Text: +1 per NonWhitespaceDivisor non-whitespace chars
// - Paragraphs: +Paragraphs each
// - Headings (h1-h3): +Headings each
// - Code blocks: +CodeBlocks each
// - List items: +ListItems each
// - Link density penalty if ratio > threshold
func calculateContentScore(node *html.Node, linkDensityThreshold float64, scoreMultiplier ContentScoreMultiplier) float64 {
	var stats struct {
		nonWhitespace int
		paragraphs    int
		headings      int
		codeBlocks    int
		listItems     int
		textLength    int
		linkTextLen   int
	}

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
			case "p":
				stats.paragraphs++
			case "h1", "h2", "h3":
				stats.headings++
			case "pre":
				// Check if contains <code>
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "code" {
						stats.codeBlocks++
						break
					}
				}
			case "code":
				// Count inline code instances separately from pre>code blocks
				// Only count if not inside a <pre> (already counted above)
				if n.Parent == nil || n.Parent.Data != "pre" {
					stats.codeBlocks++
				}
			case "li":
				stats.listItems++
			case "a":
				// Count link text for density calculation
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						stats.linkTextLen += len(strings.TrimSpace(c.Data))
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(node)

	// Calculate base score using configurable multipliers
	score := float64(stats.nonWhitespace) / scoreMultiplier.NonWhitespaceDivisor
	score += float64(stats.paragraphs) * scoreMultiplier.Paragraphs
	score += float64(stats.headings) * scoreMultiplier.Headings
	score += float64(stats.codeBlocks) * scoreMultiplier.CodeBlocks
	score += float64(stats.listItems) * scoreMultiplier.ListItems

	// Apply link density penalty
	if stats.textLength > 0 {
		linkDensity := float64(stats.linkTextLen) / float64(stats.textLength)
		if linkDensity > linkDensityThreshold {
			// Penalize proportionally to how much over threshold
			penalty := (linkDensity - linkDensityThreshold) * score
			score -= penalty
		}
	}

	return score
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
func isMeaningful(node *html.Node, threshold MeaningfulThreshold) bool {
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

	// Check basic text presence
	if stats.nonWhitespace < threshold.MinNonWhitespace {
		return false
	}

	// Check for navigation-only content (high link density)
	if stats.textLength > 0 {
		linkDensity := float64(stats.linkTextLength) / float64(stats.textLength)
		if linkDensity > threshold.MaxLinkDensity && stats.links > 2 {
			return false
		}
	}

	// Must have at least paragraphs or code blocks
	hasContent := stats.paragraphs >= threshold.MinParagraphsOrCode || stats.codeBlocks >= threshold.MinParagraphsOrCode

	// Or must have headings with some text
	hasHeadingsWithText := stats.headings > threshold.MinHeadings && stats.nonWhitespace >= 20

	return hasContent || hasHeadingsWithText
}

// removeBlacklistedElements removes elements matching the provided CSS selectors
// from the document tree. This is applied before any extraction layer (Layer 0)
// to ensure user-specified noise elements are removed regardless of which
// extraction heuristic finds the content.
// The function modifies the document tree in place.
func removeBlacklistedElements(doc *html.Node, selectors []string) {
	if len(selectors) == 0 {
		return
	}

	// Use goquery as convenience wrapper for CSS selector matching
	gqDoc := goquery.NewDocumentFromNode(doc)

	// Collect all nodes to remove (can't modify while iterating)
	var nodesToRemove []*html.Node

	for _, selector := range selectors {
		gqDoc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if len(s.Nodes) > 0 {
				nodesToRemove = append(nodesToRemove, s.Nodes...)
			}
		})
	}

	// Remove collected nodes from their parents
	for _, node := range nodesToRemove {
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
		}
	}
}

// removeBlacklistedElementsWithCount is like removeBlacklistedElements but returns
// the count of removed elements for debug logging.
func removeBlacklistedElementsWithCount(doc *html.Node, selectors []string) int {
	if len(selectors) == 0 {
		return 0
	}

	// Use goquery as convenience wrapper for CSS selector matching
	gqDoc := goquery.NewDocumentFromNode(doc)

	// Collect all nodes to remove (can't modify while iterating)
	var nodesToRemove []*html.Node

	for _, selector := range selectors {
		gqDoc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if len(s.Nodes) > 0 {
				nodesToRemove = append(nodesToRemove, s.Nodes...)
			}
		})
	}

	// Remove collected nodes from their parents
	for _, node := range nodesToRemove {
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
		}
	}

	return len(nodesToRemove)
}
