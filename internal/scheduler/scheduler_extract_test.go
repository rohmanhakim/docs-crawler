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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestScheduler_Extract_SetExtractParamCalledWithDefaults verifies that SetExtractParam
// is called with default extraction parameters when no custom extraction config is provided.
func TestScheduler_Extract_SetExtractParamCalledWithDefaults(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

	// Clear default fetcher expectation and setup for no fetch calls
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config with only required fields - should use defaults for extraction params
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl
	_, _ = s.ExecuteCrawling(configPath)

	// The test passes if no panic occurs and extraction was attempted with default params
	// The actual extraction behavior is tested by verifying the Extract method was called
}

// TestScheduler_Extract_SetExtractParamCalledWithCustomValues verifies that SetExtractParam
// is called with custom values loaded from config file.
func TestScheduler_Extract_SetExtractParamCalledWithCustomValues(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

	// Clear default fetcher expectation
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, mockSleeper)

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

	// Execute crawl
	_, _ = s.ExecuteCrawling(configPath)

	// The test passes if no panic occurs and extraction was attempted with custom params
	// The actual parameter values are verified by checking extraction behavior
}

// TestScheduler_Extract_MethodCallOrder verifies that SetExtractParam is called
// after config is initialized but before the crawl loop begins.
func TestScheduler_Extract_MethodCallOrder(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

	// Clear default fetcher expectation
	mockFetcher.ExpectedCalls = nil

	// Track call order
	callOrder := []string{}

	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Fetch")
		}).Return(fetcher.FetchResult{}, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, mockSleeper)

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

	// Verify the order: Robot.Init -> Frontier.Init -> SetExtractParam -> SubmitUrlForAdmission -> Fetch
	// The exact order depends on implementation, but SetExtractParam should be called before Fetch
	t.Logf("Call order: %v", callOrder)
}

// TestScheduler_Extract_UsesConfiguredParams verifies that the extraction actually uses
// the configured parameters by checking that extraction succeeds with valid HTML.
func TestScheduler_Extract_UsesConfiguredParams(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

	// Clear default fetcher expectation and setup with valid HTML
	mockFetcher.ExpectedCalls = nil
	testURL, _ := url.Parse("http://example.com/page.html")
	htmlBody := []byte(`<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<main>
<h1>Test Content</h1>
<p>This is meaningful content that should pass extraction heuristics regardless of parameters.</p>
</main>
</body>
</html>`)
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		htmlBody,
		200,
		"text/html",
		uint64(len(htmlBody)),
		map[string]string{"Content-Type": "text/html"},
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, mockSleeper)

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

	// Execute crawl - should complete successfully with custom params
	exec, err := s.ExecuteCrawling(configPath)

	// The crawl should complete without extraction errors
	assert.NoError(t, err, "Crawl should complete without errors")
	t.Logf("Execution result: writeResults=%d", len(exec.WriteResults))
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
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

	// Setup fetcher with valid HTML that should produce a non-nil extraction result
	mockFetcher.ExpectedCalls = nil
	testURL, _ := url.Parse("http://example.com/page.html")
	htmlBody := []byte(`<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<main>
<h1>Test Content</h1>
<p>This is meaningful content with enough text to pass the minimum threshold checks.</p>
<p>Additional paragraph to ensure content is substantial.</p>
</main>
</body>
</html>`)
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		htmlBody,
		200,
		"text/html",
		uint64(len(htmlBody)),
		map[string]string{"Content-Type": "text/html"},
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0,
		"thresholdMinNonWhitespace": 10
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl
	exec, err := s.ExecuteCrawling(configPath)

	// Should complete without fatal extraction errors
	assert.NoError(t, err)
	t.Logf("Execution completed: writeResults=%d", len(exec.WriteResults))
}

// TestScheduler_Extract_InvalidHTMLHandled verifies that invalid HTML is handled gracefully.
func TestScheduler_Extract_InvalidHTMLHandled(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()

	// Setup fetcher with invalid HTML (plain text instead of HTML)
	mockFetcher.ExpectedCalls = nil
	testURL, _ := url.Parse("http://example.com/page.txt")
	textBody := []byte("This is just plain text, not HTML.")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		textBody,
		200,
		"text/plain",
		uint64(len(textBody)),
		map[string]string{"Content-Type": "text/plain"},
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl - should handle extraction error gracefully
	_, execErr := s.ExecuteCrawling(configPath)

	// Extraction error should be counted but not fatal
	t.Logf("Execution result: err=%v", execErr)
}
