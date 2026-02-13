package scheduler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// createSchedulerForTest creates a scheduler with test-specific initialization
// that allows testing scheduler in isolation.
// If mockExtractor is nil, a real extractor will be created.
// If mockSanitizer is nil, a real sanitizer will be created.
// If mockConvert is nil, a convert mock with default success will be created.
// If mockNormalize is nil, a normalize mock with default success will be created.
func createSchedulerForTest(
	t *testing.T,
	ctx context.Context,
	mockFinalizer *mockFinalizer,
	metadataSink metadata.MetadataSink,
	mockLimiter *rateLimiterMock,
	mockRobot *robotsMock,
	mockFetcher *fetcherMock,
	mockExtractor extractor.Extractor,
	mockSanitizer sanitizer.Sanitizer,
	mockConvert mdconvert.ConvertRule,
	mockNormalize *normalizeMock,
	mockSleeper timeutil.Sleeper,
) *scheduler.Scheduler {
	t.Helper()
	// Create a real extractor if none provided
	if mockExtractor == nil {
		ext := extractor.NewDomExtractor(metadataSink)
		mockExtractor = &ext
	}
	// Create a real sanitizer if none provided
	if mockSanitizer == nil {
		san := sanitizer.NewHTMLSanitizer(metadataSink)
		mockSanitizer = &san
	}
	// Create a convert mock with default success if none provided
	if mockConvert == nil {
		mockConvert = newConvertMockForTest(t)
		setupConvertMockWithSuccess(mockConvert.(*convertMock))
	}
	// Create a resolver mock with default success
	resolverMock := newResolverMockForTest(t)
	setupResolverMockWithSuccess(resolverMock)
	// Create a normalize mock with default success if none provided
	if mockNormalize == nil {
		mockNormalize = newNormalizeMockForTest(t)
		setupNormalizeMockWithSuccess(mockNormalize)
	}

	s := scheduler.NewSchedulerWithDeps(
		ctx,
		mockFinalizer,
		metadataSink,
		mockLimiter,
		mockFetcher,
		mockRobot,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		resolverMock,
		mockNormalize, // Pass the mock pointer which implements normalize.Constraint
		mockSleeper,
	)
	return &s
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

// Helper function to create a robots decision with crawl delay
func newRobotsDecision(allowed bool, crawlDelay time.Duration) robots.Decision {
	return robots.Decision{
		Allowed:    allowed,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: crawlDelay,
	}
}
