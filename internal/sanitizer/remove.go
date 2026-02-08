package sanitizer

import "golang.org/x/net/html"

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
