package scheduler_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/build"
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
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// TestScheduler_Normalize_CalledWithResolverResult verifies that Normalize
// is called with the AssetfulMarkdownDoc from the resolver stage.
func TestScheduler_Normalize_CalledWithResolverResult(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor to return a valid content node
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer to return a valid sanitized doc
	sanitizedDoc := createSanitizedHTMLDocForTest(nil)
	mockSanitizer.On("Sanitize", contentNode).Return(sanitizedDoc, nil)

	// Setup convert to return a specific conversion result
	conversionResult := createConversionResultForTest("# Test Markdown\n\nContent", nil)
	mockConvert.On("Convert", sanitizedDoc).Return(conversionResult, nil)

	// Setup resolver to return a specific assetful markdown doc
	assetfulDoc := createAssetfulMarkdownDocForTest("# Test Markdown\n\nContent", []string{"image.png"})
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(assetfulDoc, nil)

	// Setup normalize mock to capture the input
	var receivedAssetfulDoc assets.AssetfulMarkdownDoc
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			receivedAssetfulDoc = args.Get(1).(assets.AssetfulMarkdownDoc)
		}).
		Return(createNormalizedMarkdownDocForTest("# Test Markdown"), nil)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl
	_, _ = s.ExecuteCrawling(configPath)

	// Verify Normalize was called with the AssetfulMarkdownDoc from Resolve
	mockNormalize.AssertCalled(t, "Normalize", mock.Anything, mock.Anything, mock.Anything)
	assert.Equal(t, assetfulDoc.Content(), receivedAssetfulDoc.Content(), "Normalize should be called with the AssetfulMarkdownDoc from Resolve")
}

// TestScheduler_Normalize_SuccessfulNormalization_ProceedsToWrite verifies
// that successful normalization allows the pipeline to continue to storage write.
func TestScheduler_Normalize_SuccessfulNormalization_ProceedsToWrite(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize to return successful result
	normalizedDoc := createNormalizedMarkdownDocForTest("# Normalized Markdown")
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(normalizedDoc, nil)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl
	exec, execErr := s.ExecuteCrawling(configPath)

	// Should complete without fatal error
	assert.NoError(t, execErr)
	// Normalize should be called
	mockNormalize.AssertCalled(t, "Normalize", mock.Anything, mock.Anything, mock.Anything)
	t.Logf("Execution completed with %d write results", len(exec.WriteResults()))
}

// TestScheduler_Normalize_FatalError_AbortsCrawl verifies that fatal normalization errors
// cause the crawl to abort immediately.
func TestScheduler_Normalize_FatalError_AbortsCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize to return a fatal error
	setupNormalizeMockWithFatalError(mockNormalize)

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 1
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl - should return fatal error
	_, execErr := s.ExecuteCrawling(configPath)

	// Fatal normalize error should abort the crawl
	assert.Error(t, execErr, "Expected error for fatal normalize error")
	mockNormalize.AssertCalled(t, "Normalize", mock.Anything, mock.Anything, mock.Anything)
}

// TestScheduler_Normalize_RecoverableError_ContinuesCrawl verifies that recoverable
// normalization errors are counted but the crawl continues.
func TestScheduler_Normalize_RecoverableError_ContinuesCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize to return a recoverable error
	setupNormalizeMockWithRecoverableError(mockNormalize)

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl - should not return fatal error
	_, execErr := s.ExecuteCrawling(configPath)

	// Recoverable errors should not abort the crawl
	assert.NoError(t, execErr, "Recoverable normalize error should not abort crawl")
	mockNormalize.AssertCalled(t, "Normalize", mock.Anything, mock.Anything, mock.Anything)
}

// TestScheduler_Normalize_MethodCallOrder verifies the correct order of method calls:
// Fetch → Extract → Sanitize → Convert → Resolve → Normalize → Write
func TestScheduler_Normalize_MethodCallOrder(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := new(fetcherMock)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Track call order
	callOrder := []string{}

	// Setup fetcher
	testURL, _ := url.Parse("http://example.com/page.html")
	htmlBody := []byte(`<html><body><div>Test</div></body></html>`)
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		htmlBody,
		200,
		"text/html",
		map[string]string{"Content-Type": "text/html"},
		time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Fetch")
		}).Return(fetchResult, nil).Once()

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Extract")
		}).Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).
		Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Sanitize")
		}).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	mockConvert.On("Convert", mock.Anything).
		Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Convert")
		}).Return(createConversionResultForTest("# Test", nil), nil)

	// Setup resolver
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Resolve")
		}).Return(createAssetfulMarkdownDocForTest("# Test", nil), nil)

	// Setup normalize
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Normalize")
		}).Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl
	_, _ = s.ExecuteCrawling(configPath)

	// Verify all stages were called
	mockNormalize.AssertCalled(t, "Normalize", mock.Anything, mock.Anything, mock.Anything)

	// Verify order: Normalize should be called after Resolve
	t.Logf("Call order: %v", callOrder)
	assert.Contains(t, callOrder, "Fetch", "Fetch should be called")
	assert.Contains(t, callOrder, "Extract", "Extract should be called")
	assert.Contains(t, callOrder, "Sanitize", "Sanitize should be called")
	assert.Contains(t, callOrder, "Convert", "Convert should be called")
	assert.Contains(t, callOrder, "Resolve", "Resolve should be called")
	assert.Contains(t, callOrder, "Normalize", "Normalize should be called")

	// Find positions
	fetchIdx := -1
	extractIdx := -1
	sanitizeIdx := -1
	convertIdx := -1
	resolveIdx := -1
	normalizeIdx := -1
	for i, call := range callOrder {
		switch call {
		case "Fetch":
			fetchIdx = i
		case "Extract":
			extractIdx = i
		case "Sanitize":
			sanitizeIdx = i
		case "Convert":
			convertIdx = i
		case "Resolve":
			resolveIdx = i
		case "Normalize":
			normalizeIdx = i
		}
	}

	assert.Less(t, fetchIdx, extractIdx, "Fetch should be called before Extract")
	assert.Less(t, extractIdx, sanitizeIdx, "Extract should be called before Sanitize")
	assert.Less(t, sanitizeIdx, convertIdx, "Sanitize should be called before Convert")
	assert.Less(t, convertIdx, resolveIdx, "Convert should be called before Resolve")
	assert.Less(t, resolveIdx, normalizeIdx, "Resolve should be called before Normalize")
}

// TestScheduler_Normalize_CalledExactlyOncePerPage verifies that Normalize
// is called exactly once for each page processed.
func TestScheduler_Normalize_CalledExactlyOncePerPage(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize - should be called exactly once
	setupNormalizeMockWithSuccess(mockNormalize)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl
	_, _ = s.ExecuteCrawling(configPath)

	// Verify Normalize was called exactly once
	mockNormalize.AssertNumberOfCalls(t, "Normalize", 1)
}

// TestScheduler_Normalize_ErrorDoesNotPreventWriteForRecoverable verifies that when Normalize()
// returns a recoverable error, the scheduler still continues (doesn't write but doesn't abort).
func TestScheduler_Normalize_ErrorDoesNotPreventWriteForRecoverable(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	// Only expect one Decide call for the seed URL
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize to return a recoverable error (not fatal)
	setupNormalizeMockWithRecoverableError(mockNormalize)

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Use maxDepth: 0 to process just one page
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl - should NOT return error for recoverable error
	exec, execErr := s.ExecuteCrawling(configPath)

	// Recoverable normalize error should NOT abort the crawl
	assert.NoError(t, execErr, "Recoverable normalize error should not abort crawl")

	// Verify normalize was called
	mockNormalize.AssertCalled(t, "Normalize", mock.Anything, mock.Anything, mock.Anything)

	// Verify that execution completed (even if no writes due to error)
	t.Logf("Execution completed with %d write results", len(exec.WriteResults()))
}

// TestScheduler_Normalize_FatalErrorPreventsSubsequentCalls verifies that when Normalize()
// returns a fatal error, the scheduler aborts and does not process more URLs.
func TestScheduler_Normalize_FatalErrorPreventsSubsequentCalls(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	// Only expect one Decide call for the seed URL
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize to return a fatal error using mock.Anything to ensure it gets called
	setupNormalizeMockWithFatalError(mockNormalize)

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Use maxDepth: 1 to allow for potential additional processing
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 1
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl - should return fatal error
	_, execErr := s.ExecuteCrawling(configPath)

	// Fatal normalize error should abort the crawl
	assert.Error(t, execErr, "Expected error for fatal normalize error")

	// Verify normalize was called
	mockNormalize.AssertCalled(t, "Normalize", mock.Anything, mock.Anything, mock.Anything)

	// Verify that Robot.Decide was only called once (for seed URL)
	// This proves that the crawl aborted before processing more URLs
	mockRobot.AssertNumberOfCalls(t, "Decide", 1)
	t.Logf("Normalize fatal error prevented further processing as expected")
}

// createSchedulerWithAllMocksAndNormalize creates a scheduler with all mocked dependencies including a custom normalize mock.
func createSchedulerWithAllMocksAndNormalize(
	t *testing.T,
	ctx context.Context,
	mockFinalizer *mockFinalizer,
	metadataSink metadata.MetadataSink,
	mockLimiter *rateLimiterMock,
	mockRobot *robotsMock,
	mockFrontier *frontierMock,
	mockFetcher *fetcherMock,
	mockExtractor extractor.Extractor,
	mockSanitizer sanitizer.Sanitizer,
	mockConvert mdconvert.ConvertRule,
	mockResolver assets.Resolver,
	mockNormalize *normalizeMock,
	mockStorage *storageMock,
	mockSleeper *sleeperMock,
) *scheduler.Scheduler {
	t.Helper()
	// Create real components if mocks not provided
	if mockExtractor == nil {
		ext := extractor.NewDomExtractor(metadataSink)
		mockExtractor = &ext
	}
	if mockSanitizer == nil {
		san := sanitizer.NewHTMLSanitizer(metadataSink)
		mockSanitizer = &san
	}
	if mockConvert == nil {
		mockConvert = newConvertMockForTest(t)
		setupConvertMockWithSuccess(mockConvert.(*convertMock))
	}
	if mockNormalize == nil {
		mockNormalize = newNormalizeMockForTest(t)
		setupNormalizeMockWithSuccess(mockNormalize)
	}

	s := scheduler.NewSchedulerWithDeps(
		ctx,
		mockFinalizer,
		metadataSink,
		mockLimiter,
		mockFrontier,
		mockFetcher,
		mockRobot,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
		mockStorage,
		mockSleeper,
	)
	return &s
}

// TestScheduler_NormalizeParam_CreatedWithCorrectValues verifies that NormalizeParam
// is constructed with the correct values from build version, fetch result, config, and crawl token.
func TestScheduler_NormalizeParam_CreatedWithCorrectValues(t *testing.T) {
	ctx := context.Background()

	// Define expected values
	expectedVersion := build.FullVersion()
	expectedFetchedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	expectedHashAlgo := hashutil.HashAlgoSHA256
	expectedAllowedPathPrefix := []string{"/docs", "/api"}
	expectedDepth := 0 // seed URL depth

	// Create test URL
	testURL, _ := url.Parse("https://example.com/docs/page.html")

	// Create fetch result with specific fetchedAt time (will be used by scheduler)
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte("<html><body><div>Test</div></body></html>"),
		200,
		"text/html",
		map[string]string{"Content-Type": "text/html"},
		expectedFetchedAt,
	)

	// Create config with specific values
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 3,
		"allowedPathPrefix": ["/docs", "/api"],
		"hashAlgo": "sha256"
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := new(fetcherMock)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	// Setup robots
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	// Setup frontier - disable auto-enqueue so we can control Dequeue
	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	// Setup sleeper and limiter
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0)).Maybe()

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup fetcher to return our fetchResult with specific fetchedAt
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Capture the NormalizeParam passed to Normalize
	var capturedParam normalize.NormalizeParam

	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			// Capture the NormalizeParam (3rd argument)
			capturedParam = args.Get(2).(normalize.NormalizeParam)
		}).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockNormalize,
		mockStorage,
		mockSleeper,
	)

	// Override current host (needed for some internal logic)
	s.SetCurrentHost("example.com")

	// Execute crawl - this will process the seed URL and call Normalize
	_, execErr := s.ExecuteCrawling(configPath)
	assert.NoError(t, execErr)

	// Verify Normalize was called
	mockNormalize.AssertCalled(t, "Normalize", mock.Anything, mock.Anything, mock.Anything)

	// Now verify the NormalizeParam values using getter methods
	assert.Equal(t, expectedVersion, capturedParam.AppVersion(), "appVersion should match build.FullVersion()")
	assert.Equal(t, expectedFetchedAt, capturedParam.FetchedAt(), "fetchedAt should match fetchResult.FetchedAt()")
	assert.Equal(t, string(expectedHashAlgo), string(capturedParam.HashAlgo()), "hashAlgo should match cfg.HashAlgo()")
	assert.Equal(t, expectedDepth, capturedParam.CrawlDepth(), "crawlDepth should match nextCrawlToken.Depth() (seed=0)")
	assert.Equal(t, expectedAllowedPathPrefix, capturedParam.AllowedPathPrefixes(), "allowedPathPrefixes should match config.AllowedPathPrefix()")
}

// TestScheduler_NormalizeParam_UsesTokenDepth verifies that the crawl depth
// in NormalizeParam correctly reflects the token's depth.
func TestScheduler_NormalizeParam_UsesTokenDepth(t *testing.T) {
	ctx := context.Background()

	// We'll test with depth 1 (non-seed)
	expectedDepth := 1

	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := new(fetcherMock)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	// Disable auto-enqueue to control Dequeue behavior
	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()

	// First dequeue: token with depth 1 (non-seed page)
	tokenDepth1 := frontier.NewCrawlToken(*mustParseURL("https://example.com/page1"), 1)
	mockFrontier.OnDequeue(tokenDepth1, true).Once()
	// Second dequeue: exit loop
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0)).Maybe()

	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)
	setupConvertMockWithSuccess(mockConvert)
	setupResolverMockWithSuccess(mockResolver)

	// Setup fetcher
	testURL, _ := url.Parse("https://example.com/page1")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte("<html><body><h1>Test Title</h1>\n\nContent</body></html>"),
		200,
		"text/html",
		map[string]string{"Content-Type": "text/html"},
		time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	var capturedDepth int
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			param := args.Get(2).(normalize.NormalizeParam)
			capturedDepth = param.CrawlDepth()
		}).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockNormalize,
		mockStorage,
		mockSleeper,
	)
	s.SetCurrentHost("example.com")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 3
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	_, execErr := s.ExecuteCrawling(configPath)
	assert.NoError(t, execErr)

	// Verify the captured depth is 1 (from the token), not 0 (seed depth)
	assert.Equal(t, expectedDepth, capturedDepth, "NormalizeParam CrawlDepth should match the crawl token's depth")
}

// TestScheduler_NormalizeParam_UsesConfigAllowedPathPrefix verifies that
// allowedPathPrefixes in NormalizeParam come from the config.
func TestScheduler_NormalizeParam_UsesConfigAllowedPathPrefix(t *testing.T) {
	ctx := context.Background()

	// Define expected prefixes
	expectedPrefixes := []string{"/blog", "/articles", "/docs"}

	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := new(fetcherMock)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0)).Maybe()

	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)
	setupConvertMockWithSuccess(mockConvert)
	setupResolverMockWithSuccess(mockResolver)

	// Setup fetcher
	testURL, _ := url.Parse("https://example.com")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte("<html><body><div>Test</div></body></html>"),
		200,
		"text/html",
		map[string]string{"Content-Type": "text/html"},
		time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	var capturedPrefixes []string
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			param := args.Get(2).(normalize.NormalizeParam)
			capturedPrefixes = param.AllowedPathPrefixes()
		}).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockNormalize,
		mockStorage,
		mockSleeper,
	)
	s.SetCurrentHost("example.com")

	// Create config with specific allowedPathPrefix
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 2,
		"allowedPathPrefix": ["/blog", "/articles", "/docs"]
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	_, execErr := s.ExecuteCrawling(configPath)
	assert.NoError(t, execErr)

	// Verify the captured allowedPathPrefixes match the config
	assert.Equal(t, expectedPrefixes, capturedPrefixes, "allowedPathPrefixes should match config.AllowedPathPrefix()")
}

// TestScheduler_NormalizeParam_UsesConfigHashAlgo verifies that hashAlgo
// in NormalizeParam comes from the config.
func TestScheduler_NormalizeParam_UsesConfigHashAlgo(t *testing.T) {
	ctx := context.Background()

	expectedHashAlgo := hashutil.HashAlgoSHA256

	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := new(fetcherMock)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0)).Maybe()

	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	mockExtractor.On("Extract", mock.Anything, mock.Anything).
		Return(extractor.ExtractionResult{ContentNode: contentNode}, nil)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)
	setupConvertMockWithSuccess(mockConvert)
	setupResolverMockWithSuccess(mockResolver)

	// Setup fetcher
	testURL, _ := url.Parse("https://example.com")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte("<html><body><div>Test</div></body></html>"),
		200,
		"text/html",
		map[string]string{"Content-Type": "text/html"},
		time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	var capturedHashAlgo hashutil.HashAlgo
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			param := args.Get(2).(normalize.NormalizeParam)
			capturedHashAlgo = param.HashAlgo()
		}).
		Return(createNormalizedMarkdownDocForTest("# Test"), nil)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockNormalize,
		mockStorage,
		mockSleeper,
	)
	s.SetCurrentHost("example.com")

	// Create config with explicit hashAlgo (default is sha256, but we specify it)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 2,
		"hashAlgo": "sha256"
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	_, execErr := s.ExecuteCrawling(configPath)
	assert.NoError(t, execErr)

	// Verify the captured hashAlgo matches the config
	assert.Equal(t, string(expectedHashAlgo), string(capturedHashAlgo), "hashAlgo should match cfg.HashAlgo()")
}
