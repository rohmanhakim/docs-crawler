package sanitizer

import (
	"fmt"
	"hash/fnv"
	"strings"
	"unsafe"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// UnrepairabilityReason identifies the specific structural violation that makes
// a document unrepairable. These map 1:1 to sanitizer invariants (S3, H2, H3, S5, E1).
type UnrepairabilityReason string

const (
	// ReasonCompetingRoots: Multiple article/main elements at same level (S3 invariant violation)
	ReasonCompetingRoots UnrepairabilityReason = "competing_roots"

	// ReasonNoStructuralAnchor: No headings and no structural anchors like article/main (H3 invariant violation)
	ReasonNoStructuralAnchor UnrepairabilityReason = "no_structural_anchor"

	// ReasonMultipleH1NoRoot: Multiple H1 elements without provable primary root (H2 invariant violation)
	ReasonMultipleH1NoRoot UnrepairabilityReason = "multiple_h1_no_root"

	// ReasonImpliedMultipleDocs: Document structure implies multiple documents (S5 invariant violation)
	ReasonImpliedMultipleDocs UnrepairabilityReason = "implied_multiple_docs"

	// ReasonAmbiguousDOM: Structurally ambiguous DOM with overlapping contexts (E1 invariant violation)
	ReasonAmbiguousDOM UnrepairabilityReason = "ambiguous_dom"
)

// isEmptyNode checks if a node is empty (has no children or only whitespace text nodes).
// Returns true for element nodes with no meaningful content.
func isEmptyNode(node *html.Node) bool {
	if node == nil || node.Type != html.ElementNode {
		return false
	}

	// Check all children
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.ElementNode:
			// Has a child element, not empty
			return false
		case html.TextNode:
			// Check if text is non-whitespace
			if strings.TrimSpace(child.Data) != "" {
				return false
			}
		}
	}

	// No non-whitespace content found
	return true
}

// nodeSignature generates a signature string for comparing node equality.
// It includes tag name, attributes, and text content structure.
// This is used for duplicate detection.
func nodeSignature(node *html.Node) string {
	if node == nil {
		return ""
	}

	var sig strings.Builder

	// Include node type and tag
	sig.WriteString(fmt.Sprintf("type:%d|tag:%s|", node.Type, node.Data))

	// Include attributes (sorted for consistency)
	for i, attr := range node.Attr {
		if i > 0 {
			sig.WriteString(",")
		}
		sig.WriteString(fmt.Sprintf("%s=%s", attr.Key, attr.Val))
	}
	sig.WriteString("|")

	// Include content hash
	sig.WriteString(fmt.Sprintf("content:%d", nodeContentHash(node)))

	return sig.String()
}

// nodeContentHash generates a hash of the node's content for comparison.
// It recursively hashes the structure and text content.
func nodeContentHash(node *html.Node) uint64 {
	h := fnv.New64a()

	// Hash the node itself
	if node.Type == html.ElementNode {
		h.Write([]byte(node.Data))
		for _, attr := range node.Attr {
			h.Write([]byte(attr.Key))
			h.Write([]byte(attr.Val))
		}
	} else if node.Type == html.TextNode {
		h.Write([]byte(strings.TrimSpace(node.Data)))
	}

	// Recursively hash children
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		childHash := nodeContentHash(child)
		// Mix in child hash
		h.Write([]byte(fmt.Sprintf("%d", childHash)))
	}

	return h.Sum64()
}

// nodesAreEqual compares two nodes for structural equality.
// Returns true if they have the same tag, attributes, and content structure.
func nodesAreEqual(a, b *html.Node) bool {
	if a == nil || b == nil {
		return a == b
	}

	// Must be same type
	if a.Type != b.Type {
		return false
	}

	// For element nodes, compare tag and attributes
	if a.Type == html.ElementNode {
		if a.Data != b.Data {
			return false
		}

		// Compare attributes
		if len(a.Attr) != len(b.Attr) {
			return false
		}

		// Build attribute maps for comparison
		attrMapA := make(map[string]string)
		for _, attr := range a.Attr {
			attrMapA[attr.Key] = attr.Val
		}

		for _, attr := range b.Attr {
			if attrMapA[attr.Key] != attr.Val {
				return false
			}
		}
	}

	// For text nodes, compare normalized content
	if a.Type == html.TextNode {
		return strings.TrimSpace(a.Data) == strings.TrimSpace(b.Data)
	}

	// Recursively compare children
	childA := a.FirstChild
	childB := b.FirstChild

	for childA != nil && childB != nil {
		if !nodesAreEqual(childA, childB) {
			return false
		}
		childA = childA.NextSibling
		childB = childB.NextSibling
	}

	// Both should have run out of children at the same time
	return childA == nil && childB == nil
}

// isMeaningfulElement returns true if the element type should be considered
// for deduplication. Some elements like headings are structural anchors
// and should never be removed as duplicates.
func isMeaningfulElement(tag string) bool {
	// Headings are structural anchors - never deduplicate
	if len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
		return false
	}

	// These elements are typically structural/semantic and should not be deduplicated
	switch tag {
	case "main", "article", "header", "footer", "nav", "aside":
		return false
	default:
		return true
	}
}

// hasCompetingDocumentRoots checks for S3 invariant violation.
// Returns true if there are multiple article or main elements at the same level
// that could be competing document roots.
func hasCompetingDocumentRoots(doc *goquery.Document) bool {
	// Count top-level article elements (direct children of body or direct children of html)
	// that appear to be document roots
	articles := doc.Find("article")
	mains := doc.Find("main")

	// If there are multiple main elements, that's a violation
	if mains.Length() > 1 {
		return true
	}

	// Check for multiple articles that are siblings
	// This indicates potential competing roots
	if articles.Length() > 1 {
		// Check if articles are siblings (share the same parent)
		parentMap := make(map[uintptr]int)
		articles.Each(func(i int, s *goquery.Selection) {
			if node := s.Get(0); node != nil && node.Parent != nil {
				// Use pointer address as map key
				parentPtr := uintptr(unsafe.Pointer(node.Parent))
				parentMap[parentPtr]++
			}
		})

		// If any parent has multiple articles as direct children, it's competing roots
		for _, count := range parentMap {
			if count > 1 {
				return true
			}
		}
	}

	return false
}

// extractHeadings extracts all h1-h6 headings from the document in DOM order
func extractHeadings(doc *goquery.Document) []headingInfo {
	var headings []headingInfo

	for level := 1; level <= 6; level++ {
		tag := fmt.Sprintf("h%d", level)
		doc.Find(tag).Each(func(i int, s *goquery.Selection) {
			if node := s.Get(0); node != nil {
				headings = append(headings, headingInfo{
					level: level,
					node:  node,
					text:  s.Text(),
				})
			}
		})
	}

	return headings
}

// hasStructuralAnchors checks if the document has structural anchors like
// article, main, or section elements that can provide document structure
// even without headings (H3 positive case).
func hasStructuralAnchors(doc *goquery.Document) bool {
	// Check for article, main, or meaningful section structure
	if doc.Find("article").Length() > 0 {
		return true
	}
	if doc.Find("main").Length() > 0 {
		return true
	}
	// Only consider sections if they're properly structured (have children)
	sections := doc.Find("section")
	if sections.Length() > 0 {
		sections.Each(func(i int, s *goquery.Selection) {
			if s.Children().Length() > 0 {
				return
			}
		})
		return sections.Length() > 0
	}
	return false
}

// hasMultipleH1WithoutPrimaryRoot checks for H2 invariant violation.
// Returns true if there are multiple h1 elements without a clear primary root.
func hasMultipleH1WithoutPrimaryRoot(headings []headingInfo) bool {
	var h1s []headingInfo
	for _, h := range headings {
		if h.level == 1 {
			h1s = append(h1s, h)
		}
	}

	// Single h1 is always fine
	if len(h1s) <= 1 {
		return false
	}

	// Multiple h1s - check if they're in a hierarchical relationship
	// If h1s are siblings (same parent), that's ambiguous
	if len(h1s) > 1 {
		parentSet := make(map[uintptr]bool)
		for _, h1 := range h1s {
			if h1.node.Parent != nil {
				// Use pointer address as map key
				parentPtr := uintptr(unsafe.Pointer(h1.node.Parent))
				// If we've seen this parent before, these are siblings = ambiguous
				if parentSet[parentPtr] {
					return true
				}
				parentSet[parentPtr] = true
			}
		}
	}

	// Check if there are multiple h1s with substantial content under each
	// This indicates multiple document sections
	if len(h1s) >= 2 {
		// Check if each h1 has its own substantial section
		substantialH1Count := 0
		for i, h1 := range h1s {
			// Count headings that belong to this h1's section
			sectionHeadings := 0
			nextH1Index := len(headings)
			if i+1 < len(h1s) {
				// Find index of next h1 in headings array
				for j, h := range headings {
					if h.node == h1s[i+1].node {
						nextH1Index = j
						break
					}
				}
			}
			// Find this h1's index
			h1Index := 0
			for j, h := range headings {
				if h.node == h1.node {
					h1Index = j
					break
				}
			}
			// Count headings between this h1 and next h1
			for j := h1Index + 1; j < nextH1Index; j++ {
				if headings[j].level > 1 {
					sectionHeadings++
				}
			}
			// If this h1 has its own subsection hierarchy, it's a substantial document section
			if sectionHeadings >= 2 {
				substantialH1Count++
			}
		}
		// If 2+ h1s each have substantial content, it's multiple documents
		if substantialH1Count >= 2 {
			return true
		}
	}

	return false
}

// hasImpliedMultipleDocuments checks for S5 invariant violation.
// Returns true if the document appears to contain multiple complete documents.
func hasImpliedMultipleDocuments(headings []headingInfo) bool {
	// Group headings by their top-level h1
	type documentSection struct {
		h1       *headingInfo
		headings []headingInfo
	}
	var sections []documentSection

	var currentSection *documentSection
	for i := range headings {
		h := &headings[i]
		if h.level == 1 {
			if currentSection != nil {
				sections = append(sections, *currentSection)
			}
			currentSection = &documentSection{
				h1:       h,
				headings: []headingInfo{},
			}
		} else if currentSection != nil {
			currentSection.headings = append(currentSection.headings, *h)
		}
	}
	if currentSection != nil {
		sections = append(sections, *currentSection)
	}

	// If there are 2+ sections each with h1 and substantial substructure,
	// this implies multiple documents
	if len(sections) >= 2 {
		completeDocumentCount := 0
		for _, section := range sections {
			// A "complete document" has an h1 and at least 2 levels of substructure
			if len(section.headings) >= 2 {
				// Check for hierarchical depth (e.g., h2 -> h3 or multiple h2s)
				hasHierarchy := false
				prevLevel := 0
				for _, h := range section.headings {
					if prevLevel > 0 && h.level >= prevLevel {
						hasHierarchy = true
						break
					}
					prevLevel = h.level
				}
				if hasHierarchy || len(section.headings) >= 3 {
					completeDocumentCount++
				}
			}
		}
		if completeDocumentCount >= 2 {
			return true
		}
	}

	return false
}

// hasStructurallyAmbiguousDOM checks for E1 invariant violation.
// Returns true if the DOM structure is ambiguous (overlapping contexts, orphaned headings).
func hasStructurallyAmbiguousDOM(headings []headingInfo, doc *goquery.Document) bool {
	// Check for orphaned headings (h2-h6 without preceding h1 or h2, etc.)
	if len(headings) > 0 {
		// Track the expected hierarchy
		minLevel := 7 // higher than any valid heading
		for _, h := range headings {
			if h.level < minLevel {
				minLevel = h.level
			}
		}

		// If the minimum level is > 1 and there's no h1, check if structure is still valid
		// This could be valid (e.g., API docs starting at h2), but we need to check
		// if the hierarchy is consistent
		if minLevel > 1 {
			// Check if there's a clear parent structure
			// If headings jump around without clear hierarchy, it's ambiguous
			prevLevel := minLevel
			for i, h := range headings {
				if i == 0 {
					continue
				}
				// Large jumps upward (e.g., h4 -> h2) can be valid if consistent
				// But rapid oscillation suggests ambiguity
				if h.level < prevLevel-1 {
					// Jumping up more than one level is suspicious
					// Check if this is part of a pattern
					if i >= 2 {
						prevPrevLevel := headings[i-2].level
						if prevPrevLevel == h.level {
							// Pattern like h2 -> h4 -> h2 suggests ambiguous structure
							return true
						}
					}
				}
				prevLevel = h.level
			}
		}
	}

	// Check for deeply nested conflicting article/section structures
	// that could indicate overlapping contexts
	conflictingStructures := 0
	doc.Find("article, section").Each(func(i int, s *goquery.Selection) {
		// Check if this element contains an article/section of the same type
		// at an inappropriate nesting level
		node := s.Get(0)
		if node == nil {
			return
		}

		// Count nesting depth
		depth := 0
		parent := node.Parent
		for parent != nil {
			if parent.Data == "article" || parent.Data == "section" {
				depth++
			}
			parent = parent.Parent
		}

		// Deep nesting (>3) of semantic containers suggests ambiguity
		if depth > 3 {
			conflictingStructures++
		}
	})

	if conflictingStructures > 2 {
		return true
	}

	return false
}

// isRepairable determines if the input html.Node has a repairable structure according to
// the sanitizer invariants. It returns RepairableResult with specific Reason if unrepairable.
//
// Checks performed (in order):
//   - S3: Competing document roots (multiple article/main at same level)
//   - H3: No headings and no structural anchors
//   - H2: Multiple h1 elements without a provable primary root
//   - S5: Implied multiple documents (multiple complete document hierarchies)
//   - E1: Structurally ambiguous DOM (overlapping heading contexts, orphaned headings)
//
// This method uses goquery as a convenience wrapper while treating html.Node as the
// canonical data source. No CSS inspection or semantic inference is performed.
func isRepairable(doc *html.Node) RepairableResult {
	// Use goquery as convenience wrapper
	docQuery := goquery.NewDocumentFromNode(doc)

	// Check for competing document roots (S3)
	if hasCompetingDocumentRoots(docQuery) {
		return RepairableResult{Repairable: false, Reason: ReasonCompetingRoots}
	}

	// Extract all headings
	headings := extractHeadings(docQuery)

	// Check for no headings and no structural anchors (H3)
	if len(headings) == 0 && !hasStructuralAnchors(docQuery) {
		return RepairableResult{Repairable: false, Reason: ReasonNoStructuralAnchor}
	}

	// Check for multiple h1 without primary root (H2)
	if hasMultipleH1WithoutPrimaryRoot(headings) {
		return RepairableResult{Repairable: false, Reason: ReasonMultipleH1NoRoot}
	}

	// Check for implied multiple documents (S5)
	if hasImpliedMultipleDocuments(headings) {
		return RepairableResult{Repairable: false, Reason: ReasonImpliedMultipleDocs}
	}

	// Check for structurally ambiguous DOM (E1)
	if hasStructurallyAmbiguousDOM(headings, docQuery) {
		return RepairableResult{Repairable: false, Reason: ReasonAmbiguousDOM}
	}

	return RepairableResult{Repairable: true, Reason: ""}
}
