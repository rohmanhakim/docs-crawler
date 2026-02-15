package fetcher_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// mockMetadataSink is a test double for metadata.MetadataSink
type mockMetadataSink struct {
	fetchEvents    []fetchEvent
	errorEvents    []errorEvent
	artifactEvents []string
}

type fetchEvent struct {
	fetchUrl    string
	httpStatus  int
	duration    time.Duration
	contentType string
	retryCount  int
	crawlDepth  int
}

type errorEvent struct {
	observedAt  time.Time
	packageName string
	action      string
	cause       metadata.ErrorCause
	details     string
	attrs       []metadata.Attribute
}

func (m *mockMetadataSink) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
	m.fetchEvents = append(m.fetchEvents, fetchEvent{
		fetchUrl:    fetchUrl,
		httpStatus:  httpStatus,
		duration:    duration,
		contentType: contentType,
		retryCount:  retryCount,
		crawlDepth:  crawlDepth,
	})
}

func (m *mockMetadataSink) RecordAssetFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	retryCount int,
) {
	m.fetchEvents = append(m.fetchEvents, fetchEvent{
		fetchUrl:   fetchUrl,
		httpStatus: httpStatus,
		duration:   duration,
		retryCount: retryCount,
	})
}

func (m *mockMetadataSink) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	details string,
	attrs []metadata.Attribute,
) {
	m.errorEvents = append(m.errorEvents, errorEvent{
		observedAt:  observedAt,
		packageName: packageName,
		action:      action,
		cause:       cause,
		details:     details,
		attrs:       attrs,
	})
}

func (m *mockMetadataSink) RecordArtifact(kind metadata.ArtifactKind, path string, attrs []metadata.Attribute) {
	m.artifactEvents = append(m.artifactEvents, path)
}

// createTestRetryParam creates retry parameters for testing
func createTestRetryParam(maxAttempts int) retry.RetryParam {
	return retry.NewRetryParam(
		100*time.Millisecond, // baseDelay
		50*time.Millisecond,  // jitter
		42,                   // randomSeed
		maxAttempts,          // maxAttempts
		timeutil.NewBackoffParam(
			100*time.Millisecond,
			2.0,
			1*time.Second,
		),
	)
}

func TestHtmlFetcher_Fetch_Success(t *testing.T) {
	// Create a test server that returns valid HTML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Hello World</body></html>"))
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(3)

	result, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Code() != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, result.Code())
	}

	if string(result.Body()) != "<html><body>Hello World</body></html>" {
		t.Errorf("unexpected body: %s", string(result.Body()))
	}

	// Verify fetch event was recorded
	if len(sink.fetchEvents) != 1 {
		t.Fatalf("expected 1 fetch event, got %d", len(sink.fetchEvents))
	}

	fetchEvt := sink.fetchEvents[0]
	if fetchEvt.fetchUrl != server.URL {
		t.Errorf("expected URL %s, got %s", server.URL, fetchEvt.fetchUrl)
	}
	if fetchEvt.httpStatus != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, fetchEvt.httpStatus)
	}
	if fetchEvt.crawlDepth != 0 {
		t.Errorf("expected crawl depth 0, got %d", fetchEvt.crawlDepth)
	}
	// Verify retry count records actual attempts (1 for immediate success), not MaxAttempts
	if fetchEvt.retryCount != 1 {
		t.Errorf("expected retry count 1 (actual attempts), got %d", fetchEvt.retryCount)
	}

	// Verify no error events were recorded
	if len(sink.errorEvents) != 0 {
		t.Errorf("expected 0 error events, got %d", len(sink.errorEvents))
	}
}

func TestHtmlFetcher_Fetch_NonHTMLContent(t *testing.T) {
	// Create a test server that returns non-HTML content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "not html"}`))
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(3)

	_, err := f.Fetch(context.Background(), 1, *fetchUrl, retryParam)

	if err == nil {
		t.Fatal("expected error for non-HTML content, got nil")
	}

	// Verify it's a FetchError (non-retryable)
	var fetchErr *fetcher.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("expected FetchError, got %T", err)
	}

	if fetchErr.IsRetryable() {
		t.Error("expected non-retryable error for invalid content type")
	}

	// Verify fetch event was recorded with status 0 (error case)
	if len(sink.fetchEvents) != 1 {
		t.Fatalf("expected 1 fetch event, got %d", len(sink.fetchEvents))
	}

	// Verify error event was recorded
	if len(sink.errorEvents) != 1 {
		t.Fatalf("expected 1 error event, got %d", len(sink.errorEvents))
	}

	errorEvt := sink.errorEvents[0]
	if errorEvt.packageName != "fetcher" {
		t.Errorf("expected package name 'fetcher', got %s", errorEvt.packageName)
	}
}

func TestHtmlFetcher_Fetch_HTTP404(t *testing.T) {
	// Create a test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(3)

	_, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)

	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}

	// Verify it's a non-retryable FetchError
	var fetchErr *fetcher.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("expected FetchError, got %T", err)
	}

	if fetchErr.IsRetryable() {
		t.Error("expected non-retryable error for 404")
	}
}

func TestHtmlFetcher_Fetch_HTTP403(t *testing.T) {
	// Create a test server that returns 403
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(3)

	_, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)

	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}

	// Verify it's a non-retryable FetchError
	var fetchErr *fetcher.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("expected FetchError, got %T", err)
	}

	if fetchErr.IsRetryable() {
		t.Error("expected non-retryable error for 403")
	}
}

func TestHtmlFetcher_Fetch_HTTP500_Retryable(t *testing.T) {
	// Create a test server that returns 500
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(2)

	_, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)

	if err == nil {
		t.Fatal("expected error after retries exhausted, got nil")
	}

	// Verify multiple requests were made (retries happened)
	if requestCount < 2 {
		t.Errorf("expected at least 2 requests due to retry, got %d", requestCount)
	}

	// Verify it's a RetryError after retries exhausted
	var retryErr *retry.RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryError after exhausted retries, got %T", err)
	}

	// Verify error event was recorded as retry failure
	if len(sink.errorEvents) != 1 {
		t.Fatalf("expected 1 error event, got %d", len(sink.errorEvents))
	}

	errorEvt := sink.errorEvents[0]
	if errorEvt.cause != metadata.CauseRetryFailure {
		t.Errorf("expected cause CauseRetryFailure, got %v", errorEvt.cause)
	}

	// Verify retry count records actual attempts (2), not MaxAttempts
	if len(sink.fetchEvents) != 1 {
		t.Fatalf("expected 1 fetch event, got %d", len(sink.fetchEvents))
	}
	fetchEvt := sink.fetchEvents[0]
	if fetchEvt.retryCount != 2 {
		t.Errorf("expected retry count 2 (actual attempts), got %d", fetchEvt.retryCount)
	}
}

func TestHtmlFetcher_Fetch_HTTP429_Retryable(t *testing.T) {
	// Create a test server that returns 429 (Too Many Requests)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(2)

	_, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)

	if err == nil {
		t.Fatal("expected error after retries exhausted, got nil")
	}

	// Verify multiple requests were made (retries happened)
	if requestCount < 2 {
		t.Errorf("expected at least 2 requests due to retry, got %d", requestCount)
	}

	// Verify it's a RetryError after retries exhausted
	var retryErr *retry.RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryError after exhausted retries, got %T", err)
	}
}

func TestHtmlFetcher_Fetch_SuccessAfterRetry(t *testing.T) {
	// Create a test server that fails once then succeeds
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>Success</html>"))
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(3)

	result, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)

	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests (1 fail + 1 success), got %d", requestCount)
	}

	if result.Code() != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, result.Code())
	}

	// Verify retry count records actual attempts (2), not MaxAttempts (3)
	if len(sink.fetchEvents) != 1 {
		t.Fatalf("expected 1 fetch event, got %d", len(sink.fetchEvents))
	}
	fetchEvt := sink.fetchEvents[0]
	if fetchEvt.retryCount != 2 {
		t.Errorf("expected retry count 2 (actual attempts), got %d", fetchEvt.retryCount)
	}

	// Verify no error events were recorded (success case)
	if len(sink.errorEvents) != 0 {
		t.Errorf("expected 0 error events, got %d", len(sink.errorEvents))
	}
}

func TestHtmlFetcher_FetchResult_Accessors(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>Test</html>"))
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(3)

	result, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test URL accessor - compare string representations (need to take address for String())
	resultURL := result.URL()
	if resultURL.String() != fetchUrl.String() {
		t.Errorf("expected URL %s, got %s", fetchUrl.String(), resultURL.String())
	}

	// Test Code accessor
	if result.Code() != http.StatusOK {
		t.Errorf("expected code %d, got %d", http.StatusOK, result.Code())
	}

	// Test SizeByte accessor
	expectedSize := uint64(len("<html>Test</html>"))
	if result.SizeByte() != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, result.SizeByte())
	}

	// Test Headers accessor
	headers := result.Headers()
	if headers["Content-Type"] != "text/html; charset=utf-8" {
		t.Errorf("unexpected Content-Type header: %s", headers["Content-Type"])
	}
	if headers["X-Custom-Header"] != "test-value" {
		t.Errorf("unexpected X-Custom-Header: %s", headers["X-Custom-Header"])
	}
}

func TestFetchError_Classification(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		contentType     string
		expectRetryable bool
	}{
		{
			name:            "200 OK HTML - no error",
			statusCode:      http.StatusOK,
			contentType:     "text/html",
			expectRetryable: false,
		},
		{
			name:            "500 Internal Server Error - retryable",
			statusCode:      http.StatusInternalServerError,
			contentType:     "text/html",
			expectRetryable: true,
		},
		{
			name:            "502 Bad Gateway - retryable",
			statusCode:      http.StatusBadGateway,
			contentType:     "text/html",
			expectRetryable: true,
		},
		{
			name:            "503 Service Unavailable - retryable",
			statusCode:      http.StatusServiceUnavailable,
			contentType:     "text/html",
			expectRetryable: true,
		},
		{
			name:            "400 Bad Request - not retryable",
			statusCode:      http.StatusBadRequest,
			contentType:     "text/html",
			expectRetryable: false,
		},
		{
			name:            "401 Unauthorized - not retryable",
			statusCode:      http.StatusUnauthorized,
			contentType:     "text/html",
			expectRetryable: false,
		},
		{
			name:            "403 Forbidden - not retryable",
			statusCode:      http.StatusForbidden,
			contentType:     "text/html",
			expectRetryable: false,
		},
		{
			name:            "404 Not Found - not retryable",
			statusCode:      http.StatusNotFound,
			contentType:     "text/html",
			expectRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip success case
			if tt.statusCode == http.StatusOK {
				return
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			sink := &mockMetadataSink{}
			f := fetcher.NewHtmlFetcher(sink)
			f.Init(&http.Client{}, "test-user-agent")

			fetchUrl, _ := url.Parse(server.URL)
			retryParam := createTestRetryParam(1) // Single attempt to test classification

			_, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)

			if err == nil {
				t.Fatal("expected error")
			}

			var fetchErr *fetcher.FetchError
			if errors.As(err, &fetchErr) {
				if fetchErr.IsRetryable() != tt.expectRetryable {
					t.Errorf("expected retryable=%v, got retryable=%v", tt.expectRetryable, fetchErr.IsRetryable())
				}
			}
		})
	}
}

func TestHtmlFetcher_MetadataSinkInterface(t *testing.T) {
	// Verify that mockMetadataSink implements the interface
	var _ metadata.MetadataSink = &mockMetadataSink{}
}

func TestHtmlFetcher_FetchError_Severity(t *testing.T) {
	// Test that FetchError implements ClassifiedError correctly
	err := &fetcher.FetchError{
		Message:   "test error",
		Retryable: true,
		Cause:     fetcher.ErrCauseNetworkFailure,
	}

	// Verify it implements failure.ClassifiedError
	var classifiedErr failure.ClassifiedError = err

	if classifiedErr.Severity() != failure.SeverityRecoverable {
		t.Errorf("expected SeverityRecoverable for retryable error, got %s", classifiedErr.Severity())
	}

	// Test non-retryable error
	nonRetryableErr := &fetcher.FetchError{
		Message:   "test error",
		Retryable: false,
		Cause:     fetcher.ErrCauseContentTypeInvalid,
	}

	classifiedErr = nonRetryableErr
	if classifiedErr.Severity() != failure.SeverityFatal {
		t.Errorf("expected SeverityFatal for non-retryable error, got %s", classifiedErr.Severity())
	}
}

func TestHtmlFetcher_NoMetadataSinkPanics(t *testing.T) {
	// This test verifies the fetcher works with a real (or mock) sink
	// The actual panic scenario would require nil sink which we don't support
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(3)

	// Should not panic
	_, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHtmlFetcher_Fetch_ReadResponseBodyError(t *testing.T) {
	// Test the scenario where io.ReadAll(resp.Body) returns an error.
	// We use a test server that hijacks the connection and abruptly closes it
	// after sending only partial body, causing a read error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("response writer does not support hijacking")
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			t.Fatal("hijack failed:", err)
		}
		defer conn.Close()

		// Write response headers declaring a large Content-Length
		headers := "HTTP/1.1 200 OK\r\n" +
			"Content-Type: text/html; charset=utf-8\r\n" +
			"Content-Length: 100\r\n" +
			"\r\n"
		if _, err := bufrw.WriteString(headers); err != nil {
			t.Fatal("write headers failed:", err)
		}
		// Write only a small portion of the body
		if _, err := bufrw.WriteString("partial"); err != nil {
			t.Fatal("write body failed:", err)
		}
		bufrw.Flush()
		// Close the connection abruptly to simulate read error
		conn.Close()
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	f := fetcher.NewHtmlFetcher(sink)
	f.Init(&http.Client{}, "test-user-agent")

	fetchUrl, _ := url.Parse(server.URL)
	retryParam := createTestRetryParam(1) // single attempt; since error is retryable, exhaustion yields RetryError

	_, err := f.Fetch(context.Background(), 0, *fetchUrl, retryParam)

	if err == nil {
		t.Fatal("expected error for read response body failure, got nil")
	}

	// Because the underlying FetchError is retryable, the retry wrapper will
	// return a RetryError after exhaustion (even with maxAttempts=1).
	var retryErr *retry.RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryError, got %T", err)
	}

	// Verify the error message contains the expected FetchError cause
	if !strings.Contains(retryErr.Error(), fetcher.ErrCauseReadResponseBodyError) {
		t.Errorf("expected error message to contain cause %q, got %q", fetcher.ErrCauseReadResponseBodyError, retryErr.Error())
	}

	// Verify fetch event was recorded
	if len(sink.fetchEvents) != 1 {
		t.Fatalf("expected 1 fetch event, got %d", len(sink.fetchEvents))
	}

	// Verify error event was recorded as retry failure
	if len(sink.errorEvents) != 1 {
		t.Fatalf("expected 1 error event, got %d", len(sink.errorEvents))
	}

	errorEvt := sink.errorEvents[0]
	if errorEvt.packageName != "fetcher" {
		t.Errorf("expected package name 'fetcher', got %s", errorEvt.packageName)
	}
	if errorEvt.cause != metadata.CauseRetryFailure {
		t.Errorf("expected cause CauseRetryFailure, got %v", errorEvt.cause)
	}
}
