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
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Multi-Batch Tests
// These tests verify that the scheduler correctly processes URLs across
// multiple batches, and terminates when the frontier is exhausted.
// ============================================================================

// TestMultiBatch_ProcessesMultipleBatches verifies that the scheduler processes
// URLs across multiple batch iterations of the for loop.
//
// Batch 1: [page1, page2] → process both
// Batch 2: [page3] → process it
// Batch 3: empty → loop exits
func TestMultiBatch_ProcessesMultipleBatches(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed: true, Reason: robots.EmptyRuleSet, CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()

	// Batch 1: page1 and page2
	page1URL, _ := url.Parse("https://example.com/page1")
	page2URL, _ := url.Parse("https://example.com/page2")
	page3URL, _ := url.Parse("https://example.com/page3")
	token1 := frontier.NewCrawlToken(*page1URL, 0)
	token2 := frontier.NewCrawlToken(*page2URL, 0)
	token3 := frontier.NewCrawlToken(*page3URL, 0)

	mockFrontier.OnDequeue(token1, true).Once()                  // batch 1: page1
	mockFrontier.OnDequeue(token2, true).Once()                  // batch 1: page2
	mockFrontier.OnDequeue(token3, true).Once()                  // batch 2: page3
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe() // batch 2 end + loop exit

	// Fetcher: return success for all URLs
	testURL, _ := url.Parse("https://example.com/page1")
	htmlBody := []byte("<html><body><main><h1>Test</h1><p>Content</p></main></body></html>")
	fetchResult := fetcher.NewFetchResultForTest(*testURL, htmlBody, 200, "text/html",
		map[string]string{"Content-Type": "text/html"}, time.Now())

	// Track fetch count
	fetchCount := 0
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { fetchCount++ }).
		Return(fetchResult, nil).Maybe()

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.WriteResult{}, nil).Maybe()

	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0)).Maybe()
	mockLimiter.On("Wait", mock.Anything, mock.Anything).Return(nil).Maybe()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFrontier,
		mockRobot, mockFetcher, nil, nil, nil, nil, mockStorage, mockFailureJournal)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(configPath, []byte(`{
		"seedUrls": ["https://example.com/page1"],
		"maxDepth": 0
	}`), 0644)
	assert.NoError(t, err)

	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)

	result, execErr := s.ExecuteCrawlingWithState(init)
	assert.NoError(t, execErr)
	assert.NotNil(t, result)

	// Verify: all 3 tokens were fetched across the batch(es)
	assert.Equal(t, 3, fetchCount, "all 3 tokens should have been fetched")
}

// TestMultiBatch_CrawlTerminatesWhenExhausted verifies that the crawl loop
// terminates correctly when the frontier is empty.
func TestMultiBatch_CrawlTerminatesWhenExhausted(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed: true, Reason: robots.EmptyRuleSet, CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()

	testURL, _ := url.Parse("https://example.com")
	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	htmlBody := []byte("<html><body><main><h1>Test</h1><p>Content</p></main></body></html>")
	fetchResult := fetcher.NewFetchResultForTest(*testURL, htmlBody, 200, "text/html",
		map[string]string{"Content-Type": "text/html"}, time.Now())
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetchResult, nil).Maybe()

	seedToken := frontier.NewCrawlToken(*testURL, 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()

	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0)).Maybe()
	mockLimiter.On("Wait", mock.Anything, mock.Anything).Return(nil).Maybe()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFrontier,
		mockRobot, mockFetcher, nil, nil, nil, nil, mockStorage, mockFailureJournal)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(configPath, []byte(`{
		"seedUrls": ["https://example.com"],
		"maxDepth": 0
	}`), 0644)
	assert.NoError(t, err)

	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)

	result, execErr := s.ExecuteCrawlingWithState(init)
	assert.NoError(t, execErr)
	assert.NotNil(t, result, "crawl should return a result")
}

// TestMultiBatch_EmptyFrontierTerminatesImmediately verifies that when the
// frontier is empty from the start, the loop exits immediately.
func TestMultiBatch_EmptyFrontierTerminatesImmediately(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed: true, Reason: robots.EmptyRuleSet, CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.disableAutoEnqueue = true
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()

	// Frontier is empty from the start
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Maybe()

	mockFetcher.ExpectedCalls = nil
	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, nil).Maybe()

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).
		Return(storage.WriteResult{}, nil).Maybe()

	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0)).Maybe()
	mockLimiter.On("Wait", mock.Anything, mock.Anything).Return(nil).Maybe()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFrontier,
		mockRobot, mockFetcher, nil, nil, nil, nil, mockStorage, mockFailureJournal)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(configPath, []byte(`{
		"seedUrls": ["https://example.com"],
		"maxDepth": 0
	}`), 0644)
	assert.NoError(t, err)

	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err)

	result, execErr := s.ExecuteCrawlingWithState(init)
	assert.NoError(t, execErr)
	assert.NotNil(t, result, "crawl should return a result")
	assert.Equal(t, 0, result.TotalPages(), "no pages should be processed with empty frontier")
}
