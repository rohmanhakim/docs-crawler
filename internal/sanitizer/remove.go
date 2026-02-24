package sanitizer

import (
	"strings"

	"golang.org/x/net/html"
)

// PreH1ChromeKeywords contains keywords for elements that should be removed
// if they appear BEFORE the main H1 heading.
// This is a two-factor match: position (pre-H1) + keyword match.
// Elements matching these keywords are considered chrome when they precede H1.
//
//nolint:gochecknoglobals // This is a static lookup table
var PreH1ChromeKeywords = []string{
	"eyebrow",
	// Future: other pre-heading chrome keywords like "kicker", "overline", "section-label"
}

// removeEmptyNodesBottomUpWithCount is like removeEmptyNodesBottomUp but returns the count of removed nodes.
func removeEmptyNodesBottomUpWithCount(node *html.Node) int {
	if node == nil {
		return 0
	}

	removedCount := 0

	// First, recursively process children
	// We need to be careful because removing nodes affects the linked list
	var children []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, child)
	}

	for _, child := range children {
		removedCount += removeEmptyNodesBottomUpWithCount(child)
	}

	// Now check if this node itself is empty and should be removed
	// Skip document root, html, head, body elements - they should never be removed
	if node.Type == html.ElementNode && isEmptyNode(node) && shouldRemoveEmptyElement(node.Data) {
		// Remove this node from its parent
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
			removedCount++
		}
	}

	return removedCount
}

// shouldRemoveEmptyElement returns true if an empty element of this type should be removed.
// Some empty elements like <img>, <br>, <hr> are valid even when empty.
func shouldRemoveEmptyElement(tag string) bool {
	// Void elements (self-closing) are valid when empty
	voidElements := map[string]bool{
		"area": true, "base": true, "br": true, "col": true, "embed": true,
		"hr": true, "img": true, "input": true, "link": true, "meta": true,
		"param": true, "source": true, "track": true, "wbr": true,
	}

	if voidElements[tag] {
		return false
	}

	// Never remove structural containers even if empty
	// (let higher-level logic handle structural decisions)
	structuralElements := map[string]bool{
		"html": true, "head": true, "body": true, "main": true,
	}

	if structuralElements[tag] {
		return false
	}

	return true
}

// findFirstH1 finds the first H1 element in the document using depth-first traversal.
func findFirstH1(root *html.Node) *html.Node {
	var find func(*html.Node) *html.Node
	find = func(node *html.Node) *html.Node {
		if node == nil {
			return nil
		}

		if node.Type == html.ElementNode && node.Data == "h1" {
			return node
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if result := find(child); result != nil {
				return result
			}
		}

		return nil
	}

	return find(root)
}

// hasPreH1ChromeKeyword checks if an element's class or id contains any pre-H1 chrome keyword.
func hasPreH1ChromeKeyword(node *html.Node) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" || attr.Key == "id" {
			lowerValue := strings.ToLower(attr.Val)
			for _, keyword := range PreH1ChromeKeywords {
				if strings.Contains(lowerValue, keyword) {
					return true
				}
			}
		}
	}
	return false
}

func isInCodeBlock(node *html.Node) bool {
	for n := node; n != nil; n = n.Parent {
		if n.Type == html.ElementNode && (n.Data == "pre" || n.Data == "code") {
			return true
		}
	}
	return false
}

// hasAriaHiddenTrue checks if a node has aria-hidden="true" attribute.
// This uses exact string match for the value "true" (case-sensitive per HTML spec).
func hasAriaHiddenTrue(node *html.Node) bool {
	for _, attr := range node.Attr {
		if attr.Key == "aria-hidden" && attr.Val == "true" {
			return true
		}
	}
	return false
}

// removeAriaHiddenElementsWithCount removes elements with aria-hidden="true" attribute.
// Returns the count of removed elements for debug logging.
// This removes the entire element including its children.
func removeAriaHiddenElementsWithCount(root *html.Node) int {
	if root == nil {
		return 0
	}

	removedCount := 0

	// Collect children first because we'll modify the linked list during iteration
	var children []*html.Node
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, child)
	}

	// Process children (depth-first traversal)
	for _, child := range children {
		// If this child has aria-hidden="true", remove it entirely
		if child.Type == html.ElementNode && hasAriaHiddenTrue(child) {
			root.RemoveChild(child)
			removedCount++
			continue // Skip traversing children of removed node
		}

		// Recursively process this child's subtree
		removedCount += removeAriaHiddenElementsWithCount(child)
	}

	return removedCount
}

// removeDuplicateNodesWithCount is like removeDuplicateNodes but returns the count of removed nodes.
func removeDuplicateNodesWithCount(root *html.Node) int {
	if root == nil {
		return 0
	}

	removedCount := 0

	// Track seen signatures at each sibling level
	// We use a map of parent pointer -> set of seen signatures
	seenSignatures := make(map[*html.Node]map[string]bool)

	// Traverse all element nodes and remove duplicates
	var traverse func(node *html.Node)
	traverse = func(node *html.Node) {
		if node == nil {
			return
		}

		// Process element nodes
		if node.Type == html.ElementNode {
			// Skip elements inside pre/code blocks - they preserve literal content
			// where repetition is meaningful (e.g., repeated lines in code examples)
			if isInCodeBlock(node) {
				// Don't deduplicate
				return
			} else if isMeaningfulElement(node.Data) {
				// Check if this element type should be considered for deduplication
				parent := node.Parent
				if parent != nil {
					// Initialize signature set for this parent if needed
					if seenSignatures[parent] == nil {
						seenSignatures[parent] = make(map[string]bool)
					}

					// Generate signature for this node
					sig := nodeSignature(node)

					// Check if we've seen this signature before under the same parent
					if seenSignatures[parent][sig] {
						// This is a duplicate - remove it
						parent.RemoveChild(node)
						removedCount++
						return // Node is removed, don't traverse its children
					}

					// Mark this signature as seen
					seenSignatures[parent][sig] = true
				}
			}
		}

		// Recursively traverse children
		// We need to collect children first because we might modify the list
		var children []*html.Node
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			children = append(children, child)
		}

		for _, child := range children {
			traverse(child)
		}
	}

	traverse(root)

	return removedCount
}
