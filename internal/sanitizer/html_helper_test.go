package sanitizer_test

import (
	"strings"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"golang.org/x/net/html"
)

// mockMetadataSink is a test double for metadata.MetadataSink.
type mockMetadataSink struct {
	errors         []metadata.ErrorRecord
	pipelineEvents []metadata.PipelineEvent
}

var _ metadata.MetadataSink = (*mockMetadataSink)(nil)

func (m *mockMetadataSink) RecordError(record metadata.ErrorRecord) {
	m.errors = append(m.errors, record)
}

func (m *mockMetadataSink) RecordFetch(event metadata.FetchEvent) {}

func (m *mockMetadataSink) RecordArtifact(record metadata.ArtifactRecord) {}

func (m *mockMetadataSink) RecordPipelineStage(event metadata.PipelineEvent) {
	m.pipelineEvents = append(m.pipelineEvents, event)
}

func (m *mockMetadataSink) RecordSkip(event metadata.SkipEvent) {}

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
