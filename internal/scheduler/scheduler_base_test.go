package scheduler_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/stretchr/testify/mock"
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
	mockSleeper := newSleeperMock(t)
	// Set up robot expectations
	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()
	mockSleeper.On("Sleep", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, mockSleeper)

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
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	// Set up robot expectations - Init is always called
	mockRobot.On("Init", mock.Anything).Return()
	// The malformed URL may cause an error before reaching Decide, or the URL parsing may fail.
	// Set up a permissive Decode expectation that allows any call
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    false,
		Reason:     robots.DisallowedByRobots,
		CrawlDelay: 0,
	}, nil).Maybe()
	mockSleeper.On("Sleep", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, mockSleeper)

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
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	// Set up robot expectations - Init is called once per execution
	mockRobot.On("Init", mock.Anything).Return().Maybe()
	// Expect Decide for both example1.com and example2.com
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Maybe()
	mockSleeper.On("Sleep", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, mockSleeper)

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
