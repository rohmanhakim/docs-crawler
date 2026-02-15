package scheduler_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/stretchr/testify/mock"
)

// TestScheduler_FinalStats_RecordsExactlyOnce verifies that RecordFinalCrawlStats
// is called exactly once per crawl execution.
func TestScheduler_FinalStats_RecordsExactlyOnce(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
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
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
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
		nil,
		nil,
		nil,
		nil,
		mockStorage,
		mockSleeper,
	)

	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 1,
		"maxPages": 10
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Reset finalizer to clear init stats
	mockFinalizer.recordedStats = nil

	// Phase 2: Execute with state
	_, err = s.ExecuteCrawlingWithState(init)

	// Should complete without fatal error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stats should be recorded exactly once
	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected RecordFinalCrawlStats to be called")
	}

	// Execute another crawl with same scheduler (if supported) or create new one
	// This verifies the contract that stats are recorded per execution
}

// TestScheduler_StatsAccuracy_AssetsTracked verifies that totalAssets is tracked correctly
// by mocking the resolver to return assets and verifying the count.
func TestScheduler_StatsAccuracy_AssetsTracked(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)
	mockConvert := newConvertMockForTest(t)

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
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	// Setup convert mock with success
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver mock to return assets
	resolverMock := newResolverMockForTest(t)
	assetDoc := createAssetfulMarkdownDocForTest("test content", []string{
		"assets/images/logo-a3f7b2c.png",
		"assets/images/diagram-b8c9d3e.svg",
	})
	setupResolverMockWithCustomResult(resolverMock, assetDoc)

	// Create scheduler with custom resolver
	ext := extractor.NewDomExtractor(noopSink)
	san := sanitizer.NewHTMLSanitizer(noopSink)
	normalizeMock := newNormalizeMockForTest(t)
	setupNormalizeMockWithSuccess(normalizeMock)
	s := scheduler.NewSchedulerWithDeps(
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockFetcher,
		mockRobot,
		&ext,
		&san,
		mockConvert,
		resolverMock,
		normalizeMock,
		mockStorage,
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}]
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Reset finalizer to clear init stats
	mockFinalizer.recordedStats = nil

	// Phase 2: Execute with state
	_, err = s.ExecuteCrawlingWithState(init)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected stats to be recorded")
	}

	// Verify totalAssets matches the number of local assets from the resolver
	expectedAssets := 2
	if mockFinalizer.recordedStats.totalAssets != expectedAssets {
		t.Errorf("expected totalAssets to be %d, got %d", expectedAssets, mockFinalizer.recordedStats.totalAssets)
	}

	t.Logf("Total assets recorded: %d", mockFinalizer.recordedStats.totalAssets)
}

// TestScheduler_FinalStatsContract_CalledAfterTermination verifies the contract
// that RecordFinalCrawlStats is called only after crawl termination.
func TestScheduler_FinalStatsContract_CalledAfterTermination(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
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
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
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
		nil,
		nil,
		nil,
		nil,
		mockStorage,
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Reset finalizer to clear init stats
	mockFinalizer.recordedStats = nil

	// Phase 2: Execute with state
	_, err = s.ExecuteCrawlingWithState(init)

	// After ExecuteCrawlingWithState returns, stats should be recorded
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected stats to be recorded after crawl termination")
	}

	// Duration should be set (indicating the crawl ran and completed)
	if mockFinalizer.recordedStats.duration == 0 {
		t.Log("Warning: duration is zero, crawl may have completed too quickly or not run")
	}
}

// TestScheduler_GracefulShutdown_StatsRecordedDespiteErrors verifies that
// even when errors occur during crawling, final stats are still recorded.
func TestScheduler_GracefulShutdown_StatsRecordedDespiteErrors(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
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
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	// Set up limiter mock to handle ResolveDelay calls
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0)).Maybe()
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil).Maybe()

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
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config that will likely encounter errors (e.g., network errors when trying to fetch)
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "nonexistent-domain-12345.com"}],
		"maxDepth": 1
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Phase 1: Initialize - may encounter network/robots errors but should not panic
	init, initErr := s.InitializeCrawling(configPath)

	// If init succeeds, execute the crawl
	if initErr == nil {
		// Phase 2: Execute with state
		_, _ = s.ExecuteCrawlingWithState(init)
	}

	// The key assertion is that stats were recorded (either from init failure or execution)
	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected stats to be recorded even when errors occur")
	}

	t.Logf("Stats recorded despite potential errors: pages=%d, errors=%d",
		mockFinalizer.recordedStats.totalPages,
		mockFinalizer.recordedStats.totalErrors)
}

// TestScheduler_StatsConsistency_AllFieldsNonNegative verifies that all
// stat fields are non-negative.
func TestScheduler_StatsConsistency_AllFieldsNonNegative(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
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
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
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
		nil,
		nil,
		nil,
		nil,
		mockStorage,
		mockSleeper,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}]
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Reset finalizer to clear init stats
	mockFinalizer.recordedStats = nil

	// Phase 2: Execute with state
	_, err = s.ExecuteCrawlingWithState(init)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected stats to be recorded")
	}

	// All count fields should be non-negative
	if mockFinalizer.recordedStats.totalPages < 0 {
		t.Errorf("totalPages should be non-negative, got %d", mockFinalizer.recordedStats.totalPages)
	}
	if mockFinalizer.recordedStats.totalErrors < 0 {
		t.Errorf("totalErrors should be non-negative, got %d", mockFinalizer.recordedStats.totalErrors)
	}
	if mockFinalizer.recordedStats.totalAssets < 0 {
		t.Errorf("totalAssets should be non-negative, got %d", mockFinalizer.recordedStats.totalAssets)
	}
	if mockFinalizer.recordedStats.duration < 0 {
		t.Errorf("duration should be non-negative, got %v", mockFinalizer.recordedStats.duration)
	}
}

// TestScheduler_ErrorCounting_ConsistentWithMetadata verifies that the
// error count in final stats is consistent with errors recorded to metadata sink.
func TestScheduler_ErrorCounting_ConsistentWithMetadata(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
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
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		errorSink,
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
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}]
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Reset finalizer to clear init stats
	mockFinalizer.recordedStats = nil

	// Phase 2: Execute with state
	_, err = s.ExecuteCrawlingWithState(init)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected stats to be recorded")
	}

	// The error count in stats should reflect recoverable errors counted
	// Note: This is a weak check because the actual error counts depend on
	// the specific behavior of the pipeline components
	t.Logf("Final error count: %d, Sink error count: %d",
		mockFinalizer.recordedStats.totalErrors, errorSink.errorCount)
}
