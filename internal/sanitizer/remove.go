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

// removeEmptyNodesBottomUp performs a post-order traversal to remove empty nodes.
// This ensures nested empty containers are fully cleaned (innermost first).
func removeEmptyNodesBottomUp(node *html.Node) {
	if node == nil {
		return
	}

	// First, recursively process children
	// We need to be careful because removing nodes affects the linked list
	var children []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, child)
	}

	for _, child := range children {
		removeEmptyNodesBottomUp(child)
	}

	// Now check if this node itself is empty and should be removed
	// Skip document root, html, head, body elements - they should never be removed
	if node.Type == html.ElementNode && isEmptyNode(node) && shouldRemoveEmptyElement(node.Data) {
		// Remove this node from its parent
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
		}
	}
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

// removePreH1Chrome removes elements with pre-H1 chrome keywords that appear before the first H1.
// This implements the two-factor match: position (pre-H1) + keyword match.
// Elements matching PreH1ChromeKeywords are removed only if they precede the first H1 heading.
func removePreH1Chrome(root *html.Node) {
	if root == nil {
		return
	}

	// Find the first H1 element in the document
	firstH1 := findFirstH1(root)
	if firstH1 == nil {
		// No H1 found, nothing to remove
		return
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

// removeDuplicateNodes removes duplicate structural nodes, keeping the first occurrence.
// It uses a signature-based approach to detect structural duplicates.
func removeDuplicateNodes(root *html.Node) {
	if root == nil {
		return
	}

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
			// Check if this element type should be considered for deduplication
			if isMeaningfulElement(node.Data) {
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
}
