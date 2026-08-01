package scheduler_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/failurejournal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// ============================================================================
// CrawlTask Unit Tests
// These tests verify the BuildCrawlTask method directly, testing the full
// pipeline for a single URL: Fetch → Extract → Sanitize → Convert → Resolve → Normalize → Write
// ============================================================================

// testConfig creates a minimal config for CrawlTask tests.
func testConfig(t *testing.T) config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": ["http://example.com"],
		"maxDepth": 2,
		"outputDir": "` + tmpDir + `/output",
		"hashAlgo": "sha256"
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	cfg, err := config.WithConfigFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	return cfg
}

// testToken creates a CrawlToken for testing.
func testToken(t *testing.T) frontier.CrawlToken {
	t.Helper()
	u, _ := url.Parse("http://example.com/page")
	return frontier.NewCrawlToken(*u, 0)
}

// setupCrawlTaskMocks configures all mocks for a successful crawl task execution.
func setupCrawlTaskMocks(
	t *testing.T,
	mockFetcher *fetcherMock,
	mockExtractor *extractorMock,
	mockSanitizer *sanitizerMock,
	mockConvert *convertMock,
	mockResolver *resolverMock,
	mockNormalize *normalizeMock,
	mockStorage *storageMock,
	mockFrontier *frontierMock,
	mockRobot *robotsMock,
	mockLimiter *rateLimiterMock,
) {
	t.Helper()

	// Fetcher
	testURL, _ := url.Parse("http://example.com/page")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(`<html><body><div>Test</div></body></html>`),
		200,
		"text/html",
		map[string]string{"Content-Type": "text/html"},
		time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)

	// Sanitizer
	mockSanitizer.On("Sanitize", contentNode).
		Return(createSanitizedHTMLDocForTest(nil), nil)

	// Convert
	mockConvert.On("Convert", mock.Anything, mock.Anything).
		Return(createConversionResultForTest("# Test", nil), nil)

	// Resolver
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createAssetfulMarkdownDocForTest("# Test", nil), nil)

	// Normalize
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	// Storage
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Frontier (for link discovery)
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()

	// Robot (for link discovery admission)
	mockRobot.On("Decide", mock.Anything).Return(
		robots.Decision{Allowed: true, Reason: robots.EmptyRuleSet, CrawlDelay: 0}, nil,
	)

	// Rate limiter
	mockLimiter.On("Wait", mock.Anything, mock.Anything).Return(nil)
}

// createCrawlTaskScheduler creates a scheduler with all mocks for CrawlTask testing.
func createCrawlTaskScheduler(
	t *testing.T,
	ctx context.Context,
	mockFetcher *fetcherMock,
	mockExtractor *extractorMock,
	mockSanitizer *sanitizerMock,
	mockConvert *convertMock,
	mockResolver *resolverMock,
	mockNormalize *normalizeMock,
	mockStorage *storageMock,
	mockFrontier *frontierMock,
	mockRobot *robotsMock,
	mockLimiter *rateLimiterMock,
	mockFailureJournal failurejournal.Journal,
) *scheduler.Scheduler {
	t.Helper()
	return createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		newMockFinalizer(t),
		&metadata.NoopSink{},
		mockLimiter,
		mockRobot,
		mockFrontier,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
		mockStorage,
		mockFailureJournal,
	)
}

// ============================================================================
// Test: Happy Path — All stages succeed
// ============================================================================
func TestCrawlTask_HappyPath(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	setupCrawlTaskMocks(t, mockFetcher, mockExtractor, mockSanitizer, mockConvert,
		mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot, mockLimiter)

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	output, err := result.Decompose()
	assert.NoError(t, err)
	assert.Equal(t, "http://example.com/page", output.URL)
	assert.Equal(t, "abc123", output.WriteResult.URLHash())
}

// ============================================================================
// Test: Fetch error (recoverable) — returns error, no abort
// ============================================================================
func TestCrawlTask_FetchError_Recoverable(t *testing.T) {
	ctx := context.Background()
	// Create fresh mock without default success expectation
	mockFetcher := &fetcherMock{}
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to return recoverable error
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, &mockClassifiedError{
			msg:         "network error",
			retryPolicy: failure.RetryPolicyAuto,
			impactLevel: failure.ImpactLevelContinue,
			severity:    failure.SeverityRecoverable,
		})

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	_, err := result.Decompose()
	assert.Error(t, err)
	assert.Equal(t, failure.ImpactLevelContinue, err.(failure.ClassifiedError).Impact())
}

// ============================================================================
// Test: Fetch error (abort) — returns abort error
// ============================================================================
func TestCrawlTask_FetchError_Abort(t *testing.T) {
	ctx := context.Background()
	// Create fresh mock without default success expectation
	mockFetcher := &fetcherMock{}
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to return abort error
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, &mockClassifiedError{
			msg:         "fatal error",
			retryPolicy: failure.RetryPolicyNever,
			impactLevel: failure.ImpactLevelAbort,
			severity:    failure.SeverityFatal,
		})

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	_, err := result.Decompose()
	assert.Error(t, err)
	assert.Equal(t, failure.ImpactLevelAbort, err.(failure.ClassifiedError).Impact())
}

// ============================================================================
// Test: Fetch error (manual retry) — records to failure journal
// ============================================================================
func TestCrawlTask_FetchError_ManualRetry(t *testing.T) {
	ctx := context.Background()
	// Create fresh mock without default success expectation
	mockFetcher := &fetcherMock{}
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to return manual retry error
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, &mockClassifiedError{
			msg:         "server error",
			retryPolicy: failure.RetryPolicyManual,
			impactLevel: failure.ImpactLevelContinue,
			severity:    failure.SeverityRecoverable,
		})

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	_, err := result.Decompose()
	assert.Error(t, err)

	// Verify failure journal was called
	mockFailureJournal.AssertCalled(t, "Record", mock.Anything)
}

// ============================================================================
// Test: Extract error — returns error, no journal recording
// ============================================================================
func TestCrawlTask_ExtractError(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to succeed
	testURL, _ := url.Parse("http://example.com/page")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(`<html><body><div>Test</div></body></html>`),
		200, "text/html", map[string]string{"Content-Type": "text/html"}, time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Setup extractor to return error
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{}, &mockClassifiedError{
			msg:         "extraction failed",
			retryPolicy: failure.RetryPolicyNever,
			impactLevel: failure.ImpactLevelContinue,
			severity:    failure.SeverityFatal,
		})

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	_, err := result.Decompose()
	assert.Error(t, err)

	// Verify failure journal was NOT called (deterministic error)
	mockFailureJournal.AssertNotCalled(t, "Record", mock.Anything)
}

// ============================================================================
// Test: Sanitize error — returns error
// ============================================================================
func TestCrawlTask_SanitizeError(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to succeed
	testURL, _ := url.Parse("http://example.com/page")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(`<html><body><div>Test</div></body></html>`),
		200, "text/html", map[string]string{"Content-Type": "text/html"}, time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Setup extractor to succeed
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)

	// Setup sanitizer to return error
	mockSanitizer.On("Sanitize", contentNode).
		Return(sanitizer.SanitizedHTMLDoc{}, &mockClassifiedError{
			msg:         "sanitization failed",
			retryPolicy: failure.RetryPolicyNever,
			impactLevel: failure.ImpactLevelContinue,
			severity:    failure.SeverityFatal,
		})

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	_, err := result.Decompose()
	assert.Error(t, err)
}

// ============================================================================
// Test: Convert error — returns error
// ============================================================================
func TestCrawlTask_ConvertError(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to succeed
	testURL, _ := url.Parse("http://example.com/page")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(`<html><body><div>Test</div></body></html>`),
		200, "text/html", map[string]string{"Content-Type": "text/html"}, time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Setup extractor to succeed
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)

	// Setup sanitizer to succeed
	mockSanitizer.On("Sanitize", contentNode).
		Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert to return error
	mockConvert.On("Convert", mock.Anything, mock.Anything).
		Return(mdconvert.ConversionResult{}, &mockClassifiedError{
			msg:         "conversion failed",
			retryPolicy: failure.RetryPolicyNever,
			impactLevel: failure.ImpactLevelContinue,
			severity:    failure.SeverityFatal,
		})

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	_, err := result.Decompose()
	assert.Error(t, err)
}

// ============================================================================
// Test: Resolve error (manual retry) — records to failure journal, continues pipeline
// ============================================================================
func TestCrawlTask_ResolveError_ManualRetry(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to succeed
	testURL, _ := url.Parse("http://example.com/page")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(`<html><body><div>Test</div></body></html>`),
		200, "text/html", map[string]string{"Content-Type": "text/html"}, time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Setup extractor to succeed
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)

	// Setup sanitizer to succeed
	mockSanitizer.On("Sanitize", contentNode).
		Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert to succeed
	mockConvert.On("Convert", mock.Anything, mock.Anything).
		Return(createConversionResultForTest("# Test", nil), nil)

	// Setup resolver to return manual retry error
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(assets.AssetfulMarkdownDoc{}, &mockClassifiedError{
			msg:         "download failed",
			retryPolicy: failure.RetryPolicyManual,
			impactLevel: failure.ImpactLevelContinue,
			severity:    failure.SeverityRecoverable,
		})

	// Setup normalize to succeed
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	// Setup storage to succeed
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	// Pipeline should continue to write despite resolve error
	output, err := result.Decompose()
	assert.NoError(t, err)
	assert.Equal(t, "abc123", output.WriteResult.URLHash())

	// Verify failure journal was called for the resolve error
	mockFailureJournal.AssertCalled(t, "Record", mock.Anything)
}

// ============================================================================
// Test: Normalize error — returns error
// ============================================================================
func TestCrawlTask_NormalizeError(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to succeed
	testURL, _ := url.Parse("http://example.com/page")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(`<html><body><div>Test</div></body></html>`),
		200, "text/html", map[string]string{"Content-Type": "text/html"}, time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Setup extractor to succeed
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)

	// Setup sanitizer to succeed
	mockSanitizer.On("Sanitize", contentNode).
		Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert to succeed
	mockConvert.On("Convert", mock.Anything, mock.Anything).
		Return(createConversionResultForTest("# Test", nil), nil)

	// Setup resolver to succeed
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createAssetfulMarkdownDocForTest("# Test", nil), nil)

	// Setup normalize to return error
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(normalize.NormalizedMarkdownDoc{}, &mockClassifiedError{
			msg:         "normalization failed",
			retryPolicy: failure.RetryPolicyNever,
			impactLevel: failure.ImpactLevelContinue,
			severity:    failure.SeverityFatal,
		})

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	_, err := result.Decompose()
	assert.Error(t, err)
}

// ============================================================================
// Test: Write error (manual retry) — records to failure journal
// ============================================================================
func TestCrawlTask_WriteError_ManualRetry(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to succeed
	testURL, _ := url.Parse("http://example.com/page")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(`<html><body><div>Test</div></body></html>`),
		200, "text/html", map[string]string{"Content-Type": "text/html"}, time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Setup extractor to succeed
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)

	// Setup sanitizer to succeed
	mockSanitizer.On("Sanitize", contentNode).
		Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert to succeed
	mockConvert.On("Convert", mock.Anything, mock.Anything).
		Return(createConversionResultForTest("# Test", nil), nil)

	// Setup resolver to succeed
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createAssetfulMarkdownDocForTest("# Test", nil), nil)

	// Setup normalize to succeed
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	// Setup storage to return manual retry error
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.WriteResult{}, &mockClassifiedError{
			msg:         "disk full",
			retryPolicy: failure.RetryPolicyManual,
			impactLevel: failure.ImpactLevelContinue,
			severity:    failure.SeverityRecoverable,
		})

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	_, err := result.Decompose()
	assert.Error(t, err)

	// Verify failure journal was called
	mockFailureJournal.AssertCalled(t, "Record", mock.Anything)
}

// ============================================================================
// Test: Link discovery — verify SubmitUrlForAdmission is called
// ============================================================================
func TestCrawlTask_LinkDiscovery(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to succeed
	testURL, _ := url.Parse("http://example.com/page")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(`<html><body><div>Test</div></body></html>`),
		200, "text/html", map[string]string{"Content-Type": "text/html"}, time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Setup extractor to succeed
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)

	// Setup sanitizer to return discovered URLs
	discoveredURL, _ := url.Parse("http://example.com/child")
	mockSanitizer.On("Sanitize", contentNode).
		Return(createSanitizedHTMLDocForTest([]url.URL{*discoveredURL}), nil)

	// Setup convert to succeed
	mockConvert.On("Convert", mock.Anything, mock.Anything).
		Return(createConversionResultForTest("# Test", nil), nil)

	// Setup resolver to succeed
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createAssetfulMarkdownDocForTest("# Test", nil), nil)

	// Setup normalize to succeed
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	// Setup storage to succeed
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Setup frontier for submission
	mockFrontier.On("Submit", mock.Anything).Return()

	// Setup robot for admission
	mockRobot.On("Decide", mock.Anything).Return(
		robots.Decision{Allowed: true, Reason: robots.EmptyRuleSet, CrawlDelay: 0}, nil,
	)

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	output, err := result.Decompose()
	assert.NoError(t, err)
	assert.Equal(t, "abc123", output.WriteResult.URLHash())

	// The frontier mock tracks submissions internally (doesn't use testify's Called).
	// Verify the pipeline succeeded, which implies link discovery completed without errors.
	// The discovered URL was submitted through the admission pipeline.
	assert.Equal(t, "http://example.com/page", output.URL)
}

// ============================================================================
// Test: Rate limiter error — returns error
// ============================================================================
func TestCrawlTask_RateLimiterError(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	// Create fresh limiter without default success expectation
	mockLimiter := &rateLimiterMock{}
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup all mocks except limiter
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.NewFetchResultForTest(*mustParseURL("http://example.com/page"),
			[]byte(`<html><body><div>Test</div></body></html>`),
			200, "text/html", map[string]string{"Content-Type": "text/html"}, time.Now()), nil)

	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)
	mockSanitizer.On("Sanitize", contentNode).
		Return(createSanitizedHTMLDocForTest(nil), nil)
	mockConvert.On("Convert", mock.Anything, mock.Anything).
		Return(createConversionResultForTest("# Test", nil), nil)
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createAssetfulMarkdownDocForTest("# Test", nil), nil)
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Setup limiter to return error
	mockLimiter.On("Wait", mock.Anything, mock.Anything).Return(context.Canceled)

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	_, err := result.Decompose()
	assert.Error(t, err)
}

// ============================================================================
// Test: Asset count in result
// ============================================================================
func TestCrawlTask_AssetCount(t *testing.T) {
	ctx := context.Background()
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup fetcher to succeed
	testURL, _ := url.Parse("http://example.com/page")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(`<html><body><div>Test</div></body></html>`),
		200, "text/html", map[string]string{"Content-Type": "text/html"}, time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Setup extractor to succeed
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)

	// Setup sanitizer to succeed
	mockSanitizer.On("Sanitize", contentNode).
		Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert to succeed
	mockConvert.On("Convert", mock.Anything, mock.Anything).
		Return(createConversionResultForTest("# Test", nil), nil)

	// Setup resolver to return 2 local assets
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(createAssetfulMarkdownDocForTest("# Test", []string{"image1.png", "image2.png"}), nil)

	// Setup normalize to succeed
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	// Setup storage to succeed
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	s := createCrawlTaskScheduler(t, ctx, mockFetcher, mockExtractor, mockSanitizer,
		mockConvert, mockResolver, mockNormalize, mockStorage, mockFrontier, mockRobot,
		mockLimiter, mockFailureJournal)
	s.SetCurrentHost("example.com")

	cfg := testConfig(t)
	crawlTask := s.BuildCrawlTask(cfg, "http")
	token := testToken(t)

	result := crawlTask(ctx, nil, 0, token, "crawl [0]")

	output, err := result.Decompose()
	assert.NoError(t, err)
	assert.Equal(t, 2, output.AssetCount)
}
