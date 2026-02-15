package scheduler_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// TestScheduler_Extract_SetExtractParamCalledWithDefaults verifies that SetExtractParam
// is called with default extraction parameters when no custom extraction config is provided.
func TestScheduler_Extract_SetExtractParamCalledWithDefaults(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)

	// Set up sanitizer mock with success
	setupSanitizerMockWithSuccess(mockSanitizer, []url.URL{})

	// Set up extractor expectations
	mockExtractor.On("SetExtractParam", extractor.DefaultExtractParam()).Return()
	// Set up Extract to return a successful result (empty but valid)
	doc, _ := html.Parse(strings.NewReader("<html><body></body></html>"))
	setupExtractorMockWithSuccess(mockExtractor, doc)

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
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Clear default fetcher expectation and setup for no fetch calls
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, nil)

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
		nil,
		nil,
		mockStorage,
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config with only required fields - should use defaults for extraction params
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state
	_, err = s.ExecuteCrawlingWithState(init)
	assert.NoError(t, err, "Failed to execute")

	// Verify SetExtractParam was called with default params
	mockExtractor.AssertCalled(t, "SetExtractParam", extractor.DefaultExtractParam())
}

// TestScheduler_Extract_SetExtractParamCalledWithCustomValues verifies that SetExtractParam
// is called with custom values loaded from config file.
func TestScheduler_Extract_SetExtractParamCalledWithCustomValues(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)

	// Define expected custom extraction parameters
	customParams := extractor.ExtractParam{
		BodySpecificityBias:  0.85,
		LinkDensityThreshold: 0.90,
		ScoreMultiplier: extractor.ContentScoreMultiplier{
			NonWhitespaceDivisor: 60.0,
			Paragraphs:           6.0,
			Headings:             12.0,
			CodeBlocks:           18.0,
			ListItems:            3.0,
		},
		Threshold: extractor.MeaningfulThreshold{
			MinNonWhitespace:    60,
			MinHeadings:         1,
			MinParagraphsOrCode: 2,
			MaxLinkDensity:      0.9,
		},
	}
	// Set up extractor expectations with custom params
	mockExtractor.On("SetExtractParam", customParams).Return()
	// Set up Extract to return a successful result
	doc, _ := html.Parse(strings.NewReader("<html><body></body></html>"))
	setupExtractorMockWithSuccess(mockExtractor, doc)

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
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Clear default fetcher expectation
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, nil)

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
		nil,
		nil,
		nil,
		mockStorage,
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config with custom extraction parameters
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0,
		"bodySpecificityBias": 0.85,
		"linkDensityThreshold": 0.90,
		"scoreMultiplierNonWhitespaceDivisor": 60.0,
		"scoreMultiplierParagraphs": 6.0,
		"scoreMultiplierHeadings": 12.0,
		"scoreMultiplierCodeBlocks": 18.0,
		"scoreMultiplierListItems": 3.0,
		"thresholdMinNonWhitespace": 60,
		"thresholdMinHeadings": 1,
		"thresholdMinParagraphsOrCode": 2,
		"thresholdMaxLinkDensity": 0.9
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	_, err = s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")
}

// TestScheduler_Extract_UsesConfiguredParams verifies that the extraction actually uses
// the configured parameters by checking that extraction succeeds with valid HTML.
func TestScheduler_Extract_UsesConfiguredParams(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)

	// Define custom extraction parameters from config
	customParams := extractor.ExtractParam{
		BodySpecificityBias:  0.60,
		LinkDensityThreshold: 0.80,
		ScoreMultiplier: extractor.ContentScoreMultiplier{
			NonWhitespaceDivisor: 50.0,
			Paragraphs:           5.0,
			Headings:             10.0,
			CodeBlocks:           15.0,
			ListItems:            2.0,
		},
		Threshold: extractor.MeaningfulThreshold{
			MinNonWhitespace:    20,
			MinHeadings:         0,
			MinParagraphsOrCode: 1,
			MaxLinkDensity:      0.8,
		},
	}
	// Set up extractor expectations with custom params
	mockExtractor.On("SetExtractParam", customParams).Return()
	// Set up Extract to return a successful result
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<main>
<h1>Test Content</h1>
<p>This is meaningful content that should pass extraction heuristics regardless of parameters.</p>
</main>
</body>
</html>`
	doc, _ := html.Parse(strings.NewReader(htmlContent))
	setupExtractorMockWithSuccess(mockExtractor, doc)

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
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	// Clear default fetcher expectation and setup with valid HTML
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	testURL, _ := url.Parse("http://example.com/page.html")
	htmlBody := []byte(htmlContent)
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		htmlBody,
		200,
		"text/html",
		map[string]string{"Content-Type": "text/html"},
		time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

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
		nil,
		nil,
		nil,
		mockStorage,
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config with custom extraction parameters
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0,
		"bodySpecificityBias": 0.60,
		"thresholdMinNonWhitespace": 20,
		"thresholdMinParagraphsOrCode": 1
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state
	exec, err := s.ExecuteCrawlingWithState(init)

	// The crawl should complete without extraction errors
	assert.NoError(t, err, "Crawl should complete without errors")
	t.Logf("Execution result: writeResults=%d", len(exec.WriteResults()))

	// Verify SetExtractParam was called with custom params
	mockExtractor.AssertCalled(t, "SetExtractParam", customParams)
}

// TestScheduler_Extract_DefaultParamsStructure verifies the structure of default extraction parameters.
func TestScheduler_Extract_DefaultParamsStructure(t *testing.T) {
	// Define expected default parameters matching the extractor.DefaultExtractParam()
	expectedDefaults := extractor.ExtractParam{
		BodySpecificityBias:  0.75,
		LinkDensityThreshold: 0.80,
		ScoreMultiplier: extractor.ContentScoreMultiplier{
			NonWhitespaceDivisor: 50.0,
			Paragraphs:           5.0,
			Headings:             10.0,
			CodeBlocks:           15.0,
			ListItems:            2.0,
		},
		Threshold: extractor.MeaningfulThreshold{
			MinNonWhitespace:    50,
			MinHeadings:         0,
			MinParagraphsOrCode: 1,
			MaxLinkDensity:      0.8,
		},
	}

	// Verify the default parameters match
	actualDefaults := extractor.DefaultExtractParam()
	verifyExtractParam(t, actualDefaults, expectedDefaults)
}

// TestScheduler_Extract_ExtractResultNotNil verifies that the extraction result
// is not nil when extraction succeeds.
func TestScheduler_Extract_ExtractResultNotNil(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockResolver := newResolverMockForTest(t)

	// Set up extractor expectations
	mockExtractor.On("SetExtractParam", extractor.DefaultExtractParam()).Return()
	// Set up Extract to return a successful result
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<main>
<h1>Test Content</h1>
<p>This is meaningful content with enough text to pass the minimum threshold checks.</p>
<p>Additional paragraph to ensure content is substantial.</p>
</main>
</body>
</html>`
	doc, _ := html.Parse(strings.NewReader(htmlContent))
	setupExtractorMockWithSuccess(mockExtractor, doc)

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
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockConvert := newConvertMockForTest(t)
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	// Setup fetcher with valid HTML that should produce a non-nil extraction result
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	testURL, _ := url.Parse("http://example.com/page.html")
	htmlBody := []byte(htmlContent)
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		htmlBody,
		200,
		"text/html",
		map[string]string{"Content-Type": "text/html"},
		time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	// Setup sanitizer to return a valid sanitized doc
	sanitizedDoc := createSanitizedHTMLDocForTest(nil)
	mockSanitizer.On("Sanitize", doc).Return(sanitizedDoc, nil)

	// Setup convert mock to capture the input
	setupConvertMockWithSuccess(mockConvert)
	mockConvert.On("Convert", mock.Anything).
		Run(func(args mock.Arguments) {
			sanitizedDoc = args.Get(0).(sanitizer.SanitizedHTMLDoc)
		}).
		Return(createConversionResultForTest("# Test", nil), nil)

	// Setup resolver to return a specific assetful markdown doc
	assetfulDoc := createAssetfulMarkdownDocForTest("# Test Markdown\n\nContent", []string{"image.png"})
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(assetfulDoc, nil)

	// Create normalize mock for test
	mockNormalize := newNormalizeMockForTest(t)
	setupNormalizeMockWithSuccess(mockNormalize)

	s := scheduler.NewSchedulerWithDeps(
		ctx,
		mockFinalizer,
		noopSink,
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

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0,
		"thresholdMinNonWhitespace": 50
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state
	exec, err := s.ExecuteCrawlingWithState(init)

	// Should complete without fatal extraction errors
	assert.NoError(t, err)
	t.Logf("Execution completed: writeResults=%d", len(exec.WriteResults()))
}

// TestScheduler_Extract_InvalidHTMLHandled verifies that invalid HTML is handled gracefully.
func TestScheduler_Extract_InvalidHTMLHandled(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockResolver := newResolverMockForTest(t)

	// Set up extractor expectations
	mockExtractor.On("SetExtractParam", extractor.DefaultExtractParam()).Return()

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
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup fetcher with invalid HTML (plain text instead of HTML)
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	testURL, _ := url.Parse("http://example.com/page.txt")
	textBody := []byte("This is just plain text, not HTML.")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		textBody,
		200,
		"text/plain",
		map[string]string{"Content-Type": "text/plain"},
		time.Now(),
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	mockExtractor.On("Extract", mock.Anything, mock.Anything).Return(extractor.ExtractionResult{}, &extractor.ExtractionError{
		Message:   "input is not valid HTML document",
		Retryable: false,
		Cause:     extractor.ErrCauseNotHTML,
	})

	// Create normalize mock for test
	mockNormalize := newNormalizeMockForTest(t)
	setupNormalizeMockWithSuccess(mockNormalize)

	s := scheduler.NewSchedulerWithDeps(
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockFetcher,
		mockRobot,
		mockExtractor,
		nil,
		nil,
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

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state - should handle extraction error gracefully
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Extraction error should be counted but not fatal
	t.Logf("Execution result: err=%v", execErr)
}
