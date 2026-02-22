package assets_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/metadata/metadatatest"
	"github.com/rohmanhakim/docs-crawler/pkg/debug/debugtest"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/stretchr/testify/assert"
)

// metadataSinkMock is an alias to the shared mock for backward compatibility
type metadataSinkMock = metadatatest.SinkMock

// Tests for exported Resolve() method - deriving assertions from Resolve() output

func TestResolve_Success_WithAssets(t *testing.T) {
	// Arrange - create a mock HTTP server that returns a valid image response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-image-data"))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := server.URL + "/image.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "# Test\n\n![Alt text](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error should be returned when fetching succeeds
	assert.NoError(t, err)

	// Assert - RecordFetch should be called with correct parameters
	assert.True(t, mockSink.RecordFetchCalled, "RecordFetch should be called")
	records := mockSink.GetFetchRecords()
	assert.Len(t, records, 1, "Should have 1 fetch record")
	assert.Equal(t, imageURL, records[0].FetchURL())
	assert.Equal(t, http.StatusOK, records[0].HTTPStatus())
	assert.Equal(t, 0, records[0].RetryCount(), "Retry count should be 0 for successful first attempt")
	assert.Greater(t, records[0].Duration(), time.Duration(0), "Duration should be greater than 0")

	// Assert - RecordArtifact should be called for successful asset
	assert.True(t, mockSink.RecordArtifactCalled, "RecordArtifact should be called")
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1, "Should have 1 artifact record")
	assert.Equal(t, metadata.ArtifactAsset, artifactRecords[0].Kind())
	expectedHash := "28d81db19370f98fdc1d3e43fb1ef83a7cee62f3be86fed923d5f734da41319c"
	expectedLocalPath := buildExpectedPath("image", "28d81db19370f98fdc1d3e43fb1ef83a7cee62f3be86fed923d5f734da41319c", "png")
	assert.Equal(t, expectedLocalPath, artifactRecords[0].WritePath())
	// Verify SourceURL is the remote asset URL
	assert.Equal(t, imageURL, artifactRecords[0].SourceURL())

	// Assert - No RecordError should be called for successful asset
	assert.False(t, mockSink.RecordErrorCalled, "RecordError should not be called for successful asset")

	// Assert - writtenAssets should contain URL -> contentHash mapping
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 1, len(writtenAssets))
	assert.Equal(t, expectedHash, writtenAssets[imageURL], "Asset URL should map to content hash")

	// Assert - document content should have rewritten asset URL
	output := string(doc.Content())
	assert.Contains(t, output, expectedLocalPath, "Document should contain local asset path")
	assert.NotContains(t, output, imageURL, "Document should not contain original URL")
}

func TestResolve_Success_NoAssets(t *testing.T) {
	// Arrange
	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	conversionResult := mdconvert.NewConversionResult([]byte("# Test"), []mdconvert.LinkRef{})
	pageUrl, _ := url.Parse("https://example.com/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error should be returned when there are no assets to process
	assert.NoError(t, err)
	assert.Equal(t, "# Test", string(doc.Content()))

	// Assert - RecordFetch should NOT be called when there are no assets
	assert.False(t, mockSink.RecordFetchCalled, "RecordFetch should not be called when no assets")

	// Assert - RecordArtifact should NOT be called when there are no assets
	assert.False(t, mockSink.RecordArtifactCalled, "RecordArtifact should not be called when no assets")

	// Assert - RecordError should NOT be called when there are no assets
	assert.False(t, mockSink.RecordErrorCalled, "RecordError should not be called when no assets")
}

func TestResolve_Error_CreateAssetDirFails(t *testing.T) {
	// Arrange
	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	// Use an invalid path that cannot be created (simulating permission denied)
	invalidDir := "/nonexistent/path/that/cannot/be/created"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef("https://example.com/image.png", mdconvert.KindImage),
	}
	conversionResult := mdconvert.NewConversionResult([]byte("# Test"), linkRefs)
	pageUrl, _ := url.Parse("https://example.com/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(invalidDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	_, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - error should be returned when createAssetDir fails
	assert.Error(t, err)

	// Assert - RecordError should be called for write failure
	assert.True(t, mockSink.RecordErrorCalled, "RecordError should be called for write failure")
	errorRecords := mockSink.GetErrorRecords()
	assert.Len(t, errorRecords, 1, "Should have 1 error record")
	assert.Equal(t, "assets", errorRecords[0].PackageName())
	assert.Equal(t, "Resolver.Resolve", errorRecords[0].Action())
	assert.EqualValues(t, metadata.CauseStorageFailure, errorRecords[0].Cause())

	// Verify attrs contain write path and page URL
	assert.Len(t, errorRecords[0].Attrs(), 2)
	attrMap := make(map[string]string)
	for _, attr := range errorRecords[0].Attrs() {
		attrMap[string(attr.Key())] = attr.Value()
	}
	assert.Equal(t, invalidDir, attrMap["write_path"])
	assert.Equal(t, pageUrl.String(), attrMap["url"])

	// Assert - RecordArtifact should NOT be called when there's a write error
	assert.False(t, mockSink.RecordArtifactCalled, "RecordArtifact should not be called when write fails")
}

func TestResolve_AssetFetchFails_PreservesOriginalURL(t *testing.T) {
	// Arrange - create a mock HTTP server that returns 404 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := server.URL + "/missing-image.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "# Test\n\n![Alt text](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error should be returned from Resolve (missing assets are reported, not fatal)
	assert.NoError(t, err)

	// Assert - RecordFetch should still be called even on failure
	assert.True(t, mockSink.RecordFetchCalled, "RecordFetch should be called even on failure")
	records := mockSink.GetFetchRecords()
	assert.Len(t, records, 1, "Should have 1 fetch record even for failed fetch")

	// Assert - RecordError should be called for missing URL
	assert.True(t, mockSink.RecordErrorCalled, "RecordError should be called for missing URL")
	errorRecords := mockSink.GetErrorRecords()
	assert.Len(t, errorRecords, 1, "Should have 1 error record for missing URL")
	assert.True(t, errorRecords[0].Cause() == metadata.CausePolicyDisallow || errorRecords[0].Cause() == metadata.CauseUnknown, "Expected CausePolicyDisallow or CauseUnknown, got %v", errorRecords[0].Cause())
	assert.Contains(t, errorRecords[0].ErrorString(), "missing asset")

	// Verify attrs contain missing URL and page URL
	attrMap := make(map[string]string)
	for _, attr := range errorRecords[0].Attrs() {
		attrMap[string(attr.Key())] = attr.Value()
	}
	assert.Equal(t, imageURL, attrMap["asset_url"])
	assert.Equal(t, pageUrl.String(), attrMap["url"])

	// Assert - RecordArtifact should NOT be called for failed asset
	assert.False(t, mockSink.RecordArtifactCalled, "RecordArtifact should not be called for failed asset")

	// Assert - writtenAssets should NOT contain the failed asset URL
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 0, len(writtenAssets), "Failed asset should not be in writtenAssets")

	// Assert - document content should preserve original URL (not rewritten)
	output := string(doc.Content())
	assert.Contains(t, output, imageURL, "Document should preserve original URL for failed asset")
	assert.NotContains(t, output, "assets/images/", "Document should not contain local asset path for failed download")
}

func TestResolve_MixedSuccessAndFailure(t *testing.T) {
	// Arrange - create a mock HTTP server that succeeds for one asset and fails for another
	successImageData := []byte("success-image-data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "success") {
			w.WriteHeader(http.StatusOK)
			w.Write(successImageData)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	successURL := server.URL + "/success-image.png"
	failedURL := server.URL + "/failed-image.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(successURL, mdconvert.KindImage),
		mdconvert.NewLinkRef(failedURL, mdconvert.KindImage),
	}
	inputMarkdown := "# Test\n\n![Success](" + successURL + ")\n\n![Failed](" + failedURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - writtenAssets should only contain the successful asset's URL -> contentHash mapping
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 1, len(writtenAssets))
	expectedSuccessHash := "84a1b956adcdb84c7eda72df3a39b8034ffe0303733795d11ab73797bda03a24"
	assert.Equal(t, expectedSuccessHash, writtenAssets[successURL], "Successful asset's URL should map to content hash")

	// Assert - document content: successful asset rewritten, failed asset preserved
	output := string(doc.Content())
	expectedLocalPath := buildExpectedPath("success-image", "84a1b956adcdb84c7eda72df3a39b8034ffe0303733795d11ab73797bda03a24", "png")
	assert.Contains(t, output, expectedLocalPath, "Successful asset should be rewritten to local path")
	assert.Contains(t, output, failedURL, "Failed asset should preserve original URL")

	// Assert - RecordArtifact should be called for successful asset only
	assert.True(t, mockSink.RecordArtifactCalled, "RecordArtifact should be called")
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1, "Should have 1 artifact record for successful asset")
	assert.Equal(t, expectedLocalPath, artifactRecords[0].WritePath())

	// Assert - RecordError should be called for missing URL
	assert.True(t, mockSink.RecordErrorCalled, "RecordError should be called for missing URL")
	errorRecords := mockSink.GetErrorRecords()
	assert.Len(t, errorRecords, 1, "Should have 1 error record for missing URL")
	assert.True(t, errorRecords[0].Cause() == metadata.CausePolicyDisallow || errorRecords[0].Cause() == metadata.CauseUnknown, "Expected CausePolicyDisallow or CauseUnknown, got %v", errorRecords[0].Cause())

	// Verify attrs contain failed URL
	attrMap := make(map[string]string)
	for _, attr := range errorRecords[0].Attrs() {
		attrMap[string(attr.Key())] = attr.Value()
	}
	assert.Equal(t, failedURL, attrMap["asset_url"])
}

func TestResolve_MechanicalDeduplication_SinglePage(t *testing.T) {
	// Arrange - same URL appears multiple times in one document
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("image-data"))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := server.URL + "/image.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage), // duplicate
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage), // another duplicate
	}
	inputMarkdown := "![Img1](" + imageURL + ")\n\n![Img2](" + imageURL + ")\n\n![Img3](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - Only 1 fetch should be recorded (mechanical deduplication)
	records := mockSink.GetFetchRecords()
	assert.Len(t, records, 1, "Duplicate URLs should be mechanically deduplicated to single fetch")

	// Assert - All occurrences in document should be rewritten
	output := string(doc.Content())
	expectedLocalPath := buildExpectedPath("image", "2b700b7786d5a3f0cb487c3afaccb889fae829504a0ad1b70881e4643360f344", "png")
	assert.Equal(t, 3, strings.Count(output, expectedLocalPath), "All 3 occurrences should be rewritten")

	// Assert - RecordArtifact should be called once for the single successful asset
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1, "Should have 1 artifact record")

	// Assert - No RecordError should be called
	assert.False(t, mockSink.RecordErrorCalled, "RecordError should not be called for successful assets")
}

func TestResolve_CrossCallDeduplication(t *testing.T) {
	// Arrange - two Resolve() calls with same asset URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("shared-image-data"))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := server.URL + "/image.png"

	// First call
	linkRefs1 := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown1 := "![Img](" + imageURL + ")"
	conversionResult1 := mdconvert.NewConversionResult([]byte(inputMarkdown1), linkRefs1)
	pageUrl1, _ := url.Parse(server.URL + "/page1")

	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	_, err := resolver.Resolve(ctx, *pageUrl1, conversionResult1, resolveParam, testRetryParam())
	assert.NoError(t, err)

	// Assert first call has artifact record
	assert.True(t, mockSink.RecordArtifactCalled, "RecordArtifact should be called on first call")
	assert.Len(t, mockSink.GetArtifactRecords(), 1, "Should have 1 artifact record after first call")

	// Reset mock to track second call separately
	mockSink.Reset()

	// Second call with same image URL
	linkRefs2 := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown2 := "![Img2](" + imageURL + ")"
	conversionResult2 := mdconvert.NewConversionResult([]byte(inputMarkdown2), linkRefs2)
	pageUrl2, _ := url.Parse(server.URL + "/page2")

	// Act
	doc2, err := resolver.Resolve(ctx, *pageUrl2, conversionResult2, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - No fetch should be recorded for second call (asset already in writtenAssets)
	records := mockSink.GetFetchRecords()
	assert.Len(t, records, 0, "Second call should not fetch already-written asset")

	// Assert - RecordArtifact should NOT be called for second page (asset already exists)
	assert.False(t, mockSink.RecordArtifactCalled, "RecordArtifact should not be called on second call (asset already exists)")
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 0, "Should have 0 artifact records for second call (no new write)")

	// Assert - writtenAssets should still contain the URL
	writtenAssets := resolver.WrittenAssets()
	expectedHash := "70303f6f61d6d9c4123301a2c41b55d222e64966559243972abdd8083a341adc"
	assert.Equal(t, expectedHash, writtenAssets[imageURL])

	// Assert - document should still have rewritten URL
	output := string(doc2.Content())
	expectedLocalPath := buildExpectedPath("image", "70303f6f61d6d9c4123301a2c41b55d222e64966559243972abdd8083a341adc", "png")
	assert.Contains(t, output, expectedLocalPath)
}

func TestResolve_NonImageLinksIgnored(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("image-data"))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := server.URL + "/image.png"
	pageURL := server.URL + "/other-page"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
		mdconvert.NewLinkRef(pageURL, mdconvert.KindNavigation), // should be ignored
	}
	inputMarkdown := "# Test\n\n![Image](" + imageURL + ")\n\n[Link](" + pageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - Only 1 fetch should be recorded (only image, not navigation link)
	records := mockSink.GetFetchRecords()
	assert.Len(t, records, 1, "Only image links should be fetched")
	assert.True(t, strings.HasSuffix(records[0].FetchURL(), "/image.png"))

	// Assert - Only 1 artifact should be recorded
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1, "Only 1 artifact should be recorded")

	// Assert - Navigation link should remain unchanged in document
	output := string(doc.Content())
	assert.Contains(t, output, "[Link]("+pageURL+")", "Navigation link should remain unchanged")
}

func TestResolve_ContentHashDeduplication_DifferentURLs(t *testing.T) {
	// Arrange - two different URLs returning identical content
	sharedContent := []byte("shared-image-content")
	sharedContentHash := "69259521d1d859835a3bca63d4c4c741bc5a04095d3f07624d5dfdc1269120d2"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(sharedContent)
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	url1 := server.URL + "/image1.png"
	url2 := server.URL + "/image2.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(url1, mdconvert.KindImage),
		mdconvert.NewLinkRef(url2, mdconvert.KindImage),
	}
	inputMarkdown := "![Img1](" + url1 + ")\n\n![Img2](" + url2 + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Both URLs should be tracked in writtenAssets
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 2, len(writtenAssets), "Both URLs should be in writtenAssets")

	// Both URLs should have the same content hash (content-hash deduplication)
	assert.Equal(t, sharedContentHash, writtenAssets[url1], "First URL should map to content hash")
	assert.Equal(t, sharedContentHash, writtenAssets[url2], "Second URL should map to same content hash")

	// Both fetch events should be recorded (mechanical dedup doesn't apply to different URLs)
	records := mockSink.GetFetchRecords()
	assert.Len(t, records, 2, "Both assets should be fetched")

	// Only 1 artifact should be recorded (content-hash dedup - second URL uses existing file)
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1, "Should have 1 artifact record (content-hash deduplication)")

	// Document should have both images rewritten to same local path
	output := string(doc.Content())
	expectedLocalPath := buildExpectedPath("image1", sharedContentHash, "png")
	assert.Equal(t, 2, strings.Count(output, expectedLocalPath), "Both images should use same local path")
}

func TestResolve_RelativeURLsResolved(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("image-data"))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	// Use relative URL - will be resolved using pageUrl's host/scheme

	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef("/images/logo.png", mdconvert.KindImage),
	}
	inputMarkdown := "![Logo](/images/logo.png)"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act - use server's scheme and host so relative URL resolves correctly
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - fetch should be recorded with resolved absolute URL
	records := mockSink.GetFetchRecords()
	assert.Len(t, records, 1)
	assert.Equal(t, server.URL+"/images/logo.png", records[0].FetchURL())

	// Assert - RecordArtifact should be called
	assert.True(t, mockSink.RecordArtifactCalled, "RecordArtifact should be called")
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1)

	// Assert - document should have rewritten local path
	output := string(doc.Content())
	expectedLocalPath := buildExpectedPath("logo", "2b700b7786d5a3f0cb487c3afaccb889fae829504a0ad1b70881e4643360f344", "png")
	assert.Contains(t, output, expectedLocalPath)
}

// TestResolve_ContentHashDeduplication_DeterministicPath specifically tests that
// when two different URLs share the same content hash, both are rewritten to use
// the path from the first URL that was written to disk.
//
// This is a regression test for a bug where findPathByHash iterated over writtenAssets
// and could return a path rebuilt from a deduplicated URL that was never written,
// causing markdown to reference non-existent files due to Go's non-deterministic
// map iteration order.
func TestResolve_ContentHashDeduplication_DeterministicPath(t *testing.T) {
	// Arrange - two different URLs with different basenames returning identical content
	sharedContent := []byte("shared-deterministic-content")
	sharedContentHash := "ffaaa2898a4ab725c0f03011fa0c61ae102399fbe9d77ba3ab828d3f86e38a69"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(sharedContent)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	url1 := server.URL + "/logo.png"
	url2 := server.URL + "/different-name.jpg" // Different basename, same content

	// Pre-compute expected path based on first URL's basename
	expectedLocalPath := buildExpectedPath("logo", sharedContentHash, "png")

	// Run multiple times to verify deterministic behavior
	// (With the old implementation, this would occasionally fail due to map iteration randomness)
	for i := 0; i < 10; i++ {
		mockSink := &metadataSinkMock{}
		resolver := newTestResolver(mockSink)

		linkRefs := []mdconvert.LinkRef{
			mdconvert.NewLinkRef(url1, mdconvert.KindImage),
			mdconvert.NewLinkRef(url2, mdconvert.KindImage),
		}
		inputMarkdown := "![Img1](" + url1 + ")\n\n![Img2](" + url2 + ")"
		conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
		pageUrl, _ := url.Parse(server.URL + "/page")

		// Act
		ctx := context.Background()
		resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
		doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

		// Assert
		assert.NoError(t, err)

		// Both URLs should be tracked
		writtenAssets := resolver.WrittenAssets()
		assert.Equal(t, 2, len(writtenAssets), "Both URLs should be in writtenAssets")

		// Both images should be rewritten to the SAME path (from first written URL)
		output := string(doc.Content())
		assert.Equal(t, 2, strings.Count(output, expectedLocalPath),
			"Iteration %d: Both images should use deterministic path from first written URL (expected %s)",
			i+1, expectedLocalPath)

		// Should NOT contain a path built from the second URL's basename
		unexpectedPath := buildExpectedPath("different-name", sharedContentHash, "jpg")
		assert.NotContains(t, output, unexpectedPath,
			"Iteration %d: Should not contain path from second URL (which was deduplicated, not written)",
			i+1)
	}
}

func TestResolve_AssetTooLarge_ContentLengthHeader(t *testing.T) {
	// Arrange - create a mock HTTP server that returns Content-Length exceeding limit
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		w.Write(make([]byte, 1024))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := server.URL + "/large-image.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "![Large image](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act - use maxAssetSize of 512 bytes (less than Content-Length of 1024)
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 512, hashutil.HashAlgoSHA256) // 512 bytes limit
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error should be returned from Resolve (large assets are reported, not fatal)
	assert.NoError(t, err)

	// Assert - RecordError should be called for oversized asset
	assert.True(t, mockSink.RecordErrorCalled, "RecordError should be called for oversized asset")
	errorRecords := mockSink.GetErrorRecords()
	assert.Len(t, errorRecords, 1, "Should have 1 error record for oversized asset")
	assert.True(t, errorRecords[0].Cause() == metadata.CausePolicyDisallow || errorRecords[0].Cause() == metadata.CauseUnknown, "Expected CausePolicyDisallow or CauseUnknown, got %v", errorRecords[0].Cause())

	// Assert - RecordArtifact should NOT be called for oversized asset
	assert.False(t, mockSink.RecordArtifactCalled, "RecordArtifact should not be called for oversized asset")

	// Assert - writtenAssets should NOT contain the oversized asset URL
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 0, len(writtenAssets), "Oversized asset should not be in writtenAssets")

	// Assert - document content should preserve original URL (not rewritten)
	output := string(doc.Content())
	assert.Contains(t, output, imageURL, "Document should preserve original URL for oversized asset")
	assert.NotContains(t, output, "assets/images/", "Document should not contain local asset path for oversized asset")
}

func TestResolve_AssetTooLarge_UnknownContentLength(t *testing.T) {
	// Arrange - create a mock HTTP server that streams without Content-Length
	// (simulating chunked transfer encoding or omitted header)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't set Content-Length - this forces chunked encoding in httptest
		w.WriteHeader(http.StatusOK)
		// Write more than maxAssetSize
		w.Write(make([]byte, 1024))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := server.URL + "/streaming-image.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "![Streaming image](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act - use maxAssetSize of 512 bytes (less than streamed body of 1024)
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 512, hashutil.HashAlgoSHA256) // 512 bytes limit
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error should be returned from Resolve (large assets are reported, not fatal)
	assert.NoError(t, err)

	// Assert - RecordError should be called for oversized asset
	assert.True(t, mockSink.RecordErrorCalled, "RecordError should be called for oversized asset")
	errorRecords := mockSink.GetErrorRecords()
	assert.Len(t, errorRecords, 1, "Should have 1 error record for oversized asset")
	assert.True(t, errorRecords[0].Cause() == metadata.CausePolicyDisallow || errorRecords[0].Cause() == metadata.CauseUnknown, "Expected CausePolicyDisallow or CauseUnknown, got %v", errorRecords[0].Cause())

	// Assert - RecordArtifact should NOT be called for oversized asset
	assert.False(t, mockSink.RecordArtifactCalled, "RecordArtifact should not be called for oversized asset")

	// Assert - writtenAssets should NOT contain the oversized asset URL
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 0, len(writtenAssets), "Oversized asset should not be in writtenAssets")

	// Assert - document content should preserve original URL (not rewritten)
	output := string(doc.Content())
	assert.Contains(t, output, imageURL, "Document should preserve original URL for oversized asset")
	assert.NotContains(t, output, "assets/images/", "Document should not contain local asset path for oversized asset")
}

func TestResolve_AssetTooLarge_LyingContentLength(t *testing.T) {
	// Arrange - create a mock HTTP server that sends a small Content-Length
	// but streams a larger body (simulating malicious/misconfigured server)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Claim the content is small...
		w.Header().Set("Content-Length", "256")
		w.WriteHeader(http.StatusOK)
		// ...but actually write more (httptest will ignore this mismatch)
		w.Write(make([]byte, 1024))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	imageURL := server.URL + "/lying-image.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "![Lying image](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act - use maxAssetSize of 512 bytes (less than actual body of 1024)
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 512, hashutil.HashAlgoSHA256) // 512 bytes limit
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error should be returned from Resolve (large assets are reported, not fatal)
	assert.NoError(t, err)

	// Assert - RecordError should be called for oversized asset (caught by post-read check)
	// The error may be:
	// - CausePolicyDisallow (AssetTooLarge detected post-read)
	// - CauseRetryFailure (read error retried and exhausted, when Go's HTTP transport
	//   reports an error reading beyond Content-Length)
	// - CauseUnknown (other error paths)
	assert.True(t, mockSink.RecordErrorCalled, "RecordError should be called for oversized asset")
	errorRecords := mockSink.GetErrorRecords()
	assert.Len(t, errorRecords, 1, "Should have 1 error record for oversized asset")
	validCause := errorRecords[0].Cause() == metadata.CausePolicyDisallow ||
		errorRecords[0].Cause() == metadata.CauseUnknown ||
		errorRecords[0].Cause() == metadata.CauseRetryFailure ||
		errorRecords[0].Cause() == metadata.CauseContentInvalid
	assert.True(t, validCause, "Expected CausePolicyDisallow, CauseUnknown, CauseRetryFailure, or CauseContentInvalid, got %v", errorRecords[0].Cause())

	// Assert - RecordArtifact should NOT be called for oversized asset
	assert.False(t, mockSink.RecordArtifactCalled, "RecordArtifact should not be called for oversized asset")

	// Assert - writtenAssets should NOT contain the oversized asset URL
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 0, len(writtenAssets), "Oversized asset should not be in writtenAssets")

	// Assert - document content should preserve original URL (not rewritten)
	output := string(doc.Content())
	assert.Contains(t, output, imageURL, "Document should preserve original URL for oversized asset")
	assert.NotContains(t, output, "assets/images/", "Document should not contain local asset path for oversized asset")
}

func TestResolve_AssetAtSizeBoundary(t *testing.T) {
	// Test both boundary cases: exactly at limit and one byte over

	testCases := []struct {
		name          string
		bodySize      int64
		maxAssetSize  int64
		shouldSucceed bool
	}{
		{
			name:          "exactly at limit should succeed",
			bodySize:      512,
			maxAssetSize:  512,
			shouldSucceed: true,
		},
		{
			name:          "one byte over limit should fail",
			bodySize:      513,
			maxAssetSize:  512,
			shouldSucceed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange - server returns exact body size
			bodyData := make([]byte, tc.bodySize)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write(bodyData)
			}))
			defer server.Close()

			mockSink := &metadataSinkMock{}
			resolver := newTestResolver(mockSink)

			tempDir := t.TempDir()
			imageURL := server.URL + "/boundary-image.png"
			linkRefs := []mdconvert.LinkRef{
				mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
			}
			inputMarkdown := "![Boundary image](" + imageURL + ")"
			conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
			pageUrl, _ := url.Parse(server.URL + "/page")

			// Act
			ctx := context.Background()
			resolveParam := assets.NewResolveParam(tempDir, tc.maxAssetSize, hashutil.HashAlgoSHA256)
			doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

			// Assert
			assert.NoError(t, err) // Resolve never returns error for asset failures

			if tc.shouldSucceed {
				// Asset should be successfully downloaded
				assert.True(t, mockSink.RecordArtifactCalled, "RecordArtifact should be called for asset at boundary")
				assert.False(t, mockSink.RecordErrorCalled, "RecordError should not be called for asset at boundary")
				writtenAssets := resolver.WrittenAssets()
				assert.Equal(t, 1, len(writtenAssets), "Asset at boundary should be in writtenAssets")
				output := string(doc.Content())
				assert.NotContains(t, output, imageURL, "Document should have rewritten URL for asset at boundary")
			} else {
				// Asset should be rejected
				assert.False(t, mockSink.RecordArtifactCalled, "RecordArtifact should not be called for oversized asset")
				assert.True(t, mockSink.RecordErrorCalled, "RecordError should be called for oversized asset")
				writtenAssets := resolver.WrittenAssets()
				assert.Equal(t, 0, len(writtenAssets), "Oversized asset should not be in writtenAssets")
				output := string(doc.Content())
				assert.Contains(t, output, imageURL, "Document should preserve original URL for oversized asset")
			}
		})
	}
}

// TestResolve_MaxAssetSizeZero_Unlimited tests that when maxAssetSize is 0,
// assets are downloaded without any size limit (unlimited).
// This is a regression test for the bug where maxAssetSize=0 was treated as
// a 0-byte limit, causing all assets to be rejected.
func TestResolve_MaxAssetSizeZero_Unlimited(t *testing.T) {
	// Test cases for maxAssetSize = 0 (unlimited)
	testCases := []struct {
		name          string
		bodySize      int64
		maxAssetSize  int64
		shouldSucceed bool
	}{
		{
			name:          "zero means unlimited - small asset",
			bodySize:      100,
			maxAssetSize:  0,
			shouldSucceed: true,
		},
		{
			name:          "zero means unlimited - large asset",
			bodySize:      1024,
			maxAssetSize:  0,
			shouldSucceed: true,
		},
		{
			name:          "zero means unlimited - very large asset",
			bodySize:      10 * 1024 * 1024, // 10MB
			maxAssetSize:  0,
			shouldSucceed: true,
		},
		{
			name:          "zero means unlimited - with Content-Length header",
			bodySize:      512,
			maxAssetSize:  0,
			shouldSucceed: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange - server returns a body of the specified size
			bodyData := make([]byte, tc.bodySize)
			for i := range bodyData {
				bodyData[i] = byte(i % 256)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Set Content-Length header to verify it's ignored when maxAssetSize=0
				w.Header().Set("Content-Length", fmt.Sprintf("%d", tc.bodySize))
				w.WriteHeader(http.StatusOK)
				w.Write(bodyData)
			}))
			defer server.Close()

			mockSink := &metadataSinkMock{}
			resolver := newTestResolver(mockSink)

			tempDir := t.TempDir()
			imageURL := server.URL + "/large-image.png"
			linkRefs := []mdconvert.LinkRef{
				mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
			}
			inputMarkdown := "![Large image](" + imageURL + ")"
			conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
			pageUrl, _ := url.Parse(server.URL + "/page")

			// Act - use maxAssetSize of 0 (should mean unlimited)
			ctx := context.Background()
			resolveParam := assets.NewResolveParam(tempDir, tc.maxAssetSize, hashutil.HashAlgoSHA256)
			doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

			// Assert - no error from Resolve
			assert.NoError(t, err)

			if tc.shouldSucceed {
				// Asset should be successfully downloaded (no size limit)
				assert.True(t, mockSink.RecordArtifactCalled, "RecordArtifact should be called for unlimited size")
				assert.False(t, mockSink.RecordErrorCalled, "RecordError should not be called for unlimited size")
				writtenAssets := resolver.WrittenAssets()
				assert.Equal(t, 1, len(writtenAssets), "Asset should be in writtenAssets")

				// Verify the entire body was read correctly
				artifactRecords := mockSink.GetArtifactRecords()
				assert.Len(t, artifactRecords, 1)
				assert.Equal(t, tc.bodySize, artifactRecords[0].Bytes(), "All bytes should be downloaded")

				// Document should have rewritten URL
				output := string(doc.Content())
				assert.NotContains(t, output, imageURL, "Document should have rewritten URL")
				assert.Contains(t, output, "assets/images/", "Document should contain local asset path")
			}
		})
	}
}

// TestResolve_QueryParamsPreserved tests that query parameters (like CDN signatures)
// are preserved during fetching while still allowing deduplication via canonical URL.
// This is a regression test for the bug where query params were stripped from the
// fetch URL, causing CDN-signed URLs to fail with 403 errors.
func TestResolve_QueryParamsPreserved(t *testing.T) {
	// Arrange - server expects the full URL with query params
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify that the request URL contains the query parameters
		query := r.URL.RawQuery
		if query == "" {
			t.Error("Expected query parameters in request URL, but got none")
		}
		// Check for expected query params (signature and resize params)
		if !strings.Contains(query, "s=") && !strings.Contains(query, "signature") {
			t.Errorf("Expected signature parameter in query, got: %s", query)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("signed-image-data"))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	// Image URL with CDN signature and other query params
	imageURL := server.URL + "/image.png?fit=max&auto=format&q=85&s=35268aa0ad50b8c385913810e7604550"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "![Signed image](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error
	assert.NoError(t, err)

	// Assert - fetch was called (the test server verifies query params are present)
	records := mockSink.GetFetchRecords()
	assert.Len(t, records, 1, "Should have 1 fetch record")

	// Verify the fetch URL contains the query parameters (this is the key fix!)
	assert.Contains(t, records[0].FetchURL(), "s=35268aa0ad50b8c385913810e7604550",
		"Fetch URL should contain signature parameter")

	// Assert - RecordArtifact should be called (successful fetch with params)
	assert.True(t, mockSink.RecordArtifactCalled, "RecordArtifact should be called for successful fetch")

	// Assert - writtenAssets should contain the CANONICAL URL (without query params)
	// This is the correct behavior: we store using canonical key for consistent lookups
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 1, len(writtenAssets), "Should have 1 written asset")
	// The key should be the canonical URL (without query params)
	assert.Contains(t, writtenAssets, server.URL+"/image.png",
		"writtenAssets should contain canonical URL (without query params)")

	// Assert - document should have rewritten URL
	output := string(doc.Content())
	assert.NotContains(t, output, "s=35268aa0ad50b8c385913810e7604550", "Document should have local path, not original URL")
	assert.Contains(t, output, "assets/images/", "Document should contain local asset path")
}

// TestResolve_DebugLogging_SuccessfulFetch tests that debug logging is called correctly
// for a successful asset fetch.
func TestResolve_DebugLogging_SuccessfulFetch(t *testing.T) {
	// Arrange - create a mock HTTP server that returns a valid image response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-image-data"))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	mockLogger := debugtest.NewLoggerMock()
	resolver := newTestResolverWithLogger(mockSink, mockLogger)

	tempDir := t.TempDir()
	imageURL := server.URL + "/image.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "# Test\n\n![Alt text](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	_, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error
	assert.NoError(t, err)

	// Assert - debug logging was called
	assert.True(t, mockLogger.LogStepCalled, "LogStep should be called")

	// Verify key steps are present
	steps := mockLogger.StepsByStage("assets")
	assert.GreaterOrEqual(t, len(steps), 4, "Should have at least 4 debug steps")

	// Verify extract_image_urls step
	extractSteps := mockLogger.StepsByName("extract_image_urls")
	assert.Equal(t, 1, len(extractSteps), "Should have 1 extract_image_urls step")
	assert.Equal(t, 1, extractSteps[0].Fields["count"], "Should have 1 image URL")

	// Verify deduplicate_assets step
	dedupSteps := mockLogger.StepsByName("deduplicate_assets")
	assert.Equal(t, 1, len(dedupSteps), "Should have 1 deduplicate_assets step")
	assert.Equal(t, 1, dedupSteps[0].Fields["input_count"], "Input count should be 1")
	assert.Equal(t, 1, dedupSteps[0].Fields["output_count"], "Output count should be 1")

	// Verify resolve_asset step
	resolveSteps := mockLogger.StepsByName("resolve_asset")
	assert.Equal(t, 1, len(resolveSteps), "Should have 1 resolve_asset step")
	assert.Equal(t, imageURL, resolveSteps[0].Fields["asset_url"], "Asset URL should match")

	// Verify asset_fetched step
	fetchedSteps := mockLogger.StepsByName("asset_fetched")
	assert.Equal(t, 1, len(fetchedSteps), "Should have 1 asset_fetched step")
	assert.Equal(t, http.StatusOK, fetchedSteps[0].Fields["status_code"], "Status code should be 200")

	// Verify asset_written step
	writtenSteps := mockLogger.StepsByName("asset_written")
	assert.Equal(t, 1, len(writtenSteps), "Should have 1 asset_written step")
	assert.Contains(t, writtenSteps[0].Fields["local_path"], "assets/images/", "Local path should contain assets/images/")
}

// TestResolve_DebugLogging_FailedFetch tests that debug logging is called correctly
// for a failed asset fetch.
func TestResolve_DebugLogging_FailedFetch(t *testing.T) {
	// Arrange - create a mock HTTP server that returns 404 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	mockLogger := debugtest.NewLoggerMock()
	resolver := newTestResolverWithLogger(mockSink, mockLogger)

	tempDir := t.TempDir()
	imageURL := server.URL + "/missing-image.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(imageURL, mdconvert.KindImage),
	}
	inputMarkdown := "# Test\n\n![Alt text](" + imageURL + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	_, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error from Resolve (missing assets are reported, not fatal)
	assert.NoError(t, err)

	// Verify asset_failed step is logged
	failedSteps := mockLogger.StepsByName("asset_failed")
	assert.Equal(t, 1, len(failedSteps), "Should have 1 asset_failed step")
	assert.Equal(t, imageURL, failedSteps[0].Fields["asset_url"], "Asset URL should match")
	assert.Contains(t, failedSteps[0].Fields["error"], "client error: 404", "Error message should contain 404")
}

// TestResolve_DebugLogging_ContentHashDedup tests that debug logging is called correctly
// for content-hash deduplication.
func TestResolve_DebugLogging_ContentHashDedup(t *testing.T) {
	// Arrange - two different URLs returning identical content
	sharedContent := []byte("shared-image-content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(sharedContent)
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	mockLogger := debugtest.NewLoggerMock()
	resolver := newTestResolverWithLogger(mockSink, mockLogger)

	tempDir := t.TempDir()
	url1 := server.URL + "/image1.png"
	url2 := server.URL + "/image2.png"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(url1, mdconvert.KindImage),
		mdconvert.NewLinkRef(url2, mdconvert.KindImage),
	}
	inputMarkdown := "![Img1](" + url1 + ")\n\n![Img2](" + url2 + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	_, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error
	assert.NoError(t, err)

	// Verify asset_content_dedup step is logged for second URL
	dedupSteps := mockLogger.StepsByName("asset_content_dedup")
	assert.Equal(t, 1, len(dedupSteps), "Should have 1 asset_content_dedup step for second URL")
	assert.Contains(t, dedupSteps[0].Fields["asset_url"], "/image2.png", "Dedup should be for second URL")
	assert.NotEmpty(t, dedupSteps[0].Fields["existing_path"], "Should have existing_path")
}

// TestResolve_QueryParamsDeduplication tests that URLs with different query params
// but same base path are deduplicated (mechanical dedup), while still fetching
// the correct URL for each.
func TestResolve_QueryParamsDeduplication(t *testing.T) {
	// Track which URLs were requested
	var requestedURLs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedURLs = append(requestedURLs, r.URL.String())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("image-data"))
	}))
	defer server.Close()

	mockSink := &metadataSinkMock{}
	resolver := newTestResolver(mockSink)

	tempDir := t.TempDir()
	// Same base image, different resize params (should be deduplicated)
	url1 := server.URL + "/image.png?w=100"
	url2 := server.URL + "/image.png?w=200"
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef(url1, mdconvert.KindImage),
		mdconvert.NewLinkRef(url2, mdconvert.KindImage),
	}
	inputMarkdown := "![Img1](" + url1 + ")\n\n![Img2](" + url2 + ")"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	doc, err := resolver.Resolve(ctx, *pageUrl, conversionResult, resolveParam, testRetryParam())

	// Assert - no error
	assert.NoError(t, err)

	// Assert - only ONE fetch should be recorded (mechanical deduplication by canonical URL)
	records := mockSink.GetFetchRecords()
	assert.Len(t, records, 1, "Different query params should be mechanically deduplicated to single fetch")

	// Assert - the fetch URL should be one of the original URLs (with params)
	// Note: Due to map iteration order, either could be fetched
	assert.True(t, records[0].FetchURL() == url1 || records[0].FetchURL() == url2,
		"Fetch URL should be one of the input URLs with params, got: %s", records[0].FetchURL())

	// Assert - only ONE entry in writtenAssets (both URLs map to same canonical key)
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 1, len(writtenAssets), "Different query params should map to same canonical key")

	// Assert - the key should be the canonical URL (without query params)
	assert.Contains(t, writtenAssets, server.URL+"/image.png",
		"writtenAssets should contain canonical URL")

	// Assert - document should have both rewritten to same local path
	output := string(doc.Content())
	// Both should use the same local path (first one written)
	assert.Equal(t, 2, strings.Count(output, "assets/images/"),
		"Both images should be rewritten to local paths")
}
