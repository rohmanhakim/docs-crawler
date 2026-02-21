package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
)

/*
Responsibilities

- Perform HTTP requests
- Apply headers and timeouts
- Handle redirects safely
- Classify responses

# Fetch Semantics

- Only successful HTML responses are processed
- Non-HTML content is discarded
- Redirect chains are bounded
- All responses are logged with metadata

The fetcher never parses content; it only returns bytes and metadata.
*/
type Fetcher interface {
	Init(httpClient *http.Client, userAgent string)
	Fetch(
		ctx context.Context,
		crawlDepth int,
		fetchUrl url.URL,
		retryParam retry.RetryParam,
	) (FetchResult, failure.ClassifiedError)
}

type HtmlFetcher struct {
	metadataSink metadata.MetadataSink
	httpClient   *http.Client
	userAgent    string
	debugLogger  debug.DebugLogger
}

func NewHtmlFetcher(
	metadataSink metadata.MetadataSink,
) HtmlFetcher {
	return HtmlFetcher{
		metadataSink: metadataSink,
		httpClient:   nil,
		debugLogger:  debug.NewNoOpLogger(),
	}
}

// Init initializes the HtmlFetcher with an HTTP client and user agent.
// This must be called before Fetch is invoked.
func (h *HtmlFetcher) Init(httpClient *http.Client, userAgent string) {
	h.httpClient = httpClient
	h.userAgent = userAgent
}

// SetDebugLogger sets the debug logger for the fetcher.
// This is optional and defaults to NoOpLogger.
func (h *HtmlFetcher) SetDebugLogger(logger debug.DebugLogger) {
	h.debugLogger = logger
}

func (h *HtmlFetcher) Fetch(
	ctx context.Context,
	crawlDepth int,
	fetchUrl url.URL,
	retryParam retry.RetryParam,
) (FetchResult, failure.ClassifiedError) {
	callerMethod := "HtmlFetcher.Fetch"
	startTime := time.Now()

	retryResult := h.fetchWithRetry(ctx, fetchUrl, h.userAgent, retryParam)
	result := retryResult.Value()
	err := retryResult.Err()

	duration := time.Since(startTime)

	// Record the fetch event with actual data
	var statusCode int
	var contentType string
	var retryCount int

	if err != nil {
		// Use the actual attempts count from the retry result
		retryCount = retryResult.Attempts()
	} else {
		statusCode = result.Code()
		contentType = h.extractContentType(result.Headers())
		retryCount = retryResult.Attempts()
	}

	h.metadataSink.RecordFetch(metadata.NewFetchEvent(
		startTime,
		fetchUrl.String(),
		statusCode,
		duration,
		contentType,
		retryCount,
		crawlDepth,
		metadata.KindPage,
	))

	if err != nil {
		// Use errors.Is to decide between FetchError or RetryError
		if errors.Is(err, &retry.RetryError{}) {
			// It's a RetryError
			h.recordRetryError(callerMethod, fetchUrl, err)
		} else {
			// It's a FetchError
			h.recordFetchError(callerMethod, fetchUrl, err)
		}

		return FetchResult{}, err
	}

	return result, nil
}

func (h *HtmlFetcher) extractContentType(headers map[string]string) string {
	if ct, ok := headers["Content-Type"]; ok {
		return ct
	}
	return ""
}

func (h *HtmlFetcher) recordFetchError(callerMethod string, fetchUrl url.URL, err failure.ClassifiedError) {
	var fetchError *FetchError
	if errors.As(err, &fetchError) {
		// record fetch error event
		h.metadataSink.RecordError(
			metadata.NewErrorRecord(
				time.Now(),
				"fetcher",
				callerMethod,
				mapFetchErrorToMetadataCause(fetchError),
				err.Error(),
				[]metadata.Attribute{
					metadata.NewAttr(metadata.AttrURL, fetchUrl.String()),
				},
			),
		)
	}
}

func (h *HtmlFetcher) recordRetryError(callerMethod string, fetchUrl url.URL, err failure.ClassifiedError) {
	var retryError *retry.RetryError
	if errors.As(err, &retryError) {
		// record retry error event
		h.metadataSink.RecordError(
			metadata.NewErrorRecord(
				time.Now(),
				"fetcher",
				callerMethod,
				metadata.CauseRetryFailure,
				err.Error(),
				[]metadata.Attribute{
					metadata.NewAttr(metadata.AttrMessage, retryError.Error()),
					metadata.NewAttr(metadata.AttrURL, fetchUrl.String()),
				},
			),
		)
	}
}

func (h *HtmlFetcher) fetchWithRetry(
	ctx context.Context,
	fetchUrl url.URL,
	userAgent string,
	retryParam retry.RetryParam,
) retry.Result[FetchResult] {
	fetchTask := func() (FetchResult, failure.ClassifiedError) {
		return h.performFetch(ctx, fetchUrl, userAgent)
	}

	return retry.Retry(retryParam, h.debugLogger, fetchTask)
}

func (h *HtmlFetcher) performFetch(
	ctx context.Context,
	fetchUrl url.URL,
	userAgent string,
) (FetchResult, failure.ClassifiedError) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchUrl.String(), nil)
	if err != nil {
		return FetchResult{}, NewFetchError(
			ErrCauseNetworkFailure,
			fmt.Sprintf("failed to create request: %v", err),
		)
	}

	// Apply browser-like headers
	headers := requestHeaders(userAgent)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		// Network/transport errors are retryable
		return FetchResult{}, NewFetchError(
			ErrCauseNetworkFailure,
			fmt.Sprintf("request failed: %v", err),
		)
	}
	defer resp.Body.Close()

	// Handle HTTP status codes
	switch {
	case resp.StatusCode >= 500:
		// Server errors (5xx) are retryable
		return FetchResult{}, NewFetchError(
			ErrCauseRequest5xx,
			fmt.Sprintf("server error: %d", resp.StatusCode),
		)

	case resp.StatusCode == 429:
		// Too Many Requests is retryable
		return FetchResult{}, NewFetchError(
			ErrCauseRequestTooMany,
			"rate limited (429)",
		)

	case resp.StatusCode == 403:
		// Forbidden is not retryable
		return FetchResult{}, NewFetchError(
			ErrCauseRequestPageForbidden,
			"access forbidden (403)",
		)

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// Other client errors are not retryable
		return FetchResult{}, NewFetchError(
			ErrCauseRequestPageForbidden,
			fmt.Sprintf("client error: %d", resp.StatusCode),
		)

	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		// Redirects should be handled by http.Client, but if we get here,
		// it means redirect limit exceeded
		return FetchResult{}, NewFetchError(
			ErrCauseRedirectLimitExceeded,
			fmt.Sprintf("redirect error: %d", resp.StatusCode),
		)
	}

	// Check Content-Type for HTML
	contentType := resp.Header.Get("Content-Type")
	if !isHTMLContent(contentType) {
		return FetchResult{}, NewFetchError(
			ErrCauseContentTypeInvalid,
			fmt.Sprintf("non-HTML content type: %s", contentType),
		)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, NewFetchError(
			ErrCauseReadResponseBodyError,
			fmt.Sprintf("failed to read response body: %v", err),
		)
	}

	// Build response headers map
	responseHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			responseHeaders[key] = values[0]
		}
	}

	// Create FetchResult with timestamp
	result := FetchResult{
		url:       fetchUrl,
		body:      body,
		fetchedAt: time.Now(),
		meta: ResponseMeta{
			statusCode:      resp.StatusCode,
			responseHeaders: responseHeaders,
		},
	}

	return result, nil
}

func isHTMLContent(contentType string) bool {
	// Check if content type is HTML
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "application/xhtml")
}

func requestHeaders(userAgent string) map[string]string {
	return map[string]string{
		"User-Agent":      userAgent,
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.5",
		"DNT":             "1",
		"Connection":      "keep-alive",
	}
}
