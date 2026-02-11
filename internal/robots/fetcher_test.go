package robots_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// mockMetadataSink is a test implementation of metadata.MetadataSink
type mockMetadataSink struct{}

func (m *mockMetadataSink) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	details string,
	attrs []metadata.Attribute,
) {
}

func (m *mockMetadataSink) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
}

func (m *mockMetadataSink) RecordArtifact(kind metadata.ArtifactKind, path string, attrs []metadata.Attribute) {
}
func (m *mockMetadataSink) RecordAssetFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	retryCount int,
) {
}

func TestNewRobotsFetcher(t *testing.T) {
	sink := &mockMetadataSink{}
	userAgent := "TestBot/1.0"

	fetcher := robots.NewRobotsFetcher(sink, userAgent, nil)

	if fetcher == nil {
		t.Fatal("NewRobotsFetcher returned nil")
	}

	if fetcher.UserAgent() != userAgent {
		t.Errorf("expected userAgent %q, got %q", userAgent, fetcher.UserAgent())
	}

	if fetcher.HttpClient() == nil {
		t.Error("httpClient not initialized")
	}
}

func TestRobotsFetcher_Fetch_Success(t *testing.T) {
	robotsContent := `User-agent: *
Disallow: /private/
Disallow: /admin/
Allow: /public/
Crawl-delay: 5

User-agent: Googlebot
Disallow: /no-google/

Sitemap: https://example.com/sitemap.xml
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			t.Errorf("expected path /robots.txt, got %s", r.URL.Path)
		}

		// Verify headers
		if r.Header.Get("User-Agent") != "TestBot/1.0" {
			t.Errorf("expected User-Agent header TestBot/1.0, got %s", r.Header.Get("User-Agent"))
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(robotsContent))
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	fetcher := robots.NewRobotsFetcher(sink, "TestBot/1.0", nil)

	// Extract host from server URL
	serverURL := server.URL
	parts := strings.Split(serverURL, "://")
	scheme := parts[0]
	host := parts[1]

	ctx := context.Background()
	result, err := fetcher.Fetch(ctx, scheme, host)

	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if result.HTTPStatus != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.HTTPStatus)
	}

	if result.SourceURL != fmt.Sprintf("%s/robots.txt", serverURL) {
		t.Errorf("unexpected source URL: %s", result.SourceURL)
	}

	// Check parsed response
	response := result.Response

	if response.Host != host {
		t.Errorf("expected host %q, got %q", host, response.Host)
	}

	if len(response.Sitemaps) != 1 || response.Sitemaps[0] != "https://example.com/sitemap.xml" {
		t.Errorf("unexpected sitemaps: %v", response.Sitemaps)
	}

	if len(response.UserAgents) != 2 {
		t.Errorf("expected 2 user agent groups, got %d", len(response.UserAgents))
	}

	// Check first group (*)
	group1 := response.UserAgents[0]
	if len(group1.UserAgents) != 1 || group1.UserAgents[0] != "*" {
		t.Errorf("unexpected first group user agents: %v", group1.UserAgents)
	}

	if len(group1.Disallows) != 2 {
		t.Errorf("expected 2 disallow rules, got %d", len(group1.Disallows))
	}

	if len(group1.Allows) != 1 {
		t.Errorf("expected 1 allow rule, got %d", len(group1.Allows))
	}

	if group1.CrawlDelay == nil {
		t.Error("expected crawl delay to be set")
	} else if *group1.CrawlDelay != 5*time.Second {
		t.Errorf("expected crawl delay 5s, got %v", *group1.CrawlDelay)
	}

	// Check second group (Googlebot)
	group2 := response.UserAgents[1]
	if len(group2.UserAgents) != 1 || group2.UserAgents[0] != "Googlebot" {
		t.Errorf("unexpected second group user agents: %v", group2.UserAgents)
	}
}

func TestRobotsFetcher_Fetch_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	fetcher := robots.NewRobotsFetcher(sink, "TestBot/1.0", nil)

	serverURL := server.URL
	parts := strings.Split(serverURL, "://")
	scheme := parts[0]
	host := parts[1]

	ctx := context.Background()
	result, err := fetcher.Fetch(ctx, scheme, host)

	if err != nil {
		t.Fatalf("Fetch returned error for 404: %v", err)
	}

	if result.HTTPStatus != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", result.HTTPStatus)
	}

	// For 404, we should get an empty response (no restrictions)
	if !result.Response.IsEmpty() {
		t.Error("expected empty response for 404")
	}
}

func TestRobotsFetcher_Fetch_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	fetcher := robots.NewRobotsFetcher(sink, "TestBot/1.0", nil)

	serverURL := server.URL
	parts := strings.Split(serverURL, "://")
	scheme := parts[0]
	host := parts[1]

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx, scheme, host)

	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}

	if !err.Retryable {
		t.Error("expected 500 error to be retryable")
	}

	if err.Cause != robots.ErrCauseHttpServerError {
		t.Errorf("expected cause %q, got %q", robots.ErrCauseHttpServerError, err.Cause)
	}
}

func TestRobotsFetcher_Fetch_TooManyRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	fetcher := robots.NewRobotsFetcher(sink, "TestBot/1.0", nil)

	serverURL := server.URL
	parts := strings.Split(serverURL, "://")
	scheme := parts[0]
	host := parts[1]

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx, scheme, host)

	if err == nil {
		t.Fatal("expected error for 429 response, got nil")
	}

	if !err.Retryable {
		t.Error("expected 429 error to be retryable")
	}
}

func TestRobotsFetcher_Fetch_LargeFile(t *testing.T) {
	// Create content larger than 500 KiB
	largeContent := strings.Repeat("User-agent: *\nDisallow: /test/\n", 10000)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeContent))
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	fetcher := robots.NewRobotsFetcher(sink, "TestBot/1.0", nil)

	serverURL := server.URL
	parts := strings.Split(serverURL, "://")
	scheme := parts[0]
	host := parts[1]

	ctx := context.Background()
	result, err := fetcher.Fetch(ctx, scheme, host)

	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if result.HTTPStatus != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.HTTPStatus)
	}

	// Should have parsed content (trimmed to 500 KiB)
	if result.Response.IsEmpty() {
		t.Error("expected some rules to be parsed")
	}
}

func TestRobotsFetcher_Fetch_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	fetcher := robots.NewRobotsFetcher(sink, "TestBot/1.0", nil)

	serverURL := server.URL
	parts := strings.Split(serverURL, "://")
	scheme := parts[0]
	host := parts[1]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := fetcher.Fetch(ctx, scheme, host)

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestParseRobotsTxt(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		host     string
		expected robots.RobotsResponse
	}{
		{
			name:    "empty content",
			content: "",
			host:    "example.com",
			expected: robots.RobotsResponse{
				Host:       "example.com",
				Sitemaps:   []string{},
				UserAgents: []robots.UserAgentGroup{},
			},
		},
		{
			name: "simple disallow all",
			content: `User-agent: *
Disallow: /`,
			host: "example.com",
			expected: robots.RobotsResponse{
				Host:     "example.com",
				Sitemaps: []string{},
				UserAgents: []robots.UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Disallows:  []robots.PathRule{{Path: "/"}},
					},
				},
			},
		},
		{
			name: "multiple user agents",
			content: `User-agent: Googlebot
Disallow: /no-google/

User-agent: Bingbot
Disallow: /no-bing/`,
			host: "example.com",
			expected: robots.RobotsResponse{
				Host:     "example.com",
				Sitemaps: []string{},
				UserAgents: []robots.UserAgentGroup{
					{
						UserAgents: []string{"Googlebot"},
						Disallows:  []robots.PathRule{{Path: "/no-google/"}},
					},
					{
						UserAgents: []string{"Bingbot"},
						Disallows:  []robots.PathRule{{Path: "/no-bing/"}},
					},
				},
			},
		},
		{
			name: "with sitemap",
			content: `User-agent: *
Disallow: /private/

Sitemap: https://example.com/sitemap.xml
Sitemap: https://example.com/sitemap2.xml`,
			host: "example.com",
			expected: robots.RobotsResponse{
				Host:     "example.com",
				Sitemaps: []string{"https://example.com/sitemap.xml", "https://example.com/sitemap2.xml"},
				UserAgents: []robots.UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Disallows:  []robots.PathRule{{Path: "/private/"}},
					},
				},
			},
		},
		{
			name: "with comments",
			content: `# This is a comment
User-agent: * # inline comment
Disallow: /private/ # another comment
# Disallow: /ignored/`,
			host: "example.com",
			expected: robots.RobotsResponse{
				Host:     "example.com",
				Sitemaps: []string{},
				UserAgents: []robots.UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Disallows:  []robots.PathRule{{Path: "/private/"}},
					},
				},
			},
		},
		{
			name: "case insensitive fields",
			content: `USER-AGENT: *
DISALLOW: /private/
ALLOW: /public/`,
			host: "example.com",
			expected: robots.RobotsResponse{
				Host:     "example.com",
				Sitemaps: []string{},
				UserAgents: []robots.UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Disallows:  []robots.PathRule{{Path: "/private/"}},
						Allows:     []robots.PathRule{{Path: "/public/"}},
					},
				},
			},
		},
		{
			name: "crawl delay",
			content: `User-agent: *
Crawl-delay: 10
Disallow: /`,
			host: "example.com",
			expected: robots.RobotsResponse{
				Host:     "example.com",
				Sitemaps: []string{},
				UserAgents: []robots.UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Disallows:  []robots.PathRule{{Path: "/"}},
						CrawlDelay: timeutil.DurationPtr(10 * time.Second),
					},
				},
			},
		},
		{
			name: "multiple user-agents in one group",
			content: `User-agent: Googlebot
User-agent: Bingbot
Disallow: /shared/`,
			host: "example.com",
			expected: robots.RobotsResponse{
				Host:     "example.com",
				Sitemaps: []string{},
				UserAgents: []robots.UserAgentGroup{
					{
						UserAgents: []string{"Googlebot", "Bingbot"},
						Disallows:  []robots.PathRule{{Path: "/shared/"}},
					},
				},
			},
		},
		{
			name: "rules without user-agent (global)",
			content: `Disallow: /global-private/

User-agent: *
Allow: /public/`,
			host: "example.com",
			expected: robots.RobotsResponse{
				Host:     "example.com",
				Sitemaps: []string{},
				UserAgents: []robots.UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Disallows:  []robots.PathRule{{Path: "/global-private/"}},
					},
					{
						UserAgents: []string{"*"},
						Allows:     []robots.PathRule{{Path: "/public/"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := robots.ParseRobotsTxt(tt.content, tt.host)

			if result.Host != tt.expected.Host {
				t.Errorf("expected host %q, got %q", tt.expected.Host, result.Host)
			}

			if len(result.Sitemaps) != len(tt.expected.Sitemaps) {
				t.Errorf("expected %d sitemaps, got %d", len(tt.expected.Sitemaps), len(result.Sitemaps))
			}

			if len(result.UserAgents) != len(tt.expected.UserAgents) {
				t.Errorf("expected %d user agent groups, got %d", len(tt.expected.UserAgents), len(result.UserAgents))
			}

			// Verify sitemaps
			for i, expectedSitemap := range tt.expected.Sitemaps {
				if i >= len(result.Sitemaps) || result.Sitemaps[i] != expectedSitemap {
					t.Errorf("expected sitemap %q at index %d, got %v", expectedSitemap, i, result.Sitemaps)
				}
			}

			// Verify user agents
			for i, expectedGroup := range tt.expected.UserAgents {
				if i >= len(result.UserAgents) {
					break
				}

				actualGroup := result.UserAgents[i]

				if len(actualGroup.UserAgents) != len(expectedGroup.UserAgents) {
					t.Errorf("group %d: expected %d user agents, got %d", i, len(expectedGroup.UserAgents), len(actualGroup.UserAgents))
				}

				for j, expectedUA := range expectedGroup.UserAgents {
					if j >= len(actualGroup.UserAgents) || actualGroup.UserAgents[j] != expectedUA {
						t.Errorf("group %d: expected user agent %q at index %d, got %v", i, expectedUA, j, actualGroup.UserAgents)
					}
				}

				if len(actualGroup.Disallows) != len(expectedGroup.Disallows) {
					t.Errorf("group %d: expected %d disallow rules, got %d", i, len(expectedGroup.Disallows), len(actualGroup.Disallows))
				}

				if len(actualGroup.Allows) != len(expectedGroup.Allows) {
					t.Errorf("group %d: expected %d allow rules, got %d", i, len(expectedGroup.Allows), len(actualGroup.Allows))
				}
			}

			// Check crawl delay for first group if expected
			if len(tt.expected.UserAgents) > 0 && len(result.UserAgents) > 0 {
				expectedDelay := tt.expected.UserAgents[0].CrawlDelay
				actualDelay := result.UserAgents[0].CrawlDelay

				if expectedDelay == nil && actualDelay != nil {
					t.Errorf("expected no crawl delay, got %v", *actualDelay)
				} else if expectedDelay != nil && actualDelay == nil {
					t.Errorf("expected crawl delay %v, got nil", *expectedDelay)
				} else if expectedDelay != nil && actualDelay != nil && *expectedDelay != *actualDelay {
					t.Errorf("expected crawl delay %v, got %v", *expectedDelay, *actualDelay)
				}
			}
		})
	}
}

func TestRobotsResponse_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		response robots.RobotsResponse
		expected bool
	}{
		{
			name:     "completely empty",
			response: robots.RobotsResponse{},
			expected: true,
		},
		{
			name: "has sitemaps",
			response: robots.RobotsResponse{
				Sitemaps: []string{"https://example.com/sitemap.xml"},
			},
			expected: false,
		},
		{
			name: "has disallow rules",
			response: robots.RobotsResponse{
				UserAgents: []robots.UserAgentGroup{
					{Disallows: []robots.PathRule{{Path: "/"}}},
				},
			},
			expected: false,
		},
		{
			name: "has allow rules",
			response: robots.RobotsResponse{
				UserAgents: []robots.UserAgentGroup{
					{Allows: []robots.PathRule{{Path: "/public/"}}},
				},
			},
			expected: false,
		},
		{
			name: "user agent but no rules",
			response: robots.RobotsResponse{
				UserAgents: []robots.UserAgentGroup{
					{UserAgents: []string{"*"}},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.response.IsEmpty()
			if result != tt.expected {
				t.Errorf("IsEmpty() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRobotsResponse_GetGroupForUserAgent(t *testing.T) {
	response := robots.RobotsResponse{
		UserAgents: []robots.UserAgentGroup{
			{
				UserAgents: []string{"Googlebot"},
				Disallows:  []robots.PathRule{{Path: "/no-google/"}},
			},
			{
				UserAgents: []string{"*"},
				Disallows:  []robots.PathRule{{Path: "/private/"}},
			},
			{
				UserAgents: []string{"Bingbot"},
				Disallows:  []robots.PathRule{{Path: "/no-bing/"}},
			},
		},
	}

	tests := []struct {
		userAgent string
		expected  *robots.UserAgentGroup
	}{
		{
			userAgent: "Googlebot",
			expected:  &response.UserAgents[0],
		},
		{
			userAgent: "Bingbot",
			expected:  &response.UserAgents[2],
		},
		{
			userAgent: "SomeOtherBot",
			expected:  &response.UserAgents[1], // Should match wildcard
		},
		{
			userAgent: "googlebot",
			expected:  &response.UserAgents[0], // Case-insensitive match
		},
	}

	for _, tt := range tests {
		t.Run(tt.userAgent, func(t *testing.T) {
			result := response.GetGroupForUserAgent(tt.userAgent)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("expected group, got nil")
			}

			if result.UserAgents[0] != tt.expected.UserAgents[0] {
				t.Errorf("expected user agent %q, got %q", tt.expected.UserAgents[0], result.UserAgents[0])
			}
		})
	}
}

func TestRobotsFetcher_Fetch_WithRedirects(t *testing.T) {
	redirectCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			if redirectCount < 2 {
				redirectCount++
				http.Redirect(w, r, "/robots.txt", http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "User-agent: *\nDisallow: /")
		}
	}))
	defer server.Close()

	sink := &mockMetadataSink{}
	fetcher := robots.NewRobotsFetcher(sink, "TestBot/1.0", nil)

	serverURL := server.URL
	parts := strings.Split(serverURL, "://")
	scheme := parts[0]
	host := parts[1]

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx, scheme, host)

	// http.Client follows redirects by default, but only up to a limit
	// This test verifies that redirects work
	if err != nil {
		t.Fatalf("Fetch should follow redirects: %v", err)
	}
}
