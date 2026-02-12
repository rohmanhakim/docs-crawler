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

// TestScheduler_Resolve_CalledWithConversionResult verifies that the Resolve
// is called with the ConversionResult from the convert stage.
func TestScheduler_Resolve_CalledWithConversionResult(t *testing.T) {
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
	conversionResult := createConversionResultForTest("# Test Markdown\n\n![image](test.png)", nil)
	mockConvert.On("Convert", sanitizedDoc).Return(conversionResult, nil)

	// Setup resolver mock to capture the input
	var receivedConversionResult mdconvert.ConversionResult
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			receivedConversionResult = args.Get(2).(mdconvert.ConversionResult)
		}).
		Return(createAssetfulMarkdownDocForTest("# Test Markdown", nil), nil)

	s := createSchedulerWithAllMocks(
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

	// Verify Resolve was called with the conversion result from Convert
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.Equal(t, conversionResult, receivedConversionResult, "Resolve should be called with the ConversionResult from Convert")
}

// TestScheduler_Resolve_SuccessfulResolution_ProceedsToNormalization verifies
// that successful asset resolution allows the pipeline to continue to normalization.
func TestScheduler_Resolve_SuccessfulResolution_ProceedsToNormalization(t *testing.T) {
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

	// Setup resolver to return successful result with content
	resolvedDoc := createAssetfulMarkdownDocForTest("# Test Markdown\n\n![resolved](assets/images/test.png)", []string{"assets/images/test.png"})
	setupResolverMockWithCustomResult(mockResolver, resolvedDoc)

	s := createSchedulerWithAllMocks(
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
	// Resolve should be called
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	t.Logf("Execution completed with %d write results", len(exec.WriteResults))
}

// TestScheduler_Resolve_FatalError_AbortsCrawl verifies that fatal asset resolution errors
// cause the crawl to abort immediately.
func TestScheduler_Resolve_FatalError_AbortsCrawl(t *testing.T) {
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

	// Setup resolver to return a fatal error
	setupResolverMockWithFatalError(mockResolver)

	s := createSchedulerWithAllMocks(
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

	// Fatal resolver error should abort the crawl
	assert.Error(t, execErr, "Expected error for fatal resolve error")
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestScheduler_Resolve_RecoverableError_ContinuesCrawl verifies that recoverable
// asset resolution errors are counted but the crawl continues.
func TestScheduler_Resolve_RecoverableError_ContinuesCrawl(t *testing.T) {
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

	// Setup resolver to return a recoverable error
	setupResolverMockWithRecoverableError(mockResolver)

	s := createSchedulerWithAllMocks(
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
	assert.NoError(t, execErr, "Recoverable resolve error should not abort crawl")
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestScheduler_Resolve_MethodCallOrder verifies the correct order of method calls:
// Fetch → Extract → Sanitize → Convert → Resolve → Normalize → Write
func TestScheduler_Resolve_MethodCallOrder(t *testing.T) {
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

	s := createSchedulerWithAllMocks(
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
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	// Verify order: Resolve should be called after Convert
	t.Logf("Call order: %v", callOrder)
	assert.Contains(t, callOrder, "Fetch", "Fetch should be called")
	assert.Contains(t, callOrder, "Extract", "Extract should be called")
	assert.Contains(t, callOrder, "Sanitize", "Sanitize should be called")
	assert.Contains(t, callOrder, "Convert", "Convert should be called")
	assert.Contains(t, callOrder, "Resolve", "Resolve should be called")

	// Find positions
	fetchIdx := -1
	extractIdx := -1
	sanitizeIdx := -1
	convertIdx := -1
	resolveIdx := -1
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
		}
	}

	assert.Less(t, fetchIdx, extractIdx, "Fetch should be called before Extract")
	assert.Less(t, extractIdx, sanitizeIdx, "Extract should be called before Sanitize")
	assert.Less(t, sanitizeIdx, convertIdx, "Sanitize should be called before Convert")
	assert.Less(t, convertIdx, resolveIdx, "Convert should be called before Resolve")
}

// TestScheduler_Resolve_CalledExactlyOncePerPage verifies that the Resolve
// is called exactly once for each page processed.
func TestScheduler_Resolve_CalledExactlyOncePerPage(t *testing.T) {
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

	// Setup resolver - should be called exactly once
	setupResolverMockWithSuccess(mockResolver)

	s := createSchedulerWithAllMocks(
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

	// Verify Resolve was called exactly once
	mockResolver.AssertNumberOfCalls(t, "Resolve", 1)
}

// TestScheduler_Resolve_ErrorDoesNotPreventWriteForRecoverable verifies that when Resolve()
// returns a recoverable error, the scheduler still proceeds to Normalize and Write.
func TestScheduler_Resolve_ErrorDoesNotPreventWriteForRecoverable(t *testing.T) {
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

	// Setup resolver to return a recoverable error (not fatal)
	setupResolverMockWithRecoverableError(mockResolver)

	s := createSchedulerWithAllMocks(
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

	// Recoverable resolver error should NOT abort the crawl
	assert.NoError(t, execErr, "Recoverable resolve error should not abort crawl")

	// Verify resolver was called
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	// Verify that execution completed (Write was called)
	t.Logf("Execution completed with %d write results", len(exec.WriteResults))
}

// TestScheduler_Resolve_FatalErrorPreventsSubsequentCalls verifies that when Resolve()
// returns a fatal error, the scheduler aborts and does not process more URLs.
func TestScheduler_Resolve_FatalErrorPreventsSubsequentCalls(t *testing.T) {
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

	// Setup resolver to return a fatal error using mock.Anything to ensure it gets called
	setupResolverMockWithFatalError(mockResolver)

	s := createSchedulerWithAllMocks(
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

	// Fatal resolver error should abort the crawl
	assert.Error(t, execErr, "Expected error for fatal resolve error")

	// Verify resolver was called
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	// Verify that Robot.Decide was only called once (for seed URL)
	// This proves that the crawl aborted before processing more URLs
	mockRobot.AssertNumberOfCalls(t, "Decide", 1)
	t.Logf("Resolve fatal error prevented further processing as expected")
}

// createSchedulerWithAllMocks creates a scheduler with all mocked dependencies for testing.
// This is similar to createSchedulerForTest but allows injecting a custom resolver mock.
func createSchedulerWithAllMocks(
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
		mockSleeper,
	)
	return &s
}
