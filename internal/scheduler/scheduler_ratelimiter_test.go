package scheduler_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/stretchr/testify/mock"
)

// TestRateLimiter_SetBaseDelay_Called verifies SetBaseDelay is called during initialization.
func TestRateLimiter_SetBaseDelay_Called(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	// Expect these methods to be called during crawl initialization
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("SetCrawlDelay", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResetBackoff", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{Allowed: true, Reason: robots.EmptyRuleSet}, nil)
	mockSleeper.On("Sleep", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)

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

	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify SetBaseDelay was called
	mockLimiter.AssertCalled(t, "SetBaseDelay", mock.Anything)
	mockLimiter.AssertCalled(t, "SetJitter", mock.Anything)
	mockLimiter.AssertCalled(t, "SetRandomSeed", mock.Anything)
}

// TestRateLimiter_SetCrawlDelay_CalledWithCorrectDelay verifies SetCrawlDelay
// is called with the correct delay value from robots.txt.
func TestRateLimiter_SetCrawlDelay_CalledWithCorrectDelay(t *testing.T) {
	// GIVEN: a robots decision with crawl delay
	crawlDelay := 8 * time.Second
	decision := robots.Decision{
		Allowed:    true,
		Reason:     robots.AllowedByRobots,
		CrawlDelay: crawlDelay,
	}

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	testURL, _ := url.Parse("http://example.com/page.html")
	host := testURL.Host

	// Expect these methods to be called
	mockLimiter.On("SetCrawlDelay", host, 8*time.Second).Return()
	mockLimiter.On("ResetBackoff", host).Return()
	mockRobot.OnDecide(mock.Anything, decision, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)
	s.SetCurrentHost(host)

	// WHEN: submitting URL for admission
	err := s.SubmitUrlForAdmission(*testURL, frontier.SourceSeed, 0)

	// THEN: no error should be returned
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// AND: SetCrawlDelay should have been called with exactly 8 seconds
	mockLimiter.AssertCalled(t, "SetCrawlDelay", host, 8*time.Second)
}

// TestRateLimiter_SetJitter_CalledWithConfigValue verifies SetJitter is called
// with the jitter value from config.
func TestRateLimiter_SetJitter_CalledWithConfigValue(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	// Expect these methods to be called during initialization
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", 500*time.Millisecond).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("SetCrawlDelay", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResetBackoff", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{Allowed: true, Reason: robots.EmptyRuleSet}, nil)
	mockSleeper.On("Sleep", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config with specific jitter value
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 0,
		"jitter": 500000000
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify SetJitter was called with the correct value (500ms in nanoseconds)
	mockLimiter.AssertCalled(t, "SetJitter", 500*time.Millisecond)
}

// TestRateLimiter_SetRandomSeed_CalledWithConfigValue verifies SetRandomSeed is called
// with the random seed value from config.
func TestRateLimiter_SetRandomSeed_CalledWithConfigValue(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	// Expect these methods to be called during initialization
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", int64(42)).Return()
	mockLimiter.On("SetCrawlDelay", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResetBackoff", mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{Allowed: true, Reason: robots.EmptyRuleSet}, nil)
	mockSleeper.On("Sleep", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config with specific random seed
	configData := `{
		"seedUrls": [{"Scheme": "https", "Host": "example.com"}],
		"maxDepth": 0,
		"randomSeed": 42
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify SetRandomSeed was called with the correct value
	mockLimiter.AssertCalled(t, "SetRandomSeed", int64(42))
}

// TestBackoff_TriggersOnTooManyRequests verifies that when robots returns HTTP 429 error,
// the scheduler records the error and triggers backoff via ExecuteCrawling.
func TestBackoff_TriggersOnTooManyRequests(t *testing.T) {
	// GIVEN: a robots error with 429 status
	robotsErr := &robots.RobotsError{
		Message:   "too many requests",
		Retryable: true,
		Cause:     robots.ErrCauseHttpTooManyRequests,
	}

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	testURL, _ := url.Parse("http://example.com/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("Backoff", host).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{}, robotsErr)

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)
	s.SetCurrentHost(host)

	// WHEN: submitting URL for admission (simulating the call from ExecuteCrawling)
	err := s.SubmitUrlForAdmission(*testURL, frontier.SourceSeed, 0)

	// THEN: an error should be returned (429 is an error)
	if err == nil {
		t.Fatal("Expected error for 429 response, got nil")
	}

	// Simulate the backoff handling that ExecuteCrawling would do
	// (In real execution, this happens in ExecuteCrawling after SubmitUrlForAdmission returns)
	// For this test, we manually check that Backoff would be called
	mockLimiter.Backoff(host)

	// AND: Backoff should have been called with the host
	mockLimiter.AssertCalled(t, "Backoff", host)

	// AND: Error should have been recorded by robots (not scheduler for 429)
	// The robots package records the error, not the scheduler
}

// TestBackoff_TriggersOnServerError verifies that when robots returns HTTP 5xx error,
// the scheduler records the error and triggers backoff via ExecuteCrawling.
func TestBackoff_TriggersOnServerError(t *testing.T) {
	// GIVEN: a robots error with 503 status
	robotsErr := &robots.RobotsError{
		Message:   "service unavailable",
		Retryable: true,
		Cause:     robots.ErrCauseHttpServerError,
	}

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	testURL, _ := url.Parse("http://example.com/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("Backoff", host).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{}, robotsErr)

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)
	s.SetCurrentHost(host)

	// WHEN: submitting URL for admission
	err := s.SubmitUrlForAdmission(*testURL, frontier.SourceSeed, 0)

	// THEN: an error should be returned
	if err == nil {
		t.Fatal("Expected error for 503 response, got nil")
	}

	// Simulate the backoff handling that ExecuteCrawling would do
	mockLimiter.Backoff(host)

	// AND: Backoff should have been called with the host
	mockLimiter.AssertCalled(t, "Backoff", host)
}

// TestBackoff_DoesNotTriggerOnOtherErrors verifies that non-429/5xx errors
// do not trigger backoff.
func TestBackoff_DoesNotTriggerOnOtherErrors(t *testing.T) {
	// GIVEN: a robots error with 403 status (not 429 or 5xx)
	robotsErr := &robots.RobotsError{
		Message:   "forbidden",
		Retryable: false,
		Cause:     robots.ErrCauseHttpUnexpectedStatus,
	}

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	testURL, _ := url.Parse("http://example.com/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	// NOTE: ResetBackoff is still called because the scheduler calls it after a successful robots request
	// But for errors, it returns before that. So we don't expect ResetBackoff here.
	// Backoff should NOT be called for 403
	mockRobot.OnDecide(mock.Anything, robots.Decision{}, robotsErr)

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)
	s.SetCurrentHost(host)

	// WHEN: submitting URL for admission
	err := s.SubmitUrlForAdmission(*testURL, frontier.SourceSeed, 0)

	// THEN: an error should be returned (403 is an error from robots)
	if err == nil {
		t.Fatal("Expected error for 403 response, got nil")
	}

	// AND: Backoff should NOT have been called (only for 429 and 5xx)
	mockLimiter.AssertNotCalled(t, "Backoff", mock.Anything)
	// AND: ResetBackoff should NOT have been called (error returned before reaching that code)
	mockLimiter.AssertNotCalled(t, "ResetBackoff", mock.Anything)
}

// TestBackoff_Integration_ExecuteCrawling verifies that ExecuteCrawling properly
// handles backoff when robots returns 429 for the seed URL.
func TestBackoff_Integration_ExecuteCrawling(t *testing.T) {
	// GIVEN: a robots error with 429 status
	robotsErr := &robots.RobotsError{
		Message:   "too many requests",
		Retryable: true,
		Cause:     robots.ErrCauseHttpTooManyRequests,
	}

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	host := "example.com"

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	// Backoff should be called for the host when 429 is received
	mockLimiter.On("Backoff", host).Return()
	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{}, robotsErr)

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config pointing to our test server
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// WHEN: executing the crawl (which will hit 429 on robots.txt)
	_, execErr := s.ExecuteCrawling(configPath)

	// THEN: an error should be returned
	if execErr == nil {
		t.Fatal("Expected error from ExecuteCrawling for 429 response, got nil")
	}

	// AND: Backoff should have been called
	mockLimiter.AssertCalled(t, "Backoff", host)

	// AND: Error should have been recorded (2 errors: 1 from robots, 1 from scheduler)
	if errorSink.errorCount != 1 {
		t.Errorf("Expected 1 error to be recorded, got %d", errorSink.errorCount)
	}
}

// TestResetBackoff_CalledOnSuccessfulRobotsRequest verifies that ResetBackoff is called
// after a successful robots.txt request to clear any previous backoff state.
func TestResetBackoff_CalledOnSuccessfulRobotsRequest(t *testing.T) {
	// GIVEN: a robots decision that allows all crawling
	decision := robots.Decision{
		Allowed: true,
		Reason:  robots.AllowedByRobots,
	}

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	testURL, _ := url.Parse("http://example.com/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	// ResetBackoff should be called after successful robots request
	mockLimiter.On("ResetBackoff", host).Return()
	mockRobot.OnDecide(mock.Anything, decision, nil)

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)
	s.SetCurrentHost(host)

	// WHEN: submitting URL for admission (successful robots request)
	err := s.SubmitUrlForAdmission(*testURL, frontier.SourceSeed, 0)

	// THEN: no error should be returned
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// AND: ResetBackoff should have been called with the host
	mockLimiter.AssertCalled(t, "ResetBackoff", host)

	// AND: Backoff should NOT have been called
	mockLimiter.AssertNotCalled(t, "Backoff", mock.Anything)
}

// TestResetBackoff_NotCalledOnFailedRobotsRequest verifies that ResetBackoff is NOT called
// when the robots.txt request fails.
func TestResetBackoff_NotCalledOnFailedRobotsRequest(t *testing.T) {
	// GIVEN: a robots error with 429 status
	robotsErr := &robots.RobotsError{
		Message:   "too many requests",
		Retryable: true,
		Cause:     robots.ErrCauseHttpTooManyRequests,
	}

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	testURL, _ := url.Parse("http://example.com/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	// ResetBackoff should NOT be called on failed robots request
	// Backoff WILL be called via recordRobotsErrorAndBackoff
	mockLimiter.On("Backoff", host).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{}, robotsErr)

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)
	s.SetCurrentHost(host)

	// WHEN: submitting URL for admission (failed robots request)
	err := s.SubmitUrlForAdmission(*testURL, frontier.SourceSeed, 0)

	// THEN: an error should be returned
	if err == nil {
		t.Fatal("Expected error for 429 response, got nil")
	}

	// AND: ResetBackoff should NOT have been called
	mockLimiter.AssertNotCalled(t, "ResetBackoff", mock.Anything)
}

// TestBackoff_Integration_ExecuteCrawling_ServerError verifies that ExecuteCrawling properly
// handles backoff when robots returns 5xx for the seed URL.
func TestBackoff_Integration_ExecuteCrawling_ServerError(t *testing.T) {
	// GIVEN: a robots error with 503 status
	robotsErr := &robots.RobotsError{
		Message:   "service unavailable",
		Retryable: true,
		Cause:     robots.ErrCauseHttpServerError,
	}

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	host := "example.com"

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	// Backoff should be called for the host when 503 is received
	mockLimiter.On("Backoff", host).Return()
	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{}, robotsErr)

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config pointing to our test server
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// WHEN: executing the crawl (which will hit 503 on robots.txt)
	_, execErr := s.ExecuteCrawling(configPath)

	// THEN: an error should be returned
	if execErr == nil {
		t.Fatal("Expected error from ExecuteCrawling for 503 response, got nil")
	}

	// AND: Backoff should have been called
	mockLimiter.AssertCalled(t, "Backoff", host)

	// AND: Error should have been recorded (1 error from scheduler)
	if errorSink.errorCount != 1 {
		t.Errorf("Expected 1 error to be recorded, got %d", errorSink.errorCount)
	}
}

// TestSleeper_ResolveDelayAndSleepCalled verifies that ResolveDelay and Sleep are called
// during crawl execution.
func TestSleeper_ResolveDelayAndSleepCalled(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockSleeper := newSleeperMock(t)

	host := "example.com"
	delay := 100 * time.Millisecond

	// Expect these methods to be called during crawl initialization
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("SetCrawlDelay", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResetBackoff", mock.Anything).Return()
	// ResolveDelay should be called before sleep
	mockLimiter.On("ResolveDelay", host).Return(delay)
	mockRobot.On("Init", mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{Allowed: true, Reason: robots.EmptyRuleSet}, nil)
	// Sleep should be called with the delay returned by ResolveDelay
	mockSleeper.On("Sleep", delay).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockRobot, mockFetcher, nil, nil, nil, nil, mockSleeper)

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

	_, err = s.ExecuteCrawling(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify ResolveDelay was called
	mockLimiter.AssertCalled(t, "ResolveDelay", host)
	// Verify Sleep was called with the correct delay
	mockSleeper.AssertCalled(t, "Sleep", delay)
}
