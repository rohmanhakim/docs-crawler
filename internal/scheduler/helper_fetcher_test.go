package scheduler_test

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/retrier"
	"github.com/stretchr/testify/mock"
)

// fetcherMock is a testify mock for the Fetcher.
// When useBarrier is true, Fetch() tracks concurrent calls and blocks
// at a channel-based barrier until expectedWorkers arrive simultaneously.
// This enables deterministic concurrency measurement in tests.
type fetcherMock struct {
	mock.Mock
	// Optional concurrency tracking (zero values = disabled)
	useBarrier      bool
	expectedWorkers int
	barrierReady    chan struct{}
	closeOnce       sync.Once
	peakWorkers     atomic.Int32
	activeWorkers   atomic.Int32
}

func (f *fetcherMock) Init(httpClient *http.Client, userAgent string) {
	f.Called(httpClient, userAgent)
}

func (f *fetcherMock) Fetch(
	ctx context.Context,
	crawlDepth int,
	fetchUrl url.URL,
	retryOptions []retrier.RetryOption,
) (fetcher.FetchResult, failure.ClassifiedError) {
	// Concurrency tracking: barrier + peak measurement
	if f.useBarrier {
		current := f.activeWorkers.Add(1)
		// Update peak using CAS loop
		for {
			old := f.peakWorkers.Load()
			if current <= old || f.peakWorkers.CompareAndSwap(old, current) {
				break
			}
		}
		// Close barrier channel once expectedWorkers have arrived
		if int(current) >= f.expectedWorkers {
			f.closeOnce.Do(func() { close(f.barrierReady) })
		}
		// Block until all expected workers are concurrent
		<-f.barrierReady
		f.activeWorkers.Add(-1)
	}
	args := f.Called(ctx, crawlDepth, fetchUrl, retryOptions)
	result := args.Get(0).(fetcher.FetchResult)
	var err failure.ClassifiedError
	if args.Get(1) != nil {
		err = args.Get(1).(failure.ClassifiedError)
	}
	return result, err
}

// defaultValidHTML is a minimal valid HTML document with meaningful content
// for extractor tests that ensures Layer 1 or Layer 2 heuristics succeed.
const defaultValidHTML = `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<main>
<h1>Test Content</h1>
<p>This is meaningful content that passes the extraction heuristics.</p>
</main>
</body>
</html>`

// PeakWorkers returns the peak number of concurrent Fetch calls observed.
// Only meaningful when useBarrier is true.
func (f *fetcherMock) PeakWorkers() int32 {
	return f.peakWorkers.Load()
}

// newConcurrentFetcherMockForTest creates a fetcher mock configured for
// concurrency measurement. The Fetch method blocks until expectedWorkers
// goroutines arrive simultaneously, then records the peak concurrency.
func newConcurrentFetcherMockForTest(t *testing.T, expectedWorkers int) *fetcherMock {
	t.Helper()
	m := &fetcherMock{
		useBarrier:      true,
		expectedWorkers: expectedWorkers,
		barrierReady:    make(chan struct{}),
	}
	// Set up default expectation for Init
	m.On("Init", mock.Anything, mock.Anything).Return()
	// Set up Fetch to return valid HTML — use Maybe() for concurrent calls
	testURL, _ := url.Parse("https://example.com/test")
	result := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(defaultValidHTML),
		200,
		"text/html",
		map[string]string{"Content-Type": "text/html"},
		time.Now(),
	)
	m.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(result, nil).Maybe()
	return m
}

// newFetcherMockForTest creates a properly configured fetcher mock for crawl tests
func newFetcherMockForTest(t *testing.T) *fetcherMock {
	t.Helper()
	m := new(fetcherMock)
	// Set up default expectation for Init
	m.On("Init", mock.Anything, mock.Anything).Return()
	// Set up default expectation to return valid HTML with meaningful content
	// This ensures the extractor won't fail with "no content" errors
	testURL, _ := url.Parse("https://example.com/test")
	result := fetcher.NewFetchResultForTest(
		*testURL,
		[]byte(defaultValidHTML),
		200,
		"text/html",
		map[string]string{
			"Content-Type": "text/html",
		},
		time.Now(),
	)
	m.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(result, nil)
	return m
}

// setupFetcherMockWithSuccess sets up the fetcher mock to return a successful response
func setupFetcherMockWithSuccess(m *fetcherMock, urlStr string, body []byte, statusCode int) {
	testURL, _ := url.Parse(urlStr)
	result := fetcher.NewFetchResultForTest(
		*testURL,
		body,
		statusCode,
		"text/html",
		map[string]string{
			"Content-Type": "text/html",
		},
		time.Now(),
	)
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
