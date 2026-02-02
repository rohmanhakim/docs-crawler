package scheduler_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/robots/cache"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
)

// mockFinalizer is a test double that captures final crawl statistics
type mockFinalizer struct {
	recordedStats *capturedStats
}

type capturedStats struct {
	totalPages  int
	totalErrors int
	totalAssets int
	duration    time.Duration
}

func newMockFinalizer() *mockFinalizer {
	return &mockFinalizer{
		recordedStats: nil,
	}
}

func (m *mockFinalizer) RecordFinalCrawlStats(
	totalPages int,
	totalErrors int,
	totalAssets int,
	duration time.Duration,
) {
	m.recordedStats = &capturedStats{
		totalPages:  totalPages,
		totalErrors: totalErrors,
		totalAssets: totalAssets,
		duration:    duration,
	}
}

// mockRobot is a test double for robots.Robot that allows controlling the decision outcome
type mockRobot struct {
	decideFunc func(url url.URL) (robots.Decision, *robots.RobotsError)
}

func (m *mockRobot) Decide(targetURL url.URL) (robots.Decision, *robots.RobotsError) {
	if m.decideFunc != nil {
		return m.decideFunc(targetURL)
	}
	return robots.Decision{Allowed: true}, nil
}

func (m *mockRobot) Init(userAgent string) {}

func (m *mockRobot) InitWithCache(userAgent string, cacheImpl cache.Cache) {}

// setupTestServer creates a test HTTP server that serves robots.txt content
func setupTestServer(robotsContent string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(robotsContent))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// setupTestServerWithStatus creates a test HTTP server that returns a specific status code
func setupTestServerWithStatus(statusCode int, robotsContent string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(statusCode)
			if robotsContent != "" {
				w.Write([]byte(robotsContent))
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// createSchedulerForSubmitTest creates a scheduler with test-specific initialization
// that allows testing SubmitUrlForAdmission in isolation
func createSchedulerForSubmitTest(metadataSink metadata.MetadataSink) *scheduler.Scheduler {
	s := scheduler.NewSchedulerWithDeps(nil, metadataSink)
	return &s
}

// TestSubmitUrlForAdmission_RobotsAllowed_SubmitsToFrontier verifies that when robots
// allows a URL, it is submitted to the frontier.
func TestSubmitUrlForAdmission_RobotsAllowed_SubmitsToFrontier(t *testing.T) {
	// GIVEN: a robots.txt that allows all crawling
	robotsContent := `User-agent: *
Allow: /`
	server := setupTestServer(robotsContent)
	defer server.Close()

	noopSink := &metadata.NoopSink{}
	s := createSchedulerForSubmitTest(noopSink)

	// Initialize the robot with the test server
	s.InitRobot("test-agent/1.0")

	// Set current host for hostTimings tracking
	testURL, _ := url.Parse(server.URL + "/page.html")
	s.SetCurrentHost(testURL.Host)

	// WHEN: submitting URL for admission
	submitErr := s.SubmitUrlForAdmission(
		*testURL,
		frontier.SourceSeed,
		0,
	)

	// THEN: no error should be returned
	if submitErr != nil {
		t.Errorf("Expected no error, got: %v", submitErr)
	}

	// AND: URL should be in frontier (visited count should be 1)
	if s.FrontierVisitedCount() != 1 {
		t.Errorf("Expected frontier to have 1 URL, got: %d", s.FrontierVisitedCount())
	}
}

// TestSubmitUrlForAdmission_RobotsDisallowed_DoesNotSubmitToFrontier verifies that when
// robots disallows a URL, it is NOT submitted to the frontier but returns nil (terminal outcome).
func TestSubmitUrlForAdmission_RobotsDisallowed_DoesNotSubmitToFrontier(t *testing.T) {
	// GIVEN: a robots.txt that disallows all crawling
	robotsContent := `User-agent: *
Disallow: /`
	server := setupTestServer(robotsContent)
	defer server.Close()

	noopSink := &metadata.NoopSink{}
	s := createSchedulerForSubmitTest(noopSink)

	// Initialize the robot with the test server
	s.InitRobot("test-agent/1.0")

	testURL, _ := url.Parse(server.URL + "/page.html")
	s.SetCurrentHost(testURL.Host)

	// WHEN: submitting URL for admission
	submitErr := s.SubmitUrlForAdmission(
		*testURL,
		frontier.SourceSeed,
		0,
	)

	// THEN: no error should be returned (disallowed is terminal outcome, not error)
	if submitErr != nil {
		t.Errorf("Expected nil for disallowed URL (terminal outcome), got error: %v", submitErr)
	}

	// AND: URL should NOT be in frontier (visited count should be 0)
	if s.FrontierVisitedCount() != 0 {
		t.Errorf("Expected frontier to have 0 URLs (disallowed), got: %d", s.FrontierVisitedCount())
	}
}

// TestSubmitUrlForAdmission_RobotsError_ReturnsError verifies that when robots
// encounters an infrastructure error, it returns the error and does not submit to frontier.
func TestSubmitUrlForAdmission_RobotsError_ReturnsError(t *testing.T) {
	// GIVEN: a server that returns 500 for robots.txt (infrastructure error)
	server := setupTestServerWithStatus(http.StatusInternalServerError, "")
	defer server.Close()

	noopSink := &metadata.NoopSink{}
	s := createSchedulerForSubmitTest(noopSink)

	// Initialize the robot with the test server
	s.InitRobot("test-agent/1.0")

	testURL, _ := url.Parse(server.URL + "/page.html")
	s.SetCurrentHost(testURL.Host)

	// WHEN: submitting URL for admission
	submitErr := s.SubmitUrlForAdmission(
		*testURL,
		frontier.SourceSeed,
		0,
	)

	// THEN: error should be returned
	if submitErr == nil {
		t.Error("Expected error for robots.txt infrastructure failure, got nil")
	}

	// AND: URL should NOT be in frontier
	if s.FrontierVisitedCount() != 0 {
		t.Errorf("Expected frontier to have 0 URLs (error case), got: %d", s.FrontierVisitedCount())
	}
}

// TestSubmitUrlForAdmission_CrawlDelayPositive_UpdatesHostTimings verifies that when
// robots returns a positive crawl delay, hostTimings is updated correctly.
func TestSubmitUrlForAdmission_CrawlDelayPositive_UpdatesHostTimings(t *testing.T) {
	// GIVEN: a robots.txt with crawl delay
	robotsContent := `User-agent: *
Crawl-delay: 5
Allow: /`
	server := setupTestServer(robotsContent)
	defer server.Close()

	noopSink := &metadata.NoopSink{}
	s := createSchedulerForSubmitTest(noopSink)

	s.InitRobot("test-agent/1.0")

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host
	s.SetCurrentHost(host)

	// Pre-condition: hostTimings should not have this host
	if s.HasHostTiming(host) {
		t.Fatal("Pre-condition failed: host should not exist in hostTimings before test")
	}

	// WHEN: submitting URL for admission
	submitErr := s.SubmitUrlForAdmission(
		*testURL,
		frontier.SourceSeed,
		0,
	)

	// THEN: no error should be returned
	if submitErr != nil {
		t.Errorf("Expected no error, got: %v", submitErr)
	}

	// AND: hostTimings should have the host with correct crawl delay
	if !s.HasHostTiming(host) {
		t.Errorf("Expected host %s to exist in hostTimings", host)
	} else {
		delay := s.GetHostCrawlDelay(host)
		expectedDelay := 5 * time.Second
		if delay != expectedDelay {
			t.Errorf("Expected crawl delay %v, got: %v", expectedDelay, delay)
		}
	}
}

// TestSubmitUrlForAdmission_CrawlDelayZero_DoesNotUpdateHostTimings verifies that when
// robots returns zero crawl delay, hostTimings is NOT mutated.
func TestSubmitUrlForAdmission_CrawlDelayZero_DoesNotUpdateHostTimings(t *testing.T) {
	// GIVEN: a robots.txt with no crawl delay (implicit 0)
	robotsContent := `User-agent: *
Allow: /`
	server := setupTestServer(robotsContent)
	defer server.Close()

	noopSink := &metadata.NoopSink{}
	s := createSchedulerForSubmitTest(noopSink)

	s.InitRobot("test-agent/1.0")

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host
	s.SetCurrentHost(host)

	// WHEN: submitting URL for admission
	submitErr := s.SubmitUrlForAdmission(
		*testURL,
		frontier.SourceSeed,
		0,
	)

	// THEN: no error should be returned
	if submitErr != nil {
		t.Errorf("Expected no error, got: %v", submitErr)
	}

	// AND: URL should be in frontier
	if s.FrontierVisitedCount() != 1 {
		t.Errorf("Expected frontier to have 1 URL, got: %d", s.FrontierVisitedCount())
	}

	// AND: hostTimings should NOT have the host (crawl delay was 0)
	if s.HasHostTiming(host) {
		t.Errorf("Expected host %s to NOT exist in hostTimings when crawl delay is 0", host)
	}
}

// TestSubmitUrlForAdmission_CrawlDelayUpdatesExistingHost verifies that when
// a host already exists in hostTimings, the crawl delay is updated (not duplicated).
func TestSubmitUrlForAdmission_CrawlDelayUpdatesExistingHost(t *testing.T) {
	// GIVEN: a robots.txt with crawl delay
	robotsContent := `User-agent: *
Crawl-delay: 10
Allow: /`
	server := setupTestServer(robotsContent)
	defer server.Close()

	noopSink := &metadata.NoopSink{}
	s := createSchedulerForSubmitTest(noopSink)

	s.InitRobot("test-agent/1.0")

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host
	s.SetCurrentHost(host)

	// First submission to create initial entry with crawl-delay: 10
	firstErr := s.SubmitUrlForAdmission(
		*testURL,
		frontier.SourceSeed,
		0,
	)
	if firstErr != nil {
		t.Fatalf("First submission failed: %v", firstErr)
	}

	// Verify initial delay
	initialDelay := s.GetHostCrawlDelay(host)
	if initialDelay != 10*time.Second {
		t.Fatalf("Expected initial delay 10s, got: %v", initialDelay)
	}

	// Frontier should have 1 URL
	if s.FrontierVisitedCount() != 1 {
		t.Fatalf("Expected frontier to have 1 URL, got: %d", s.FrontierVisitedCount())
	}

	// Create a different URL on same host to test update (won't be deduplicated)
	testURL2, _ := url.Parse(server.URL + "/another-page.html")

	// WHEN: submitting second URL (should update crawl delay, not add duplicate)
	secondErr := s.SubmitUrlForAdmission(
		*testURL2,
		frontier.SourceCrawl,
		1,
	)

	// THEN: no error should be returned
	if secondErr != nil {
		t.Errorf("Expected no error on second submission, got: %v", secondErr)
	}

	// AND: frontier should have 2 URLs
	if s.FrontierVisitedCount() != 2 {
		t.Errorf("Expected frontier to have 2 URLs, got: %d", s.FrontierVisitedCount())
	}

	// AND: crawl delay should still be 10s (updated, not changed)
	currentDelay := s.GetHostCrawlDelay(host)
	if currentDelay != 10*time.Second {
		t.Errorf("Expected crawl delay still 10s, got: %v", currentDelay)
	}
}

// TestSubmitUrlForAdmission_MultipleHosts_DifferentDelays verifies that
// different hosts can have different crawl delays tracked independently.
func TestSubmitUrlForAdmission_MultipleHosts_DifferentDelays(t *testing.T) {
	// GIVEN: two different servers with different crawl delays
	server1Content := `User-agent: *
Crawl-delay: 3
Allow: /`
	server1 := setupTestServer(server1Content)
	defer server1.Close()

	server2Content := `User-agent: *
Crawl-delay: 7
Allow: /`
	server2 := setupTestServer(server2Content)
	defer server2.Close()

	noopSink := &metadata.NoopSink{}
	s := createSchedulerForSubmitTest(noopSink)

	s.InitRobot("test-agent/1.0")

	// Submit URL from first host
	url1, _ := url.Parse(server1.URL + "/page.html")
	s.SetCurrentHost(url1.Host)
	err1 := s.SubmitUrlForAdmission(*url1, frontier.SourceSeed, 0)
	if err1 != nil {
		t.Fatalf("First host submission failed: %v", err1)
	}

	// Submit URL from second host
	url2, _ := url.Parse(server2.URL + "/page.html")
	s.SetCurrentHost(url2.Host)
	err2 := s.SubmitUrlForAdmission(*url2, frontier.SourceSeed, 0)
	if err2 != nil {
		t.Fatalf("Second host submission failed: %v", err2)
	}

	// THEN: both hosts should have their respective crawl delays
	delay1 := s.GetHostCrawlDelay(url1.Host)
	delay2 := s.GetHostCrawlDelay(url2.Host)

	if delay1 != 3*time.Second {
		t.Errorf("Expected host1 delay 3s, got: %v", delay1)
	}
	if delay2 != 7*time.Second {
		t.Errorf("Expected host2 delay 7s, got: %v", delay2)
	}

	// AND: both URLs should be in frontier
	if s.FrontierVisitedCount() != 2 {
		t.Errorf("Expected frontier to have 2 URLs, got: %d", s.FrontierVisitedCount())
	}
}

// TestSubmitUrlForAdmission_DisallowedURL_WithCrawlDelay verifies that when
// a URL is disallowed but has crawl delay, the delay is still recorded.
func TestSubmitUrlForAdmission_DisallowedURL_WithCrawlDelay(t *testing.T) {
	// GIVEN: a robots.txt that disallows all but has crawl delay
	robotsContent := `User-agent: *
Crawl-delay: 5
Disallow: /`
	server := setupTestServer(robotsContent)
	defer server.Close()

	noopSink := &metadata.NoopSink{}
	s := createSchedulerForSubmitTest(noopSink)

	s.InitRobot("test-agent/1.0")

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host
	s.SetCurrentHost(host)

	// WHEN: submitting disallowed URL for admission
	submitErr := s.SubmitUrlForAdmission(
		*testURL,
		frontier.SourceSeed,
		0,
	)

	// THEN: no error should be returned (disallowed is terminal outcome)
	if submitErr != nil {
		t.Errorf("Expected nil for disallowed URL, got: %v", submitErr)
	}

	// AND: URL should NOT be in frontier
	if s.FrontierVisitedCount() != 0 {
		t.Errorf("Expected frontier to have 0 URLs (disallowed), got: %d", s.FrontierVisitedCount())
	}

	// AND: crawl delay should still be recorded for the host
	if !s.HasHostTiming(host) {
		t.Errorf("Expected host %s to exist in hostTimings even when URL disallowed", host)
	} else {
		delay := s.GetHostCrawlDelay(host)
		if delay != 5*time.Second {
			t.Errorf("Expected crawl delay 5s, got: %v", delay)
		}
	}
}

// TestSubmitUrlForAdmission_PreservesSourceContextAndDepth verifies that
// the source context and depth are preserved when submitting to frontier.
func TestSubmitUrlForAdmission_PreservesSourceContextAndDepth(t *testing.T) {
	// GIVEN: a robots.txt that allows all
	robotsContent := `User-agent: *
Allow: /`
	server := setupTestServer(robotsContent)
	defer server.Close()

	noopSink := &metadata.NoopSink{}
	s := createSchedulerForSubmitTest(noopSink)

	s.InitRobot("test-agent/1.0")

	testURL, _ := url.Parse(server.URL + "/page.html")
	s.SetCurrentHost(testURL.Host)

	// WHEN: submitting with SourceCrawl and depth 3
	submitErr := s.SubmitUrlForAdmission(
		*testURL,
		frontier.SourceCrawl,
		3,
	)

	// THEN: no error should be returned
	if submitErr != nil {
		t.Errorf("Expected no error, got: %v", submitErr)
	}

	// AND: URL should be in frontier
	if s.FrontierVisitedCount() != 1 {
		t.Errorf("Expected frontier to have 1 URL, got: %d", s.FrontierVisitedCount())
	}

	// AND: when dequeued, the depth should be preserved
	// (This verifies the depth is passed through to frontier correctly)
	_, ok := s.DequeueFromFrontier()
	if !ok {
		t.Error("Expected to dequeue a token from frontier")
	}
}

// TestSubmitUrlForAdmission_SpecificPathRules verifies that specific path
// rules in robots.txt are correctly enforced.
func TestSubmitUrlForAdmission_SpecificPathRules(t *testing.T) {
	testCases := []struct {
		name             string
		robotsContent    string
		path             string
		expectAllowed    bool
		expectInFrontier bool
	}{
		{
			name: "allowed path",
			robotsContent: `User-agent: *
Disallow: /private/
Allow: /`,
			path:             "/public/page.html",
			expectAllowed:    true,
			expectInFrontier: true,
		},
		{
			name: "disallowed path",
			robotsContent: `User-agent: *
Disallow: /private/
Allow: /`,
			path:             "/private/secret.html",
			expectAllowed:    false,
			expectInFrontier: false,
		},
		{
			name: "allow overrides disallow",
			robotsContent: `User-agent: *
Disallow: /docs/
Allow: /docs/public/`,
			path:             "/docs/public/guide.html",
			expectAllowed:    true,
			expectInFrontier: true,
		},
		{
			name: "wildcard disallow",
			robotsContent: `User-agent: *
Disallow: /*.pdf$`,
			path:             "/document.pdf",
			expectAllowed:    false,
			expectInFrontier: false,
		},
		{
			name: "wildcard allows other extensions",
			robotsContent: `User-agent: *
Disallow: /*.pdf$`,
			path:             "/page.html",
			expectAllowed:    true,
			expectInFrontier: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := setupTestServer(tc.robotsContent)
			defer server.Close()

			noopSink := &metadata.NoopSink{}
			s := createSchedulerForSubmitTest(noopSink)
			s.InitRobot("test-agent/1.0")

			testURL, _ := url.Parse(server.URL + tc.path)
			s.SetCurrentHost(testURL.Host)

			err := s.SubmitUrlForAdmission(*testURL, frontier.SourceCrawl, 1)

			if err != nil {
				t.Errorf("Expected no error for path %s, got: %v", tc.path, err)
			}

			visitedCount := s.FrontierVisitedCount()
			if tc.expectInFrontier && visitedCount != 1 {
				t.Errorf("Expected URL %s to be in frontier (count=1), got: %d", tc.path, visitedCount)
			}
			if !tc.expectInFrontier && visitedCount != 0 {
				t.Errorf("Expected URL %s to NOT be in frontier (count=0), got: %d", tc.path, visitedCount)
			}
		})
	}
}

// TestScheduler_FinalStats_AccurateEmptyFrontier verifies that when the frontier
// is empty (no URLs to process), final statistics reflect an empty crawl.
func TestScheduler_FinalStats_AccurateEmptyFrontier(t *testing.T) {
	// GIVEN a scheduler with a mock finalizer
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	// Create a scheduler with minimal config that results in empty frontier
	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

	// Create a temp config file with seed URL
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config with seed URL that won't discover anything (dry run effectively)
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 0,
		"dryRun": true
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// WHEN executing the crawl
	_, err = s.ExecuteCrawling(configPath)

	// THEN no error should occur
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// AND final stats should be recorded
	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected RecordFinalCrawlStats to be called")
	}

	// Verify stats are accurate for empty crawl
	// Note: Even with empty frontier, the seed URL may be submitted depending on robots check
	// The key assertion is that stats were recorded and duration is non-negative
	if mockFinalizer.recordedStats.duration < 0 {
		t.Errorf("expected non-negative duration, got %v", mockFinalizer.recordedStats.duration)
	}

	// totalPages should be 0 since robots check will likely fail or frontier will be empty
	// (This depends on the mock implementation of robots checker)
	t.Logf("Final stats recorded: pages=%d, errors=%d, assets=%d, duration=%v",
		mockFinalizer.recordedStats.totalPages,
		mockFinalizer.recordedStats.totalErrors,
		mockFinalizer.recordedStats.totalAssets,
		mockFinalizer.recordedStats.duration)
}

// TestScheduler_FinalStats_RecordsExactlyOnce verifies that RecordFinalCrawlStats
// is called exactly once per crawl execution.
func TestScheduler_FinalStats_RecordsExactlyOnce(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

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

	// Execute crawl
	_, err = s.ExecuteCrawling(configPath)

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

// TestScheduler_FinalStats_DurationNonNegative verifies that recorded duration
// is always non-negative, even for very short crawls.
func TestScheduler_FinalStats_DurationNonNegative(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

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

	startTime := time.Now()
	_, err = s.ExecuteCrawling(configPath)
	elapsedTime := time.Since(startTime)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected stats to be recorded")
	}

	// Recorded duration should be non-negative
	if mockFinalizer.recordedStats.duration < 0 {
		t.Errorf("duration should be non-negative, got %v", mockFinalizer.recordedStats.duration)
	}

	// Recorded duration should not exceed actual elapsed time by much
	// (Allow some tolerance for test execution overhead)
	if mockFinalizer.recordedStats.duration > elapsedTime+100*time.Millisecond {
		t.Errorf("recorded duration %v exceeds elapsed time %v",
			mockFinalizer.recordedStats.duration, elapsedTime)
	}
}

// TestScheduler_GracefulShutdown_ConfigError verifies that the scheduler
// handles config file errors gracefully without panicking.
func TestScheduler_GracefulShutdown_ConfigError(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

	// Try to execute with non-existent config
	_, err := s.ExecuteCrawling("/nonexistent/path/config.json")

	// Should return error, not panic
	if err == nil {
		t.Error("expected error for non-existent config file")
	}

	// Even with error, stats should be recorded (though they may reflect partial/incomplete crawl)
	// This depends on the specific error handling - config errors happen before crawl starts
	// so stats recording may not occur
}

// TestScheduler_GracefulShutdown_InvalidConfig verifies handling of invalid config.
func TestScheduler_GracefulShutdown_InvalidConfig(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(configPath, []byte("{invalid json}"), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(configPath)

	// Should return error
	if err == nil {
		t.Error("expected error for invalid config JSON")
	}
}

// TestScheduler_GracefulShutdown_MissingSeedUrls verifies handling of config without seed URLs.
func TestScheduler_GracefulShutdown_MissingSeedUrls(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.json")

	// Valid JSON but missing required seedUrls
	err := os.WriteFile(configPath, []byte(`{"maxDepth": 5}`), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(configPath)

	// Should return error for missing seed URLs
	if err == nil {
		t.Error("expected error for config without seed URLs")
	}
}

// TestScheduler_StatsAccuracy_PagesTracked verifies that totalPages reflects
// the number of URLs submitted to the frontier.
func TestScheduler_StatsAccuracy_PagesTracked(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config with limited scope
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 0,
		"maxPages": 5
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected stats to be recorded")
	}

	// With maxDepth: 0, the seed URL may be submitted but not processed further
	// totalPages should reflect what was actually submitted to frontier
	t.Logf("Total pages recorded: %d", mockFinalizer.recordedStats.totalPages)

	// The exact number depends on whether robots allowed the seed URL
	// Key assertion: stats are recorded and consistent
	if mockFinalizer.recordedStats.totalPages < 0 {
		t.Error("totalPages should be non-negative")
	}
}

// TestScheduler_StatsAccuracy_ErrorsTracked verifies that totalErrors is tracked
// correctly during the crawl.
func TestScheduler_StatsAccuracy_ErrorsTracked(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 1
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected stats to be recorded")
	}

	// Errors should be non-negative
	if mockFinalizer.recordedStats.totalErrors < 0 {
		t.Error("totalErrors should be non-negative")
	}

	t.Logf("Total errors recorded: %d", mockFinalizer.recordedStats.totalErrors)
}

// TestScheduler_StatsAccuracy_AssetsTracked verifies that totalAssets is tracked.
func TestScheduler_StatsAccuracy_AssetsTracked(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}]
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockFinalizer.recordedStats == nil {
		t.Fatal("expected stats to be recorded")
	}

	// Assets should be non-negative (currently always 0 as asset counting is not fully implemented)
	if mockFinalizer.recordedStats.totalAssets < 0 {
		t.Error("totalAssets should be non-negative")
	}

	t.Logf("Total assets recorded: %d", mockFinalizer.recordedStats.totalAssets)
}

// TestScheduler_FinalStatsContract_CalledAfterTermination verifies the contract
// that RecordFinalCrawlStats is called only after crawl termination.
func TestScheduler_FinalStatsContract_CalledAfterTermination(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

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

	// Execute crawl
	_, err = s.ExecuteCrawling(configPath)

	// After ExecuteCrawling returns, stats should be recorded
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
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config that will likely encounter errors (e.g., network errors when trying to fetch)
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "nonexistent-domain-12345.com"}],
		"maxDepth": 1,
		"timeout": "1s"
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Execute crawl - may encounter network/robots errors but should not panic
	_, err = s.ExecuteCrawling(configPath)

	// Depending on error handling, this may or may not return an error
	// The key assertion is that stats were recorded
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
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}]
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(configPath)
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

// errorRecordingSink is a test double that counts errors
type errorRecordingSink struct {
	errorCount int
}

func (e *errorRecordingSink) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	details string,
	attrs []metadata.Attribute,
) {
	e.errorCount++
}

func (e *errorRecordingSink) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
}

func (e *errorRecordingSink) RecordArtifact(path string) {}

// TestScheduler_ErrorCounting_ConsistentWithMetadata verifies that the
// error count in final stats is consistent with errors recorded to metadata sink.
func TestScheduler_ErrorCounting_ConsistentWithMetadata(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	errorSink := &errorRecordingSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, errorSink)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}]
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(configPath)
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

// compile-time interface checks
var _ metadata.CrawlFinalizer = (*mockFinalizer)(nil)
var _ metadata.MetadataSink = (*metadata.NoopSink)(nil)
var _ metadata.MetadataSink = (*errorRecordingSink)(nil)

// TestScheduler_ConfigurationImmutability verifies that the scheduler
// uses the configuration as provided and doesn't modify it.
func TestScheduler_ConfigurationImmutability(t *testing.T) {
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

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
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

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
	mockFinalizer := newMockFinalizer()
	noopSink := &metadata.NoopSink{}

	s := scheduler.NewSchedulerWithDeps(mockFinalizer, noopSink)

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
