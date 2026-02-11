package assets_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// assetFetchRecord stores the parameters passed to RecordAssetFetch
type assetFetchRecord struct {
	FetchUrl   string
	HTTPStatus int
	Duration   time.Duration
	RetryCount int
}

// errorRecord stores the parameters passed to RecordError
type errorRecord struct {
	ObservedAt  time.Time
	PackageName string
	Action      string
	Cause       metadata.ErrorCause
	Details     string
	Attrs       []metadata.Attribute
}

// artifactRecord stores the parameters passed to RecordArtifact
type artifactRecord struct {
	Kind  metadata.ArtifactKind
	Path  string
	Attrs []metadata.Attribute
}

// metadataSinkMock is a mock for metadata.MetadataSink
type metadataSinkMock struct {
	recordErrorCalled      bool
	recordFetchCalled      bool
	recordAssetFetchCalled bool
	recordArtifactCalled   bool
	assetFetchRecords      []assetFetchRecord
	errorRecords           []errorRecord
	artifactRecords        []artifactRecord
}

func (m *metadataSinkMock) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	details string,
	attrs []metadata.Attribute,
) {
	m.recordErrorCalled = true
	m.errorRecords = append(m.errorRecords, errorRecord{
		ObservedAt:  observedAt,
		PackageName: packageName,
		Action:      action,
		Cause:       cause,
		Details:     details,
		Attrs:       attrs,
	})
}

func (m *metadataSinkMock) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
	m.recordFetchCalled = true
}

func (m *metadataSinkMock) RecordAssetFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	retryCount int,
) {
	m.recordAssetFetchCalled = true
	m.assetFetchRecords = append(m.assetFetchRecords, assetFetchRecord{
		FetchUrl:   fetchUrl,
		HTTPStatus: httpStatus,
		Duration:   duration,
		RetryCount: retryCount,
	})
}

func (m *metadataSinkMock) RecordArtifact(kind metadata.ArtifactKind, path string, attrs []metadata.Attribute) {
	m.recordArtifactCalled = true
	m.artifactRecords = append(m.artifactRecords, artifactRecord{
		Kind:  kind,
		Path:  path,
		Attrs: attrs,
	})
}

// GetAssetFetchRecords returns all recorded asset fetch calls
func (m *metadataSinkMock) GetAssetFetchRecords() []assetFetchRecord {
	return m.assetFetchRecords
}

// GetErrorRecords returns all recorded error calls
func (m *metadataSinkMock) GetErrorRecords() []errorRecord {
	return m.errorRecords
}

// GetArtifactRecords returns all recorded artifact calls
func (m *metadataSinkMock) GetArtifactRecords() []artifactRecord {
	return m.artifactRecords
}

// Reset clears all recorded state
func (m *metadataSinkMock) Reset() {
	m.recordErrorCalled = false
	m.recordFetchCalled = false
	m.recordAssetFetchCalled = false
	m.recordArtifactCalled = false
	m.assetFetchRecords = nil
	m.errorRecords = nil
	m.artifactRecords = nil
}

// computeHash computes SHA-256 hash of data (matches resolver implementation)
func computeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// computeShortHash returns first 7 characters of hash (like git)
func computeShortHash(data []byte) string {
	return computeHash(data)[:7]
}

// buildExpectedPath builds the expected asset path using the new format:
// assets/images/<name>-<short-hash>.<ext>
func buildExpectedPath(originalName string, data []byte, ext string) string {
	shortHash := computeShortHash(data)
	return fmt.Sprintf("assets/images/%s-%s.%s", originalName, shortHash, ext)
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
	return assets.NewLocalResolver(
		mockSink,
		&http.Client{Timeout: 5 * time.Second},
		"test-user-agent",
	)
}

// resolveWithTestParams is a helper that calls Resolve with test retry params
func resolveWithTestParams(
	resolver assets.LocalResolver,
	ctx context.Context,
	pageUrl url.URL,
	conversionResult mdconvert.ConversionResult,
	outputDir string,
	host string,
	scheme string,
) (assets.AssetfulMarkdownDoc, error) {
	resolveParam := assets.NewResolveParam(outputDir, 10*1024*1024)
	return resolver.Resolve(ctx, pageUrl, host, scheme, conversionResult, resolveParam, testRetryParam())
}
