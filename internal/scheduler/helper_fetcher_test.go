package scheduler_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/stretchr/testify/mock"
)

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
