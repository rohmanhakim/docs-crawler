package assets_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/stretchr/testify/assert"
)

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
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
	doc, err := resolver.Resolve(ctx, *pageUrl, "example.com", "https", conversionResult, resolveParam, testRetryParam())

	// Assert - no error should be returned when fetching succeeds
	assert.NoError(t, err)

	// Assert - RecordAssetFetch should be called with correct parameters
	assert.True(t, mockSink.recordAssetFetchCalled, "RecordAssetFetch should be called")
	records := mockSink.GetAssetFetchRecords()
	assert.Len(t, records, 1, "Should have 1 asset fetch record")
	assert.Equal(t, imageURL, records[0].FetchUrl)
	assert.Equal(t, http.StatusOK, records[0].HTTPStatus)
	assert.Equal(t, 0, records[0].RetryCount, "Retry count should be 0 for successful first attempt")
	assert.Greater(t, records[0].Duration, time.Duration(0), "Duration should be greater than 0")

	// Assert - RecordArtifact should be called for successful asset
	assert.True(t, mockSink.recordArtifactCalled, "RecordArtifact should be called")
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1, "Should have 1 artifact record")
	assert.Equal(t, metadata.ArtifactAsset, artifactRecords[0].Kind)
	expectedHash := computeHash([]byte("fake-image-data"))
	expectedLocalPath := buildExpectedPath("image", []byte("fake-image-data"), "png")
	assert.Equal(t, expectedLocalPath, artifactRecords[0].Path)
	// Verify attrs contain page URL
	assert.Len(t, artifactRecords[0].Attrs, 1)
	assert.Equal(t, metadata.AttrURL, artifactRecords[0].Attrs[0].Key)
	assert.Equal(t, pageUrl.String(), artifactRecords[0].Attrs[0].Value)

	// Assert - No RecordError should be called for successful asset
	assert.False(t, mockSink.recordErrorCalled, "RecordError should not be called for successful asset")

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
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
	doc, err := resolver.Resolve(ctx, *pageUrl, "example.com", "https", conversionResult, resolveParam, testRetryParam())

	// Assert - no error should be returned when there are no assets to process
	assert.NoError(t, err)
	assert.Equal(t, "# Test", string(doc.Content()))

	// Assert - RecordAssetFetch should NOT be called when there are no assets
	assert.False(t, mockSink.recordAssetFetchCalled, "RecordAssetFetch should not be called when no assets")

	// Assert - RecordArtifact should NOT be called when there are no assets
	assert.False(t, mockSink.recordArtifactCalled, "RecordArtifact should not be called when no assets")

	// Assert - RecordError should NOT be called when there are no assets
	assert.False(t, mockSink.recordErrorCalled, "RecordError should not be called when no assets")
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
	resolveParam := assets.NewResolveParam(invalidDir, 10*1024*1024)
	_, err := resolver.Resolve(ctx, *pageUrl, "example.com", "https", conversionResult, resolveParam, testRetryParam())

	// Assert - error should be returned when createAssetDir fails
	assert.Error(t, err)

	// Assert - RecordError should be called for write failure
	assert.True(t, mockSink.recordErrorCalled, "RecordError should be called for write failure")
	errorRecords := mockSink.GetErrorRecords()
	assert.Len(t, errorRecords, 1, "Should have 1 error record")
	assert.Equal(t, "assets", errorRecords[0].PackageName)
	assert.Equal(t, "Resolver.Resolve", errorRecords[0].Action)
	assert.EqualValues(t, metadata.CauseStorageFailure, errorRecords[0].Cause)

	// Verify attrs contain write path and page URL
	assert.Len(t, errorRecords[0].Attrs, 2)
	attrMap := make(map[string]string)
	for _, attr := range errorRecords[0].Attrs {
		attrMap[string(attr.Key)] = attr.Value
	}
	assert.Equal(t, invalidDir, attrMap["write_path"])
	assert.Equal(t, pageUrl.String(), attrMap["url"])

	// Assert - RecordArtifact should NOT be called when there's a write error
	assert.False(t, mockSink.recordArtifactCalled, "RecordArtifact should not be called when write fails")
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
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
	doc, err := resolver.Resolve(ctx, *pageUrl, "example.com", "https", conversionResult, resolveParam, testRetryParam())

	// Assert - no error should be returned from Resolve (missing assets are reported, not fatal)
	assert.NoError(t, err)

	// Assert - RecordAssetFetch should still be called even on failure
	assert.True(t, mockSink.recordAssetFetchCalled, "RecordAssetFetch should be called even on failure")
	records := mockSink.GetAssetFetchRecords()
	assert.Len(t, records, 1, "Should have 1 asset fetch record even for failed fetch")

	// Assert - RecordError should be called for missing URL
	assert.True(t, mockSink.recordErrorCalled, "RecordError should be called for missing URL")
	errorRecords := mockSink.GetErrorRecords()
	assert.Len(t, errorRecords, 1, "Should have 1 error record for missing URL")
	assert.EqualValues(t, metadata.CauseNetworkFailure, errorRecords[0].Cause)
	assert.Contains(t, errorRecords[0].Details, "missing asset")

	// Verify attrs contain missing URL and page URL
	attrMap := make(map[string]string)
	for _, attr := range errorRecords[0].Attrs {
		attrMap[string(attr.Key)] = attr.Value
	}
	assert.Equal(t, imageURL, attrMap["message"])
	assert.Equal(t, pageUrl.String(), attrMap["url"])

	// Assert - RecordArtifact should NOT be called for failed asset
	assert.False(t, mockSink.recordArtifactCalled, "RecordArtifact should not be called for failed asset")

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
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
	doc, err := resolver.Resolve(ctx, *pageUrl, "example.com", "https", conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - writtenAssets should only contain the successful asset's URL -> contentHash mapping
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 1, len(writtenAssets))
	expectedSuccessHash := computeHash(successImageData)
	assert.Equal(t, expectedSuccessHash, writtenAssets[successURL], "Successful asset's URL should map to content hash")

	// Assert - document content: successful asset rewritten, failed asset preserved
	output := string(doc.Content())
	expectedLocalPath := buildExpectedPath("success-image", successImageData, "png")
	assert.Contains(t, output, expectedLocalPath, "Successful asset should be rewritten to local path")
	assert.Contains(t, output, failedURL, "Failed asset should preserve original URL")

	// Assert - RecordArtifact should be called for successful asset only
	assert.True(t, mockSink.recordArtifactCalled, "RecordArtifact should be called")
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1, "Should have 1 artifact record for successful asset")
	assert.Equal(t, expectedLocalPath, artifactRecords[0].Path)

	// Assert - RecordError should be called for missing URL
	assert.True(t, mockSink.recordErrorCalled, "RecordError should be called for missing URL")
	errorRecords := mockSink.GetErrorRecords()
	assert.Len(t, errorRecords, 1, "Should have 1 error record for missing URL")
	assert.EqualValues(t, metadata.CauseNetworkFailure, errorRecords[0].Cause)

	// Verify attrs contain failed URL
	attrMap := make(map[string]string)
	for _, attr := range errorRecords[0].Attrs {
		attrMap[string(attr.Key)] = attr.Value
	}
	assert.Equal(t, failedURL, attrMap["message"])
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
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
	doc, err := resolver.Resolve(ctx, *pageUrl, "example.com", "https", conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - Only 1 fetch should be recorded (mechanical deduplication)
	records := mockSink.GetAssetFetchRecords()
	assert.Len(t, records, 1, "Duplicate URLs should be mechanically deduplicated to single fetch")

	// Assert - All occurrences in document should be rewritten
	output := string(doc.Content())
	expectedLocalPath := buildExpectedPath("image", []byte("image-data"), "png")
	assert.Equal(t, 3, strings.Count(output, expectedLocalPath), "All 3 occurrences should be rewritten")

	// Assert - RecordArtifact should be called once for the single successful asset
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1, "Should have 1 artifact record")

	// Assert - No RecordError should be called
	assert.False(t, mockSink.recordErrorCalled, "RecordError should not be called for successful assets")
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
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
	_, err := resolver.Resolve(ctx, *pageUrl1, "example.com", "https", conversionResult1, resolveParam, testRetryParam())
	assert.NoError(t, err)

	// Assert first call has artifact record
	assert.True(t, mockSink.recordArtifactCalled, "RecordArtifact should be called on first call")
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
	doc2, err := resolver.Resolve(ctx, *pageUrl2, "example.com", "https", conversionResult2, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - No fetch should be recorded for second call (asset already in writtenAssets)
	records := mockSink.GetAssetFetchRecords()
	assert.Len(t, records, 0, "Second call should not fetch already-written asset")

	// Assert - RecordArtifact should NOT be called for second page (asset already exists)
	assert.False(t, mockSink.recordArtifactCalled, "RecordArtifact should not be called on second call (asset already exists)")
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 0, "Should have 0 artifact records for second call (no new write)")

	// Assert - writtenAssets should still contain the URL
	writtenAssets := resolver.WrittenAssets()
	expectedHash := computeHash([]byte("shared-image-data"))
	assert.Equal(t, expectedHash, writtenAssets[imageURL])

	// Assert - document should still have rewritten URL
	output := string(doc2.Content())
	expectedLocalPath := buildExpectedPath("image", []byte("shared-image-data"), "png")
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
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
	doc, err := resolver.Resolve(ctx, *pageUrl, "example.com", "https", conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - Only 1 fetch should be recorded (only image, not navigation link)
	records := mockSink.GetAssetFetchRecords()
	assert.Len(t, records, 1, "Only image links should be fetched")
	assert.True(t, strings.HasSuffix(records[0].FetchUrl, "/image.png"))

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
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
	doc, err := resolver.Resolve(ctx, *pageUrl, "example.com", "https", conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Both URLs should be tracked in writtenAssets
	writtenAssets := resolver.WrittenAssets()
	assert.Equal(t, 2, len(writtenAssets), "Both URLs should be in writtenAssets")

	// Both URLs should have the same content hash (content-hash deduplication)
	expectedHash := computeHash(sharedContent)
	assert.Equal(t, expectedHash, writtenAssets[url1], "First URL should map to content hash")
	assert.Equal(t, expectedHash, writtenAssets[url2], "Second URL should map to same content hash")

	// Both fetch events should be recorded (mechanical dedup doesn't apply to different URLs)
	records := mockSink.GetAssetFetchRecords()
	assert.Len(t, records, 2, "Both assets should be fetched")

	// Only 1 artifact should be recorded (content-hash dedup - second URL uses existing file)
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1, "Should have 1 artifact record (content-hash deduplication)")

	// Document should have both images rewritten to same local path
	output := string(doc.Content())
	expectedLocalPath := buildExpectedPath("image1", sharedContent, "png")
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
	// Parse server URL to extract host for the test
	serverURL, _ := url.Parse(server.URL)
	// Use relative URL - will be resolved using server's host
	linkRefs := []mdconvert.LinkRef{
		mdconvert.NewLinkRef("/images/logo.png", mdconvert.KindImage),
	}
	inputMarkdown := "![Logo](/images/logo.png)"
	conversionResult := mdconvert.NewConversionResult([]byte(inputMarkdown), linkRefs)
	pageUrl, _ := url.Parse(server.URL + "/page")

	// Act - use server's scheme and host so relative URL resolves correctly
	ctx := context.Background()
	resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
	doc, err := resolver.Resolve(ctx, *pageUrl, serverURL.Host, serverURL.Scheme, conversionResult, resolveParam, testRetryParam())

	// Assert
	assert.NoError(t, err)

	// Assert - fetch should be recorded with resolved absolute URL
	records := mockSink.GetAssetFetchRecords()
	assert.Len(t, records, 1)
	assert.Equal(t, server.URL+"/images/logo.png", records[0].FetchUrl)

	// Assert - RecordArtifact should be called
	assert.True(t, mockSink.recordArtifactCalled, "RecordArtifact should be called")
	artifactRecords := mockSink.GetArtifactRecords()
	assert.Len(t, artifactRecords, 1)

	// Assert - document should have rewritten local path
	output := string(doc.Content())
	expectedLocalPath := buildExpectedPath("logo", []byte("image-data"), "png")
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(sharedContent)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	url1 := server.URL + "/logo.png"
	url2 := server.URL + "/different-name.jpg" // Different basename, same content

	// Pre-compute expected path based on first URL's basename
	expectedLocalPath := buildExpectedPath("logo", sharedContent, "png")

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
		resolveParam := assets.NewResolveParam(tempDir, 10*1024*1024)
		doc, err := resolver.Resolve(ctx, *pageUrl, "example.com", "https", conversionResult, resolveParam, testRetryParam())

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
		unexpectedPath := buildExpectedPath("different-name", sharedContent, "jpg")
		assert.NotContains(t, output, unexpectedPath,
			"Iteration %d: Should not contain path from second URL (which was deduplicated, not written)",
			i+1)
	}
}
