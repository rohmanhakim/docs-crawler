package scheduler_test

import (
	"testing"
	"time"
)

// mockFinalizer is a test double that captures final crawl statistics
type mockFinalizer struct {
	recordedStats *capturedStats
}

type capturedStats struct {
	totalPages            int
	totalErrors           int
	totalAssets           int
	duration              time.Duration
	manualRetryQueueCount int
}

func newMockFinalizer(t *testing.T) *mockFinalizer {
	t.Helper()
	return &mockFinalizer{
		recordedStats: nil,
	}
}

func (m *mockFinalizer) RecordFinalCrawlStats(
	totalPages int,
	totalErrors int,
	totalAssets int,
	duration time.Duration,
	manualRetryQueueCount int,
) {
	m.recordedStats = &capturedStats{
		totalPages:            totalPages,
		totalErrors:           totalErrors,
		totalAssets:           totalAssets,
		duration:              duration,
		manualRetryQueueCount: manualRetryQueueCount,
	}
}
