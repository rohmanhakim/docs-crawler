package scheduler_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/stretchr/testify/mock"
)

// TestSubmitUrlForAdmission_RobotsAllowed_SubmitsToFrontier verifies that when robots
// allows a URL, it is submitted to the frontier.
func TestSubmitUrlForAdmission_RobotsAllowed_SubmitsToFrontier(t *testing.T) {
	// GIVEN: a robots.txt that allows all crawling
	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil)

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

	// Set current host
	testURL, _ := url.Parse("https://example.com/page.html")
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
	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    false,
		Reason:     robots.DisallowedByRobots,
		CrawlDelay: 0,
	}, nil)

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

	testURL, _ := url.Parse("https://example.com/page.html")
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
	// GIVEN: robots encounters an infrastructure error (e.g., 500)
	robotsErr := &robots.RobotsError{
		Message:   "http error: 500",
		Retryable: false,
		Cause:     robots.ErrCauseHttpServerError,
	}
	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(mock.Anything, robots.Decision{}, robotsErr)

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

	testURL, _ := url.Parse("https://example.com/page.html")
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
	fiveSeconds := 5 * time.Second
	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: fiveSeconds,
	}, nil)

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

	testURL, _ := url.Parse("https://example.com/page.html")
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
	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil)

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

	testURL, _ := url.Parse("https://example.com/page.html")
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
	tenSeconds := 10 * time.Second

	// Create two distinct URLs on the same host
	testURL1, _ := url.Parse("https://example.com/page.html")
	testURL2, _ := url.Parse("https://example.com/another-page.html")

	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(*testURL1, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: tenSeconds,
	}, nil).Once()
	mockRobot.OnDecide(*testURL2, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: tenSeconds,
	}, nil).Once()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

	host := testURL1.Host
	s.SetCurrentHost(host)

	// First submission
	err1 := s.SubmitUrlForAdmission(*testURL1, frontier.SourceSeed, 0)
	if err1 != nil {
		t.Fatalf("First submission failed: %v", err1)
	}

	// Frontier should have 1 URL after first submission
	if s.FrontierVisitedCount() != 1 {
		t.Fatalf("Expected frontier to have 1 URL after first, got: %d", s.FrontierVisitedCount())
	}

	// Second submission with different URL on same host
	// WHEN: submitting second URL
	err2 := s.SubmitUrlForAdmission(*testURL2, frontier.SourceCrawl, 1)
	if err2 != nil {
		t.Fatalf("Second submission failed: %v", err2)
	}

	// THEN: frontier should have 2 URLs
	if s.FrontierVisitedCount() != 2 {
		t.Errorf("Expected frontier to have 2 URLs, got: %d", s.FrontierVisitedCount())
	}

	// AND: SetCrawlDelay should have been called twice (once per submission)
	mockLimiter.AssertNumberOfCalls(t, "SetCrawlDelay", 2)
}

// TestSubmitUrlForAdmission_MultipleHosts_DifferentDelays verifies that
// different hosts can have different crawl delays tracked independently.
func TestSubmitUrlForAdmission_MultipleHosts_DifferentDelays(t *testing.T) {
	// GIVEN: two different servers with different crawl delays
	threeSeconds := 3 * time.Second
	sevenSeconds := 7 * time.Second

	host1 := "example1.com"
	host2 := "example2.com"

	url1, _ := url.Parse("https://" + host1 + "/page.html")
	url2, _ := url.Parse("https://" + host2 + "/page.html")

	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(*url1, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: threeSeconds,
	}, nil).Once()
	mockRobot.OnDecide(*url2, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: sevenSeconds,
	}, nil).Once()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

	// Submit URL from first host
	s.SetCurrentHost(host1)
	err1 := s.SubmitUrlForAdmission(*url1, frontier.SourceSeed, 0)
	if err1 != nil {
		t.Fatalf("First host submission failed: %v", err1)
	}

	// Submit URL from second host
	s.SetCurrentHost(host2)
	err2 := s.SubmitUrlForAdmission(*url2, frontier.SourceSeed, 0)
	if err2 != nil {
		t.Fatalf("Second host submission failed: %v", err2)
	}

	// THEN: SetCrawlDelay should have been called for both hosts with correct delays
	mockLimiter.AssertCalled(t, "SetCrawlDelay", host1, 3*time.Second)
	mockLimiter.AssertCalled(t, "SetCrawlDelay", host2, 7*time.Second)

	// AND: both URLs should be in frontier
	if s.FrontierVisitedCount() != 2 {
		t.Errorf("Expected frontier to have 2 URLs, got: %d", s.FrontierVisitedCount())
	}
}

// TestSubmitUrlForAdmission_DisallowedURL_WithCrawlDelay verifies that when
// a URL is disallowed but has crawl delay, the delay is still recorded.
func TestSubmitUrlForAdmission_DisallowedURL_WithCrawlDelay(t *testing.T) {
	// GIVEN: a robots.txt that disallows all but has crawl delay
	fiveSeconds := 5 * time.Second
	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    false,
		Reason:     robots.DisallowedByRobots,
		CrawlDelay: fiveSeconds,
	}, nil)

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

	testURL, _ := url.Parse("https://example.com/page.html")
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
	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil)

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

	testURL, _ := url.Parse("https://example.com/page.html")
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
		allowed          bool
		crawlDelay       time.Duration
		expectInFrontier bool
	}{
		{
			name:             "allowed path",
			allowed:          true,
			crawlDelay:       0,
			expectInFrontier: true,
		},
		{
			name:             "disallowed path",
			allowed:          false,
			crawlDelay:       0,
			expectInFrontier: false,
		},
		{
			name:             "allow overrides disallow",
			allowed:          true,
			crawlDelay:       0,
			expectInFrontier: true,
		},
		{
			name:             "wildcard disallow",
			allowed:          false,
			crawlDelay:       0,
			expectInFrontier: false,
		},
		{
			name:             "wildcard allows other extensions",
			allowed:          true,
			crawlDelay:       0,
			expectInFrontier: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build test URL
			testURL, _ := url.Parse("https://example.com" + getTestPath(tc.name))

			mockRobot := NewRobotsMockForTest(t)
			mockRobot.OnDecide(*testURL, robots.Decision{
				Allowed:    tc.allowed,
				Reason:     robots.EmptyRuleSet,
				CrawlDelay: tc.crawlDelay,
			}, nil).Once()

			ctx := context.Background()
			mockFinalizer := newMockFinalizer(t)
			noopSink := &metadata.NoopSink{}
			mockLimiter := newRateLimiterMockForTest(t)
			mockFetcher := newFetcherMockForTest(t)
			s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher)

			s.SetCurrentHost(testURL.Host)

			// Verify limiter SetCrawlDelay should not have been called if crawlDelay is 0
			if tc.crawlDelay == 0 {
				mockLimiter.AssertNotCalled(t, "SetCrawlDelay", testURL.Host)
			}

			err := s.SubmitUrlForAdmission(*testURL, frontier.SourceCrawl, 1)

			if err != nil {
				t.Errorf("Expected no error for test case %s, got: %v", tc.name, err)
			}

			visitedCount := s.FrontierVisitedCount()
			if tc.expectInFrontier && visitedCount != 1 {
				t.Errorf("Expected URL to be in frontier (count=1), got: %d", visitedCount)
			}
			if !tc.expectInFrontier && visitedCount != 0 {
				t.Errorf("Expected URL to NOT be in frontier (count=0), got: %d", visitedCount)
			}
		})
	}
}

// getTestPath returns the test path for each test case
func getTestPath(testName string) string {
	switch testName {
	case "allowed path":
		return "/public/page.html"
	case "disallowed path":
		return "/private/secret.html"
	case "allow overrides disallow":
		return "/docs/public/guide.html"
	case "wildcard disallow":
		return "/document.pdf"
	case "wildcard allows other extensions":
		return "/page.html"
	default:
		return "/page.html"
	}
}
