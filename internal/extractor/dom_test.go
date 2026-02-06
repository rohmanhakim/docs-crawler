package extractor_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

// mockMetadataSink is a test spy that captures recorded errors
type mockMetadataSink struct {
	metadata.NoopSink
	errors []recordedError
}

type recordedError struct {
	PackageName string
	Action      string
	Cause       metadata.ErrorCause
	ErrorString string
}

func (m *mockMetadataSink) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	errorString string,
	attrs []metadata.Attribute,
) {
	m.errors = append(m.errors, recordedError{
		PackageName: packageName,
		Action:      action,
		Cause:       cause,
		ErrorString: errorString,
	})
}

func setupExtractor() (*extractor.DomExtractor, *mockMetadataSink) {
	sink := &mockMetadataSink{}
	ext := extractor.NewDomExtractor(sink)
	return &ext, sink
}

func mustParseURL(t *testing.T, raw string) url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return *u
}

// isElementNode checks if the node is the specified HTML element
func isElementNode(node *html.Node, tag string) bool {
	return node != nil && node.Type == html.ElementNode && node.Data == tag
}

// TestExtract_Case_A_MainValid tests: <main> with meaningful content
// Expected: Extraction succeeds, <main> chosen
func TestExtract_Case_A_MainValid(t *testing.T) {
	ext, _ := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/docs")
	htmlBytes := loadFixture(t, "case_a_main_valid.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.NoError(t, err, "Expected successful extraction")
	assert.NotNil(t, result.DocumentRoot, "DocumentRoot should not be nil")
	assert.NotNil(t, result.ContentNode, "ContentNode should not be nil")
	assert.True(t, isElementNode(result.ContentNode, "main"), "ContentNode should be <main> element")
}

// TestExtract_Case_B_MainEmpty tests: <main> exists but empty
// Expected: Returns ErrCauseNoContent (fallback to next layer not implemented yet)
func TestExtract_Case_B_MainEmpty(t *testing.T) {
	ext, sink := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/empty")
	htmlBytes := loadFixture(t, "case_b_main_empty.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.Error(t, err, "Expected extraction to fail")
	assert.Nil(t, result.ContentNode, "ContentNode should be nil on error")

	// Check it's the right error type
	assert.Equal(t, string(failure.SeverityFatal), string(err.Severity()), "Should be fatal error")

	// Verify metadata sink received the error
	require.Len(t, sink.errors, 1, "Should have recorded one error")
	assert.Equal(t, int(metadata.CauseContentInvalid), int(sink.errors[0].Cause))
}

// TestExtract_Case_C_MainNavOnly tests: <main> contains only navigation
// Expected: Returns ErrCauseNoContent (nav-only content is not meaningful)
func TestExtract_Case_C_MainNavOnly(t *testing.T) {
	ext, sink := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/nav-only")
	htmlBytes := loadFixture(t, "case_c_main_nav_only.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.Error(t, err, "Expected extraction to fail for nav-only content")
	assert.Nil(t, result.ContentNode, "ContentNode should be nil on error")

	assert.Equal(t, string(failure.SeverityFatal), string(err.Severity()))

	require.Len(t, sink.errors, 1, "Should have recorded one error")
	assert.Equal(t, int(metadata.CauseContentInvalid), int(sink.errors[0].Cause))
}

// TestExtract_Case_D_ArticleFallback tests: <main> invalid, <article> valid
// Expected: Accept <article> when <main> is not meaningful
func TestExtract_Case_D_ArticleFallback(t *testing.T) {
	ext, _ := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/article-fallback")
	htmlBytes := loadFixture(t, "case_d_article_fallback.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.NoError(t, err, "Expected successful extraction via article fallback")
	assert.NotNil(t, result.DocumentRoot, "DocumentRoot should not be nil")
	assert.NotNil(t, result.ContentNode, "ContentNode should not be nil")
	assert.True(t, isElementNode(result.ContentNode, "article"), "ContentNode should be <article> element")
}

// TestExtract_Case_F_CodeContent tests: Code-dominant content
// Expected: Code blocks are considered meaningful
func TestExtract_Case_F_CodeContent(t *testing.T) {
	ext, _ := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/code-docs")
	htmlBytes := loadFixture(t, "case_f_code_content.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.NoError(t, err, "Expected successful extraction for code-heavy docs")
	assert.NotNil(t, result.DocumentRoot, "DocumentRoot should not be nil")
	assert.NotNil(t, result.ContentNode, "ContentNode should not be nil")
	assert.True(t, isElementNode(result.ContentNode, "article"), "ContentNode should be <article> element")
}

// TestExtract_Case_G_NoContent tests: No meaningful content anywhere
// Expected: Returns ErrCauseNoContent
func TestExtract_Case_G_NoContent(t *testing.T) {
	ext, sink := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/no-content")
	htmlBytes := loadFixture(t, "case_g_no_content.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.Error(t, err, "Expected extraction to fail when no meaningful content")
	assert.Nil(t, result.ContentNode, "ContentNode should be nil on error")

	assert.Equal(t, string(failure.SeverityFatal), string(err.Severity()))

	require.Len(t, sink.errors, 1, "Should have recorded one error")
	assert.Equal(t, int(metadata.CauseContentInvalid), int(sink.errors[0].Cause))
}

// TestExtract_Case_H_NotHTML_XML tests non-HTML XML content
// Expected: Returns ErrCauseNotHTML
func TestExtract_Case_H_NotHTML_XML(t *testing.T) {
	ext, sink := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/api")
	htmlBytes := loadFixture(t, "case_h_not_html.xml")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.Error(t, err, "Expected extraction to fail for XML content")
	assert.Nil(t, result.ContentNode, "ContentNode should be nil on error")

	assert.Equal(t, string(failure.SeverityFatal), string(err.Severity()))

	require.Len(t, sink.errors, 1, "Should have recorded one error")
	assert.Equal(t, int(metadata.CauseContentInvalid), int(sink.errors[0].Cause))
}

// TestExtract_Case_I_NotHTML_Text tests plain text content
// Expected: Returns ErrCauseNotHTML
func TestExtract_Case_I_NotHTML_Text(t *testing.T) {
	ext, sink := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/plaintext")
	htmlBytes := loadFixture(t, "case_i_plain_text.txt")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.Error(t, err, "Expected extraction to fail for plain text")
	assert.Nil(t, result.ContentNode, "ContentNode should be nil on error")

	assert.Equal(t, string(failure.SeverityFatal), string(err.Severity()))

	require.Len(t, sink.errors, 1, "Should have recorded one error")
	assert.Equal(t, int(metadata.CauseContentInvalid), int(sink.errors[0].Cause))
}
