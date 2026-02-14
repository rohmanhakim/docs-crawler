package normalize_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
)

func TestNormalize_SuccessfulFrontmatterGeneration(t *testing.T) {
	// Arrange
	metadataSink := &metadataSinkMock{}
	constraint := normalize.NewMarkdownConstraint(metadataSink)

	fetchURL, _ := url.Parse("https://docs.example.com/guide/getting-started")
	content := loadFixture(t, "pass/success.md")

	assetfulDoc := assets.NewAssetfulMarkdownDoc(content, nil, nil, nil)
	normalizeParam := normalize.NewNormalizeParam(
		"v1.0.0",
		time.Date(2026, 2, 12, 10, 15, 0, 0, time.UTC),
		hashutil.HashAlgoSHA256,
		2,
		[]string{"/docs"},
	)

	// Act
	result, err := constraint.Normalize(*fetchURL, assetfulDoc, normalizeParam)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	frontmatter := result.Frontmatter()

	// Verify title extracted from H1
	if frontmatter.Title() != "Getting Started" {
		t.Errorf("expected title 'Getting Started', got: %s", frontmatter.Title())
	}

	// Verify sourceURL matches input
	if frontmatter.SourceURL() != "https://docs.example.com/guide/getting-started" {
		t.Errorf("expected sourceURL 'https://docs.example.com/guide/getting-started', got: %s", frontmatter.SourceURL())
	}

	// Verify canonicalURL is normalized
	if frontmatter.CanonicalURL() != "https://docs.example.com/guide/getting-started" {
		t.Errorf("expected canonicalURL 'https://docs.example.com/guide/getting-started', got: %s", frontmatter.CanonicalURL())
	}

	// Verify section is derived from first path segment (no prefix match)
	if frontmatter.Section() != "guide" {
		t.Errorf("expected section 'guide', got: %s", frontmatter.Section())
	}

	// Verify crawlDepth from param
	if frontmatter.CrawlDepth() != 2 {
		t.Errorf("expected crawlDepth 2, got: %d", frontmatter.CrawlDepth())
	}

	// Verify crawlerVersion from param
	if frontmatter.CrawlerVersion() != "v1.0.0" {
		t.Errorf("expected crawlerVersion 'v1.0.0', got: %s", frontmatter.CrawlerVersion())
	}

	// Verify fetchedAt from param
	expectedTime := time.Date(2026, 2, 12, 10, 15, 0, 0, time.UTC)
	if !frontmatter.FetchedAt().Equal(expectedTime) {
		t.Errorf("expected fetchedAt %v, got: %v", expectedTime, frontmatter.FetchedAt())
	}

	// Verify docID has sha256: prefix
	if !strings.HasPrefix(frontmatter.DocID(), "sha256:") {
		t.Errorf("expected docID to have 'sha256:' prefix, got: %s", frontmatter.DocID())
	}

	// Verify contentHash has sha256: prefix
	if !strings.HasPrefix(frontmatter.ContentHash(), "sha256:") {
		t.Errorf("expected contentHash to have 'sha256:' prefix, got: %s", frontmatter.ContentHash())
	}

	// Verify content is included in result
	if result.Content() == nil || len(result.Content()) == 0 {
		t.Error("expected content to be included in normalized document")
	}
}

func TestNormalize_CanonicalURLNormalization(t *testing.T) {
	// Arrange
	metadataSink := &metadataSinkMock{}
	constraint := normalize.NewMarkdownConstraint(metadataSink)

	// URL with uppercase, fragment, and query that should be normalized
	fetchURL, _ := url.Parse("https://DOCS.Example.com/Guide/Page#section?foo=bar")
	content := loadFixture(t, "input/simple_test_page.md")

	assetfulDoc := assets.NewAssetfulMarkdownDoc(content, nil, nil, nil)
	normalizeParam := normalize.NewNormalizeParam("v1.0.0", time.Now(), hashutil.HashAlgoSHA256, 1, nil)

	// Act
	result, err := constraint.Normalize(*fetchURL, assetfulDoc, normalizeParam)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	frontmatter := result.Frontmatter()

	// Canonical URL should be lowercase, no fragment, no query
	expectedCanonical := "https://docs.example.com/Guide/Page"
	if frontmatter.CanonicalURL() != expectedCanonical {
		t.Errorf("expected canonicalURL '%s', got: %s", expectedCanonical, frontmatter.CanonicalURL())
	}

	// Source URL should remain original
	if frontmatter.SourceURL() != "https://DOCS.Example.com/Guide/Page#section?foo=bar" {
		t.Errorf("expected sourceURL to remain original, got: %s", frontmatter.SourceURL())
	}
}

func TestNormalize_DifferentHashAlgorithms(t *testing.T) {
	testCases := []struct {
		name      string
		hashAlgo  hashutil.HashAlgo
		expPrefix string
	}{
		{
			name:      "SHA256",
			hashAlgo:  hashutil.HashAlgoSHA256,
			expPrefix: "sha256:",
		},
		{
			name:      "BLAKE3",
			hashAlgo:  hashutil.HashAlgoBLAKE3,
			expPrefix: "blake3:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			metadataSink := &metadataSinkMock{}
			constraint := normalize.NewMarkdownConstraint(metadataSink)

			fetchURL, _ := url.Parse("https://example.com/docs/page")
			content := loadFixture(t, "input/simple_test_page_short.md")

			assetfulDoc := assets.NewAssetfulMarkdownDoc(content, nil, nil, nil)
			normalizeParam := normalize.NewNormalizeParam("v1.0.0", time.Now(), tc.hashAlgo, 1, nil)

			// Act
			result, err := constraint.Normalize(*fetchURL, assetfulDoc, normalizeParam)

			// Assert
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			frontmatter := result.Frontmatter()

			if !strings.HasPrefix(frontmatter.DocID(), tc.expPrefix) {
				t.Errorf("expected docID to have '%s' prefix, got: %s", tc.expPrefix, frontmatter.DocID())
			}

			if !strings.HasPrefix(frontmatter.ContentHash(), tc.expPrefix) {
				t.Errorf("expected contentHash to have '%s' prefix, got: %s", tc.expPrefix, frontmatter.ContentHash())
			}
		})
	}
}

func TestNormalize_ConstraintViolations(t *testing.T) {
	testCases := []struct {
		name      string
		fixture   string
		invariant string
	}{
		{
			name:      "empty content",
			fixture:   "fail/empty_content.md",
			invariant: "content exists",
		},
		{
			name:      "no H1 present",
			fixture:   "fail/no_h1.md",
			invariant: "N1 - exactly one H1",
		},
		{
			name:      "empty H1",
			fixture:   "fail/empty_h1.md",
			invariant: "N1 - H1 has content",
		},
		{
			name:      "multiple H1s",
			fixture:   "fail/multiple_h1s.md",
			invariant: "N1 - single H1 only",
		},
		{
			name:      "skipped heading H1 to H3",
			fixture:   "fail/skipped_heading_h1_to_h3.md",
			invariant: "N3 - no skipped levels",
		},
		{
			name:      "skipped heading H2 to H4",
			fixture:   "fail/skipped_heading_h2_to_h4.md",
			invariant: "N3 - no skipped levels",
		},
		{
			name:      "orphan content before H1",
			fixture:   "fail/orphan_content_before_h1.md",
			invariant: "N4 - no orphan content",
		},
		{
			name:      "paragraph before H1",
			fixture:   "fail/paragraph_before_h1.md",
			invariant: "N4 - content belongs to hierarchy",
		},
		{
			name:      "empty section - consecutive same level headings",
			fixture:   "fail/empty_section_consecutive.md",
			invariant: "N5 - no empty sections",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			metadataSink := &metadataSinkMock{}
			constraint := normalize.NewMarkdownConstraint(metadataSink)

			fetchURL, _ := url.Parse("https://example.com/docs/page")
			content := loadFixture(t, tc.fixture)

			assetfulDoc := assets.NewAssetfulMarkdownDoc(content, nil, nil, nil)
			normalizeParam := normalize.NewNormalizeParam("v1.0.0", time.Now(), hashutil.HashAlgoSHA256, 1, nil)

			// Act
			_, err := constraint.Normalize(*fetchURL, assetfulDoc, normalizeParam)

			// Assert
			if err == nil {
				t.Fatalf("expected error for %s (%s), got nil", tc.name, tc.invariant)
			}

			if !metadataSink.recordErrorCalled {
				t.Error("expected metadata sink RecordError to be called")
			}

			// Verify the attrs contain the URL attribute with correct value
			if len(metadataSink.recordErrorAttrs) == 0 {
				t.Error("expected RecordError attrs to contain at least one attribute")
			} else {
				foundURL := false
				for _, attr := range metadataSink.recordErrorAttrs {
					if attr.Key == metadata.AttrURL {
						foundURL = true
						if attr.Value != fetchURL.String() {
							t.Errorf("expected AttrURL to be '%s', got '%s'", fetchURL.String(), attr.Value)
						}
						break
					}
				}
				if !foundURL {
					t.Error("expected RecordError attrs to contain AttrURL")
				}
			}
		})
	}
}

func TestNormalize_ValidDocuments(t *testing.T) {
	testCases := []struct {
		name          string
		fixture       string
		expectedTitle string
		validateFunc  func(t *testing.T, result normalize.NormalizedMarkdownDoc)
	}{
		{
			name:          "successful frontmatter generation",
			fixture:       "pass/success.md",
			expectedTitle: "Getting Started",
			validateFunc: func(t *testing.T, result normalize.NormalizedMarkdownDoc) {
				frontmatter := result.Frontmatter()

				if frontmatter.SourceURL() != "https://example.com/docs/page" {
					t.Errorf("expected sourceURL 'https://example.com/docs/page', got: %s", frontmatter.SourceURL())
				}

				if frontmatter.CrawlDepth() != 1 {
					t.Errorf("expected crawlDepth 1, got: %d", frontmatter.CrawlDepth())
				}

				if frontmatter.CrawlerVersion() != "v1.0.0" {
					t.Errorf("expected crawlerVersion 'v1.0.0', got: %s", frontmatter.CrawlerVersion())
				}

				if !strings.HasPrefix(frontmatter.DocID(), "sha256:") {
					t.Errorf("expected docID to have 'sha256:' prefix, got: %s", frontmatter.DocID())
				}

				if !strings.HasPrefix(frontmatter.ContentHash(), "sha256:") {
					t.Errorf("expected contentHash to have 'sha256:' prefix, got: %s", frontmatter.ContentHash())
				}
			},
		},
		{
			name:          "title with inline formatting stripped",
			fixture:       "pass/title_with_inline_formatting.md",
			expectedTitle: "Installing mytool now",
		},
		{
			name:          "valid heading levels progression",
			fixture:       "pass/valid_heading_levels.md",
			expectedTitle: "Main Title",
		},
		{
			name:          "content preserved unchanged",
			fixture:       "pass/content_preserved.md",
			expectedTitle: "Test Page",
			validateFunc: func(t *testing.T, result normalize.NormalizedMarkdownDoc) {
				expectedContent := loadFixture(t, "pass/content_preserved.md")
				if string(result.Content()) != string(expectedContent) {
					t.Errorf("content should be preserved unchanged\nexpected:\n%s\ngot:\n%s", string(expectedContent), string(result.Content()))
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			metadataSink := &metadataSinkMock{}
			constraint := normalize.NewMarkdownConstraint(metadataSink)

			fetchURL, _ := url.Parse("https://example.com/docs/page")
			content := loadFixture(t, tc.fixture)

			assetfulDoc := assets.NewAssetfulMarkdownDoc(content, nil, nil, nil)
			normalizeParam := normalize.NewNormalizeParam("v1.0.0", time.Now(), hashutil.HashAlgoSHA256, 1, nil)

			// Act
			result, err := constraint.Normalize(*fetchURL, assetfulDoc, normalizeParam)

			// Assert
			if err != nil {
				t.Fatalf("expected no error for %s, got: %v", tc.name, err)
			}

			if result.Frontmatter().Title() != tc.expectedTitle {
				t.Errorf("expected title '%s', got: '%s'", tc.expectedTitle, result.Frontmatter().Title())
			}

			if tc.validateFunc != nil {
				tc.validateFunc(t, result)
			}
		})
	}
}

func TestNormalize_SectionDerivation(t *testing.T) {
	testCases := []struct {
		name            string
		url             string
		prefixes        []string
		expectedSection string
		expectError     bool
	}{
		{
			name:            "simple path - no prefix",
			url:             "https://example.com/guide/page",
			prefixes:        nil,
			expectedSection: "guide",
			expectError:     false,
		},
		{
			name:            "nested path - no prefix",
			url:             "https://example.com/api/auth/login",
			prefixes:        nil,
			expectedSection: "api",
			expectError:     false,
		},
		{
			name:            "deep nested path - no prefix",
			url:             "https://example.com/docs/guides/tutorials/basic",
			prefixes:        nil,
			expectedSection: "docs",
			expectError:     false,
		},
		{
			name:            "root path only - error",
			url:             "https://example.com/",
			prefixes:        nil,
			expectedSection: "",
			expectError:     true,
		},
		{
			name:            "with matching prefix - strip docs",
			url:             "https://example.com/docs/guide/page",
			prefixes:        []string{"/docs"},
			expectedSection: "guide",
			expectError:     false,
		},
		{
			name:            "with matching prefix - strip api",
			url:             "https://example.com/api/v1/users",
			prefixes:        []string{"/api"},
			expectedSection: "v1",
			expectError:     false,
		},
		{
			name:            "with multi-segment prefix",
			url:             "https://example.com/docs/api/auth/login",
			prefixes:        []string{"/docs/api"},
			expectedSection: "auth",
			expectError:     false,
		},
		{
			name:            "prefix without leading slash",
			url:             "https://example.com/docs/page",
			prefixes:        []string{"docs"},
			expectedSection: "page",
			expectError:     false,
		},
		{
			name:            "no matching prefix - use first segment",
			url:             "https://example.com/other/page",
			prefixes:        []string{"/docs"},
			expectedSection: "other",
			expectError:     false,
		},
		{
			name:            "empty after prefix - error",
			url:             "https://example.com/docs/",
			prefixes:        []string{"/docs"},
			expectedSection: "",
			expectError:     true,
		},
		{
			name:            "multiple prefixes - first match wins",
			url:             "https://example.com/docs/api/page",
			prefixes:        []string{"/docs", "/docs/api"},
			expectedSection: "api",
			expectError:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			metadataSink := &metadataSinkMock{}
			constraint := normalize.NewMarkdownConstraint(metadataSink)

			fetchURL, _ := url.Parse(tc.url)
			content := loadFixture(t, "input/simple_test_page_short.md")

			assetfulDoc := assets.NewAssetfulMarkdownDoc(content, nil, nil, nil)
			normalizeParam := normalize.NewNormalizeParam("v1.0.0", time.Now(), hashutil.HashAlgoSHA256, 1, tc.prefixes)

			// Act
			result, err := constraint.Normalize(*fetchURL, assetfulDoc, normalizeParam)

			// Assert
			if tc.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !metadataSink.recordErrorCalled {
					t.Error("expected metadata sink RecordError to be called")
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			frontmatter := result.Frontmatter()
			if frontmatter.Section() != tc.expectedSection {
				t.Errorf("expected section '%s', got: '%s'", tc.expectedSection, frontmatter.Section())
			}
		})
	}
}

func TestNormalize_ContentHashDeterminism(t *testing.T) {
	// Arrange
	metadataSink := &metadataSinkMock{}
	constraint := normalize.NewMarkdownConstraint(metadataSink)

	fetchURL, _ := url.Parse("https://example.com/docs/page")
	content := loadFixture(t, "input/simple_test_page.md")

	assetfulDoc := assets.NewAssetfulMarkdownDoc(content, nil, nil, nil)
	normalizeParam := normalize.NewNormalizeParam("v1.0.0", time.Now(), hashutil.HashAlgoSHA256, 1, nil)

	// Act - run twice with same inputs
	result1, err1 := constraint.Normalize(*fetchURL, assetfulDoc, normalizeParam)
	result2, err2 := constraint.Normalize(*fetchURL, assetfulDoc, normalizeParam)

	// Assert
	if err1 != nil || err2 != nil {
		t.Fatalf("expected no errors, got: %v, %v", err1, err2)
	}

	// Content hash should be identical for identical content
	if result1.Frontmatter().ContentHash() != result2.Frontmatter().ContentHash() {
		t.Error("content hash should be deterministic for identical content")
	}

	// DocID should be identical for identical URL
	if result1.Frontmatter().DocID() != result2.Frontmatter().DocID() {
		t.Error("docID should be deterministic for identical URL")
	}

	// Content should be identical
	if string(result1.Content()) != string(result2.Content()) {
		t.Error("content should be identical between runs")
	}
}
