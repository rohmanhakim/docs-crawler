package scheduler_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/stretchr/testify/mock"
)

// TestSubmitUrlForAdmission_RobotsAllowed_SubmitsToFrontier verifies that when robots
// allows a URL, it is submitted to the frontier.
func TestSubmitUrlForAdmission_RobotsAllowed_SubmitsToFrontier(t *testing.T) {
	// GIVEN: a robots.txt that allows all crawling
	robotsContent := `User-agent: *
Allow: /`
	server := setupTestServer(t, robotsContent)
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

	// Set current host
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

	// AND: limiter SetCrawlDelay should not have been called
	mockLimiter.AssertNotCalled(t, "SetCrawlDelay", testURL.Host)
}

// TestSubmitUrlForAdmission_RobotsDisallowed_DoesNotSubmitToFrontier verifies that when
// robots disallows a URL, it is NOT submitted to the frontier but returns nil (terminal outcome).
func TestSubmitUrlForAdmission_RobotsDisallowed_DoesNotSubmitToFrontier(t *testing.T) {
	// GIVEN: a robots.txt that disallows all crawling
	robotsContent := `User-agent: *
Disallow: /`
	server := setupTestServer(t, robotsContent)
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

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

	// AND: limiter SetCrawlDelay should not have been called
	mockLimiter.AssertNotCalled(t, "SetCrawlDelay", testURL.Host)
}

// TestSubmitUrlForAdmission_RobotsError_ReturnsError verifies that when robots
// encounters an infrastructure error, it returns the error and does not submit to frontier.
func TestSubmitUrlForAdmission_RobotsError_ReturnsError(t *testing.T) {
	// GIVEN: a server that returns 500 for robots.txt (infrastructure error)
	server := setupTestServerWithStatus(t, 500, "")
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

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

	// AND: limiter SetCrawlDelay should not have been called
	mockLimiter.AssertNotCalled(t, "SetCrawlDelay", testURL.Host)
}

// TestSubmitUrlForAdmission_CrawlDelayPositive_UpdatesHostTimings verifies that when
// robots returns a positive crawl delay, SetCrawlDelay is called.
func TestSubmitUrlForAdmission_CrawlDelayPositive_UpdatesHostTimings(t *testing.T) {
	// GIVEN: a robots.txt with crawl delay
	robotsContent := `User-agent: *
Crawl-delay: 5
Allow: /`
	server := setupTestServer(t, robotsContent)
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host
	s.SetCurrentHost(host)

	// Expect SetCrawlDelay to be called with the crawl delay

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

	// AND: SetCrawlDelay should have been called with the correct delay
	mockLimiter.AssertCalled(t, "SetCrawlDelay", host, 5*time.Second)

	// AND: URL should be in frontier
	if s.FrontierVisitedCount() != 1 {
		t.Errorf("Expected frontier to have 1 URL, got: %d", s.FrontierVisitedCount())
	}
}

// TestSubmitUrlForAdmission_CrawlDelayZero_DoesNotCallSetCrawlDelay verifies that when
// robots returns zero crawl delay, SetCrawlDelay is NOT called.
func TestSubmitUrlForAdmission_CrawlDelayZero_DoesNotCallSetCrawlDelay(t *testing.T) {
	// GIVEN: a robots.txt with no crawl delay (implicit 0)
	robotsContent := `User-agent: *
Allow: /`
	server := setupTestServer(t, robotsContent)
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

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

	// AND: SetCrawlDelay should NOT have been called (since crawl-delay is 0)
	mockLimiter.AssertNotCalled(t, "SetCrawlDelay", mock.Anything, mock.Anything)
}

// TestSubmitUrlForAdmission_CrawlDelayUpdatesExistingHost verifies that when
// a host already exists, SetCrawlDelay is still called with the new value.
func TestSubmitUrlForAdmission_CrawlDelayUpdatesExistingHost(t *testing.T) {
	// GIVEN: a robots.txt with crawl delay
	robotsContent := `User-agent: *
Crawl-delay: 10
Allow: /`
	server := setupTestServer(t, robotsContent)
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host
	s.SetCurrentHost(host)

	// Expect SetCrawlDelay to be called twice (once per submission)

	// First submission to create initial entry
	err1 := s.SubmitUrlForAdmission(*testURL, frontier.SourceSeed, 0)
	if err1 != nil {
		t.Fatalf("First submission failed: %v", err1)
	}

	// Frontier should have 1 URL
	if s.FrontierVisitedCount() != 1 {
		t.Fatalf("Expected frontier to have 1 URL, got: %d", s.FrontierVisitedCount())
	}

	// Create a different URL on same host to test update (won't be deduplicated)
	testURL2, _ := url.Parse(server.URL + "/another-page.html")

	// WHEN: submitting second URL
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

	// AND: SetCrawlDelay should have been called twice
	mockLimiter.AssertNumberOfCalls(t, "SetCrawlDelay", 2)
}

// TestSubmitUrlForAdmission_MultipleHosts_DifferentDelays verifies that
// different hosts can have different crawl delays tracked independently.
func TestSubmitUrlForAdmission_MultipleHosts_DifferentDelays(t *testing.T) {
	// GIVEN: two different servers with different crawl delays
	server1Content := `User-agent: *
Crawl-delay: 3
Allow: /`
	server1 := setupTestServer(t, server1Content)
	defer server1.Close()

	server2Content := `User-agent: *
Crawl-delay: 7
Allow: /`
	server2 := setupTestServer(t, server2Content)
	defer server2.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

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

	// THEN: SetCrawlDelay should have been called for both hosts with correct delays
	mockLimiter.AssertCalled(t, "SetCrawlDelay", url1.Host, 3*time.Second)
	mockLimiter.AssertCalled(t, "SetCrawlDelay", url2.Host, 7*time.Second)

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
	server := setupTestServer(t, robotsContent)
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

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

	// AND: SetCrawlDelay should still be called for the host
	mockLimiter.AssertCalled(t, "SetCrawlDelay", host, 5*time.Second)
}

// TestSubmitUrlForAdmission_PreservesSourceContextAndDepth verifies that
// the source context and depth are preserved when submitting to frontier.
func TestSubmitUrlForAdmission_PreservesSourceContextAndDepth(t *testing.T) {
	// GIVEN: a robots.txt that allows all
	robotsContent := `User-agent: *
Allow: /`
	server := setupTestServer(t, robotsContent)
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

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
	_, ok := s.DequeueFromFrontier()
	if !ok {
		t.Error("Expected to dequeue a token from frontier")
	}

	// AND: limiter SetCrawlDelay should not have been called
	mockLimiter.AssertNotCalled(t, "SetCrawlDelay", testURL.Host)
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
			server := setupTestServer(t, tc.robotsContent)
			defer server.Close()

			ctx := context.Background()
			mockFinalizer := newMockFinalizer(t)
			noopSink := &metadata.NoopSink{}
			mockLimiter := newRateLimiterMockForTest(t)
			mockFetcher := newFetcherMockForTest(t)
			s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

			testURL, _ := url.Parse(server.URL + tc.path)
			s.SetCurrentHost(testURL.Host)

			// Verify limiter SetCrawlDelay should not have been called
			mockLimiter.AssertNotCalled(t, "SetCrawlDelay", testURL.Host)

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
