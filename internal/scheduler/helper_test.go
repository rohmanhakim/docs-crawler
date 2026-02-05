package scheduler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/stretchr/testify/mock"
)

// createSchedulerForTest creates a scheduler with test-specific initialization
// that allows testing scheduler in isolation
func createSchedulerForTest(
	t *testing.T,
	ctx context.Context,
	mockFinalizer *mockFinalizer,
	metadataSink metadata.MetadataSink,
	mockLimiter *rateLimiterMock,
	mockFetcher *fetcherMock,
) *scheduler.Scheduler {
	t.Helper()
	robot := robots.NewRobot(metadataSink)
	robot.Init("testAgent")
	s := scheduler.NewSchedulerWithDeps(ctx, mockFinalizer, metadataSink, mockLimiter, mockFetcher, robot)
	return &s
}

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

func newMockFinalizer(t *testing.T) *mockFinalizer {
	t.Helper()
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

// rateLimiterMock is a testify mock for the RateLimiter
type rateLimiterMock struct {
	mock.Mock
}

// newRateLimiterMockForTest creates a properly configured rate limiter mock for crawl tests
func newRateLimiterMockForTest(t *testing.T) *rateLimiterMock {
	t.Helper()
	m := new(rateLimiterMock)
	// Set up default expectations for crawl tests
	m.On("SetBaseDelay", mock.Anything).Return()
	m.On("SetJitter", mock.Anything).Return()
	m.On("SetRandomSeed", mock.Anything).Return()
	m.On("SetCrawlDelay", mock.Anything, mock.Anything).Return()
	m.On("Backoff", mock.Anything).Return()
	m.On("ResetBackoff", mock.Anything).Return()
	m.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	return m
}

func (m *rateLimiterMock) SetBaseDelay(baseDelay time.Duration) {
	m.Called(baseDelay)
}

func (m *rateLimiterMock) SetJitter(jitter time.Duration) {
	m.Called(jitter)
}

func (m *rateLimiterMock) SetRandomSeed(randomSeed int64) {
	m.Called(randomSeed)
}

func (m *rateLimiterMock) SetCrawlDelay(host string, delay time.Duration) {
	m.Called(host, delay)
}

func (m *rateLimiterMock) Backoff(host string) {
	m.Called(host)
}

func (m *rateLimiterMock) ResetBackoff(host string) {
	m.Called(host)
}

func (m *rateLimiterMock) MarkLastFetchAsNow(host string) {
	m.Called(host)
}

func (m *rateLimiterMock) Jitter(base time.Duration) time.Duration {
	args := m.Called(base)
	return args.Get(0).(time.Duration)
}

func (m *rateLimiterMock) SetRNG(rng interface{}) {
	m.Called(rng)
}

func (m *rateLimiterMock) ResolveDelay(host string) time.Duration {
	args := m.Called(host)
	return args.Get(0).(time.Duration)
}

// setupTestServer creates a test HTTP server that serves robots.txt content
func setupTestServer(t *testing.T, robotsContent string) *httptest.Server {
	t.Helper()
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
func setupTestServerWithStatus(t *testing.T, statusCode int, robotsContent string) *httptest.Server {
	t.Helper()
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

// fetcherMock is a testify mock for the Fetcher
type fetcherMock struct {
	mock.Mock
}

func (f *fetcherMock) Fetch(
	ctx context.Context,
	crawlDepth int,
	fetchParam fetcher.FetchParam,
	retryParam retry.RetryParam,
) (fetcher.FetchResult, failure.ClassifiedError) {
	args := f.Called(ctx, crawlDepth, fetchParam, retryParam)
	result := args.Get(0).(fetcher.FetchResult)
	var err failure.ClassifiedError
	if args.Get(1) != nil {
		err = args.Get(1).(failure.ClassifiedError)
	}
	return result, err
}

// newFetcherMockForTest creates a properly configured fetcher mock for crawl tests
func newFetcherMockForTest(t *testing.T) *fetcherMock {
	t.Helper()
	m := new(fetcherMock)
	// Set up default expectation to return empty result (no error)
	// Tests can override this by setting their own expectations
	m.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fetcher.FetchResult{}, nil)
	return m
}

// setupFetcherMockWithSuccess sets up the fetcher mock to return a successful response
func setupFetcherMockWithSuccess(m *fetcherMock, urlStr string, body []byte, statusCode int) {
	testURL, _ := url.Parse(urlStr)
	result := fetcher.NewFetchResultForTest(*testURL, body, statusCode, "text/html", uint64(len(body)), map[string]string{
		"Content-Type": "text/html",
	})
	m.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(result, nil)
}

// setupFetcherMockWithError sets up the fetcher mock to return an error
func setupFetcherMockWithError(m *fetcherMock, err failure.ClassifiedError) {
	m.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fetcher.FetchResult{}, err)
}

// setupFetcherMockWithNetworkError sets up the fetcher mock to return a network error
func setupFetcherMockWithNetworkError(m *fetcherMock) {
	testErr := &mockClassifiedError{
		msg:      "network error: connection refused",
		severity: failure.SeverityRecoverable,
	}
	m.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fetcher.FetchResult{}, testErr)
}

// setupFetcherMockWithFatalError sets up the fetcher mock to return a fatal error
func setupFetcherMockWithFatalError(m *fetcherMock) {
	testErr := &mockClassifiedError{
		msg:      "fatal error: invalid URL scheme",
		severity: failure.SeverityFatal,
	}
	m.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fetcher.FetchResult{}, testErr)
}

// mockClassifiedError is a mock implementation of failure.ClassifiedError for testing
type mockClassifiedError struct {
	msg      string
	severity failure.Severity
}

func (e *mockClassifiedError) Error() string {
	return e.msg
}

func (e *mockClassifiedError) Severity() failure.Severity {
	return e.severity
}
