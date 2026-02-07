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
	metadataSink    metadata.MetadataSink
	customSelectors []string
	params          ExtractParam
}

func NewDomExtractor(
	metadataSink metadata.MetadataSink,
	params ExtractParam,
	customSelectors ...string,
) DomExtractor {
	return DomExtractor{
		metadataSink:    metadataSink,
		customSelectors: customSelectors,
		params:          params,
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

	// Layer 1: Extract semantic container (main, article, [role="main"])
	contentNode := extractSemanticContainer(doc)
	if contentNode != nil {
		return ExtractionResult{
			DocumentRoot: doc,
			ContentNode:  contentNode,
		}, nil
	}

	// Layer 2: Try known documentation container selectors
	contentNode = d.extractKnownDocContainer(doc)
	if contentNode != nil {
		return ExtractionResult{
			DocumentRoot: doc,
			ContentNode:  contentNode,
		}, nil
	}

	// Layer 3: Explicit chrome removal + text-density scoring
	contentNode = d.extractContainerAfterExplicitChromesRemoval(*doc)
	if contentNode != nil {
		return ExtractionResult{
			DocumentRoot: doc,
			ContentNode:  contentNode,
		}, nil
	}

	// All layers failed to find meaningful content
	return ExtractionResult{}, &ExtractionError{
		Message:   "no meaningful content container found",
		Retryable: false,
		Cause:     ErrCauseNoContent,
	}
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

// extractKnownDocContainer applies the second heuristic layer:
// Known documentation container selectors from popular frameworks.
// Combines default selectors with user-provided custom selectors (deduplicated).
// Returns the first meaningful match, or nil if none found.
func (d *DomExtractor) extractKnownDocContainer(doc *html.Node) *html.Node {
	// Get all default selectors
	defaultSelectors := getAllSelectors()

	// Merge with custom selectors, deduplicating
	allSelectors := mergeSelectors(defaultSelectors, d.customSelectors)

	// Use goquery as convenience wrapper
	gqDoc := goquery.NewDocumentFromNode(doc)

	// Try each selector in priority order
	for _, selector := range allSelectors {
		if elem := gqDoc.Find(selector).First(); elem.Length() > 0 {
			if node := elem.Nodes[0]; isMeaningful(node) {
				return node
			}
		}
	}

	return nil
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
	if !isMeaningful(contentNode) {
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

// removeChromeElements removes elements that are always chrome (nav, header, footer, aside)
func removeChromeElements(root *html.Node) {
	var nodesToRemove []*html.Node

	// First pass: collect all chrome elements
	var collectChromeElements func(*html.Node)
	collectChromeElements = func(n *html.Node) {
		if n == nil {
			return
		}

		if n.Type == html.ElementNode && chromeElementNames[n.Data] {
			nodesToRemove = append(nodesToRemove, n)
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
func removeElementsWithChromeAttributes(root *html.Node) {
	var nodesToRemove []*html.Node

	// First pass: collect elements with chrome-related attributes
	var collectChromeAttributedElements func(*html.Node)
	collectChromeAttributedElements = func(n *html.Node) {
		if n == nil {
			return
		}

		if n.Type == html.ElementNode && hasChromeAttribute(n) {
			nodesToRemove = append(nodesToRemove, n)
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
		score := calculateContentScore(candidate, d.params.LinkDensityThreshold)
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

	// Apply specificity bias: if <body> is best, check if a child is close enough
	if bestNode == bodyNode && bodyNode != nil {
		for node, score := range scores {
			if node == bodyNode {
				continue
			}
			// If child score is >= bias * bodyScore, prefer the child
			if score >= d.params.BodySpecificityBias*bodyScore {
				if score > bestScore*0.9 { // Must also be reasonably close to best
					bestNode = node
					bestScore = score
					break
				}
			}
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
// - Text: +1 per 50 non-whitespace chars
// - Paragraphs: +5 each
// - Headings (h1-h3): +10 each
// - Code blocks: +15 each
// - List items: +2 each
// - Link density penalty if ratio > threshold
func calculateContentScore(node *html.Node, linkDensityThreshold float64) float64 {
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

	// Calculate base score
	// TODO: move the multiplier into ExtractParam
	score := float64(stats.nonWhitespace) / 50.0 // +1 per 50 chars
	score += float64(stats.paragraphs) * 5.0
	score += float64(stats.headings) * 10.0
	score += float64(stats.codeBlocks) * 15.0
	score += float64(stats.listItems) * 2.0

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
	// TODO: move these into ExtractParam
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
