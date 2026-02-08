package sanitizer_test

import (
	"strings"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"golang.org/x/net/html"
)

// mockMetadataSink is a test double for metadata.MetadataSink
type mockMetadataSink struct {
	errors []recordedError
}

type recordedError struct {
	timestamp   time.Time
	packageName string
	action      string
	cause       metadata.ErrorCause
	details     string
	attrs       []metadata.Attribute
}

func (m *mockMetadataSink) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	details string,
	attrs []metadata.Attribute,
) {
	m.errors = append(m.errors, recordedError{
		timestamp:   observedAt,
		packageName: packageName,
		action:      action,
		cause:       cause,
		details:     details,
		attrs:       attrs,
	})
}

func (m *mockMetadataSink) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
}

func (m *mockMetadataSink) RecordArtifact(path string) {}

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
