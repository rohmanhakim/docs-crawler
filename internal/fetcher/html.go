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
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
)

/*
Responsibilities

- Perform HTTP requests
- Apply headers and timeouts
- Handle redirects safely
- Classify responses

Fetch Semantics

- Only successful HTML responses are processed
- Non-HTML content is discarded
- Redirect chains are bounded
- All responses are logged with metadata

The fetcher never parses content; it only returns bytes and metadata.
*/

type HtmlFetcher struct {
	metadataSink metadata.MetadataSink
	httpClient   *http.Client
}

func NewHtmlFetcher(
	metadataSink metadata.MetadataSink,
) HtmlFetcher {
	return HtmlFetcher{
		metadataSink: metadataSink,
		httpClient:   &http.Client{},
	}
}

func (h *HtmlFetcher) Fetch(
	ctx context.Context,
	crawlDepth int,
	fetchParam FetchParam,
	retryParam retry.RetryParam,
) (FetchResult, failure.ClassifiedError) {
	callerMethod := "HtmlFetcher.Fetch"
	startTime := time.Now()

	result, err := h.fetchWithRetry(ctx, fetchParam.fetchUrl, fetchParam.userAgent, retryParam)

	duration := time.Since(startTime)

	// Record the fetch event with actual data
	var statusCode int
	var contentType string
	var retryCount int

	if err != nil {
		// Extract retry count from error if it's a RetryError
		var retryErr *retry.RetryError
		if errors.As(err, &retryErr) {
			retryCount = retryParam.MaxAttempts
		}
	} else {
		statusCode = result.Code()
		contentType = h.extractContentType(result.Headers())
	}

	h.metadataSink.RecordFetch(
		fetchParam.fetchUrl.String(),
		statusCode,
		duration,
		contentType,
		retryCount,
		crawlDepth,
	)

	if err != nil {
		// Use errors.Is to decide between FetchError or RetryError
		if errors.Is(err, &retry.RetryError{}) {
			// It's a RetryError
			h.recordRetryError(callerMethod, fetchParam.fetchUrl, err)
		} else {
			// It's a FetchError
			h.recordFetchError(callerMethod, fetchParam.fetchUrl, err)
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
			time.Now(),
			"fetcher",
			callerMethod,
			mapFetchErrorToMetadataCause(fetchError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, fetchUrl.String()),
			},
		)
	}
}

func (h *HtmlFetcher) recordRetryError(callerMethod string, fetchUrl url.URL, err failure.ClassifiedError) {
	var retryError *retry.RetryError
	if errors.As(err, &retryError) {
		// record retry error event
		h.metadataSink.RecordError(
			time.Now(),
			"fetcher",
			callerMethod,
			metadata.CauseRetryFailure,
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrMessage, retryError.Error()),
				metadata.NewAttr(metadata.AttrURL, fetchUrl.String()),
			},
		)
	}
}

func (h *HtmlFetcher) fetchWithRetry(ctx context.Context, fetchUrl url.URL, userAgent string, retryParam retry.RetryParam) (FetchResult, failure.ClassifiedError) {
	fetchTask := func() (FetchResult, failure.ClassifiedError) {
		return h.performFetch(ctx, fetchUrl, userAgent)
	}

	result, retryErr := retry.Retry(retryParam, fetchTask)

	if retryErr != nil {
		// Handle error - decide what to return based on error type
		// Check if it's a FetchError (returned by the task) or RetryError (from retry.Retry)
		var fetchErr *FetchError
		if errors.As(retryErr, &fetchErr) {
			// The underlying error is a FetchError, return it directly
			return FetchResult{}, fetchErr
		}

		// It's a RetryError, return it as-is
		return FetchResult{}, retryErr
	}

	return result, nil
}

func (h *HtmlFetcher) performFetch(ctx context.Context, fetchUrl url.URL, userAgent string) (FetchResult, failure.ClassifiedError) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchUrl.String(), nil)
	if err != nil {
		return FetchResult{}, &FetchError{
			Message:   fmt.Sprintf("failed to create request: %v", err),
			Retryable: false,
			Cause:     ErrCauseNetworkFailure,
		}
	}

	// Apply browser-like headers
	headers := requestHeaders(userAgent)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		// Network/transport errors are retryable
		return FetchResult{}, &FetchError{
			Message:   fmt.Sprintf("request failed: %v", err),
			Retryable: true,
			Cause:     ErrCauseNetworkFailure,
		}
	}
	defer resp.Body.Close()

	// Handle HTTP status codes
	switch {
	case resp.StatusCode >= 500:
		// Server errors (5xx) are retryable
		return FetchResult{}, &FetchError{
			Message:   fmt.Sprintf("server error: %d", resp.StatusCode),
			Retryable: true,
			Cause:     ErrCauseRequest5xx,
		}

	case resp.StatusCode == 429:
		// Too Many Requests is retryable
		return FetchResult{}, &FetchError{
			Message:   "rate limited (429)",
			Retryable: true,
			Cause:     ErrCauseRequestTooMany,
		}

	case resp.StatusCode == 403:
		// Forbidden is not retryable
		return FetchResult{}, &FetchError{
			Message:   "access forbidden (403)",
			Retryable: false,
			Cause:     ErrCauseRequestPageForbidden,
		}

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// Other client errors are not retryable
		return FetchResult{}, &FetchError{
			Message:   fmt.Sprintf("client error: %d", resp.StatusCode),
			Retryable: false,
			Cause:     ErrCauseRequestPageForbidden,
		}

	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		// Redirects should be handled by http.Client, but if we get here,
		// it means redirect limit exceeded
		return FetchResult{}, &FetchError{
			Message:   fmt.Sprintf("redirect error: %d", resp.StatusCode),
			Retryable: false,
			Cause:     ErrCauseRedirectLimitExceeded,
		}
	}

	// Check Content-Type for HTML
	contentType := resp.Header.Get("Content-Type")
	if !isHTMLContent(contentType) {
		return FetchResult{}, &FetchError{
			Message:   fmt.Sprintf("non-HTML content type: %s", contentType),
			Retryable: false,
			Cause:     ErrCauseContentTypeInvalid,
		}
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, &FetchError{
			Message:   fmt.Sprintf("failed to read response body: %v", err),
			Retryable: true,
			Cause:     ErrCauseReadResponseBodyError,
		}
	}

	// Build response headers map
	responseHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			responseHeaders[key] = values[0]
		}
	}

	// Create FetchResult
	result := FetchResult{
		url:  fetchUrl,
		body: body,
		meta: ResponseMeta{
			statusCode:          resp.StatusCode,
			transferredSizeByte: uint64(len(body)),
			responseHeaders:     responseHeaders,
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
		"Accept-Encoding": "gzip, deflate, br",
		"DNT":             "1",
		"Connection":      "keep-alive",
	}
}
