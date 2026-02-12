package robots_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/robots/cache"
)

// robotTestMetadataSink is a test double for metadata.MetadataSink
type robotTestMetadataSink struct {
	fetchEvents  []robotTestFetchEvent
	errorRecords []robotTestErrorRecord
	crawlStats   []robotTestCrawlStats
}

type robotTestFetchEvent struct {
	fetchURL    string
	httpStatus  int
	duration    time.Duration
	contentType string
	retryCount  int
	crawlDepth  int
}

type robotTestErrorRecord struct {
	packageName string
	action      string
	cause       int
	errorString string
	observedAt  time.Time
	attrs       []metadata.Attribute
}

type robotTestCrawlStats struct {
	totalPages  int
	totalErrors int
	totalAssets int
	durationMs  int64
}

func (m *robotTestMetadataSink) RecordFetch(
	fetchURL string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
	m.fetchEvents = append(m.fetchEvents, robotTestFetchEvent{
		fetchURL:    fetchURL,
		httpStatus:  httpStatus,
		duration:    duration,
		contentType: contentType,
		retryCount:  retryCount,
		crawlDepth:  crawlDepth,
	})
}

func (m *robotTestMetadataSink) RecordAssetFetch(
	fetchURL string,
	httpStatus int,
	duration time.Duration,
	retryCount int,
) {
	m.fetchEvents = append(m.fetchEvents, robotTestFetchEvent{
		fetchURL:   fetchURL,
		httpStatus: httpStatus,
		duration:   duration,
		retryCount: retryCount,
	})
}

func (m *robotTestMetadataSink) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	errorString string,
	attrs []metadata.Attribute,
) {
	m.errorRecords = append(m.errorRecords, robotTestErrorRecord{
		packageName: packageName,
		action:      action,
		cause:       int(cause),
		errorString: errorString,
		observedAt:  observedAt,
		attrs:       attrs,
	})
}

func (m *robotTestMetadataSink) RecordArtifact(kind metadata.ArtifactKind, path string, attrs []metadata.Attribute) {
	// No-op for testing
}

func (m *robotTestMetadataSink) RecordFinalCrawlStats(
	totalPages int,
	totalErrors int,
	totalAssets int,
	duration time.Duration,
) {
	m.crawlStats = append(m.crawlStats, robotTestCrawlStats{
		totalPages:  totalPages,
		totalErrors: totalErrors,
		totalAssets: totalAssets,
		durationMs:  duration.Milliseconds(),
	})
}

// setupTestServer creates a test HTTP server that serves robots.txt content
func setupTestServer(robotsContent string) *httptest.Server {
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
func setupTestServerWithStatus(statusCode int, robotsContent string) *httptest.Server {
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

func TestRobot_NewRobot(t *testing.T) {
	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	if robot == (robots.CachedRobot{}) {
		t.Error("NewRobot should return a non-empty Robot")
	}
}

func TestRobot_NewRobotWithCache(t *testing.T) {
	sink := &robotTestMetadataSink{}
	customCache := cache.NewMemoryCache()
	robot := robots.NewCachedRobot(sink)
	robot.InitWithCache("test-agent/1.0", customCache)

	if robot == (robots.CachedRobot{}) {
		t.Error("NewRobotWithCache should return a non-empty Robot")
	}
}

func TestRobot_Decide_AllowAll(t *testing.T) {
	// robots.txt that allows all crawling
	robotsContent := `User-agent: *
Allow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	serverURL, _ := url.Parse(server.URL + "/page.html")
	decision, err := robot.Decide(*serverURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !decision.Allowed {
		t.Error("Expected URL to be allowed")
	}

	if decision.Reason != robots.AllowedByRobots && decision.Reason != robots.EmptyRuleSet && decision.Reason != robots.NoMatchingRules {
		t.Errorf("Expected positive reason, got: %s", decision.Reason)
	}
}

func TestRobot_Decide_DisallowAll(t *testing.T) {
	// robots.txt that disallows all crawling
	robotsContent := `User-agent: *
Disallow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	serverURL, _ := url.Parse(server.URL + "/page.html")
	decision, err := robot.Decide(*serverURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if decision.Allowed {
		t.Error("Expected URL to be disallowed")
	}

	if decision.Reason != robots.DisallowedByRobots {
		t.Errorf("Expected reason DisallowedByRobots, got: %s", decision.Reason)
	}
}

func TestRobot_Decide_DisallowSpecificPath(t *testing.T) {
	// robots.txt that disallows a specific path
	robotsContent := `User-agent: *
Disallow: /private/`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	// Test disallowed path
	privateURL, _ := url.Parse(server.URL + "/private/page.html")
	decision, err := robot.Decide(*privateURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if decision.Allowed {
		t.Error("Expected /private/ URL to be disallowed")
	}

	// Test allowed path
	publicURL, _ := url.Parse(server.URL + "/public/page.html")
	decision, err = robot.Decide(*publicURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !decision.Allowed {
		t.Error("Expected /public/ URL to be allowed")
	}
}

func TestRobot_Decide_AllowOverridesDisallow(t *testing.T) {
	// robots.txt with allow overriding disallow for specific path
	robotsContent := `User-agent: *
Disallow: /docs/
Allow: /docs/public/`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	// Test that /docs/public/ is allowed despite /docs/ being disallowed
	publicDocsURL, _ := url.Parse(server.URL + "/docs/public/page.html")
	decision, err := robot.Decide(*publicDocsURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !decision.Allowed {
		t.Error("Expected /docs/public/ URL to be allowed (allow overrides disallow)")
	}

	// Test that /docs/private/ is still disallowed
	privateDocsURL, _ := url.Parse(server.URL + "/docs/private/page.html")
	decision, err = robot.Decide(*privateDocsURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if decision.Allowed {
		t.Error("Expected /docs/private/ URL to be disallowed")
	}
}

func TestRobot_Decide_UserAgentSpecific(t *testing.T) {
	// robots.txt with different rules for different user agents
	robotsContent := `User-agent: bad-bot
Disallow: /

User-agent: *
Allow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	// Test with good bot (should be allowed)
	sink := &robotTestMetadataSink{}
	goodBot := robots.NewCachedRobot(sink)
	goodBot.Init("good-bot/1.0")

	serverURL, _ := url.Parse(server.URL + "/page.html")
	decision, err := goodBot.Decide(*serverURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !decision.Allowed {
		t.Error("Expected good-bot to be allowed")
	}

	// Test with bad bot (should be disallowed)
	sink2 := &robotTestMetadataSink{}
	badBot := robots.NewCachedRobot(sink2)
	badBot.InitWithCache("bad-bot/1.0", cache.NewMemoryCache())

	decision, err = badBot.Decide(*serverURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if decision.Allowed {
		t.Error("Expected bad-bot to be disallowed")
	}
}

func TestRobot_Decide_WildcardPatterns(t *testing.T) {
	// robots.txt with wildcard patterns
	robotsContent := `User-agent: *
Disallow: /*.pdf$`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	// Test PDF file (should be disallowed)
	pdfURL, _ := url.Parse(server.URL + "/document.pdf")
	decision, err := robot.Decide(*pdfURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if decision.Allowed {
		t.Error("Expected PDF URL to be disallowed")
	}

	// Test HTML file (should be allowed)
	htmlURL, _ := url.Parse(server.URL + "/page.html")
	decision, err = robot.Decide(*htmlURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !decision.Allowed {
		t.Error("Expected HTML URL to be allowed")
	}
}

func TestRobot_Decide_CrawlDelay(t *testing.T) {
	// robots.txt with crawl delay
	robotsContent := `User-agent: *
Crawl-delay: 5
Allow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	serverURL, _ := url.Parse(server.URL + "/page.html")
	decision, err := robot.Decide(*serverURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !decision.Allowed {
		t.Error("Expected URL to be allowed")
	}

	if decision.CrawlDelay == 0 {
		t.Error("Expected crawl delay to be set")
	} else if decision.CrawlDelay != 5*time.Second {
		t.Errorf("Expected crawl delay of 5s, got: %v", decision.CrawlDelay)
	}
}

func TestRobot_Decide_NoRobotsFile_404(t *testing.T) {
	// Server that returns 404 for robots.txt (should allow all)
	server := setupTestServerWithStatus(http.StatusNotFound, "")
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	serverURL, _ := url.Parse(server.URL + "/page.html")
	decision, err := robot.Decide(*serverURL)

	if err != nil {
		t.Errorf("Expected no error for 404 response, got: %v", err)
	}

	if !decision.Allowed {
		t.Error("Expected URL to be allowed when robots.txt returns 404")
	}

	if decision.Reason != robots.EmptyRuleSet {
		t.Errorf("Expected reason EmptyRuleSet, got: %s", decision.Reason)
	}
}

func TestRobot_Decide_Caching(t *testing.T) {
	// robots.txt that allows all
	robotsContent := `User-agent: *
Allow: /`

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			requestCount++
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(robotsContent))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	serverURL, _ := url.Parse(server.URL + "/page.html")

	// Make multiple decisions for the same host
	for i := 0; i < 3; i++ {
		_, err := robot.Decide(*serverURL)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	}

	// Due to caching, robots.txt should only be fetched once
	if requestCount != 1 {
		t.Errorf("Expected robots.txt to be fetched once due to caching, but was fetched %d times", requestCount)
	}
}

func TestRobot_Decide_MultipleURLs(t *testing.T) {
	// robots.txt with various rules
	robotsContent := `User-agent: *
Disallow: /admin/
Disallow: /api/
Allow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	testCases := []struct {
		path     string
		expected bool
	}{
		{"/", true},
		{"/page.html", true},
		{"/docs/guide.html", true},
		{"/admin/", false},
		{"/admin/users.html", false},
		{"/api/v1/data", false},
		{"/api/internal", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			testURL, _ := url.Parse(server.URL + tc.path)
			decision, err := robot.Decide(*testURL)

			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
				return
			}

			if decision.Allowed != tc.expected {
				t.Errorf("Expected Allowed=%v for path %s, got Allowed=%v", tc.expected, tc.path, decision.Allowed)
			}
		})
	}
}

func TestRobot_Decide_ExactMatchEndOfURL(t *testing.T) {
	// robots.txt with exact match patterns
	robotsContent := `User-agent: *
Allow: /$
Disallow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	// Root should be allowed (exact match with /$)
	rootURL, _ := url.Parse(server.URL + "/")
	decision, err := robot.Decide(*rootURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !decision.Allowed {
		t.Error("Expected root URL to be allowed due to exact match /$")
	}

	// Other paths should be disallowed
	otherURL, _ := url.Parse(server.URL + "/page.html")
	decision, err = robot.Decide(*otherURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if decision.Allowed {
		t.Error("Expected non-root URL to be disallowed")
	}
}

func TestRobot_Decide_DecisionURLField(t *testing.T) {
	robotsContent := `User-agent: *
Allow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	testURL, _ := url.Parse(server.URL + "/test/page.html")
	decision, err := robot.Decide(*testURL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify the URL in the decision matches the input
	if decision.Url.String() != testURL.String() {
		t.Errorf("Expected decision URL to match input URL, got: %s", decision.Url.String())
	}
}

func TestRobot_Decide_ServerError(t *testing.T) {
	// Server that returns 500 for robots.txt
	server := setupTestServerWithStatus(http.StatusInternalServerError, "")
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	robot.Init("test-agent/1.0")

	serverURL, _ := url.Parse(server.URL + "/page.html")
	_, err := robot.Decide(*serverURL)

	// Server errors should return an error
	if err == nil {
		t.Error("Expected error for 500 response, got nil")
	}

	// Verify error was recorded
	if len(sink.errorRecords) == 0 {
		t.Error("Expected error to be recorded in metadata sink")
	}
}
