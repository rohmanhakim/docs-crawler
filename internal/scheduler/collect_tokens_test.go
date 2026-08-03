package scheduler_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Token Collection Unit Tests
// These tests verify the batching logic for collecting tokens from the frontier.
// ============================================================================

// Test helper: create a scheduler with a frontier that has tokens pre-loaded
func createSchedulerWithFrontierTokens(t *testing.T, urls []string) *scheduler.Scheduler {
	t.Helper()
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	mockFetcher := newFetcherMockForTest(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockNormalize := newNormalizeMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockLimiter := newRateLimiterMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	// Submit URLs to the frontier
	for _, urlStr := range urls {
		u, _ := url.Parse(urlStr)
		candidate := frontier.NewCrawlAdmissionCandidate(
			*u,
			frontier.SourceSeed,
			frontier.NewDiscoveryMetadata(0, nil),
		)
		mockFrontier.Submit(candidate)
	}

	s := createSchedulerWithAllMocksAndNormalize(
		t, ctx, newMockFinalizer(t), &metadata.NoopSink{}, mockLimiter, mockRobot,
		mockFrontier, mockFetcher, mockExtractor, mockSanitizer, mockConvert,
		mockResolver, mockNormalize, mockStorage, mockFailureJournal,
	)
	return s
}

// ============================================================================
// Test: Empty frontier — returns empty slice
// ============================================================================
func TestCollectTokensFromFrontier_Empty(t *testing.T) {
	s := createSchedulerWithFrontierTokens(t, []string{})

	// Dequeue should return nothing
	token, ok := s.DequeueFromFrontier()
	assert.False(t, ok)
	assert.Equal(t, frontier.CrawlToken{}, token)
}

// ============================================================================
// Test: Single token — returns one token
// ============================================================================
func TestCollectTokensFromFrontier_SingleToken(t *testing.T) {
	s := createSchedulerWithFrontierTokens(t, []string{"http://example.com/page1"})

	// Dequeue should return the token
	token, ok := s.DequeueFromFrontier()
	assert.True(t, ok)
	u := token.URL()
	assert.Equal(t, "http://example.com/page1", u.String())

	// Second dequeue should return nothing
	token2, ok2 := s.DequeueFromFrontier()
	assert.False(t, ok2)
	assert.Equal(t, frontier.CrawlToken{}, token2)
}

// ============================================================================
// Test: Multiple tokens — returns all tokens in FIFO order
// ============================================================================
func TestCollectTokensFromFrontier_MultipleTokens(t *testing.T) {
	urls := []string{
		"http://example.com/page1",
		"http://example.com/page2",
		"http://example.com/page3",
	}
	s := createSchedulerWithFrontierTokens(t, urls)

	// Dequeue all tokens
	var dequeued []string
	for {
		token, ok := s.DequeueFromFrontier()
		if !ok {
			break
		}
		u := token.URL()
		dequeued = append(dequeued, u.String())
	}

	assert.Equal(t, urls, dequeued)
}

// ============================================================================
// Test: Frontier exhaustion — after collecting all tokens, frontier is empty
// ============================================================================
func TestCollectTokensFromFrontier_Exhaustion(t *testing.T) {
	urls := []string{
		"http://example.com/page1",
		"http://example.com/page2",
	}
	s := createSchedulerWithFrontierTokens(t, urls)

	// Drain all tokens
	_, ok1 := s.DequeueFromFrontier()
	assert.True(t, ok1)
	_, ok2 := s.DequeueFromFrontier()
	assert.True(t, ok2)

	// Frontier should be empty
	_, ok3 := s.DequeueFromFrontier()
	assert.False(t, ok3)

	// Visited count should reflect submissions
	assert.Equal(t, 2, s.FrontierVisitedCount())
}
