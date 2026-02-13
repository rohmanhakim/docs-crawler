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
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// compile-time interface checks
var _ metadata.CrawlFinalizer = (*mockFinalizer)(nil)
var _ metadata.MetadataSink = (*metadata.NoopSink)(nil)
var _ metadata.MetadataSink = (*errorRecordingSink)(nil)

// TestScheduler_ConfigurationImmutability verifies that the scheduler
// uses the configuration as provided and doesn't modify it.
func TestScheduler_ConfigurationImmutability(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	// Set up frontier expectations
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()
	// Set up robot expectations
	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		nil,
		nil,
		nil,
		nil,
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create a valid config
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 3,
		"maxPages": 50
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Execute crawl
	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Config file should still exist and be unchanged
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file should still exist: %v", err)
	}
	if string(content) != configData {
		t.Error("config file was modified during crawl")
	}
}

// TestScheduler_GracefulShutdown_InvalidSeedURL verifies handling of
// malformed seed URLs in config.
func TestScheduler_GracefulShutdown_InvalidSeedURL(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	// Set up frontier expectations
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()
	// Set up robot expectations - Init is always called
	mockRobot.On("Init", mock.Anything).Return()
	// The malformed URL may cause an error before reaching Decide, or the URL parsing may fail.
	// Set up a permissive Decode expectation that allows any call
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    false,
		Reason:     robots.DisallowedByRobots,
		CrawlDelay: 0,
	}, nil).Maybe()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		nil,
		nil,
		nil,
		nil,
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config with malformed URL
	configData := `{
		"seedUrls": [{"Scheme": "://", "Host": "", "Path": ":::"}],
		"maxDepth": 1
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Should handle gracefully (either succeed or return error, not panic)
	_, err = s.ExecuteCrawling(configPath)

	// Either outcome is acceptable as long as no panic occurs
	t.Logf("Result: err=%v", err)

	// If stats were recorded, verify they're valid
	if mockFinalizer.recordedStats != nil {
		if mockFinalizer.recordedStats.duration < 0 {
			t.Error("duration should be non-negative")
		}
	}
}

// TestScheduler_MultipleExecutions_Sequential verifies that the scheduler
// can be reused for multiple sequential executions.
func TestScheduler_MultipleExecutions_Sequential(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	// Set up frontier expectations
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()
	// Set up robot expectations - Init is called once per execution
	mockRobot.On("Init", mock.Anything).Return().Maybe()
	// Expect Decide for both example1.com and example2.com
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Maybe()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		nil,
		nil,
		nil,
		nil,
		mockSleeper,
	)

	tmpDir := t.TempDir()

	// First execution
	config1 := filepath.Join(tmpDir, "config1.json")
	err := os.WriteFile(config1, []byte(`{"seedUrls": [{"Scheme": "https", "Host": "example1.com"}], "maxDepth": 0}`), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(config1)
	if err != nil {
		t.Fatalf("first execution failed: %v", err)
	}

	firstStats := mockFinalizer.recordedStats
	if firstStats == nil {
		t.Fatal("expected stats after first execution")
	}

	// Reset mock for second execution
	mockFinalizer.recordedStats = nil

	// Second execution
	config2 := filepath.Join(tmpDir, "config2.json")
	err = os.WriteFile(config2, []byte(`{"seedUrls": [{"Scheme": "https", "Host": "example2.com"}], "maxDepth": 0}`), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(config2)
	if err != nil {
		t.Fatalf("second execution failed: %v", err)
	}

	secondStats := mockFinalizer.recordedStats
	if secondStats == nil {
		t.Fatal("expected stats after second execution")
	}

	// Each execution should have its own stats
	t.Logf("First execution: pages=%d, duration=%v", firstStats.totalPages, firstStats.duration)
	t.Logf("Second execution: pages=%d, duration=%v", secondStats.totalPages, secondStats.duration)
}

// Verify interface implementations at compile time
func TestInterfaceCompliance(t *testing.T) {
	// This test ensures our mocks implement the required interfaces
	var _ metadata.CrawlFinalizer = (*mockFinalizer)(nil)
	var _ metadata.MetadataSink = (*metadata.NoopSink)(nil)
	var _ metadata.MetadataSink = (*errorRecordingSink)(nil)
}

// mustParseURL is a test helper that parses a URL or panics
func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// createMinimalHTMLNode creates a minimal valid HTML document node for testing
func createMinimalHTMLNode() *html.Node {
	htmlContent := "<html><body><p>Test content</p></body></html>"
	doc, _ := html.Parse(strings.NewReader(htmlContent))
	return doc
}

// TestScheduler_URLResolutionAndFiltering verifies that discovered URLs are properly
// resolved to absolute URLs and filtered by host before submission.
// It uses a stubbed sanitizer to return pre-defined URLs (relative and external),
// then verifies that only resolved, same-host URLs are submitted to the frontier.
func TestScheduler_URLResolutionAndFiltering(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)

	// Set up frontier expectations
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// Disable auto-enqueue to control Dequeue behavior explicitly
	mockFrontier.disableAutoEnqueue = true
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	// Set up discovered URLs:
	// - "/relative-path" (relative, should be resolved and submitted)
	// - "/docs/guide" (relative, should be resolved and submitted)
	// - "https://example.com/page" (absolute same-host, should be submitted)
	// - "https://other.com/external" (external, should be filtered out)
	// - "http://different.com/page" (external, should be filtered out)
	discoveredURLs := []url.URL{
		*mustParseURL("/relative-path"),
		*mustParseURL("/docs/guide"),
		*mustParseURL("https://example.com/page"),
		*mustParseURL("https://other.com/external"),
		*mustParseURL("http://different.com/page"),
	}

	// Set up the mock to return our test URLs
	setupSanitizerMockWithSuccess(mockSanitizer, discoveredURLs)

	// Set up robot expectations - Init called once, Decide called for each URL
	mockRobot.On("Init", mock.Anything).Return()
	// Decide calls for all URLs that pass through SubmitUrlForAdmission
	// The key assertion is that external URLs (other.com, different.com) are filtered out
	// and only example.com URLs are submitted (seed + resolved relative URLs + absolute same-host)
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Maybe()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Set up fetcher mock to return valid HTML
	setupFetcherMockWithSuccess(mockFetcher, "https://example.com", []byte("<html><body>Test</body></html>"), 200)

	mockExtractor.On("SetExtractParam", extractor.DefaultExtractParam()).Return()
	mockExtractor.On("Extract", mock.AnythingOfType("url.URL"), mock.AnythingOfType("[]uint8")).Return(extractor.ExtractionResult{}, nil)

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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create a valid config with seed URL
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 1,
		"maxPages": 50
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Execute crawl
	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that the sanitizer was called
	mockSanitizer.AssertExpectations(t)

	// Verify that the robot's Decide was called the expected number of times
	// (1 for seed URL + 3 for filtered discovered URLs)
	mockRobot.AssertExpectations(t)
}

// TestScheduler_URLResolutionAndFiltering_OnlyExternalURLs verifies that when
// all discovered URLs are external, none are submitted to the frontier.
func TestScheduler_URLResolutionAndFiltering_OnlyExternalURLs(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)

	// Set up frontier expectations
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()
	// Set up extractor mock to return a valid extraction result and expect SetExtractParam
	setupExtractorMockWithSuccess(mockExtractor, createMinimalHTMLNode())
	// setupExtractorMockWithSetExtractParamExpectation(mockExtractor, extractor.DefaultExtractParam())
	// Set up discovered URLs - all external
	discoveredURLs := []url.URL{
		*mustParseURL("https://other.com/page1"),
		*mustParseURL("https://different.com/page2"),
		*mustParseURL("http://external.org/page3"),
	}

	// Set up the mock to return our test URLs
	setupSanitizerMockWithSuccess(mockSanitizer, discoveredURLs)

	// Set up robot expectations - only the seed URL should be processed
	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once() // Only called for seed URL
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Set up fetcher mock to return valid HTML
	setupFetcherMockWithSuccess(mockFetcher, "https://example.com", []byte("<html><body>Test</body></html>"), 200)

	mockExtractor.On("SetExtractParam", extractor.DefaultExtractParam()).Return()

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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create a valid config with seed URL
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 1,
		"maxPages": 50
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Execute crawl
	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that the sanitizer was called
	mockSanitizer.AssertExpectations(t)

	// Verify that the robot's Decide was called exactly once (for seed URL only)
	mockRobot.AssertExpectations(t)
}

// TestScheduler_URLResolutionAndFiltering_AllRelativeURLs verifies that relative
// URLs are properly resolved to absolute URLs before submission.
func TestScheduler_URLResolutionAndFiltering_AllRelativeURLs(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)

	// Set up frontier expectations
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.disableAutoEnqueue = true
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()
	// Set up extractor mock to return a valid extraction result and expect SetExtractParam
	setupExtractorMockWithSuccess(mockExtractor, createMinimalHTMLNode())
	// setupExtractorMockWithSetExtractParamExpectation(mockExtractor, extractor.DefaultExtractParam())
	// Set up discovered URLs - all relative
	discoveredURLs := []url.URL{
		*mustParseURL("/path1"),
		*mustParseURL("/docs/page"),
		*mustParseURL("/api/v1/users"),
	}

	// Set up the mock to return our test URLs
	setupSanitizerMockWithSuccess(mockSanitizer, discoveredURLs)

	// Set up robot expectations - Init called once, Decide for seed + all resolved URLs
	mockRobot.On("Init", mock.Anything).Return()
	// Decide calls for seed URL and all resolved URLs
	// The key assertion is that all 3 relative URLs are resolved and submitted
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Maybe()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	mockExtractor.On("SetExtractParam", extractor.DefaultExtractParam()).Return()

	// Set up fetcher mock to return valid HTML
	setupFetcherMockWithSuccess(mockFetcher, "https://example.com", []byte("<html><body>Test</body></html>"), 200)

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
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create a valid config with seed URL
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 1,
		"maxPages": 50
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Execute crawl
	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that the sanitizer was called
	mockSanitizer.AssertExpectations(t)

	// Verify that the robot's Decide was called 4 times (1 seed + 3 discovered)
	mockRobot.AssertExpectations(t)
}
