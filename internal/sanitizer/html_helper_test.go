package sanitizer_test

import (
	"strings"

	"github.com/rohmanhakim/docs-crawler/internal/metadata/metadatatest"
	"github.com/rohmanhakim/docs-crawler/pkg/debug/debugtest"
	"golang.org/x/net/html"
)

// mockMetadataSink is an alias to the shared mock in metadatatest package.
type mockMetadataSink = metadatatest.SinkMock

// renderHtmlForTest serializes an html.Node to its HTML string representation.
// This is used to compare sanitized output against expected fixtures.
func renderHtmlForTest(node *html.Node) string {
	if node == nil {
		return ""
	}
	var buf strings.Builder
	html.Render(&buf, node)
	return buf.String()
}

// normalizeHtmlForTest removes whitespace variations for comparison
func normalizeHtmlForTest(s string) string {
	// Remove extra whitespace and normalize
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}

// hasStep checks if a step with the given name exists in the entries.
func hasStep(entries []debugtest.StepEntry, stepName string) bool {
	for _, e := range entries {
		if e.Step == stepName {
			return true
		}
	}
	return false
}
