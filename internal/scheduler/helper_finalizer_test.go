package scheduler_test

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

var _ metadata.CrawlFinalizer = (*mockFinalizer)(nil)

// mockFinalizer is a test double that captures final crawl statistics
type mockFinalizer struct {
	recordedStats *metadata.CrawlStats
}

func newMockFinalizer(t *testing.T) *mockFinalizer {
	t.Helper()
	return &mockFinalizer{
		recordedStats: nil,
	}
}

func (m *mockFinalizer) RecordFinalCrawlStats(stats metadata.CrawlStats) {
	m.recordedStats = &stats
}
