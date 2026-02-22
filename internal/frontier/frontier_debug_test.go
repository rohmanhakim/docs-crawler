package frontier_test

import (
	"net/url"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/pkg/debug/debugtest"
)

// =============================================================================
// Debug Logger Integration Tests
// =============================================================================

// TestFrontier_DebugLogger_DepthExceeded verifies that debug logging is called
// when a URL is skipped due to exceeding max depth.
func TestFrontier_DebugLogger_DepthExceeded(t *testing.T) {
	// GIVEN a frontier with max depth of 2 and a mock logger
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, err := config.WithDefault([]url.URL{*seedURL}).
		WithMaxDepth(2).
		Build()
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	f := frontier.NewCrawlFrontier()
	f.Init(cfg)

	mockLogger := debugtest.NewLoggerMock()
	f.SetDebugLogger(mockLogger)

	deepURL := mustURLDebug(t, "https://example.com/deep")

	// WHEN a URL at depth 5 is submitted (exceeds limit)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		deepURL,
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(5, nil),
	))

	// THEN debug logging should be called
	if !mockLogger.LogStepCalled {
		t.Fatal("Expected LogStep to be called for depth skip")
	}

	step := mockLogger.LastStep()
	if step.Step != "submit_skipped_depth" {
		t.Errorf("Expected step 'submit_skipped_depth', got %q", step.Step)
	}
	if step.Fields["depth"] != 5 {
		t.Errorf("Expected depth 5, got %v", step.Fields["depth"])
	}
	if step.Fields["max_depth"] != 2 {
		t.Errorf("Expected max_depth 2, got %v", step.Fields["max_depth"])
	}
	if step.Fields["url"] != "https://example.com/deep" {
		t.Errorf("Expected url 'https://example.com/deep', got %v", step.Fields["url"])
	}
}

// TestFrontier_DebugLogger_MaxPagesReached verifies that debug logging is called
// when a URL is skipped due to max pages limit reached.
func TestFrontier_DebugLogger_MaxPagesReached(t *testing.T) {
	// GIVEN a frontier with max pages of 2 and a mock logger
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, err := config.WithDefault([]url.URL{*seedURL}).
		WithMaxPages(2).
		Build()
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	f := frontier.NewCrawlFrontier()
	f.Init(cfg)

	mockLogger := debugtest.NewLoggerMock()
	f.SetDebugLogger(mockLogger)

	// Submit 2 URLs to fill the limit
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/page1"),
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/page2"),
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))

	// Reset the mock to check the third submission
	mockLogger.Reset()

	// WHEN a third URL is submitted (exceeds limit)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/page3"),
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))

	// THEN debug logging should be called
	if !mockLogger.LogStepCalled {
		t.Fatal("Expected LogStep to be called for max pages skip")
	}

	step := mockLogger.LastStep()
	if step.Step != "submit_skipped_max_pages" {
		t.Errorf("Expected step 'submit_skipped_max_pages', got %q", step.Step)
	}
	if step.Fields["max_pages"] != 2 {
		t.Errorf("Expected max_pages 2, got %v", step.Fields["max_pages"])
	}
	if step.Fields["visited_count"] != 2 {
		t.Errorf("Expected visited_count 2, got %v", step.Fields["visited_count"])
	}
}

// TestFrontier_DebugLogger_DuplicateURL verifies that debug logging is called
// when a duplicate URL is submitted.
func TestFrontier_DebugLogger_DuplicateURL(t *testing.T) {
	// GIVEN a frontier with a mock logger
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	mockLogger := debugtest.NewLoggerMock()
	f.SetDebugLogger(mockLogger)

	urlA := mustURLDebug(t, "https://example.com/docs")

	// Submit first URL
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urlA,
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))

	// Reset the mock to check the second submission
	mockLogger.Reset()

	// WHEN the same URL is submitted again
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		urlA,
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(1, nil),
	))

	// THEN debug logging should be called
	if !mockLogger.LogStepCalled {
		t.Fatal("Expected LogStep to be called for duplicate skip")
	}

	step := mockLogger.LastStep()
	if step.Step != "submit_skipped_duplicate" {
		t.Errorf("Expected step 'submit_skipped_duplicate', got %q", step.Step)
	}
	if step.Fields["url"] != "https://example.com/docs" {
		t.Errorf("Expected url 'https://example.com/docs', got %v", step.Fields["url"])
	}
}

// TestFrontier_DebugLogger_DepthAdvanced verifies that debug logging is called
// when the crawl depth advances to a new level.
func TestFrontier_DebugLogger_DepthAdvanced(t *testing.T) {
	// GIVEN a frontier with a mock logger
	f := frontier.NewCrawlFrontier()
	f.Init(config.Config{})

	mockLogger := debugtest.NewLoggerMock()
	f.SetDebugLogger(mockLogger)

	// Submit URL at depth 0
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/root"),
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))

	// Reset the mock to check depth advancement
	mockLogger.Reset()

	// WHEN a URL at depth 2 is submitted (advancing from depth 0 to 2)
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/deep"),
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(2, nil),
	))

	// THEN debug logging should be called for depth advancement
	if !mockLogger.LogStepCalled {
		t.Fatal("Expected LogStep to be called for depth advancement")
	}

	step := mockLogger.LastStep()
	if step.Step != "depth_advanced" {
		t.Errorf("Expected step 'depth_advanced', got %q", step.Step)
	}
	if step.Fields["old_depth"] != 0 {
		t.Errorf("Expected old_depth 0, got %v", step.Fields["old_depth"])
	}
	if step.Fields["new_depth"] != 2 {
		t.Errorf("Expected new_depth 2, got %v", step.Fields["new_depth"])
	}
}

// TestFrontier_DebugLogger_Disabled verifies that no logging occurs when
// debug logger is disabled (NoOpLogger is used by default).
func TestFrontier_DebugLogger_Disabled(t *testing.T) {
	// GIVEN a frontier WITHOUT setting a mock logger (uses NoOpLogger by default)
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, err := config.WithDefault([]url.URL{*seedURL}).
		WithMaxDepth(2).
		Build()
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	f := frontier.NewCrawlFrontier()
	f.Init(cfg)

	// No SetDebugLogger call - uses NoOpLogger

	// WHEN a URL at depth 5 is submitted (exceeds limit)
	// This should NOT panic or cause any issues
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/deep"),
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(5, nil),
	))

	// THEN the URL should be skipped without error
	token, ok := f.Dequeue()
	if ok {
		t.Fatalf("Expected no URL to be dequeued, got %v", token.URL())
	}
}

// TestFrontier_DebugLogger_MultipleSkipReasons verifies that debug logging
// correctly identifies different skip reasons in sequence.
func TestFrontier_DebugLogger_MultipleSkipReasons(t *testing.T) {
	// GIVEN a frontier with max pages of 3 and a mock logger
	seedURL, _ := url.Parse("https://example.com/seed")
	cfg, err := config.WithDefault([]url.URL{*seedURL}).
		WithMaxDepth(2).
		WithMaxPages(3).
		Build()
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	f := frontier.NewCrawlFrontier()
	f.Init(cfg)

	mockLogger := debugtest.NewLoggerMock()
	f.SetDebugLogger(mockLogger)

	// Submit URLs to fill the limit
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/page1"),
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/page2"),
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/page3"),
		frontier.SourceSeed,
		frontier.NewDiscoveryMetadata(0, nil),
	))

	// Reset to check subsequent skips
	mockLogger.Reset()

	// WHEN submitting a URL that exceeds depth AND max pages
	// Max pages check comes first in Submit(), so it should log max_pages skip
	f.Submit(frontier.NewCrawlAdmissionCandidate(
		mustURLDebug(t, "https://example.com/deep"),
		frontier.SourceCrawl,
		frontier.NewDiscoveryMetadata(5, nil),
	))

	// THEN max_pages skip should be logged (max pages check comes before depth check)
	step := mockLogger.LastStep()
	if step.Step != "submit_skipped_max_pages" {
		t.Errorf("Expected step 'submit_skipped_max_pages', got %q", step.Step)
	}
}

// Helper for debug test file
func mustURLDebug(t *testing.T, raw string) url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid url %q: %v", raw, err)
	}
	return *u
}
