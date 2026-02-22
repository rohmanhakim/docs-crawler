package assets_test

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata/metadatatest"
	"github.com/rohmanhakim/docs-crawler/pkg/debug/debugtest"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

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
func newTestResolver(mockSink *metadatatest.SinkMock) assets.LocalResolver {
	resolver := assets.NewLocalResolver(mockSink)
	resolver.Init(&http.Client{Timeout: 5 * time.Second}, "test-user-agent")
	return resolver
}

// newTestResolverWithLogger creates a LocalResolver with test dependencies and a debug logger
func newTestResolverWithLogger(mockSink *metadatatest.SinkMock, logger *debugtest.LoggerMock) assets.LocalResolver {
	resolver := assets.NewLocalResolver(mockSink)
	resolver.Init(&http.Client{Timeout: 5 * time.Second}, "test-user-agent")
	resolver.SetDebugLogger(logger)
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
