package assets_test

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// compile-time interface check
var _ metadata.MetadataSink = (*metadataSinkMock)(nil)

// metadataSinkMock implements metadata.MetadataSink for use in assets tests.
type metadataSinkMock struct {
	recordErrorCalled    bool
	recordFetchCalled    bool
	recordArtifactCalled bool
	fetchRecords         []metadata.FetchEvent
	errorRecords         []metadata.ErrorRecord
	artifactRecords      []metadata.ArtifactRecord
}

func (m *metadataSinkMock) RecordFetch(event metadata.FetchEvent) {
	m.recordFetchCalled = true
	m.fetchRecords = append(m.fetchRecords, event)
}

func (m *metadataSinkMock) RecordArtifact(record metadata.ArtifactRecord) {
	m.recordArtifactCalled = true
	m.artifactRecords = append(m.artifactRecords, record)
}

func (m *metadataSinkMock) RecordPipelineStage(_ metadata.PipelineEvent) {}

func (m *metadataSinkMock) RecordSkip(_ metadata.SkipEvent) {}

func (m *metadataSinkMock) RecordError(record metadata.ErrorRecord) {
	m.recordErrorCalled = true
	m.errorRecords = append(m.errorRecords, record)
}

// GetFetchRecords returns all recorded FetchEvent calls.
func (m *metadataSinkMock) GetFetchRecords() []metadata.FetchEvent {
	return m.fetchRecords
}

// GetErrorRecords returns all recorded ErrorRecord calls.
func (m *metadataSinkMock) GetErrorRecords() []metadata.ErrorRecord {
	return m.errorRecords
}

// GetArtifactRecords returns all recorded ArtifactRecord calls.
func (m *metadataSinkMock) GetArtifactRecords() []metadata.ArtifactRecord {
	return m.artifactRecords
}

// Reset clears all recorded state.
func (m *metadataSinkMock) Reset() {
	m.recordErrorCalled = false
	m.recordFetchCalled = false
	m.recordArtifactCalled = false
	m.fetchRecords = nil
	m.errorRecords = nil
	m.artifactRecords = nil
}

// buildExpectedPath builds the expected asset path using the format:
// assets/images/<name>-<short-hash>.<ext>
// The contentHash should be the full SHA-256 hash string.
func buildExpectedPath(originalName string, contentHash string, ext string) string {
	shortHash := contentHash[:7]
	return "assets/images/" + originalName + "-" + shortHash + "." + ext
}

// testRetryParam returns a retry param with minimal delays for testing
func testRetryParam() retry.RetryParam {
	return retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		2,
		timeutil.NewBackoffParam(10*time.Millisecond, 2.0, 100*time.Millisecond),
	)
}

// newTestResolver creates a LocalResolver with test dependencies
func newTestResolver(mockSink *metadataSinkMock) assets.LocalResolver {
	resolver := assets.NewLocalResolver(mockSink)
	resolver.Init(&http.Client{Timeout: 5 * time.Second}, "test-user-agent")
	return resolver
}

// resolveWithTestParams is a helper that calls Resolve with test retry params
func resolveWithTestParams(
	resolver assets.LocalResolver,
	ctx context.Context,
	pageUrl url.URL,
	conversionResult mdconvert.ConversionResult,
	outputDir string,
) (assets.AssetfulMarkdownDoc, error) {
	resolveParam := assets.NewResolveParam(outputDir, 10*1024*1024, hashutil.HashAlgoSHA256)
	return resolver.Resolve(ctx, pageUrl, conversionResult, resolveParam, testRetryParam())
}
