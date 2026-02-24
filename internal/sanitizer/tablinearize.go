/*
Package sanitizer provides HTML sanitization utilities.

This file contains tab container linearization logic that transforms tabbed
UI components into sequential content blocks suitable for static documentation.

The linearization process:
1. Finds all tablist containers in the document
2. For each tab, resolves its associated tabpanel
3. Inserts a context-appropriate heading before each panel
4. Adjusts heading levels within panels according to THAS specification
5. Removes the original tablist UI elements

THAS (Tab Heading Adjustment Specification):
- TH1: Uniform Downward Shift - All headings in a panel are shifted by the same delta
- TH2: Maximum Depth Clamp - Adjusted levels are clamped to h6 maximum
*/
package sanitizer

import (
	"fmt"
	stdhtml "html"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// linearizeTabContainers transforms tabbed UI components into sequential content blocks.
// Each tab becomes a section with a heading derived from the tab label, and heading
// levels within tabpanels are adjusted to maintain document hierarchy.
func linearizeTabContainers(doc *html.Node) *html.Node {
	docQuery := goquery.NewDocumentFromNode(doc)
	clonedDoc := goquery.CloneDocument(docQuery)

	clonedDoc.Find("[role='tablist']").Each(func(i int, tablist *goquery.Selection) {
		tabs := tablist.Find("[role='tab']")

		tabs.Each(func(j int, tab *goquery.Selection) {
			panel := resolvePanel(clonedDoc, tablist, tab, j)
			if panel == nil || panel.Length() == 0 {
				return
			}

			insertTabLabel(tablist, tab, panel)
			adjustPanelHeadings(panel)
		})

		// Remove the original tablist to clean up the UI elements
		tablist.Remove()
	})

	return clonedDoc.Get(0)
}

// resolvePanel finds the tabpanel associated with a tab using multiple strategies.
// Strategies are attempted in order:
//  1. Use aria-controls attribute to find panel by ID
//  2. Find by index among sibling panels (fallback)
func resolvePanel(doc *goquery.Document, tablist, tab *goquery.Selection, tabIndex int) *goquery.Selection {
	// Strategy 1: Use aria-controls attribute
	if ariaControls, exists := tab.Attr("aria-controls"); exists && ariaControls != "" {
		// Try finding by exact ID match
		panel := doc.Find(fmt.Sprintf("[id='%s']", ariaControls))
		if panel.Length() > 0 {
			return panel
		}
		// Also try with explicit role=tabpanel
		panel = doc.Find(fmt.Sprintf("[role='tabpanel'][id='%s']", ariaControls))
		if panel.Length() > 0 {
			return panel
		}
	}

	// Strategy 2: Find by index among sibling panels
	container := tablist.Parent()
	if container.Length() > 0 {
		panels := container.Find("[role='tabpanel']")
		if panels.Length() > tabIndex {
			return panels.Eq(tabIndex)
		}
	}

	return nil
}

// insertTabLabel creates and inserts a heading before the panel based on the tab text.
// The heading level is determined by the document context (preceding headings).
func insertTabLabel(tablist, tab, panel *goquery.Selection) {
	tabText := strings.TrimSpace(tab.Text())
	if tabText == "" {
		return
	}

	// Determine context-aware heading level
	contextLevel := getContextHeadingLevel(tablist.Get(0))

	// The new label level should be one level deeper than the context, clamped at max
	labelLevel := contextLevel + 1
	if labelLevel > maxHeadingLevel {
		labelLevel = maxHeadingLevel
	}

	// Create and insert the heading
	headingHTML := fmt.Sprintf("<h%d>%s</h%d>", labelLevel, stdhtml.EscapeString(tabText), labelLevel)
	panel.BeforeHtml(headingHTML)
}

// adjustPanelHeadings implements the Tab Heading Adjustment Specification (THAS).
// It shifts all headings in a panel to maintain proper hierarchy after the
// tab label heading is inserted.
//
// Rules:
//   - TH1: Uniform Downward Shift - All headings shift by the same delta
//   - TH2: Maximum Depth Clamp - No heading exceeds h6
func adjustPanelHeadings(panel *goquery.Selection) {
	headings := panel.Find("h1, h2, h3, h4, h5, h6")
	if headings.Length() == 0 {
		return
	}

	// Calculate Hmin (minimum heading level = highest visual priority)
	hMin := findMinHeadingLevelInSelection(headings)

	// Find the label level from the heading we just inserted before this panel
	labelLevel := findLabelLevelBeforePanel(panel)

	// TH1: Only adjust if panel's minimum heading is at or above the label level
	// (meaning we need to push them down to stay under the label)
	if hMin > labelLevel {
		return
	}

	// Calculate the shift delta
	// The panel's top-level heading should be one level under the label
	delta := (labelLevel + 1) - hMin

	headings.Each(func(_ int, h *goquery.Selection) {
		node := h.Get(0)
		shiftHeadingLevel(node, delta)
	})
}

// findMinHeadingLevelInSelection finds the minimum heading level in a goquery selection.
func findMinHeadingLevelInSelection(headings *goquery.Selection) int {
	hMin := maxHeadingLevel
	headings.Each(func(_ int, h *goquery.Selection) {
		node := h.Get(0)
		if level := extractHeadingLevel(node); level > 0 && level < hMin {
			hMin = level
		}
	})
	return hMin
}

// findLabelLevelBeforePanel finds the heading level of the most recently inserted
// tab label heading immediately before this panel.
func findLabelLevelBeforePanel(panel *goquery.Selection) int {
	// The label heading was inserted immediately before the panel
	// Walk backwards to find it
	for prev := panel.Prev(); prev.Length() > 0; prev = prev.Prev() {
		node := prev.Get(0)
		if level := extractHeadingLevel(node); level > 0 {
			return level
		}
	}
	// Fallback: use context heading level + 1
	return getContextHeadingLevel(panel.Get(0)) + 1
}
