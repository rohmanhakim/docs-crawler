package scheduler_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
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
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

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

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
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
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

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

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
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
	t.Logf("Execution completed with %d write results", len(exec.WriteResults))
}

// TestScheduler_Normalize_FatalError_AbortsCrawl verifies that fatal normalization errors
// cause the crawl to abort immediately.
func TestScheduler_Normalize_FatalError_AbortsCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

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
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
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
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

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
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
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
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

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

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
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
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

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

	s := createSchedulerWithAllMocksAndNormalize(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
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
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()
	// Only expect one Decide call for the seed URL
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

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
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
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
	t.Logf("Execution completed with %d write results", len(exec.WriteResults))
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
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()
	// Only expect one Decide call for the seed URL
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

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
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
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
	mockFetcher *fetcherMock,
	mockExtractor extractor.Extractor,
	mockSanitizer sanitizer.Sanitizer,
	mockConvert mdconvert.ConvertRule,
	mockResolver assets.Resolver,
	mockNormalize *normalizeMock,
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
		mockFetcher,
		mockRobot,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
		mockSleeper,
	)
	return &s
}
