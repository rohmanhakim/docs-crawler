package robots_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/metadata/metadatatest"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/robots/cache"
	"github.com/rohmanhakim/docs-crawler/pkg/debug/debugtest"
)

// robotTestMetadataSink is an alias to the shared mock in metadatatest package.
type robotTestMetadataSink = metadatatest.SinkMock

var _ metadata.MetadataSink = (*robotTestMetadataSink)(nil)

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
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

	if robot == (robots.CachedRobot{}) {
		t.Error("NewRobot should return a non-empty Robot")
	}
}

func TestRobot_NewRobotWithCache(t *testing.T) {
	sink := &robotTestMetadataSink{}
	customCache := cache.NewMemoryCache()
	robot := robots.NewCachedRobot(sink)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.InitWithCache("test-agent/1.0", httpClient, customCache)

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
	mockLogger := debugtest.NewLoggerMock()
	robot := robots.NewCachedRobot(sink)
	robot.SetDebugLogger(mockLogger)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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

	// Assert debug logging was called
	if !mockLogger.LogStepCalled {
		t.Error("Expected LogStep to be called")
	}

	// Verify robots_fetch step
	fetchSteps := mockLogger.StepsByName("robots_fetch")
	if len(fetchSteps) != 1 {
		t.Errorf("Expected 1 robots_fetch step, got %d", len(fetchSteps))
	} else {
		if fetchSteps[0].Fields["host"] == nil {
			t.Error("Expected host field in robots_fetch step")
		}
		if fetchSteps[0].Fields["from_cache"] != false {
			t.Error("Expected from_cache=false on first fetch")
		}
	}

	// Verify parse_rules step
	parseSteps := mockLogger.StepsByName("parse_rules")
	if len(parseSteps) != 1 {
		t.Errorf("Expected 1 parse_rules step, got %d", len(parseSteps))
	}

	// Verify decision_made step
	decisionSteps := mockLogger.StepsByName("decision_made")
	if len(decisionSteps) != 1 {
		t.Errorf("Expected 1 decision_made step, got %d", len(decisionSteps))
	} else {
		if decisionSteps[0].Fields["allowed"] != true {
			t.Error("Expected allowed=true in decision_made step")
		}
	}
}

func TestRobot_Decide_DisallowAll(t *testing.T) {
	// robots.txt that disallows all crawling
	robotsContent := `User-agent: *
Disallow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	mockLogger := debugtest.NewLoggerMock()
	robot := robots.NewCachedRobot(sink)
	robot.SetDebugLogger(mockLogger)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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

	// Verify decision_made step shows disallowed
	decisionSteps := mockLogger.StepsByName("decision_made")
	if len(decisionSteps) != 1 {
		t.Errorf("Expected 1 decision_made step, got %d", len(decisionSteps))
	} else {
		if decisionSteps[0].Fields["allowed"] != false {
			t.Error("Expected allowed=false in decision_made step")
		}
		if decisionSteps[0].Fields["reason"] != string(robots.DisallowedByRobots) {
			t.Errorf("Expected reason=%s in decision_made step, got %v", robots.DisallowedByRobots, decisionSteps[0].Fields["reason"])
		}
	}

	// Verify match_rules step was logged
	matchSteps := mockLogger.StepsByName("match_rules")
	if len(matchSteps) == 0 {
		t.Error("Expected at least one match_rules step for disallow rule")
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
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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
	httpClient := &http.Client{Timeout: 30 * time.Second}
	goodBot.Init("good-bot/1.0", httpClient)

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
	httpClient2 := &http.Client{Timeout: 30 * time.Second}
	badBot.InitWithCache("bad-bot/1.0", httpClient2, cache.NewMemoryCache())

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
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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
	mockLogger := debugtest.NewLoggerMock()
	robot := robots.NewCachedRobot(sink)
	robot.SetDebugLogger(mockLogger)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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

	// Verify parse_rules step contains crawl_delay_ms
	parseSteps := mockLogger.StepsByName("parse_rules")
	if len(parseSteps) != 1 {
		t.Errorf("Expected 1 parse_rules step, got %d", len(parseSteps))
	} else {
		if parseSteps[0].Fields["crawl_delay_ms"] != int64(5000) {
			t.Errorf("Expected crawl_delay_ms=5000, got %v", parseSteps[0].Fields["crawl_delay_ms"])
		}
	}

	// Verify decision_made step contains crawl_delay_ms
	decisionSteps := mockLogger.StepsByName("decision_made")
	if len(decisionSteps) != 1 {
		t.Errorf("Expected 1 decision_made step, got %d", len(decisionSteps))
	} else {
		if decisionSteps[0].Fields["crawl_delay_ms"] != int64(5000) {
			t.Errorf("Expected crawl_delay_ms=5000 in decision_made, got %v", decisionSteps[0].Fields["crawl_delay_ms"])
		}
	}
}

func TestRobot_Decide_NoRobotsFile_404(t *testing.T) {
	// Server that returns 404 for robots.txt (should allow all)
	server := setupTestServerWithStatus(http.StatusNotFound, "")
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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
	mockLogger := debugtest.NewLoggerMock()
	robot := robots.NewCachedRobot(sink)
	robot.SetDebugLogger(mockLogger)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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

	// Verify robots_fetch shows from_cache correctly
	fetchSteps := mockLogger.StepsByName("robots_fetch")
	if len(fetchSteps) != 3 {
		t.Errorf("Expected 3 robots_fetch steps (one per Decide call), got %d", len(fetchSteps))
	} else {
		// First fetch should be from_cache=false
		if fetchSteps[0].Fields["from_cache"] != false {
			t.Error("Expected first fetch to have from_cache=false")
		}
		// Second and third fetches should be from_cache=true
		if fetchSteps[1].Fields["from_cache"] != true {
			t.Error("Expected second fetch to have from_cache=true")
		}
		if fetchSteps[2].Fields["from_cache"] != true {
			t.Error("Expected third fetch to have from_cache=true")
		}
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
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

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

// TestRobot_Decide_EmitsFetchEventWithKindRobots verifies that a successful
// robots.txt fetch produces exactly one FetchEvent with Kind == KindRobots
// and a non-zero FetchedAt timestamp.
func TestRobot_Decide_EmitsFetchEventWithKindRobots(t *testing.T) {
	robotsContent := `User-agent: *
Allow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

	serverURL, _ := url.Parse(server.URL + "/page.html")
	_, err := robot.Decide(*serverURL)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(sink.FetchEvents) != 1 {
		t.Fatalf("Expected exactly 1 FetchEvent, got %d", len(sink.FetchEvents))
	}

	event := sink.FetchEvents[0]

	if event.Kind() != metadata.KindRobots {
		t.Errorf("Expected FetchEvent.Kind == KindRobots, got %q", event.Kind())
	}

	if event.FetchedAt().IsZero() {
		t.Error("Expected FetchEvent.FetchedAt to be non-zero")
	}
}

func TestRobot_Decide_ServerError(t *testing.T) {
	// Server that returns 500 for robots.txt
	server := setupTestServerWithStatus(http.StatusInternalServerError, "")
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

	serverURL, _ := url.Parse(server.URL + "/page.html")
	_, err := robot.Decide(*serverURL)

	// Server errors should return an error
	if err == nil {
		t.Error("Expected error for 500 response, got nil")
	}

	// Verify error was recorded
	if len(sink.ErrorRecords) == 0 {
		t.Error("Expected error to be recorded in metadata sink")
	}
}

// TestRobot_Decide_RecordsFetchOnlyOncePerHost verifies that RecordFetch
// is called exactly once per host, not on cache hits. This prevents
// inflating fetch metrics when multiple URLs share the same host.
func TestRobot_Decide_RecordsFetchOnlyOncePerHost(t *testing.T) {
	robotsContent := `User-agent: *
Allow: /`

	server := setupTestServer(robotsContent)
	defer server.Close()

	sink := &robotTestMetadataSink{}
	robot := robots.NewCachedRobot(sink)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	robot.Init("test-agent/1.0", httpClient)

	// First Decide — should record fetch
	url1, _ := url.Parse(server.URL + "/page1.html")
	_, err := robot.Decide(*url1)
	if err != nil {
		t.Fatalf("First Decide failed: %v", err)
	}

	if len(sink.FetchEvents) != 1 {
		t.Fatalf("Expected 1 FetchEvent after first Decide, got %d", len(sink.FetchEvents))
	}

	// Second Decide for same host: should NOT record fetch (cache hit)
	url2, _ := url.Parse(server.URL + "/page2.html")
	_, err = robot.Decide(*url2)
	if err != nil {
		t.Fatalf("Second Decide failed: %v", err)
	}

	if len(sink.FetchEvents) != 1 {
		t.Errorf("Expected still 1 FetchEvent after second Decide (cache hit), got %d", len(sink.FetchEvents))
	}

	// Third Decide for same host: should still not record fetch
	url3, _ := url.Parse(server.URL + "/page3.html")
	_, err = robot.Decide(*url3)
	if err != nil {
		t.Fatalf("Third Decide failed: %v", err)
	}

	if len(sink.FetchEvents) != 1 {
		t.Errorf("Expected still 1 FetchEvent after third Decide (cache hit), got %d", len(sink.FetchEvents))
	}
}
