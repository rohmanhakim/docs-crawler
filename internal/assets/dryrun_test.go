package assets_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata/metadatatest"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/stretchr/testify/assert"
)

// newDryRunTestResolver creates a DryRunResolver with test dependencies
func newDryRunTestResolver(mockSink *metadatatest.SinkMock) *assets.DryRunResolver {
	resolver := assets.NewDryRunResolver(mockSink)
	resolver.Init(http.DefaultClient, "test-user-agent")
	return resolver
}

func TestDryRunResolver_Resolve_NoNetworkRequests(t *testing.T) {
	// Setup
	mockSink := &metadatatest.SinkMock{}
	resolver := newDryRunTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := "https://example.com/images/logo.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "# Test Document\n\n![image](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageURL := url.URL{Scheme: "https", Host: "example.com", Path: "/docs/page"}

	// Execute
	resolveParam := assets.NewResolveParam(tempDir, 0, hashutil.HashAlgoSHA256)
	result, err := resolver.Resolve(context.Background(), pageURL, conversionResult, resolveParam, testRetryOptions())

	// Verify
	assert.NoError(t, err)

	// Verify content is returned (with rewritten link)
	assert.True(t, len(result.Content()) > 0, "expected non-empty content")

	// Verify no missing assets (dry-run simulates success)
	assert.Equal(t, 0, len(result.MissingAssets()), "expected no missing assets in dry-run")

	// Verify local assets are tracked
	assert.Equal(t, 1, len(result.LocalAssets()), "expected 1 local asset")

	// Verify the asset path starts with assets/images/
	if len(result.LocalAssets()) > 0 {
		localPath := result.LocalAssets()[0]
		assert.True(t, strings.HasPrefix(localPath, "assets/images/"),
			"expected local path to start with 'assets/images/', got: %s", localPath)
	}

	// Verify RecordArtifact was called
	assert.True(t, mockSink.RecordArtifactCalled, "RecordArtifact should be called")
}

func TestDryRunResolver_Resolve_DeduplicatesIdenticalURLs(t *testing.T) {
	// Setup
	mockSink := &metadatatest.SinkMock{}
	resolver := newDryRunTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := "https://example.com/images/logo.png"
	// Same image referenced twice
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "![img1](" + imageURL + ")\n\n![img2](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageURL := url.URL{Scheme: "https", Host: "example.com", Path: "/docs/page"}

	// Execute
	resolveParam := assets.NewResolveParam(tempDir, 0, hashutil.HashAlgoSHA256)
	result, err := resolver.Resolve(context.Background(), pageURL, conversionResult, resolveParam, testRetryOptions())

	// Verify
	assert.NoError(t, err)

	// Should only have 1 local asset (deduplicated)
	assert.Equal(t, 1, len(result.LocalAssets()), "expected 1 local asset after deduplication")

	// Verify writtenAssets map has only 1 entry
	assert.Equal(t, 1, len(resolver.WrittenAssets()), "expected 1 written asset entry")

	// Only 1 artifact record (deduplicated)
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Equal(t, 1, len(artifactRecords), "expected 1 artifact record after deduplication")
}

func TestDryRunResolver_Resolve_RelativeURLs(t *testing.T) {
	// Setup
	mockSink := &metadatatest.SinkMock{}
	resolver := newDryRunTestResolver(mockSink)

	tempDir := t.TempDir()
	// Relative image URL
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef("/images/relative.png", mdconvert.KindImage),
	}
	inputMarkdown := "![relative](/images/relative.png)"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageURL := url.URL{Scheme: "https", Host: "example.com", Path: "/docs/page"}

	// Execute
	resolveParam := assets.NewResolveParam(tempDir, 0, hashutil.HashAlgoSHA256)
	result, err := resolver.Resolve(context.Background(), pageURL, conversionResult, resolveParam, testRetryOptions())

	// Verify
	assert.NoError(t, err)

	// Should have 1 local asset
	assert.Equal(t, 1, len(result.LocalAssets()), "expected 1 local asset")
}

func TestDryRunResolver_Resolve_MultiplePages_ShareAssetCache(t *testing.T) {
	// Setup
	mockSink := &metadatatest.SinkMock{}
	resolver := newDryRunTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := "https://example.com/images/shared.png"
	resolveParam := assets.NewResolveParam(tempDir, 0, hashutil.HashAlgoSHA256)

	// First page with image
	linkRefs1 := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown1 := "![shared](" + imageURL + ")"
	conversionResult1 := mdconvert.NewConversionResult([]byte(inputMarkdown1), linkRefs1)
	pageURL1 := url.URL{Scheme: "https", Host: "example.com", Path: "/docs/page1"}
	result1, err := resolver.Resolve(context.Background(), pageURL1, conversionResult1, resolveParam, testRetryOptions())
	assert.NoError(t, err)

	// Second page with same image
	linkRefs2 := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown2 := "![shared](" + imageURL + ")"
	conversionResult2 := mdconvert.NewConversionResult([]byte(inputMarkdown2), linkRefs2)
	pageURL2 := url.URL{Scheme: "https", Host: "example.com", Path: "/docs/page2"}
	result2, err := resolver.Resolve(context.Background(), pageURL2, conversionResult2, resolveParam, testRetryOptions())
	assert.NoError(t, err)

	// Both should have 1 local asset
	assert.Equal(t, 1, len(result1.LocalAssets()), "expected 1 local asset on page1")
	assert.Equal(t, 1, len(result2.LocalAssets()), "expected 1 local asset on page2")

	// Both should have the same path (deduplicated across pages)
	assert.Equal(t, result1.LocalAssets()[0], result2.LocalAssets()[0],
		"expected same asset path for shared image")

	// Should only have 1 entry in writtenAssets
	assert.Equal(t, 1, len(resolver.WrittenAssets()),
		"expected 1 written asset entry after processing both pages")
}

func TestDryRunResolver_Resolve_UnparseableURLs(t *testing.T) {
	// Setup
	mockSink := &metadatatest.SinkMock{}
	resolver := newDryRunTestResolver(mockSink)

	tempDir := t.TempDir()

	// Create a conversion result with an unparseable URL
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef("://invalid-url", mdconvert.KindImage), // Invalid URL
	}
	inputMarkdown := "# Test\n\n![bad](://invalid-url)"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageURL := url.URL{Scheme: "https", Host: "example.com", Path: "/docs/page"}

	// Execute
	resolveParam := assets.NewResolveParam(tempDir, 0, hashutil.HashAlgoSHA256)
	result, err := resolver.Resolve(context.Background(), pageURL, conversionResult, resolveParam, testRetryOptions())

	// Verify
	assert.NoError(t, err)

	// Should have unparseable URLs recorded
	assert.Equal(t, 1, len(result.UnparseableURLs()), "expected 1 unparseable URL")

	// Should have RecordError called for unparseable URL
	assert.True(t, mockSink.RecordErrorCalled, "RecordError should be called for unparseable URL")
}

func TestDryRunResolver_Resolve_DeterministicPaths(t *testing.T) {
	// Setup - same image should produce same path across different runs
	mockSink1 := &metadatatest.SinkMock{}
	resolver1 := newDryRunTestResolver(mockSink1)

	mockSink2 := &metadatatest.SinkMock{}
	resolver2 := newDryRunTestResolver(mockSink2)

	tempDir := t.TempDir()
	imageURL := "https://example.com/images/deterministic.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "![det](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageURL := url.URL{Scheme: "https", Host: "example.com", Path: "/docs/page"}
	resolveParam := assets.NewResolveParam(tempDir, 0, hashutil.HashAlgoSHA256)

	// Execute both resolvers
	result1, _ := resolver1.Resolve(context.Background(), pageURL, conversionResult, resolveParam, testRetryOptions())
	result2, _ := resolver2.Resolve(context.Background(), pageURL, conversionResult, resolveParam, testRetryOptions())

	// Verify paths are identical
	assert.Equal(t, result1.LocalAssets()[0], result2.LocalAssets()[0],
		"expected deterministic paths across different resolver instances")
}
