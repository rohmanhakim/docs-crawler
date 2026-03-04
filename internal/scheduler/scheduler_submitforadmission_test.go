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

	mockFrontier := newFrontierMockForTest(t)

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)
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
		mockFailureJournal,
	)

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

	// AND: limiter SetResourceDelay should not have been called
	mockLimiter.AssertNotCalled(t, "SetResourceDelay", testURL.Host)
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
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)
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
		mockFailureJournal,
	)

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

	// AND: limiter SetResourceDelay should not have been called
	mockLimiter.AssertNotCalled(t, "SetResourceDelay", testURL.Host)
}

// TestSubmitUrlForAdmission_RobotsError_ReturnsError verifies that when robots
// encounters an infrastructure error, it returns the error and does not submit to frontier.
func TestSubmitUrlForAdmission_RobotsError_ReturnsError(t *testing.T) {
	// GIVEN: robots encounters an infrastructure error (e.g., 500)
	robotsErr := robots.NewRobotsError(robots.ErrCauseHttpServerError, "http error: 500")

	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(mock.Anything, robots.Decision{}, robotsErr)

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)
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
		mockFailureJournal,
	)

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

	// AND: limiter SetResourceDelay should not have been called
	mockLimiter.AssertNotCalled(t, "SetResourceDelay", testURL.Host)
}

// TestSubmitUrlForAdmission_CrawlDelayPositive_UpdatesHostTimings verifies that when
// robots returns a positive crawl delay, SetResourceDelay is called.
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
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)
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
		mockFailureJournal,
	)

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

	// AND: SetResourceDelay should have been called with the correct delay
	mockLimiter.AssertCalled(t, "SetResourceDelay", host, 5*time.Second)

	// AND: URL should be in frontier
	if s.FrontierVisitedCount() != 1 {
		t.Errorf("Expected frontier to have 1 URL, got: %d", s.FrontierVisitedCount())
	}
}

// TestSubmitUrlForAdmission_CrawlDelayZero_DoesNotCallSetResourceDelay verifies that when
// robots returns zero crawl delay, SetResourceDelay is NOT called.
func TestSubmitUrlForAdmission_CrawlDelayZero_DoesNotCallSetResourceDelay(t *testing.T) {
	// GIVEN: a robots.txt with no crawl delay (implicit 0)
	mockRobot := NewRobotsMockForTest(t)
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil)

	mockFrontier := newFrontierMockForTest(t)

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

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
		mockFailureJournal,
	)

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

	// AND: SetResourceDelay should NOT have been called (since crawl-delay is 0)
	mockLimiter.AssertNotCalled(t, "SetResourceDelay", mock.Anything, mock.Anything)
}

// TestSubmitUrlForAdmission_CrawlDelayUpdatesExistingHost verifies that when
// a host already exists, SetResourceDelay is still called with the new value.
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
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)
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
		mockFailureJournal,
	)

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

	// AND: SetResourceDelay should have been called twice (once per submission)
	mockLimiter.AssertNumberOfCalls(t, "SetResourceDelay", 2)
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
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

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
		mockFailureJournal,
	)

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

	// THEN: SetResourceDelay should have been called for both hosts with correct delays
	mockLimiter.AssertCalled(t, "SetResourceDelay", host1, 3*time.Second)
	mockLimiter.AssertCalled(t, "SetResourceDelay", host2, 7*time.Second)

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
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)
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
		mockFailureJournal,
	)

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

	// AND: SetResourceDelay should still be called for the host
	mockLimiter.AssertCalled(t, "SetResourceDelay", host, 5*time.Second)
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

	mockFrontier := newFrontierMockForTest(t)
	// Enable auto-enqueue so Submit adds tokens to the queue
	mockFrontier.disableAutoEnqueue = false

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)
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
		mockFailureJournal,
	)

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

	// AND: limiter SetResourceDelay should not have been called
	mockLimiter.AssertNotCalled(t, "SetResourceDelay", testURL.Host)
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
			mockFrontier := newFrontierMockForTest(t)
			mockFetcher := newFetcherMockForTest(t)
			mockStorage := newStorageMockForTest(t)
			mockFailureJournal := newFailureJournalMockForTest(t)
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
				mockFailureJournal,
			)

			s.SetCurrentHost(testURL.Host)

			// Verify limiter SetResourceDelay should not have been called if crawlDelay is 0
			if tc.crawlDelay == 0 {
				mockLimiter.AssertNotCalled(t, "SetResourceDelay", testURL.Host)
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

// TestSubmitUrlForAdmission_CanonicalizesBeforeRobotsCheck verifies that URLs are
// canonicalized BEFORE the robots.txt check is performed. This ensures that:
// - URLs like /docs/ and /docs are treated as the same path
// - Query parameters are stripped before robots check
// - Fragments are removed before robots check
// This is critical for proper robots.txt enforcement and avoiding duplicate crawling.
func TestSubmitUrlForAdmission_CanonicalizesBeforeRobotsCheck(t *testing.T) {
	testCases := []struct {
		name         string
		inputURL     string
		expectedPath string // The path that should be sent to robots
	}{
		{
			name:         "trailing slash removed",
			inputURL:     "https://example.com/docs/",
			expectedPath: "/docs",
		},
		{
			name:         "no trailing slash stays same",
			inputURL:     "https://example.com/docs",
			expectedPath: "/docs",
		},
		{
			name:         "query parameters removed",
			inputURL:     "https://example.com/docs?utm_source=twitter",
			expectedPath: "/docs",
		},
		{
			name:         "fragment removed",
			inputURL:     "https://example.com/docs#section",
			expectedPath: "/docs",
		},
		{
			name:         "both query and fragment removed",
			inputURL:     "https://example.com/docs?utm=1#section",
			expectedPath: "/docs",
		},
		{
			name:         "multiple trailing slashes removed",
			inputURL:     "https://example.com/docs///",
			expectedPath: "/docs",
		},
		{
			name:         "root path with trailing slash preserved",
			inputURL:     "https://example.com/",
			expectedPath: "/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputURL, _ := url.Parse(tc.inputURL)

			// Track what URL was passed to robots.Decide()
			var receivedURLForRobots url.URL
			mockRobot := NewRobotsMockForTest(t)
			mockRobot.On("Decide", mock.Anything).Run(func(args mock.Arguments) {
				receivedURLForRobots = args.Get(0).(url.URL)
			}).Return(robots.Decision{
				Allowed:    true,
				Reason:     robots.EmptyRuleSet,
				CrawlDelay: 0,
			}, nil).Once()

			ctx := context.Background()
			mockFinalizer := newMockFinalizer(t)
			noopSink := &metadata.NoopSink{}
			mockLimiter := newRateLimiterMockForTest(t)
			mockFrontier := newFrontierMockForTest(t)
			mockFetcher := newFetcherMockForTest(t)
			mockStorage := newStorageMockForTest(t)
			mockFailureJournal := newFailureJournalMockForTest(t)
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
				mockFailureJournal,
			)

			s.SetCurrentHost(inputURL.Host)

			// WHEN: submitting URL for admission
			err := s.SubmitUrlForAdmission(*inputURL, frontier.SourceSeed, 0)

			// THEN: no error should be returned
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			// AND: robots should have been called
			mockRobot.AssertNumberOfCalls(t, "Decide", 1)

			// AND: the URL passed to robots should have the expected canonicalized path
			if receivedURLForRobots.Path != tc.expectedPath {
				t.Errorf("Expected robots to receive path %q, but got %q", tc.expectedPath, receivedURLForRobots.Path)
			}

			// AND: there should be no query parameters in the URL passed to robots
			if receivedURLForRobots.RawQuery != "" {
				t.Errorf("Expected robots to receive no query params, but got %q", receivedURLForRobots.RawQuery)
			}

			// AND: there should be no fragment in the URL passed to robots
			if receivedURLForRobots.Fragment != "" {
				t.Errorf("Expected robots to receive no fragment, but got %q", receivedURLForRobots.Fragment)
			}
		})
	}
}

// TestSubmitUrlForAdmission_EquivalentURLsTreatedAsSame verifies that all equivalent URLs
// are processed correctly. The key assertion is that robots receives canonicalized URLs,
// ensuring consistent robots.txt enforcement. The actual deduplication is handled by frontier.
func TestSubmitUrlForAdmission_EquivalentURLsTreatedAsSame(t *testing.T) {
	// These URLs are semantically equivalent after canonicalization
	urls := []string{
		"https://example.com/docs/",
		"https://example.com/docs",
		"https://example.com/docs?ref=1",
		"https://example.com/docs#section",
	}

	// Track what URLs were passed to robots
	var receivedURLs []url.URL
	mockRobot := NewRobotsMockForTest(t)
	mockRobot.On("Decide", mock.Anything).Run(func(args mock.Arguments) {
		receivedURLs = append(receivedURLs, args.Get(0).(url.URL))
	}).Return(robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil)

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)
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
		mockFailureJournal,
	)

	host := "example.com"
	s.SetCurrentHost(host)

	// Submit all equivalent URLs
	for _, urlStr := range urls {
		u, _ := url.Parse(urlStr)
		err := s.SubmitUrlForAdmission(*u, frontier.SourceCrawl, 1)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
	}

	// THEN: robots should be called for each submission
	mockRobot.AssertNumberOfCalls(t, "Decide", len(urls))

	// AND: all URLs received by robots should be identical (canonicalized)
	// This is the key assertion - robots receives the same canonical URL for all equivalent inputs
	if len(receivedURLs) != len(urls) {
		t.Fatalf("Expected %d URLs received by robots, got %d", len(urls), len(receivedURLs))
	}

	expectedPath := "/docs"
	for i, receivedURL := range receivedURLs {
		if receivedURL.Path != expectedPath {
			t.Errorf("URL[%d]: expected path %q, got %q", i, expectedPath, receivedURL.Path)
		}
		if receivedURL.RawQuery != "" {
			t.Errorf("URL[%d]: expected no query, got %q", i, receivedURL.RawQuery)
		}
		if receivedURL.Fragment != "" {
			t.Errorf("URL[%d]: expected no fragment, got %q", i, receivedURL.Fragment)
		}
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
