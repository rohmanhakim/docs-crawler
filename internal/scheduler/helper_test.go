package scheduler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
)

// createSchedulerForTest creates a scheduler with test-specific initialization
// that allows testing scheduler in isolation
func createSchedulerForTest(
	t *testing.T,
	ctx context.Context,
	mockFinalizer *mockFinalizer,
	metadataSink metadata.MetadataSink,
	mockLimiter *rateLimiterMock,
	mockRobot *robotsMock,
	mockFetcher *fetcherMock,
) *scheduler.Scheduler {
	t.Helper()
	s := scheduler.NewSchedulerWithDeps(ctx, mockFinalizer, metadataSink, mockLimiter, mockFetcher, mockRobot)
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
