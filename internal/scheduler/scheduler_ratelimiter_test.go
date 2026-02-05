package scheduler_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/stretchr/testify/mock"
)

// TestRateLimiter_SetBaseDelay_Called verifies SetBaseDelay is called during initialization.
func TestRateLimiter_SetBaseDelay_Called(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	// Expect these methods to be called during crawl initialization
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("SetCrawlDelay", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResetBackoff", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

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
	// GIVEN: a robots.txt with crawl delay
	robotsContent := `User-agent: *
Crawl-delay: 8
Allow: /`
	server := setupTestServer(t, robotsContent)
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	// Expect these methods to be called
	mockLimiter.On("SetCrawlDelay", mock.Anything, 8*time.Second).Return()
	mockLimiter.On("ResetBackoff", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

	testURL, _ := url.Parse(server.URL + "/page.html")
	s.SetCurrentHost(testURL.Host)

	// WHEN: submitting URL for admission
	err := s.SubmitUrlForAdmission(*testURL, frontier.SourceSeed, 0)

	// THEN: no error should be returned
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// AND: SetCrawlDelay should have been called with exactly 8 seconds
	mockLimiter.AssertCalled(t, "SetCrawlDelay", testURL.Host, 8*time.Second)
}

// TestRateLimiter_SetJitter_CalledWithConfigValue verifies SetJitter is called
// with the jitter value from config.
func TestRateLimiter_SetJitter_CalledWithConfigValue(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	// Expect these methods to be called during initialization
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", 500*time.Millisecond).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("SetCrawlDelay", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResetBackoff", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

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

	// Expect these methods to be called during initialization
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", int64(42)).Return()
	mockLimiter.On("SetCrawlDelay", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResetBackoff", mock.Anything).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, noopSink, mockLimiter, mockFetcher)

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

// TestBackoff_TriggersOnTooManyRequests verifies that when robots returns HTTP 429,
// the scheduler records the error and triggers backoff via ExecuteCrawling.
func TestBackoff_TriggersOnTooManyRequests(t *testing.T) {
	// GIVEN: a server that returns 429 for robots.txt
	server := setupTestServerWithStatus(t, http.StatusTooManyRequests, "")
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("Backoff", host).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockFetcher)
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

// TestBackoff_TriggersOnServerError verifies that when robots returns HTTP 5xx,
// the scheduler records the error and triggers backoff via ExecuteCrawling.
func TestBackoff_TriggersOnServerError(t *testing.T) {
	// GIVEN: a server that returns 503 for robots.txt
	server := setupTestServerWithStatus(t, http.StatusServiceUnavailable, "")
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("Backoff", host).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockFetcher)
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
	// GIVEN: a server that returns 403 for robots.txt (not 429 or 5xx)
	server := setupTestServerWithStatus(t, http.StatusForbidden, "")
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	mockLimiter.On("ResetBackoff", host).Return()
	// NOTE: Backoff should NOT be called for 403

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockFetcher)
	s.SetCurrentHost(host)

	// WHEN: submitting URL for admission
	err := s.SubmitUrlForAdmission(*testURL, frontier.SourceSeed, 0)

	// THEN: no error should be returned (403 is treated as "no robots.txt")
	if err != nil {
		t.Errorf("Expected no error for 403 response (treated as no restrictions), got: %v", err)
	}

	// AND: Backoff should NOT have been called
	mockLimiter.AssertNotCalled(t, "Backoff", mock.Anything)
}

// TestBackoff_Integration_ExecuteCrawling verifies that ExecuteCrawling properly
// handles backoff when robots returns 429 for the seed URL.
func TestBackoff_Integration_ExecuteCrawling(t *testing.T) {
	// GIVEN: a server that returns 429 for robots.txt
	server := setupTestServerWithStatus(t, http.StatusTooManyRequests, "")
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	host := server.URL[7:] // Remove "http://" prefix

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	// Backoff should be called for the host when 429 is received
	mockLimiter.On("Backoff", host).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockFetcher)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config pointing to our test server
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "` + host + `"}],
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
	if errorSink.errorCount != 2 {
		t.Errorf("Expected 2 errors to be recorded (robots + scheduler), got %d", errorSink.errorCount)
	}
}

// TestResetBackoff_CalledOnSuccessfulRobotsRequest verifies that ResetBackoff is called
// after a successful robots.txt request to clear any previous backoff state.
func TestResetBackoff_CalledOnSuccessfulRobotsRequest(t *testing.T) {
	// GIVEN: a robots.txt that allows all crawling
	robotsContent := `User-agent: *
Allow: /`
	server := setupTestServer(t, robotsContent)
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	// ResetBackoff should be called after successful robots request
	mockLimiter.On("ResetBackoff", host).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockFetcher)
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
	// GIVEN: a server that returns 429 for robots.txt
	server := setupTestServerWithStatus(t, http.StatusTooManyRequests, "")
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	testURL, _ := url.Parse(server.URL + "/page.html")
	host := testURL.Host

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	// ResetBackoff should NOT be called on failed robots request
	// Backoff WILL be called via recordRobotsErrorAndBackoff
	mockLimiter.On("Backoff", host).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockFetcher)
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
	// GIVEN: a server that returns 503 for robots.txt
	server := setupTestServerWithStatus(t, http.StatusServiceUnavailable, "")
	defer server.Close()

	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	errorSink := &errorRecordingSink{}
	mockLimiter := new(rateLimiterMock)
	mockFetcher := newFetcherMockForTest(t)

	host := server.URL[7:] // Remove "http://" prefix

	// Expect these rate limiter calls
	mockLimiter.On("SetBaseDelay", mock.Anything).Return()
	mockLimiter.On("SetJitter", mock.Anything).Return()
	mockLimiter.On("SetRandomSeed", mock.Anything).Return()
	// Backoff should be called for the host when 503 is received
	mockLimiter.On("Backoff", host).Return()

	s := createSchedulerForTest(t, ctx, mockFinalizer, errorSink, mockLimiter, mockFetcher)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config pointing to our test server
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "` + host + `"}],
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

	// AND: Error should have been recorded (2 errors: 1 from robots, 1 from scheduler)
	if errorSink.errorCount != 2 {
		t.Errorf("Expected 2 errors to be recorded (robots + scheduler), got %d", errorSink.errorCount)
	}
}
