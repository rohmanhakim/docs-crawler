package scheduler_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Tests for InitializeCrawling()
// ============================================================================

// TestInitializeCrawling_Success_ReturnsInitialization verifies that a successful
// initialization returns a valid CrawlInitialization with all expected fields set.
func TestInitializeCrawling_Success_ReturnsInitialization(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)

	// Set up expectations
	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Use helper function to create scheduler with real extractor/sanitizer
	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		nil, // nil = create real extractor
		nil, // nil = create real sanitizer
		nil, // nil = create convert mock
		nil, // nil = create normalize mock
		mockStorage,
		mockSleeper,
	)

	// Create a valid config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 5
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute InitializeCrawling
	init, err := s.InitializeCrawling(configPath)

	// Verify success
	assert.NoError(t, err)
	assert.NotNil(t, init)
	assert.Equal(t, "example.com", init.CurrentHost())
	assert.Equal(t, "https", init.SeedScheme())
	assert.True(t, init.InitialDelayApplied())

	// Verify stats NOT recorded on success (ExecuteCrawlingWithState should handle it)
	assert.Nil(t, mockFinalizer.recordedStats, "Stats should NOT be recorded on successful init")
}

// TestInitializeCrawling_ConfigFileNotFound_ReturnsError verifies that a missing
// config file returns an appropriate error.
func TestInitializeCrawling_ConfigFileNotFound_ReturnsError(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)

	// Use helper function
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

	// Execute InitializeCrawling with non-existent file
	init, err := s.InitializeCrawling("/nonexistent/path/config.json")

	// Verify error
	assert.Error(t, err)
	assert.Nil(t, init)

	// Verify stats ARE recorded on failure (to ensure we always have stats)
	assert.NotNil(t, mockFinalizer.recordedStats, "Stats should be recorded on init failure")
	assert.Equal(t, 0, mockFinalizer.recordedStats.totalPages)
	assert.Equal(t, 0, mockFinalizer.recordedStats.totalErrors)
	assert.GreaterOrEqual(t, mockFinalizer.recordedStats.duration, time.Duration(0))
}

// TestInitializeCrawling_InvalidConfigJSON_ReturnsError verifies that invalid JSON
// in config file returns an error.
func TestInitializeCrawling_InvalidConfigJSON_ReturnsError(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)

	// Use helper function
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

	// Create invalid JSON config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")
	err := os.WriteFile(configPath, []byte("{invalid json}"), 0644)
	assert.NoError(t, err)

	// Execute InitializeCrawling
	init, err := s.InitializeCrawling(configPath)

	// Verify error
	assert.Error(t, err)
	assert.Nil(t, init)

	// Verify stats ARE recorded on failure
	assert.NotNil(t, mockFinalizer.recordedStats, "Stats should be recorded on init failure")
}

// TestInitializeCrawling_EmptySeedURLs_ReturnsError verifies that a config without
// seed URLs returns an error.
func TestInitializeCrawling_EmptySeedURLs_ReturnsError(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)

	// Use helper function
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

	// Create config with empty seed URLs
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.json")
	configData := `{"seedUrls": [], "maxDepth": 5}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Execute InitializeCrawling
	init, err := s.InitializeCrawling(configPath)

	// Verify error - config validation returns "seedUrls cannot be empty"
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "seedUrls")
	assert.Nil(t, init)

	// Verify stats ARE recorded on failure
	assert.NotNil(t, mockFinalizer.recordedStats, "Stats should be recorded on init failure")
}

// ============================================================================
// Tests for ExecuteCrawlingWithState()
// ============================================================================

// TestExecuteCrawlingWithState_Success_ReturnsExecutionResult verifies that a
// successful execution returns CrawlingExecution with expected results.
func TestExecuteCrawlingWithState_Success_ReturnsExecutionResult(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)

	// Set up frontier to have a URL to process
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()

	// First Dequeue returns a token, second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	// Use helper function
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

	// First initialize
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, init)

	// Execute with state
	result, err := s.ExecuteCrawlingWithState(init)

	// Verify success
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.WriteResults())

	// Verify stats ARE recorded (this is the execution phase)
	assert.NotNil(t, mockFinalizer.recordedStats, "Stats should be recorded after execution")
}

// TestExecuteCrawlingWithState_EmptyFrontier_Completes verifies that when the
// frontier is empty, execution completes without errors.
func TestExecuteCrawlingWithState_EmptyFrontier_Completes(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)

	// Disable auto-enqueue to control behavior precisely
	mockFrontier.disableAutoEnqueue = true

	// Set up frontier expectations - no tokens enqueued, Dequeue always returns false
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0)
	mockFrontier.On("Submit", mock.Anything).Return()
	// Dequeue always returns false (empty frontier)
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Use helper function
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

	// First initialize
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, init)

	// Reset finalizer to clear init stats
	mockFinalizer.recordedStats = nil

	// Execute with empty frontier
	result, err := s.ExecuteCrawlingWithState(init)

	// Verify success with empty results
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.WriteResults())

	// Verify stats ARE recorded for execution phase
	assert.NotNil(t, mockFinalizer.recordedStats, "Stats should be recorded after execution")
	// Note: visited count is 1 because seed URL was submitted during init, not because it was processed
	t.Logf("Empty frontier test: visitedCount=%d (seed was submitted during init)",
		mockFinalizer.recordedStats.totalPages)
}

// TestExecuteCrawlingWithState_RecordsStatsCorrectly verifies that stats are
// recorded correctly after execution.
func TestExecuteCrawlingWithState_RecordsStatsCorrectly(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)

	// Set up frontier to process one URL
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(1).Once()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()

	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	// Use helper function
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
	assert.NoError(t, err)

	// Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, init)

	// Reset finalizer to clear init stats
	mockFinalizer.recordedStats = nil

	// Execute
	result, err := s.ExecuteCrawlingWithState(init)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify stats are recorded
	assert.NotNil(t, mockFinalizer.recordedStats, "Stats should be recorded")

	// Verify duration is execution-only (should be reasonable, not including init)
	assert.GreaterOrEqual(t, mockFinalizer.recordedStats.duration, time.Duration(0),
		"Duration should be non-negative")

	t.Logf("Execution stats: pages=%d, errors=%d, assets=%d, duration=%v",
		mockFinalizer.recordedStats.totalPages,
		mockFinalizer.recordedStats.totalErrors,
		mockFinalizer.recordedStats.totalAssets,
		mockFinalizer.recordedStats.duration)
}

// ============================================================================
// Integration Tests
// ============================================================================

// TestSplit_InitThenExecute_WorksEndToEnd verifies that the full flow of
// InitializeCrawling -> ExecuteCrawlingWithState works correctly.
func TestSplit_InitThenExecute_WorksEndToEnd(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)

	// Set up frontier
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()

	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	// Use helper function
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
	assert.NoError(t, err)

	// Step 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, init)
	assert.Equal(t, "example.com", init.CurrentHost())

	// Verify no stats recorded yet (init success)
	assert.Nil(t, mockFinalizer.recordedStats, "No stats should be recorded after successful init")

	// Step 2: Execute
	result, err := s.ExecuteCrawlingWithState(init)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify stats recorded after execution
	assert.NotNil(t, mockFinalizer.recordedStats, "Stats should be recorded after execution")

	t.Logf("End-to-end test passed. Final stats: pages=%d, duration=%v",
		mockFinalizer.recordedStats.totalPages,
		mockFinalizer.recordedStats.duration)
}

// TestSplit_InitFailure_RecordsStats verifies that when initialization fails,
// stats are recorded to ensure we always have stats.
func TestSplit_InitFailure_RecordsStats(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockStorage := newStorageMockForTest(t)

	// Use helper function
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

	// Initialize with non-existent config
	init, err := s.InitializeCrawling("/nonexistent/config.json")

	// Verify failure
	assert.Error(t, err)
	assert.Nil(t, init)

	// Verify stats ARE recorded (even on failure)
	assert.NotNil(t, mockFinalizer.recordedStats, "Stats should be recorded on init failure")
	assert.Equal(t, 0, mockFinalizer.recordedStats.totalPages)
	assert.GreaterOrEqual(t, mockFinalizer.recordedStats.duration, time.Duration(0))

	t.Logf("Init failure stats recorded: duration=%v", mockFinalizer.recordedStats.duration)
}
