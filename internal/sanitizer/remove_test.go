package sanitizer

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// TestIsInCodeBlock verifies that the isInCodeBlock helper correctly identifies
// nodes that are descendants of pre or code elements.
func TestIsInCodeBlock(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		target   string // text content to find target node
		expected bool
	}{
		{
			name:     "span inside pre",
			html:     `<pre><span>target</span></pre>`,
			target:   "target",
			expected: true,
		},
		{
			name:     "span inside code",
			html:     `<code><span>target</span></code>`,
			target:   "target",
			expected: true,
		},
		{
			name:     "nested inside pre > code > span",
			html:     `<pre><code><span>target</span></code></pre>`,
			target:   "target",
			expected: true,
		},
		{
			name:     "span outside code block",
			html:     `<div><span>target</span></div>`,
			target:   "target",
			expected: false,
		},
		{
			name:     "span after pre (sibling)",
			html:     `<pre>code</pre><span>target</span>`,
			target:   "target",
			expected: false,
		},
		{
			name:     "div inside code (should be protected)",
			html:     `<code><div>target</div></code>`,
			target:   "target",
			expected: true,
		},
		{
			name:     "deeply nested in pre",
			html:     `<pre><div><section><span>target</span></section></div></pre>`,
			target:   "target",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse HTML
			doc, err := html.Parse(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("Failed to parse HTML: %v", err)
			}

			// Find target node by traversing
			targetNode := findNodeByText(doc, tt.target)
			if targetNode == nil {
				t.Fatalf("Could not find target node with text: %s", tt.target)
			}

			// Test isInCodeBlock
			result := isInCodeBlock(targetNode)
			if result != tt.expected {
				t.Errorf("isInCodeBlock() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// findNodeByText finds a node by its text content using depth-first traversal.
func findNodeByText(node *html.Node, text string) *html.Node {
	if node == nil {
		return nil
	}

	// Check if this is a text node with the target content
	if node.Type == html.TextNode && strings.Contains(node.Data, text) {
		return node
	}

	// Check element nodes that might contain just the text
	if node.Type == html.ElementNode {
		// Check if this element's direct text content matches
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode && strings.Contains(child.Data, text) {
				return node
			}
		}
	}

	// Recursively search children
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if result := findNodeByText(child, text); result != nil {
			return result
		}
	}

	return nil
}
