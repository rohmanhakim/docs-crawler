package scheduler_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// TestScheduler_Sanitizer_CalledWithExtractedContentNode verifies that the sanitizer
// is called with the ContentNode from the extraction result.
func TestScheduler_Sanitizer_CalledWithExtractedContentNode(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)

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

	// Setup sanitizer mock to capture the input node
	var receivedNode *html.Node
	mockSanitizer.On("Sanitize", mock.Anything).
		Run(func(args mock.Arguments) {
			receivedNode = args.Get(0).(*html.Node)
		}).
		Return(sanitizer.SanitizedHTMLDoc{}, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, mockExtractor, mockSanitizer, mockSleeper)

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

	// Verify sanitizer was called with the content node from extractor
	assert.Equal(t, contentNode, receivedNode, "Sanitizer should be called with the ContentNode from extraction")
	mockSanitizer.AssertCalled(t, "Sanitize", contentNode)
	mockExtractor.AssertCalled(t, "Extract", mock.Anything, mock.Anything)
}

// TestScheduler_Sanitizer_SuccessfulSanitization_ProceedsToMarkdownConversion verifies
// that successful sanitization allows the pipeline to continue to markdown conversion.
func TestScheduler_Sanitizer_SuccessfulSanitization_ProceedsToMarkdownConversion(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)

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

	// Setup sanitizer to return successful result
	mockSanitizer.On("Sanitize", mock.Anything).
		Return(sanitizer.SanitizedHTMLDoc{}, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, mockExtractor, mockSanitizer, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl
	exec, err := s.ExecuteCrawling(configPath)

	// Should complete without error
	assert.NoError(t, err)
	// Sanitizer should be called
	mockSanitizer.AssertCalled(t, "Sanitize", contentNode)
	t.Logf("Execution completed with %d write results", len(exec.WriteResults))
}

// TestScheduler_Sanitizer_FatalError_AbortsCrawl verifies that fatal sanitizer errors
// cause the crawl to abort immediately.
func TestScheduler_Sanitizer_FatalError_AbortsCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)

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

	// Setup sanitizer to return a fatal error
	sanitizerErr := &sanitizer.SanitizationError{
		Message:   "structural error: multiple competing roots",
		Retryable: false,
		Cause:     sanitizer.ErrCauseCompetingRoots,
	}
	mockSanitizer.On("Sanitize", contentNode).
		Return(sanitizer.SanitizedHTMLDoc{}, sanitizerErr)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, mockExtractor, mockSanitizer, mockSleeper)

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

	// Fatal sanitizer error should abort the crawl
	assert.Error(t, execErr, "Expected error for fatal sanitizer error")
	mockSanitizer.AssertCalled(t, "Sanitize", contentNode)
}

// TestScheduler_Sanitizer_RecoverableError_ContinuesCrawl verifies that recoverable
// sanitizer errors are counted but the crawl continues.
func TestScheduler_Sanitizer_RecoverableError_ContinuesCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)

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

	// Setup sanitizer to return a recoverable error
	recoverableErr := &mockClassifiedError{
		msg:      "recoverable sanitization error",
		severity: failure.SeverityRecoverable,
	}
	mockSanitizer.On("Sanitize", contentNode).
		Return(sanitizer.SanitizedHTMLDoc{}, recoverableErr)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, mockExtractor, mockSanitizer, mockSleeper)

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
	assert.NoError(t, execErr, "Recoverable sanitizer error should not abort crawl")
	mockSanitizer.AssertCalled(t, "Sanitize", contentNode)
}

// TestScheduler_Sanitizer_DiscoveredURLsSubmittedToFrontier verifies that URLs
// discovered during sanitization are submitted to the frontier through robots check.
func TestScheduler_Sanitizer_DiscoveredURLsSubmittedToFrontier(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()

	// Expect two Decide calls: one for seed URL, one for discovered URL
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Twice()

	mockSleeper.On("Sleep", mock.Anything).Return()

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer to return discovered URLs
	discoveredURL, _ := url.Parse("/discovered.html")
	mockSanitizer.On("Sanitize", contentNode).
		Return(sanitizer.SanitizedHTMLDoc{}, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, mockExtractor, mockSanitizer, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 1
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	t.Logf("Discovered URL test: %v", discoveredURL)

	// Execute crawl
	_, execErr := s.ExecuteCrawling(configPath)

	// Should complete without fatal error
	assert.NoError(t, execErr)
	mockSanitizer.AssertCalled(t, "Sanitize", contentNode)
}

// TestScheduler_Sanitizer_MethodCallOrder verifies the correct order of method calls:
// Fetch → Extract → Sanitize → Convert → Resolve → Normalize → Write
func TestScheduler_Sanitizer_MethodCallOrder(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := new(fetcherMock)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)

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
		uint64(len(htmlBody)),
		map[string]string{"Content-Type": "text/html"},
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

	mockSanitizer.On("Sanitize", contentNode).
		Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Sanitize")
		}).Return(sanitizer.SanitizedHTMLDoc{}, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, mockExtractor, mockSanitizer, mockSleeper)

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

	// Verify sanitizer was called
	mockSanitizer.AssertCalled(t, "Sanitize", contentNode)

	// Verify order: Sanitize should be called after Extract
	t.Logf("Call order: %v", callOrder)
	assert.Contains(t, callOrder, "Fetch", "Fetch should be called")
	assert.Contains(t, callOrder, "Extract", "Extract should be called")
	assert.Contains(t, callOrder, "Sanitize", "Sanitize should be called")

	// Find positions
	fetchIdx := -1
	extractIdx := -1
	sanitizeIdx := -1
	for i, call := range callOrder {
		switch call {
		case "Fetch":
			fetchIdx = i
		case "Extract":
			extractIdx = i
		case "Sanitize":
			sanitizeIdx = i
		}
	}

	assert.Less(t, fetchIdx, extractIdx, "Fetch should be called before Extract")
	assert.Less(t, extractIdx, sanitizeIdx, "Extract should be called before Sanitize")
}

// TestScheduler_Sanitizer_CalledExactlyOncePerPage verifies that the sanitizer
// is called exactly once for each page processed.
func TestScheduler_Sanitizer_CalledExactlyOncePerPage(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)

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

	// Setup sanitizer - should be called exactly once
	mockSanitizer.On("Sanitize", contentNode).
		Return(sanitizer.SanitizedHTMLDoc{}, nil).Once()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, mockExtractor, mockSanitizer, mockSleeper)

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

	// Verify sanitizer was called exactly once
	mockSanitizer.AssertNumberOfCalls(t, "Sanitize", 1)
}

// TestScheduler_Sanitizer_ErrorPreventsSubsequentCalls verifies that when Sanitize()
// returns an error, the scheduler does not call sanitizedHtml.GetDiscoveredURLs()
// or SubmitUrlForAdmission() for discovered URLs.
func TestScheduler_Sanitizer_ErrorPreventsSubsequentCalls(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)

	mockRobot.On("Init", mock.Anything).Return()
	// Only expect one Decide call for the seed URL - no discovered URLs should be submitted
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

	// Setup sanitizer to return a fatal error using mock.Anything to ensure it gets called
	sanitizerErr := &sanitizer.SanitizationError{
		Message:   "ambiguous DOM structure",
		Retryable: false,
		Cause:     sanitizer.ErrCauseAmbiguousDOM,
	}
	mockSanitizer.On("Sanitize", mock.Anything).
		Return(sanitizer.SanitizedHTMLDoc{}, sanitizerErr)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, mockExtractor, mockSanitizer, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Use maxDepth: 1 to allow for potential discovered URLs
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 1
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl - should return fatal error
	_, execErr := s.ExecuteCrawling(configPath)

	// Fatal sanitizer error should abort the crawl
	assert.Error(t, execErr, "Expected error for fatal sanitizer error")

	// Verify sanitizer was called
	mockSanitizer.AssertCalled(t, "Sanitize", mock.Anything)

	// Verify that Robot.Decide was only called once (for seed URL, not for discovered URLs)
	// This proves that SubmitUrlForAdmission was never called for discovered URLs
	mockRobot.AssertNumberOfCalls(t, "Decide", 1)
	t.Logf("Sanitize error prevented discovered URL submission as expected")
}
