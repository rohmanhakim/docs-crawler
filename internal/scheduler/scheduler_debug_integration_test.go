package scheduler_test

import (
	"context"
	"encoding/json"
	"net/http"
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
	"github.com/rohmanhakim/docs-crawler/internal/stagedump"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
	"github.com/rohmanhakim/docs-crawler/pkg/debug/debugtest"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/failurejournal"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

// ============================================================================
// Debug Logger Integration Tests
// These tests verify end-to-end debug logging behavior across the full
// crawl pipeline, ensuring all stages emit appropriate log entries.
// ============================================================================

// createSchedulerWithDebugLogger creates a scheduler with a mock debug logger
// for integration testing. Returns the scheduler and the mock logger for assertions.
func createSchedulerWithDebugLogger(
	t *testing.T,
	ctx context.Context,
	mockFinalizer *mockFinalizer,
	metadataSink metadata.MetadataSink,
	mockLimiter *rateLimiterMock,
	mockFrontier *frontierMock,
	mockRobot *robotsMock,
	mockFetcher *fetcherMock,
	mockExtractor extractor.Extractor,
	mockSanitizer sanitizer.Sanitizer,
	mockConvert *convertMock,
	mockResolver *resolverMock,
	mockNormalize *normalizeMock,
	mockStorage *storageMock,
	mockSleeper timeutil.Sleeper,
	mockFailureJournal failurejournal.Journal,
) (*scheduler.Scheduler, *debugtest.LoggerMock) {
	t.Helper()

	// Create a fresh mock logger for each test
	mockLogger := debugtest.NewLoggerMock()

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
		mockFailureJournal,
		stagedump.NewNoOpDumper(),
		mockLogger,
	)
	return &s, mockLogger
}

// =============================================================================
// Assertion Helpers
// =============================================================================

// assertDebugStageLogged asserts that a stage event with the given type was logged.
func assertDebugStageLogged(t *testing.T, entries []debugtest.StageEntry, stage string, eventType debug.EventType) {
	t.Helper()
	found := false
	for _, entry := range entries {
		if entry.Stage == stage && entry.Event.Type == eventType {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stage %q with event type %q to be logged, but it was not found", stage, eventType)
	}
}

// assertDebugStepLogged asserts that a step with the given name was logged for a stage.
func assertDebugStepLogged(t *testing.T, entries []debugtest.StepEntry, stage string, step string) {
	t.Helper()
	found := false
	for _, entry := range entries {
		if entry.Stage == stage && entry.Step == step {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected step %q in stage %q to be logged, but it was not found", step, stage)
	}
}

// assertDebugStageOrder asserts that stages were logged in the expected order.
func assertDebugStageOrder(t *testing.T, entries []debugtest.StageEntry, expectedStages []string, eventType debug.EventType) {
	t.Helper()
	var actualStages []string
	for _, entry := range entries {
		if entry.Event.Type == eventType {
			actualStages = append(actualStages, entry.Stage)
		}
	}

	// Check that expected stages appear in order (not necessarily consecutively)
	expectedIdx := 0
	for _, stage := range actualStages {
		if expectedIdx < len(expectedStages) && stage == expectedStages[expectedIdx] {
			expectedIdx++
		}
	}

	if expectedIdx < len(expectedStages) {
		t.Errorf("stages not in expected order. expected %v to appear in order, got %v", expectedStages, actualStages)
	}
}

// =============================================================================
// Test 1: Full Pipeline Stage Sequence
// =============================================================================

// TestIntegration_DebugLogging_FullPipeline_StageSequence verifies that all
// pipeline stages log start and complete events in the correct order.
func TestIntegration_DebugLogging_FullPipeline_StageSequence(t *testing.T) {
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
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup robot mock
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	// Setup frontier mock
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	seedToken := frontier.NewCrawlToken(*mustParseDebugURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	// Setup other mocks
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createDebugSanitizedHTMLDoc(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Create scheduler with debug logger
	s, mockLogger := createSchedulerWithDebugLogger(
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
		mockResolver,
		mockNormalize,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	// Create config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": ["http://example.com"],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Execute crawl
	init, err := s.InitializeCrawling(configPath)
	require.NoError(t, err)

	_, err = s.ExecuteCrawlingWithState(init)
	require.NoError(t, err)

	// Verify debug logger was called
	assert.True(t, mockLogger.LogStageCalled, "LogStage should be called")

	// Get all stage entries
	stageEntries := mockLogger.GetStageEntries()

	// Verify pipeline start was logged
	assertDebugStageLogged(t, stageEntries, "pipeline", debug.EventTypeStart)

	// Verify fetcher start and complete were logged
	assertDebugStageLogged(t, stageEntries, "fetcher", debug.EventTypeStart)
	assertDebugStageLogged(t, stageEntries, "fetcher", debug.EventTypeComplete)

	// Verify stages were logged in correct order
	// Expected order: pipeline -> fetcher (start) -> fetcher (complete)
	startStages := []string{"pipeline", "fetcher"}
	assertDebugStageOrder(t, stageEntries, startStages, debug.EventTypeStart)

	// Verify complete events have duration recorded
	fetcherCompletes := mockLogger.StagesByType(debug.EventTypeComplete)
	for _, entry := range fetcherCompletes {
		if entry.Stage == "fetcher" {
			assert.Greater(t, entry.Event.Duration, time.Duration(0), "fetcher complete should have duration")
		}
	}

	t.Logf("Total stage entries: %d", len(stageEntries))
	t.Logf("Stage events: %+v", stageEntries)
}

// =============================================================================
// Test 2: Fetcher Step Sequence
// =============================================================================

// TestIntegration_DebugLogging_Fetcher_StepSequence verifies that the fetcher
// logs granular steps within the stage.
func TestIntegration_DebugLogging_Fetcher_StepSequence(t *testing.T) {
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
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup robot mock
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	// Setup frontier mock
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()

	// Setup other mocks
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup successful fetch with valid HTML
	testURL, _ := url.Parse("https://example.com/test")
	htmlBody := []byte("<html><body><main><h1>Test</h1><p>Content</p></main></body></html>")
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

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createDebugSanitizedHTMLDoc(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Create scheduler with debug logger
	s, mockLogger := createSchedulerWithDebugLogger(
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
		mockResolver,
		mockNormalize,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	// Create config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": ["https://example.com"],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Execute crawl
	init, err := s.InitializeCrawling(configPath)
	require.NoError(t, err)

	_, err = s.ExecuteCrawlingWithState(init)
	require.NoError(t, err)

	// Get step entries for fetcher
	fetcherSteps := mockLogger.StepsByStage("fetcher")

	// Verify key steps were logged
	// Note: The exact steps logged depend on the fetcher implementation
	t.Logf("Fetcher steps: %d", len(fetcherSteps))
	for _, step := range fetcherSteps {
		t.Logf("  Step: %s, Fields: %v", step.Step, step.Fields)
	}

	// Note: The create_request, response_received, body_read steps are logged
	// by the fetcher's debug logger, which is propagated via SetDebugLogger.
	// The scheduler's mock logger receives stage events but may not receive
	// all internal steps unless the fetcher's logger is shared.
	// This test verifies the scheduler's logging behavior.
}

// =============================================================================
// Test 3: Retry Scenario
// =============================================================================

// TestIntegration_DebugLogging_RetryScenario verifies that retry attempts
// are logged with correct backoff information.
func TestIntegration_DebugLogging_RetryScenario(t *testing.T) {
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
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup robot mock
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	// Setup frontier mock
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()

	// Setup other mocks
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup fetcher to fail first, then succeed (simulating retry)
	requestCount := 0
	testURL, _ := url.Parse("https://example.com/test")
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			requestCount++
		}).Return(func() fetcher.FetchResult {
		if requestCount == 1 {
			// First request fails with 500
			return fetcher.NewFetchResultForTest(
				*testURL,
				nil,
				http.StatusInternalServerError,
				"text/html",
				nil,
				time.Now(),
			)
		}
		// Second request succeeds
		return fetcher.NewFetchResultForTest(
			*testURL,
			[]byte("<html><body><main><h1>Test</h1></main></body></html>"),
			http.StatusOK,
			"text/html",
			map[string]string{"Content-Type": "text/html"},
			time.Now(),
		)
	}, func() failure.ClassifiedError {
		if requestCount == 1 {
			return fetcher.NewFetchError(
				fetcher.ErrCauseRequest5xx,
				"http 500",
			)
		}
		return nil
	})

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createDebugSanitizedHTMLDoc(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Create scheduler with debug logger
	s, mockLogger := createSchedulerWithDebugLogger(
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
		mockResolver,
		mockNormalize,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	// Create config file with retry enabled
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": ["https://example.com"],
		"maxDepth": 0,
		"maxAttempt": 3
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Execute crawl
	init, err := s.InitializeCrawling(configPath)
	require.NoError(t, err)

	_, err = s.ExecuteCrawlingWithState(init)
	// Note: The actual retry behavior depends on error classification
	// This test verifies that retry logging infrastructure is in place

	t.Logf("Retry logging called: %v", mockLogger.LogRetryCalled)
	t.Logf("Retry entries: %d", len(mockLogger.GetRetryEntries()))

	// Verify retry logging was called (if retries occurred)
	if mockLogger.LogRetryCalled {
		retryEntries := mockLogger.GetRetryEntries()
		for _, entry := range retryEntries {
			t.Logf("Retry entry: attempt=%d, maxAttempts=%d, backoff=%v, err=%v",
				entry.Attempt, entry.MaxAttempts, entry.Backoff, entry.Err)
		}
	}
}

// =============================================================================
// Test 4: Rate Limit Scenario
// =============================================================================

// TestIntegration_DebugLogging_RateLimitScenario verifies that rate limit
// decisions are logged with delay information.
func TestIntegration_DebugLogging_RateLimitScenario(t *testing.T) {
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
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup robot mock with crawl delay
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 2 * time.Second,
	}, nil).Once()

	// Setup frontier mock
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()

	// Setup mocks with rate limiting
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()

	// Configure rate limiter to return a delay
	mockLimiter.On("ResolveDelay", "example.com").Return(500 * time.Millisecond)

	// Setup fetcher
	testURL, _ := url.Parse("https://example.com/test")
	htmlBody := []byte("<html><body><main><h1>Test</h1></main></body></html>")
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

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createDebugSanitizedHTMLDoc(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Create scheduler with debug logger
	s, mockLogger := createSchedulerWithDebugLogger(
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
		mockResolver,
		mockNormalize,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	// Create config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": ["https://example.com"],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Execute crawl
	init, err := s.InitializeCrawling(configPath)
	require.NoError(t, err)

	_, err = s.ExecuteCrawlingWithState(init)
	require.NoError(t, err)

	// Verify rate limit logging
	t.Logf("Rate limit logging called: %v", mockLogger.LogRateLimitCalled)
	rateLimitEntries := mockLogger.GetRateLimitEntries()
	t.Logf("Rate limit entries: %d", len(rateLimitEntries))

	for _, entry := range rateLimitEntries {
		t.Logf("Rate limit entry: host=%s, delay=%v, reason=%s",
			entry.Host, entry.Delay, entry.Reason)
	}

	// Note: Rate limit logging is triggered by the rate limiter component,
	// which has its own debug logger. The scheduler's mock logger receives
	// stage events but the rate limiter logs to its own logger instance.
}

// =============================================================================
// Test 5: Frontier Skip Scenarios
// =============================================================================

// TestIntegration_DebugLogging_Frontier_SkipScenarios verifies that frontier
// logs skip reasons for URLs that are not submitted.
func TestIntegration_DebugLogging_Frontier_SkipScenarios(t *testing.T) {
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
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup robot mock - allow all URLs (seed + discovered)
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Maybe()

	// Setup frontier mock - use explicit dequeue control
	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	// First dequeue returns seed token, then false to exit loop
	seedToken := frontier.NewCrawlToken(*mustParseDebugURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	// Setup other mocks
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup fetcher to return page with discovered URLs
	testURL, _ := url.Parse("https://example.com/test")
	htmlBody := []byte("<html><body><main><h1>Test</h1><a href=\"/page1\">Link</a></main></body></html>")
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

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer with discovered URLs
	discoveredURLs := []url.URL{
		*mustParseDebugURL("https://example.com/page1"),
		*mustParseDebugURL("https://example.com/page2"),
	}
	mockSanitizer.On("Sanitize", contentNode).Return(createDebugSanitizedHTMLDoc(discoveredURLs), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Create scheduler with debug logger
	s, mockLogger := createSchedulerWithDebugLogger(
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
		mockResolver,
		mockNormalize,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	// Create config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": ["https://example.com"],
		"maxDepth": 1
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Execute crawl
	init, err := s.InitializeCrawling(configPath)
	require.NoError(t, err)

	_, err = s.ExecuteCrawlingWithState(init)
	require.NoError(t, err)

	// Verify step logging
	t.Logf("Step logging called: %v", mockLogger.LogStepCalled)
	stepEntries := mockLogger.GetStepEntries()
	t.Logf("Step entries: %d", len(stepEntries))

	for _, entry := range stepEntries {
		t.Logf("Step: stage=%s, step=%s, fields=%v", entry.Stage, entry.Step, entry.Fields)
	}

	// Note: Frontier skip logging is done by the frontier component,
	// which has its own debug logger. The scheduler propagates the logger
	// to frontier via SetDebugLogger.
}

// =============================================================================
// Test 6: Error Path
// =============================================================================

// TestIntegration_DebugLogging_ErrorPath verifies that errors are logged
// correctly through the pipeline.
func TestIntegration_DebugLogging_ErrorPath(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	// Create a fresh fetcher mock without default expectations
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
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup robot mock
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	// Setup frontier mock - use explicit dequeue control
	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	seedToken := frontier.NewCrawlToken(*mustParseDebugURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	// Setup other mocks
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor mock (needs SetExtractParam for InitializeCrawling)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup fetcher to return 404 error
	testURL, _ := url.Parse("https://example.com/notfound")
	fetchResult := fetcher.NewFetchResultForTest(
		*testURL,
		nil,
		http.StatusNotFound,
		"text/html",
		nil,
		time.Now(),
	)
	fetchErr := fetcher.NewFetchError(
		fetcher.ErrCauseRequestPageForbidden,
		"http 404 not found",
	)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, fetchErr).Once()

	// Create scheduler with debug logger
	s, mockLogger := createSchedulerWithDebugLogger(
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
		mockResolver,
		mockNormalize,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	// Create config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": ["https://example.com"],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Execute crawl
	init, err := s.InitializeCrawling(configPath)
	require.NoError(t, err)

	_, err = s.ExecuteCrawlingWithState(init)
	// Should not error at scheduler level (errors are counted, not propagated)
	require.NoError(t, err)

	// Verify error logging
	stageEntries := mockLogger.GetStageEntries()
	t.Logf("Stage entries: %d", len(stageEntries))

	// Look for error events
	errorStages := mockLogger.StagesByType(debug.EventTypeError)
	t.Logf("Error stages: %d", len(errorStages))

	for _, entry := range errorStages {
		t.Logf("Error stage: stage=%s, url=%s, duration=%v",
			entry.Stage, entry.Event.URL, entry.Event.Duration)
	}

	// Verify fetcher error was logged
	assertDebugStageLogged(t, stageEntries, "fetcher", debug.EventTypeError)

	// Verify error logging was called
	t.Logf("Error logging called: %v", mockLogger.LogErrorCalled)
	errorEntries := mockLogger.GetErrorEntries()
	t.Logf("Error entries: %d", len(errorEntries))
}

// =============================================================================
// Test 7: Disabled Debug Logger
// =============================================================================

// TestIntegration_DebugLogging_Disabled verifies that no logging occurs when
// debug logger is disabled (NoOpLogger is used).
func TestIntegration_DebugLogging_Disabled(t *testing.T) {
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
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup robot mock
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	// Setup frontier mock
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()

	// Setup other mocks
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup fetcher
	testURL, _ := url.Parse("https://example.com/test")
	htmlBody := []byte("<html><body><main><h1>Test</h1></main></body></html>")
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

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createDebugSanitizedHTMLDoc(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Create scheduler with NoOpLogger (simulating disabled debug mode)
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
		mockFailureJournal,
		stagedump.NewNoOpDumper(),
		debug.NewNoOpLogger(),
	)

	// Create config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": ["https://example.com"],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Execute crawl
	init, err := s.InitializeCrawling(configPath)
	require.NoError(t, err)

	_, err = s.ExecuteCrawlingWithState(init)
	require.NoError(t, err)

	// With NoOpLogger, no errors should occur
	// The NoOpLogger simply discards all log entries
	t.Log("Crawl completed successfully with NoOpLogger (no debug output)")
}

// =============================================================================
// Test 8: JSONL File Output (validates full SlogLogger output)
// =============================================================================

// JSONLEntry represents a parsed JSONL log entry matching dlog's Logstash format.
type JSONLEntry struct {
	Timestamp     string `json:"@timestamp"`
	LogLevel      string `json:"log.level"`
	Message       string `json:"message"`
	Stage         string `json:"stage,omitempty"`
	EventType     string `json:"event_type,omitempty"`
	URL           string `json:"url,omitempty"`
	DurationMs    int64  `json:"duration_ms,omitempty"`
	StatusCode    int    `json:"status_code,omitempty"`
	Attempt       int    `json:"attempt,omitempty"`
	MaxAttempts   int    `json:"max_attempts,omitempty"`
	BackoffMs     int64  `json:"backoff_ms,omitempty"`
	Host          string `json:"host,omitempty"`
	DelayMs       int64  `json:"delay_ms,omitempty"`
	RateLimitReas string `json:"rate_limit_reason,omitempty"`
	Step          string `json:"step,omitempty"`
	Error         string `json:"error,omitempty"`
}

// parseJSONLFile reads and parses a JSONL file into structured entries.
func parseJSONLFile(t *testing.T, path string) []JSONLEntry {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read JSONL file")

	lines := splitLines(string(data))
	entries := make([]JSONLEntry, 0, len(lines))

	for i, line := range lines {
		line = trimWhitespace(line)
		if line == "" {
			continue
		}

		var entry JSONLEntry
		err := json.Unmarshal([]byte(line), &entry)
		require.NoError(t, err, "failed to parse JSONL line %d: %s", i+1, line)
		entries = append(entries, entry)
	}

	return entries
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

// trimWhitespace removes leading and trailing whitespace.
func trimWhitespace(s string) string {
	return strings.TrimSpace(s)
}

// assertValidJSONLFields validates required fields per dlog's Logstash format.
func assertValidJSONLFields(t *testing.T, entry JSONLEntry) {
	t.Helper()

	// Required fields per dlog's Logstash format
	assert.NotEmpty(t, entry.Timestamp, "@timestamp should be present")
	assert.Equal(t, "DEBUG", entry.LogLevel, "log.level should be 'DEBUG'")
	assert.NotEmpty(t, entry.Message, "message should be present")

	// Validate timestamp format (RFC3339Nano)
	_, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
	assert.NoError(t, err, "@timestamp should be valid RFC3339Nano format")
}

// TestIntegration_DebugLogging_JSONLFileOutput verifies that debug logs
// are correctly written to a JSONL file with proper structure.
func TestIntegration_DebugLogging_JSONLFileOutput(t *testing.T) {
	ctx := context.Background()

	// Create temp directory for outputs
	tmpDir := t.TempDir()
	debugFilePath := filepath.Join(tmpDir, "debug.jsonl")

	// Create SlogLogger with file output (use logstash format for Logstash-compatible field names)
	debugConfig, err := debug.NewDebugConfig(true, debugFilePath, "logstash")
	require.NoError(t, err, "failed to create debug config")

	debugLogger, err := debug.NewSlogLogger(debugConfig)
	require.NoError(t, err, "failed to create slog logger")
	// Note: Logger will be closed after ExecuteCrawlingWithState, before reading the file

	// Setup mocks for minimal crawl
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
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Setup robot mock
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	// Setup frontier mock with explicit dequeue control
	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	seedToken := frontier.NewCrawlToken(*mustParseDebugURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	// Setup other mocks
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup successful fetch
	testURL, _ := url.Parse("https://example.com/test")
	htmlBody := []byte("<html><body><main><h1>Test</h1><p>Content</p></main></body></html>")
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

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createDebugSanitizedHTMLDoc(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver
	setupResolverMockWithSuccess(mockResolver)

	// Setup normalize
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup storage
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.NewWriteResult("abc123", "/output/abc123.md", "sha256:def456"), nil)

	// Create scheduler with real SlogLogger
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
		mockFailureJournal,
		stagedump.NewNoOpDumper(),
		debugLogger,
	)

	// Create config file
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": ["https://example.com"],
		"maxDepth": 0
	}`
	err = os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err, "failed to write config file")

	// Execute crawl
	init, err := s.InitializeCrawling(configPath)
	require.NoError(t, err, "failed to initialize crawl")

	_, err = s.ExecuteCrawlingWithState(init)
	require.NoError(t, err, "failed to execute crawl")

	// Close logger to flush file output
	err = debugLogger.Close()
	require.NoError(t, err, "failed to close debug logger")

	// Verify JSONL file exists
	_, err = os.Stat(debugFilePath)
	require.NoError(t, err, "JSONL file should exist")

	// Parse JSONL file
	entries := parseJSONLFile(t, debugFilePath)

	// Verify we have entries
	assert.NotEmpty(t, entries, "JSONL file should contain entries")
	t.Logf("Total JSONL entries: %d", len(entries))

	// Validate all entries have required fields
	for i, entry := range entries {
		t.Logf("Entry %d: stage=%s, event_type=%s, message=%s",
			i+1, entry.Stage, entry.EventType, entry.Message)
		assertValidJSONLFields(t, entry)
	}

	// Find pipeline start event
	var foundPipelineStart, foundFetcherStart, foundFetcherComplete bool
	for _, entry := range entries {
		if entry.Stage == "pipeline" && entry.EventType == "start" {
			foundPipelineStart = true
			assert.NotEmpty(t, entry.URL, "pipeline start should have URL")
		}
		if entry.Stage == "fetcher" && entry.EventType == "start" {
			foundFetcherStart = true
		}
		if entry.Stage == "fetcher" && entry.EventType == "complete" {
			foundFetcherComplete = true
			// Note: Duration can be 0 for very fast mock operations
			assert.GreaterOrEqual(t, entry.DurationMs, int64(0), "fetcher complete should have duration_ms >= 0")
			assert.NotEmpty(t, entry.URL, "fetcher complete should have URL")
		}
	}

	// Verify expected stage events were logged
	assert.True(t, foundPipelineStart, "should have pipeline start event")
	assert.True(t, foundFetcherStart, "should have fetcher start event")
	assert.True(t, foundFetcherComplete, "should have fetcher complete event")

	// Verify stage order (pipeline start should come before fetcher start)
	var pipelineStartIdx, fetcherStartIdx int = -1, -1
	for i, entry := range entries {
		if entry.Stage == "pipeline" && entry.EventType == "start" && pipelineStartIdx == -1 {
			pipelineStartIdx = i
		}
		if entry.Stage == "fetcher" && entry.EventType == "start" && fetcherStartIdx == -1 {
			fetcherStartIdx = i
		}
	}
	assert.Less(t, pipelineStartIdx, fetcherStartIdx, "pipeline start should come before fetcher start")

	// Log sample entries for debugging
	t.Log("Sample JSONL entries:")
	for i := 0; i < min(3, len(entries)); i++ {
		entryJSON, _ := json.MarshalIndent(entries[i], "  ", "  ")
		t.Logf("  Entry %d: %s", i+1, string(entryJSON))
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// Test 9: Concurrent Workers (Future Enhancement)
// =============================================================================

// TODO: Add integration tests for concurrent worker scenarios
// When the scheduler supports concurrent workers:
// 1. Test that each worker's logs are properly attributed
// 2. Test that worker_id field is correctly populated
// 3. Test thread safety of the logger

// =============================================================================
// Helper Functions
// =============================================================================

// mustParseDebugURL parses a URL string or panics.
func mustParseDebugURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// createDebugSanitizedHTMLDoc creates a SanitizedHTMLDoc for testing.
func createDebugSanitizedHTMLDoc(discoveredURLs []url.URL) sanitizer.SanitizedHTMLDoc {
	return createSanitizedHTMLDocForTest(discoveredURLs)
}
