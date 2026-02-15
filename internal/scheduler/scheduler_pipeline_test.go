package scheduler_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// ============================================================================
// Pipeline Integration Tests
// These tests verify the end-to-end behavior of the crawl pipeline,
// covering all stages: Fetch → Extract → Sanitize → Convert → Resolve → Normalize → Write
// ============================================================================

// TestPipeline_MethodCallOrder verifies the correct order of method calls
// through all pipeline stages.
func TestPipeline_MethodCallOrder(t *testing.T) {
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
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
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

	// Setup storage
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Write")
		}).Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

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
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)

	_, _ = s.ExecuteCrawlingWithState(init)

	// Verify all stages were called in correct order
	t.Logf("Call order: %v", callOrder)
	assert.Contains(t, callOrder, "Fetch", "Fetch should be called")
	assert.Contains(t, callOrder, "Extract", "Extract should be called")
	assert.Contains(t, callOrder, "Sanitize", "Sanitize should be called")
	assert.Contains(t, callOrder, "Convert", "Convert should be called")
	assert.Contains(t, callOrder, "Resolve", "Resolve should be called")
	assert.Contains(t, callOrder, "Normalize", "Normalize should be called")
	assert.Contains(t, callOrder, "Write", "Write should be called")

	// Verify order
	fetchIdx, extractIdx, sanitizeIdx, convertIdx, resolveIdx, normalizeIdx, writeIdx := -1, -1, -1, -1, -1, -1, -1
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
		case "Write":
			writeIdx = i
		}
	}

	assert.Less(t, fetchIdx, extractIdx, "Fetch before Extract")
	assert.Less(t, extractIdx, sanitizeIdx, "Extract before Sanitize")
	assert.Less(t, sanitizeIdx, convertIdx, "Sanitize before Convert")
	assert.Less(t, convertIdx, resolveIdx, "Convert before Resolve")
	assert.Less(t, resolveIdx, normalizeIdx, "Resolve before Normalize")
	assert.Less(t, normalizeIdx, writeIdx, "Normalize before Write")
}

// TestPipeline_CalledExactlyOncePerPage verifies that each pipeline stage
// is called exactly once for each page processed.
func TestPipeline_CalledExactlyOncePerPage(t *testing.T) {
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
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup all mocks to return success - each should be called exactly once
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)
	setupConvertMockWithSuccess(mockConvert)
	setupResolverMockWithSuccess(mockResolver)
	setupNormalizeMockWithSuccess(mockNormalize)
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil).Once()

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

	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)

	_, _ = s.ExecuteCrawlingWithState(init)

	// Verify each stage was called exactly once
	mockExtractor.AssertNumberOfCalls(t, "Extract", 1)
	mockSanitizer.AssertNumberOfCalls(t, "Sanitize", 1)
	mockConvert.AssertNumberOfCalls(t, "Convert", 1)
	mockResolver.AssertNumberOfCalls(t, "Resolve", 1)
	mockNormalize.AssertNumberOfCalls(t, "Normalize", 1)
	mockStorage.AssertNumberOfCalls(t, "Write", 1)
}

// TestPipeline_AllStagesCalled verifies that all pipeline stages are called
// during a successful crawl execution.
func TestPipeline_AllStagesCalled(t *testing.T) {
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
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup all mocks to return success
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)
	setupConvertMockWithSuccess(mockConvert)
	setupResolverMockWithSuccess(mockResolver)
	setupNormalizeMockWithSuccess(mockNormalize)
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

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

	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)

	_, _ = s.ExecuteCrawlingWithState(init)

	// Verify all stages were called
	mockSanitizer.AssertCalled(t, "Sanitize", mock.Anything)
	mockConvert.AssertCalled(t, "Convert", mock.Anything)
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mockNormalize.AssertCalled(t, "Normalize", mock.Anything, mock.Anything, mock.Anything)
	mockStorage.AssertCalled(t, "Write", mock.Anything, mock.Anything, mock.Anything)
}
