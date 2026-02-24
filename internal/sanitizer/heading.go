/*
Package sanitizer provides HTML sanitization utilities.

This file contains heading-related utilities for extracting, finding, and
manipulating HTML heading elements (h1-h6). These utilities are used across
multiple sanitization operations including heading normalization and tab
container linearization.
*/
package sanitizer

import (
	"golang.org/x/net/html"
)

const maxHeadingLevel = 6

// extractHeadingLevel returns the heading level (1-6) from a node.
// Returns 0 if the node is not a valid heading element (h1-h6).
func extractHeadingLevel(node *html.Node) int {
	if node == nil {
		return 0
	}
	if node.Type != html.ElementNode {
		return 0
	}
	if len(node.Data) != 2 || node.Data[0] != 'h' {
		return 0
	}
	level := int(node.Data[1] - '0')
	if level < 1 || level > maxHeadingLevel {
		return 0
	}
	return level
}

// findLastHeadingInSubtree traverses the subtree in DOM order and returns
// the heading level of the last heading element found.
// Returns 0 if no heading is found in the subtree.
func findLastHeadingInSubtree(root *html.Node) int {
	lastLevel := 0
	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node == nil {
			return
		}
		if level := extractHeadingLevel(node); level > 0 {
			lastLevel = level
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(root)
	return lastLevel
}

// findHeadingInPrecedingSiblings searches backwards through a node's
// preceding siblings to find the nearest heading. It also checks inside
// sibling containers for trailing headings.
// Returns the heading level found, or 0 if none found.
func findHeadingInPrecedingSiblings(node *html.Node) int {
	for n := node.PrevSibling; n != nil; n = n.PrevSibling {
		// Check if the sibling itself is a heading
		if level := extractHeadingLevel(n); level > 0 {
			return level
		}
		// Check if sibling container has a trailing heading
		if level := findLastHeadingInSubtree(n); level > 0 {
			return level
		}
	}
	return 0
}

// getContextHeadingLevel finds the nearest heading level preceding the given node.
// It searches in three phases:
//  1. Preceding siblings of the start node
//  2. Up the ancestor chain, checking each ancestor's preceding siblings
//  3. Falls back to level 1 if no heading is found
//
// This is used to determine the appropriate heading level for generated content
// (e.g., tab labels) so it fits correctly in the document hierarchy.
func getContextHeadingLevel(startNode *html.Node) int {
	if startNode == nil {
		return 1
	}

	// Phase 1: Check preceding siblings of the start node
	if level := findHeadingInPrecedingSiblings(startNode); level > 0 {
		return level
	}

	// Phase 2: Walk up the ancestor chain
	for p := startNode.Parent; p != nil; p = p.Parent {
		// Check if the parent itself is a heading
		if level := extractHeadingLevel(p); level > 0 {
			return level
		}
		// Check preceding siblings of this ancestor
		if level := findHeadingInPrecedingSiblings(p); level > 0 {
			return level
		}
	}

	// Phase 3: Default starting level
	return 1
}

// shiftHeadingLevel adjusts a heading's level by a delta amount.
// The new level is clamped to the valid range [1, maxHeadingLevel].
// Does nothing if the node is not a valid heading.
func shiftHeadingLevel(node *html.Node, delta int) {
	currentLevel := extractHeadingLevel(node)
	if currentLevel == 0 {
		return
	}
	newLevel := currentLevel + delta
	if newLevel < 1 {
		newLevel = 1
	}
	if newLevel > maxHeadingLevel {
		newLevel = maxHeadingLevel
	}
	node.Data = formatHeadingTag(newLevel)
}

// formatHeadingTag returns the heading tag name for a given level (e.g., "h1", "h2").
func formatHeadingTag(level int) string {
	if level < 1 {
		return "h1"
	}
	if level > maxHeadingLevel {
		return "h6"
	}
	return []string{"", "h1", "h2", "h3", "h4", "h5", "h6"}[level]
}
