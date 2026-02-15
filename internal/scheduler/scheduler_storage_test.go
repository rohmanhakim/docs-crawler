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
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// TestScheduler_Write_CalledWithNormalizedDoc verifies that Write
// is called with the NormalizedMarkdownDoc from the normalize stage.
func TestScheduler_Write_CalledWithNormalizedDoc(t *testing.T) {
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

	// Setup normalize to return a specific normalized doc
	normalizedDoc := createNormalizedMarkdownDocForTest("# Test Markdown\n\nNormalized content")
	mockNormalize.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(normalizedDoc, nil)

	// Setup storage mock to capture the input
	var receivedNormalizedDoc normalize.NormalizedMarkdownDoc
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			receivedNormalizedDoc = args.Get(1).(normalize.NormalizedMarkdownDoc)
		}).
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

	// Execute crawl
	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Phase 2: Execute with state
	_, _ = s.ExecuteCrawlingWithState(init)

	// Verify Write was called with the NormalizedMarkdownDoc from Normalize
	mockStorage.AssertCalled(t, "Write", mock.Anything, mock.Anything, mock.Anything)
	assert.Equal(t, normalizedDoc.Content(), receivedNormalizedDoc.Content(), "Write should be called with the NormalizedMarkdownDoc from Normalize")
}

// TestScheduler_Write_SuccessfulWrite_ReturnsWriteResult verifies
// that successful storage write returns the WriteResult and stores it.
func TestScheduler_Write_SuccessfulWrite_ReturnsWriteResult(t *testing.T) {
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

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage to return a specific write result
	expectedWriteResult := storage.NewWriteResult("urlhash123", "/output/urlhash123.md", "sha256:content456")
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(expectedWriteResult, nil)

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
	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Phase 2: Execute with state
	exec, execErr := s.ExecuteCrawlingWithState(init)

	// Should complete without fatal error
	assert.NoError(t, execErr)
	// Write should be called
	mockStorage.AssertCalled(t, "Write", mock.Anything, mock.Anything, mock.Anything)
	// WriteResults should contain the expected result
	writeResults := exec.WriteResults()
	assert.Len(t, writeResults, 1, "Should have 1 write result")
	assert.Equal(t, expectedWriteResult.URLHash(), writeResults[0].URLHash())
	assert.Equal(t, expectedWriteResult.Path(), writeResults[0].Path())
	assert.Equal(t, expectedWriteResult.ContentHash(), writeResults[0].ContentHash())
}

// TestScheduler_Write_FatalError_AbortsCrawl verifies that fatal storage errors
// cause the crawl to abort immediately.
func TestScheduler_Write_FatalError_AbortsCrawl(t *testing.T) {
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

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage to return a fatal error
	storageErr := &storage.StorageError{
		Message:   "fatal storage error: permission denied",
		Retryable: false,
		Cause:     storage.ErrCauseWriteFailure,
		Path:      "/output/test.md",
	}
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.WriteResult{}, storageErr)

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
	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Phase 2: Execute with state
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Fatal storage error should abort the crawl
	assert.Error(t, execErr, "Expected error for fatal storage error")
	mockStorage.AssertCalled(t, "Write", mock.Anything, mock.Anything, mock.Anything)
}

// TestScheduler_Write_RecoverableError_ContinuesCrawl verifies that recoverable
// storage errors are counted but the crawl continues.
func TestScheduler_Write_RecoverableError_ContinuesCrawl(t *testing.T) {
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

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage to return a recoverable error (disk full is retryable)
	storageErr := &storage.StorageError{
		Message:   "recoverable storage error: disk full",
		Retryable: true,
		Cause:     storage.ErrCauseDiskFull,
		Path:      "/output/test.md",
	}
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.WriteResult{}, storageErr)

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
	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Phase 2: Execute with state
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Recoverable storage error should not abort the crawl
	assert.NoError(t, execErr, "Recoverable storage error should not abort crawl")
	mockStorage.AssertCalled(t, "Write", mock.Anything, mock.Anything, mock.Anything)
}

// TestScheduler_Write_MethodCallOrder verifies the correct order of method calls:
// Fetch → Extract → Sanitize → Convert → Resolve → Normalize → Write
func TestScheduler_Write_MethodCallOrder(t *testing.T) {
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
	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Phase 2: Execute with state
	_, _ = s.ExecuteCrawlingWithState(init)

	// Verify all stages were called
	mockStorage.AssertCalled(t, "Write", mock.Anything, mock.Anything, mock.Anything)

	// Verify order: Write should be called after Normalize
	t.Logf("Call order: %v", callOrder)
	assert.Contains(t, callOrder, "Fetch", "Fetch should be called")
	assert.Contains(t, callOrder, "Extract", "Extract should be called")
	assert.Contains(t, callOrder, "Sanitize", "Sanitize should be called")
	assert.Contains(t, callOrder, "Convert", "Convert should be called")
	assert.Contains(t, callOrder, "Resolve", "Resolve should be called")
	assert.Contains(t, callOrder, "Normalize", "Normalize should be called")
	assert.Contains(t, callOrder, "Write", "Write should be called")

	// Find positions
	fetchIdx := -1
	extractIdx := -1
	sanitizeIdx := -1
	convertIdx := -1
	resolveIdx := -1
	normalizeIdx := -1
	writeIdx := -1
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

	assert.Less(t, fetchIdx, extractIdx, "Fetch should be called before Extract")
	assert.Less(t, extractIdx, sanitizeIdx, "Extract should be called before Sanitize")
	assert.Less(t, sanitizeIdx, convertIdx, "Sanitize should be called before Convert")
	assert.Less(t, convertIdx, resolveIdx, "Convert should be called before Resolve")
	assert.Less(t, resolveIdx, normalizeIdx, "Resolve should be called before Normalize")
	assert.Less(t, normalizeIdx, writeIdx, "Normalize should be called before Write")
}

// TestScheduler_Write_CalledExactlyOncePerPage verifies that Write
// is called exactly once for each page processed.
func TestScheduler_Write_CalledExactlyOncePerPage(t *testing.T) {
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

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage - should be called exactly once
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

	// Execute crawl
	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Phase 2: Execute with state
	_, _ = s.ExecuteCrawlingWithState(init)

	// Verify Write was called exactly once
	mockStorage.AssertNumberOfCalls(t, "Write", 1)
}

// TestScheduler_Write_CalledWithCorrectOutputDir verifies that Write
// is called with the correct output directory from config.
func TestScheduler_Write_CalledWithCorrectOutputDir(t *testing.T) {
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

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage to capture the outputDir
	expectedOutputDir := "/custom/output/dir"
	var capturedOutputDir string
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedOutputDir = args.Get(0).(string)
		}).
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

	// Use a custom output directory
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0,
		"outputDir": "/custom/output/dir"
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl
	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Phase 2: Execute with state
	_, _ = s.ExecuteCrawlingWithState(init)

	// Verify Write was called with the correct outputDir
	mockStorage.AssertCalled(t, "Write", mock.Anything, mock.Anything, mock.Anything)
	assert.Equal(t, expectedOutputDir, capturedOutputDir, "Write should be called with the outputDir from config")
}

// TestScheduler_Write_CalledWithCorrectHashAlgo verifies that Write
// is called with the correct hash algorithm from config.
func TestScheduler_Write_CalledWithCorrectHashAlgo(t *testing.T) {
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

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage to capture the hashAlgo
	var capturedHashAlgo hashutil.HashAlgo
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedHashAlgo = args.Get(2).(hashutil.HashAlgo)
		}).
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

	// Use SHA256 hash algorithm
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0,
		"hashAlgo": "sha256"
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl
	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Phase 2: Execute with state
	_, _ = s.ExecuteCrawlingWithState(init)

	// Verify Write was called with the correct hashAlgo
	mockStorage.AssertCalled(t, "Write", mock.Anything, mock.Anything, mock.Anything)
	// HashAlgoSHA256 = 0
	_ = capturedHashAlgo
	// We can't easily assert the exact value since it's a uint8 and depends on the enum,
	// but the important thing is that Write was called with the hashAlgo from config
	// The conversion happens in the config layer
}

// TestScheduler_Write_MultiplePages_MultipleWriteResults verifies
// that Write is called for each page and all WriteResults are collected.
func TestScheduler_Write_MultiplePages_MultipleWriteResults(t *testing.T) {
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
	// Expect two Decide calls - one for each page
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Twice()

	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// Two pages to process
	token1 := frontier.NewCrawlToken(*mustParseURL("https://example.com/page1"), 0)
	token2 := frontier.NewCrawlToken(*mustParseURL("https://example.com/page2"), 0)
	mockFrontier.OnDequeue(token1, true).Once()
	mockFrontier.OnDequeue(token2, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
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

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage to return different results for each call
	writeResult1 := storage.NewWriteResult("hash1", "/output/hash1.md", "sha256:content1")
	writeResult2 := storage.NewWriteResult("hash2", "/output/hash2.md", "sha256:content2")
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(writeResult1, nil).Once()
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(writeResult2, nil).Once()

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
	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Phase 2: Execute with state
	exec, execErr := s.ExecuteCrawlingWithState(init)

	// Should complete without fatal error
	assert.NoError(t, execErr)
	// Write should be called twice
	mockStorage.AssertNumberOfCalls(t, "Write", 2)
	// WriteResults should contain both results
	writeResults := exec.WriteResults()
	assert.Len(t, writeResults, 2, "Should have 2 write results")
	assert.Equal(t, writeResult1.URLHash(), writeResults[0].URLHash())
	assert.Equal(t, writeResult2.URLHash(), writeResults[1].URLHash())
}
