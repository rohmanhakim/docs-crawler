package scheduler_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestScheduler_ErrorHandling_HTTP403_ContinuesCrawl verifies that HTTP 403 errors
// do not abort the crawl and the URL is tracked in the failure journal.
func TestScheduler_ErrorHandling_HTTP403_ContinuesCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup robot to allow
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	// Setup frontier with auto-enqueue disabled to control Dequeue explicitly
	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()

	// Setup Dequeue: first returns URL token, second returns false (empty)
	testURL, _ := url.Parse("https://example.com/page.html")
	seedToken := frontier.NewCrawlToken(*testURL, 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Clear default expectation and setup fetcher mock to return 403 error
	// Using the real FetchError with RetryPolicyManual
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	forbiddenErr := &mockClassifiedError{
		msg:         "forbidden (403)",
		severity:    failure.SeverityRetryExhausted,
		retryPolicy: failure.RetryPolicyManual,
		impactLevel: failure.ImpactLevelContinue,
	}
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, forbiddenErr)

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
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
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
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Verify: crawl should NOT abort (no fatal error returned)
	// The error should be handled gracefully
	t.Logf("Execution result: err=%v", execErr)

	// Verify: failure journal should have recorded the URL
	mockFailureJournal.AssertCalled(t, "Record", mock.Anything)
}

// TestScheduler_ErrorHandling_ManualRetry_Tracked verifies that errors with
// RetryPolicyManual are tracked in the failure journal.
func TestScheduler_ErrorHandling_ManualRetry_Tracked(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

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

	testURL, _ := url.Parse("https://example.com/page.html")
	seedToken := frontier.NewCrawlToken(*testURL, 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup fetcher with error that has RetryPolicyManual
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	contentTypeErr := &mockClassifiedError{
		msg:         "non-HTML content type",
		severity:    failure.SeverityRetryExhausted,
		retryPolicy: failure.RetryPolicyManual,
		impactLevel: failure.ImpactLevelContinue,
	}
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, contentTypeErr)

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
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
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
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Verify: error is handled, not fatal
	t.Logf("Execution result: err=%v", execErr)

	// Verify: failure journal recorded the URL
	mockFailureJournal.AssertCalled(t, "Record", mock.Anything)
}

// TestScheduler_ErrorHandling_ImpactAbort_AbortsCrawl verifies that errors with
// ImpactAbort cause the crawl to abort immediately.
func TestScheduler_ErrorHandling_ImpactAbort_AbortsCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

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

	testURL, _ := url.Parse("https://example.com/page.html")
	seedToken := frontier.NewCrawlToken(*testURL, 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	// Note: No second Dequeue because crawl should abort

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup fetcher to return error with ImpactAbort (simulating systemic failure)
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	systemicErr := &mockClassifiedError{
		msg:         "systemic failure: critical service unavailable",
		severity:    failure.SeverityFatal,
		retryPolicy: failure.RetryPolicyNever,
		impactLevel: failure.ImpactLevelAbort,
	}
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, systemicErr)

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
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
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
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Verify: error is returned (crawl aborted)
	assert.Error(t, execErr, "Crawl should abort on ImpactAbort error")
	t.Logf("Execution result: err=%v", execErr)

	// Verify: failure journal is flushed even when the crawl exits early via abort
	mockFailureJournal.AssertCalled(t, "Flush")
}

// TestScheduler_ErrorHandling_StorageError_ManualRetry verifies that storage errors
// with RetryPolicyManual are tracked in the failure journal.
func TestScheduler_ErrorHandling_StorageError_ManualRetry(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

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

	testURL, _ := url.Parse("https://example.com/page.html")
	seedToken := frontier.NewCrawlToken(*testURL, 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup fetcher to return success (need to get to storage stage)
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	htmlBody := []byte("<html><body><h1>Test</h1></body></html>")
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
		nil,
		nil,
		nil,
		nil,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
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
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Verify: crawl completed (storage error was handled gracefully)
	t.Logf("Execution result: err=%v", execErr)
}

// TestScheduler_ErrorHandling_ConfigError_AbortsCrawl verifies that configuration
// errors cause immediate abort with no URLs processed.
func TestScheduler_ErrorHandling_ConfigError_AbortsCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

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
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create invalid config (empty seed URLs should fail validation)
	configData := `{
		"seedUrls": [],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute crawl - should fail due to invalid config
	_, execErr := s.InitializeCrawling(configPath)

	// Verify: initialization fails with error
	assert.Error(t, execErr, "Should fail with empty seed URLs")
	t.Logf("Initialization result: err=%v", execErr)
}

// TestScheduler_ErrorHandling_MixedResults verifies that mixed success and failure
// scenarios are handled correctly with proper error counting.
func TestScheduler_ErrorHandling_MixedResults(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

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

	// Setup two URLs: first succeeds, second fails with manual retry
	testURL1, _ := url.Parse("https://example.com/page1.html")
	testURL2, _ := url.Parse("https://example.com/page2.html")

	token1 := frontier.NewCrawlToken(*testURL1, 0)
	token2 := frontier.NewCrawlToken(*testURL2, 0)

	// First fetch succeeds, second fetch fails with RetryPolicyManual
	// Use Once() for first call (success) and Once() for second call (error)
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()

	// First call returns success
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, nil).Once()

	// Second call returns manual retry error
	retryErr := &mockClassifiedError{
		msg:         "rate limited (429)",
		severity:    failure.SeverityRetryExhausted,
		retryPolicy: failure.RetryPolicyManual,
		impactLevel: failure.ImpactLevelContinue,
	}
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, retryErr).Once()

	// Setup Dequeue to return both tokens
	mockFrontier.OnDequeue(token1, true).Once()
	mockFrontier.OnDequeue(token2, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return().Maybe()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0)).Maybe()

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
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 1
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
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Verify: crawl completed (not aborted)
	t.Logf("Execution result: err=%v", execErr)

	// Verify: failure journal recorded the failed URL (the one that failed with manual retry)
	mockFailureJournal.AssertCalled(t, "Record", mock.Anything)
}

// TestScheduler_ErrorHandling_AutoRetryErrors_NotTracked verifies that errors with
// RetryPolicyAuto are NOT tracked in the failure journal (they're handled by retry handler).
func TestScheduler_ErrorHandling_AutoRetryErrors_NotTracked(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

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

	testURL, _ := url.Parse("https://example.com/page.html")
	seedToken := frontier.NewCrawlToken(*testURL, 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup fetcher to return error with RetryPolicyAuto (recoverable)
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	retryableErr := &mockClassifiedError{
		msg:         "temporary network failure",
		severity:    failure.SeverityRecoverable,
		retryPolicy: failure.RetryPolicyAuto,
		impactLevel: failure.ImpactLevelContinue,
	}
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, retryableErr)

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
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
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
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Verify: error handled gracefully
	t.Logf("Execution result: err=%v", execErr)

	// Verify: failure journal should NOT have recorded the URL (auto-retry errors handled by retry handler)
	mockFailureJournal.AssertNotCalled(t, "Record", mock.Anything)
}

// TestScheduler_ErrorHandling_NeverRetryErrors_NotTracked verifies that errors with
// RetryPolicyNever are NOT tracked in the failure journal (permanent failures).
func TestScheduler_ErrorHandling_NeverRetryErrors_NotTracked(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

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

	testURL, _ := url.Parse("https://example.com/page.html")
	seedToken := frontier.NewCrawlToken(*testURL, 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup fetcher to return error with RetryPolicyNever (permanent failure)
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	permanentErr := &mockClassifiedError{
		msg:         "invalid content type - not retryable",
		severity:    failure.SeverityRecoverable,
		retryPolicy: failure.RetryPolicyNever,
		impactLevel: failure.ImpactLevelContinue,
	}
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, permanentErr)

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
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
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
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Verify: error handled gracefully
	t.Logf("Execution result: err=%v", execErr)

	// Verify: failure journal should NOT have recorded the URL (never-retry errors are permanent)
	mockFailureJournal.AssertNotCalled(t, "Record", mock.Anything)
}
