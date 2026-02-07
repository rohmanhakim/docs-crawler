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

func setupExtractor(customSelectors ...string) (*extractor.DomExtractor, *mockMetadataSink) {
	sink := &mockMetadataSink{}
	ext := extractor.NewDomExtractor(sink, customSelectors...)
	return &ext, sink
}

func setupExtractorWithParams(params extractor.ExtractParam, customSelectors ...string) (*extractor.DomExtractor, *mockMetadataSink) {
	sink := &mockMetadataSink{}
	ext := extractor.NewDomExtractor(sink, customSelectors...)
	ext.SetExtractParam(params)
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

// TestExtract_Case_E_KnownDocContainer tests: No semantic containers, known doc container present
// Expected: Extraction succeeds, known doc container chosen (Layer 2 heuristic)
func TestExtract_Case_E_KnownDocContainer(t *testing.T) {
	ext, _ := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/known-doc")
	htmlBytes := loadFixture(t, "case_e_known_doc_container.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.NoError(t, err, "Expected successful extraction via Layer 2 heuristic")
	assert.NotNil(t, result.DocumentRoot, "DocumentRoot should not be nil")
	assert.NotNil(t, result.ContentNode, "ContentNode should not be nil")
	// The fixture has .markdown-body which should be selected
	assert.Equal(t, "div", result.ContentNode.Data, "ContentNode should be <div> element")
	// Verify it has the correct class by checking an attribute
	var hasClass bool
	for _, attr := range result.ContentNode.Attr {
		if attr.Key == "class" && attr.Val == "markdown-body" {
			hasClass = true
			break
		}
	}
	assert.True(t, hasClass, "ContentNode should have class 'markdown-body'")
}

// TestExtract_Case_B_MainFallback tests: <main> empty, known doc container has content
// Expected: Falls back to Layer 2 and extracts from known doc container
func TestExtract_Case_B_MainFallback(t *testing.T) {
	ext, _ := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/main-fallback")
	htmlBytes := loadFixture(t, "case_b_main_fallback.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.NoError(t, err, "Expected successful extraction via Layer 2 fallback")
	assert.NotNil(t, result.DocumentRoot, "DocumentRoot should not be nil")
	assert.NotNil(t, result.ContentNode, "ContentNode should not be nil")
	// The fixture has .content which should be selected
	assert.Equal(t, "div", result.ContentNode.Data, "ContentNode should be <div> element")
	// Verify it has the correct class
	var hasClass bool
	for _, attr := range result.ContentNode.Attr {
		if attr.Key == "class" && attr.Val == "content" {
			hasClass = true
			break
		}
	}
	assert.True(t, hasClass, "ContentNode should have class 'content'")
}

// TestExtract_Case_C_MainFallback tests: <main> nav-only, known doc container has content
// Expected: Falls back to Layer 2 and extracts from known doc container
func TestExtract_Case_C_MainFallback(t *testing.T) {
	ext, _ := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/main-nav-fallback")
	htmlBytes := loadFixture(t, "case_c_main_fallback.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.NoError(t, err, "Expected successful extraction via Layer 2 fallback")
	assert.NotNil(t, result.DocumentRoot, "DocumentRoot should not be nil")
	assert.NotNil(t, result.ContentNode, "ContentNode should not be nil")
	// The fixture has .doc-content which should be selected
	assert.Equal(t, "div", result.ContentNode.Data, "ContentNode should be <div> element")
	// Verify it has the correct class
	var hasClass bool
	for _, attr := range result.ContentNode.Attr {
		if attr.Key == "class" && attr.Val == "doc-content" {
			hasClass = true
			break
		}
	}
	assert.True(t, hasClass, "ContentNode should have class 'doc-content'")
}

// TestExtract_Case_Layer2Empty tests: Semantic container empty, known doc container also empty
// Expected: Returns ErrCauseNoContent
func TestExtract_Case_Layer2Empty(t *testing.T) {
	ext, sink := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/layer2-empty")
	htmlBytes := loadFixture(t, "case_layer2_empty.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.Error(t, err, "Expected extraction to fail when both layers find no meaningful content")
	assert.Nil(t, result.ContentNode, "ContentNode should be nil on error")

	assert.Equal(t, string(failure.SeverityFatal), string(err.Severity()))

	require.Len(t, sink.errors, 1, "Should have recorded one error")
	assert.Equal(t, int(metadata.CauseContentInvalid), int(sink.errors[0].Cause))
}

// TestExtract_Case_Layer2NavOnly tests: Semantic container nav-only, known doc container also nav-only
// Expected: Returns ErrCauseNoContent
func TestExtract_Case_Layer2NavOnly(t *testing.T) {
	ext, sink := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/layer2-nav-only")
	htmlBytes := loadFixture(t, "case_layer2_nav_only.html")

	result, err := ext.Extract(sourceURL, htmlBytes)

	require.Error(t, err, "Expected extraction to fail when both layers find only navigation")
	assert.Nil(t, result.ContentNode, "ContentNode should be nil on error")

	assert.Equal(t, string(failure.SeverityFatal), string(err.Severity()))

	require.Len(t, sink.errors, 1, "Should have recorded one error")
	assert.Equal(t, int(metadata.CauseContentInvalid), int(sink.errors[0].Cause))
}

// TestExtract_CustomSelector tests: Using custom selector provided via constructor
// Expected: Custom selector is considered alongside defaults
func TestExtract_CustomSelector(t *testing.T) {
	// Use a custom selector that's not in the defaults
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>Custom Selector Test</title></head>
<body>
<nav><a href="/">Home</a></nav>
<div class="custom-docs-container">
<h1>Custom Container</h1>
<p>This is in a custom container that should be selected.</p>
</div>
<footer><p>Copyright</p></footer>
</body>
</html>`

	// Pass custom selector to the extractor
	ext, _ := setupExtractor(".custom-docs-container")
	sourceURL := mustParseURL(t, "https://example.com/custom")

	result, err := ext.Extract(sourceURL, []byte(htmlContent))

	require.NoError(t, err, "Expected successful extraction with custom selector")
	assert.NotNil(t, result.DocumentRoot, "DocumentRoot should not be nil")
	assert.NotNil(t, result.ContentNode, "ContentNode should not be nil")
	assert.Equal(t, "div", result.ContentNode.Data, "ContentNode should be <div> element")
	var hasClass bool
	for _, attr := range result.ContentNode.Attr {
		if attr.Key == "class" && attr.Val == "custom-docs-container" {
			hasClass = true
			break
		}
	}
	assert.True(t, hasClass, "ContentNode should have class 'custom-docs-container'")
}

// TestExtract_CustomSelectorDeduplication tests: Custom selector duplicates default
// Expected: Duplicate is ignored, no error occurs
func TestExtract_CustomSelectorDeduplication(t *testing.T) {
	// .markdown-body is in the default selectors
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>Deduplication Test</title></head>
<body>
<nav><a href="/">Home</a></nav>
<div class="markdown-body">
<h1>Markdown Body</h1>
<p>This container should be selected even with duplicate selector.</p>
</div>
<footer><p>Copyright</p></footer>
</body>
</html>`

	// Pass a duplicate selector (already in defaults)
	ext, _ := setupExtractor(".markdown-body")
	sourceURL := mustParseURL(t, "https://example.com/dedup")

	result, err := ext.Extract(sourceURL, []byte(htmlContent))

	require.NoError(t, err, "Expected successful extraction without duplicate errors")
	assert.NotNil(t, result.ContentNode, "ContentNode should not be nil")
	assert.Equal(t, "div", result.ContentNode.Data, "ContentNode should be <div> element")
}

// TestSetExtractParam tests that SetExtractParam correctly overrides default parameters
// Expected: Custom parameters are applied to extraction behavior
func TestSetExtractParam(t *testing.T) {
	// Create HTML with content that meets default thresholds (at least 50 non-whitespace chars)
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>SetExtractParam Test</title></head>
<body>
<nav><a href="/">Home</a></nav>
<div class="content">
<h1>Main Content</h1>
<p>This is a paragraph with enough text content to meet the default threshold requirements.</p>
<p>Additional paragraph to ensure sufficient content for extraction.</p>
</div>
<footer><p>Copyright</p></footer>
</body>
</html>`

	// Test with default params - should work
	ext, _ := setupExtractor()
	sourceURL := mustParseURL(t, "https://example.com/param-test")

	result, err := ext.Extract(sourceURL, []byte(htmlContent))
	require.NoError(t, err, "Expected successful extraction with default params")
	assert.NotNil(t, result.ContentNode, "ContentNode should not be nil")

	// Test with custom params via SetExtractParam - different thresholds to verify override works
	customParams := extractor.ExtractParam{
		BodySpecificityBias:  0.60, // Changed from default 0.75
		LinkDensityThreshold: 0.85, // Changed from default 0.80
		ScoreMultiplier: extractor.ContentScoreMultiplier{
			NonWhitespaceDivisor: 40.0, // Changed from default 50.0
			Paragraphs:           6.0,  // Changed from default 5.0
			Headings:             12.0, // Changed from default 10.0
			CodeBlocks:           15.0,
			ListItems:            2.0,
		},
		Threshold: extractor.MeaningfulThreshold{
			MinNonWhitespace:    30, // Changed from default 50
			MinHeadings:         0,
			MinParagraphsOrCode: 1,
			MaxLinkDensity:      0.9, // Changed from default 0.8
		},
	}

	ext2, _ := setupExtractorWithParams(customParams, ".content")
	result2, err2 := ext2.Extract(sourceURL, []byte(htmlContent))

	require.NoError(t, err2, "Expected successful extraction with custom params")
	assert.NotNil(t, result2.ContentNode, "ContentNode should not be nil with custom params")
	assert.Equal(t, "div", result2.ContentNode.Data, "ContentNode should be <div> element")

	// Verify the extractor has the custom params
	var hasClass bool
	for _, attr := range result2.ContentNode.Attr {
		if attr.Key == "class" && attr.Val == "content" {
			hasClass = true
			break
		}
	}
	assert.True(t, hasClass, "ContentNode should have class 'content'")
}
