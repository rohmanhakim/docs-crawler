package mdconvert_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

// fixtureDir returns the path to the fixture directory
func fixtureDir() string {
	return filepath.Join(".", "fixture")
}

// loadHtmlFixture reads an HTML fixture file from the input directory and returns its contents as bytes.
// This is used for black box testing via the Convert() method.
func loadHtmlFixture(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join(fixtureDir(), "input", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read fixture %s: %v", filename, err)
	}
	return data
}

// loadExpectedMarkdown reads the expected markdown file for a fixture.
// Trailing newlines are trimmed to match library output format.
func loadExpectedMarkdown(t *testing.T, fixtureName string) []byte {
	t.Helper()
	expectedPath := filepath.Join("fixture", "expected", fixtureName+".md")
	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err, "Failed to read expected markdown for %s", fixtureName)
	// Trim trailing newlines to match library output
	return bytes.TrimRight(data, "\n")
}

// createTestRule creates a StrictConversionRule with a NoopSink for testing.
func createTestRule() *mdconvert.StrictConversionRule {
	return mdconvert.NewRule(&metadata.NoopSink{})
}

// createSanitizedDoc creates a SanitizedHTMLDoc from HTML content for testing.
func createSanitizedDoc(t *testing.T, htmlContent string) sanitizer.SanitizedHTMLDoc {
	t.Helper()
	node := parseHTML(t, htmlContent)
	return sanitizer.NewSanitizedHTMLDoc(node, nil)
}

// parseHTML parses an HTML string and returns the body node.
// This helper mimics how the sanitizer would provide content nodes.
func parseHTML(t *testing.T, htmlContent string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(htmlContent))
	require.NoError(t, err)

	// Find the body node
	var body *html.Node
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)

	if body != nil {
		return body
	}
	return doc
}
