package robots

import (
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

func TestMapResponseToRuleSet(t *testing.T) {
	fetchTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name               string
		response           RobotsResponse
		targetUA           string
		expectedHost       string
		expectedUserUA     string
		expectedAllows     int
		expectedDisallows  int
		expectedCrawlDelay bool
	}{
		{
			name: "map wildcard group",
			response: RobotsResponse{
				Host: "example.com",
				UserAgents: []UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Allows:     []PathRule{{Path: "/public/"}},
						Disallows:  []PathRule{{Path: "/private/"}},
					},
				},
			},
			targetUA:           "TestBot/1.0",
			expectedHost:       "example.com",
			expectedUserUA:     "TestBot/1.0",
			expectedAllows:     1,
			expectedDisallows:  1,
			expectedCrawlDelay: false,
		},
		{
			name: "map specific user agent",
			response: RobotsResponse{
				Host: "example.com",
				UserAgents: []UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Disallows:  []PathRule{{Path: "/"}},
					},
					{
						UserAgents: []string{"TestBot"},
						Allows:     []PathRule{{Path: "/"}},
					},
				},
			},
			targetUA:           "TestBot",
			expectedHost:       "example.com",
			expectedUserUA:     "TestBot",
			expectedAllows:     1,
			expectedDisallows:  0,
			expectedCrawlDelay: false,
		},
		{
			name: "map with crawl delay",
			response: RobotsResponse{
				Host: "example.com",
				UserAgents: []UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Disallows:  []PathRule{{Path: "/admin/"}},
						CrawlDelay: timeutil.DurationPtr(5 * time.Second),
					},
				},
			},
			targetUA:           "AnyBot",
			expectedHost:       "example.com",
			expectedUserUA:     "AnyBot",
			expectedAllows:     0,
			expectedDisallows:  1,
			expectedCrawlDelay: true,
		},
		{
			name: "no matching group",
			response: RobotsResponse{
				Host:       "example.com",
				UserAgents: []UserAgentGroup{},
			},
			targetUA:           "TestBot",
			expectedHost:       "example.com",
			expectedUserUA:     "TestBot",
			expectedAllows:     0,
			expectedDisallows:  0,
			expectedCrawlDelay: false,
		},
		{
			name: "normalize paths without leading slash",
			response: RobotsResponse{
				Host: "example.com",
				UserAgents: []UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Allows:     []PathRule{{Path: "public/"}},
						Disallows:  []PathRule{{Path: "private/"}},
					},
				},
			},
			targetUA:           "TestBot",
			expectedHost:       "example.com",
			expectedUserUA:     "TestBot",
			expectedAllows:     1,
			expectedDisallows:  1,
			expectedCrawlDelay: false,
		},
		{
			name: "skip empty paths",
			response: RobotsResponse{
				Host: "example.com",
				UserAgents: []UserAgentGroup{
					{
						UserAgents: []string{"*"},
						Allows:     []PathRule{{Path: ""}, {Path: "/valid/"}},
						Disallows:  []PathRule{{Path: ""}},
					},
				},
			},
			targetUA:           "TestBot",
			expectedHost:       "example.com",
			expectedUserUA:     "TestBot",
			expectedAllows:     1, // Only "/valid/" is included
			expectedDisallows:  0, // Empty path is skipped
			expectedCrawlDelay: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := MapResponseToRuleSet(tt.response, tt.targetUA, fetchTime)

			if rs.Host() != tt.expectedHost {
				t.Errorf("expected host %q, got %q", tt.expectedHost, rs.Host())
			}

			if rs.UserAgent() != tt.expectedUserUA {
				t.Errorf("expected user agent %q, got %q", tt.expectedUserUA, rs.UserAgent())
			}

			if !rs.FetchedAt().Equal(fetchTime) {
				t.Errorf("expected fetched at %v, got %v", fetchTime, rs.FetchedAt())
			}

			expectedSourceURL := "https://" + tt.expectedHost + "/robots.txt"
			if rs.SourceURL() != expectedSourceURL {
				t.Errorf("expected source URL %q, got %q", expectedSourceURL, rs.SourceURL())
			}

			allows := rs.AllowRules()
			if len(allows) != tt.expectedAllows {
				t.Errorf("expected %d allow rules, got %d", tt.expectedAllows, len(allows))
			}

			disallows := rs.DisallowRules()
			if len(disallows) != tt.expectedDisallows {
				t.Errorf("expected %d disallow rules, got %d", tt.expectedDisallows, len(disallows))
			}

			hasCrawlDelay := rs.CrawlDelay() != nil
			if hasCrawlDelay != tt.expectedCrawlDelay {
				t.Errorf("expected crawl delay %v, got %v", tt.expectedCrawlDelay, hasCrawlDelay)
			}

			// Verify immutability - modifying returned slices shouldn't affect the original
			if len(allows) > 0 {
				allows[0] = pathRule{prefix: "/modified/"}
				allowsAfter := rs.AllowRules()
				if allowsAfter[0].Prefix() == "/modified/" {
					t.Error("AllowRules() returned mutable slice")
				}
			}
		})
	}
}

func TestFindBestMatchingGroup(t *testing.T) {
	groups := []UserAgentGroup{
		{
			UserAgents: []string{"Googlebot"},
			Disallows:  []PathRule{{Path: "/no-google/"}},
		},
		{
			UserAgents: []string{"Googlebot-Image"},
			Disallows:  []PathRule{{Path: "/no-images/"}},
		},
		{
			UserAgents: []string{"*"},
			Disallows:  []PathRule{{Path: "/private/"}},
		},
		{
			UserAgents: []string{"Bingbot"},
			Disallows:  []PathRule{{Path: "/no-bing/"}},
		},
	}

	tests := []struct {
		userAgent     string
		expectedGroup int // -1 for nil
	}{
		{
			userAgent:     "Googlebot",
			expectedGroup: 0, // Exact match
		},
		{
			userAgent:     "googlebot",
			expectedGroup: 0, // Case-insensitive match with Googlebot
		},
		{
			userAgent:     "Googlebot-Image",
			expectedGroup: 1, // Exact match (more specific)
		},
		{
			userAgent:     "Googlebot-News",
			expectedGroup: 0, // Googlebot prefix match
		},
		{
			userAgent:     "Bingbot",
			expectedGroup: 3, // Exact match
		},
		{
			userAgent:     "SomeOtherBot",
			expectedGroup: 2, // Wildcard
		},
		{
			userAgent:     "",
			expectedGroup: 2, // Wildcard matches empty too
		},
	}

	for _, tt := range tests {
		t.Run(tt.userAgent, func(t *testing.T) {
			result := findBestMatchingGroup(groups, tt.userAgent)

			if tt.expectedGroup == -1 {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("expected group at index %d, got nil", tt.expectedGroup)
			}

			expectedFirstUA := groups[tt.expectedGroup].UserAgents[0]
			if result.UserAgents[0] != expectedFirstUA {
				t.Errorf("expected group with user agent %q, got %q", expectedFirstUA, result.UserAgents[0])
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "",
			expected: "/",
		},
		{
			input:    "/",
			expected: "/",
		},
		{
			input:    "/private/",
			expected: "/private/",
		},
		{
			input:    "private/",
			expected: "/private/",
		},
		{
			input:    "path/to/resource",
			expected: "/path/to/resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRuleSetImmutability(t *testing.T) {
	fetchTime := time.Now()
	response := RobotsResponse{
		Host: "example.com",
		UserAgents: []UserAgentGroup{
			{
				UserAgents: []string{"*"},
				Allows:     []PathRule{{Path: "/public/"}},
				Disallows:  []PathRule{{Path: "/private/"}},
				CrawlDelay: timeutil.DurationPtr(10 * time.Second),
			},
		},
	}

	rs := MapResponseToRuleSet(response, "TestBot", fetchTime)

	t.Run("CrawlDelay returns copy", func(t *testing.T) {
		delay1 := rs.CrawlDelay()
		if delay1 == nil {
			t.Fatal("expected crawl delay")
		}

		// Modify the returned pointer
		*delay1 = 20 * time.Second

		delay2 := rs.CrawlDelay()
		if *delay2 != 10*time.Second {
			t.Error("CrawlDelay() returned mutable pointer")
		}
	})

	t.Run("AllowRules returns copy", func(t *testing.T) {
		rules1 := rs.AllowRules()
		if len(rules1) == 0 {
			t.Fatal("expected allow rules")
		}

		// Modify the returned slice
		rules1[0] = pathRule{prefix: "/modified/"}

		rules2 := rs.AllowRules()
		if rules2[0].Prefix() != "/public/" {
			t.Error("AllowRules() returned mutable slice")
		}
	})

	t.Run("DisallowRules returns copy", func(t *testing.T) {
		rules1 := rs.DisallowRules()
		if len(rules1) == 0 {
			t.Fatal("expected disallow rules")
		}

		// Modify the returned slice
		rules1[0] = pathRule{prefix: "/modified/"}

		rules2 := rs.DisallowRules()
		if rules2[0].Prefix() != "/private/" {
			t.Error("DisallowRules() returned mutable slice")
		}
	})
}

func TestPathRulePrefix(t *testing.T) {
	rule := pathRule{prefix: "/test/path/"}
	if rule.Prefix() != "/test/path/" {
		t.Errorf("expected prefix %q, got %q", "/test/path/", rule.Prefix())
	}
}

func TestRuleSetGetters(t *testing.T) {
	fetchTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	response := RobotsResponse{
		Host: "example.com",
		UserAgents: []UserAgentGroup{
			{
				UserAgents: []string{"*"},
				Allows:     []PathRule{{Path: "/public/"}},
				Disallows:  []PathRule{{Path: "/private/"}},
			},
		},
	}

	rs := MapResponseToRuleSet(response, "TestBot", fetchTime)

	// Test each getter
	if rs.Host() != "example.com" {
		t.Errorf("Host() = %q, expected %q", rs.Host(), "example.com")
	}

	if rs.UserAgent() != "TestBot" {
		t.Errorf("UserAgent() = %q, expected %q", rs.UserAgent(), "TestBot")
	}

	if !rs.FetchedAt().Equal(fetchTime) {
		t.Errorf("FetchedAt() = %v, expected %v", rs.FetchedAt(), fetchTime)
	}

	expectedSourceURL := "https://example.com/robots.txt"
	if rs.SourceURL() != expectedSourceURL {
		t.Errorf("SourceURL() = %q, expected %q", rs.SourceURL(), expectedSourceURL)
	}

	// Test with no crawl delay
	if rs.CrawlDelay() != nil {
		t.Error("CrawlDelay() should be nil when not set")
	}
}

func TestUserAgentCaseInsensitivity(t *testing.T) {
	groups := []UserAgentGroup{
		{
			UserAgents: []string{"Googlebot"},
			Disallows:  []PathRule{{Path: "/no-google/"}},
		},
	}

	// Test that "googlebot" (lowercase) matches "Googlebot" via case-insensitive comparison
	result := findBestMatchingGroup(groups, "googlebot")

	// According to spec, user-agent matching should be case-insensitive
	if result == nil {
		t.Error("Implementation should be case-insensitive for user-agent matching")
	}

	if result != nil && result.UserAgents[0] != "Googlebot" {
		t.Errorf("Expected to match Googlebot, got %s", result.UserAgents[0])
	}
}

func TestMapResponseToRuleSet_MultipleUserAgentsInGroup(t *testing.T) {
	fetchTime := time.Now()
	response := RobotsResponse{
		Host: "example.com",
		UserAgents: []UserAgentGroup{
			{
				UserAgents: []string{"Googlebot", "Bingbot"},
				Disallows:  []PathRule{{Path: "/shared/"}},
			},
		},
	}

	// Should match Googlebot
	rs1 := MapResponseToRuleSet(response, "Googlebot", fetchTime)
	if len(rs1.DisallowRules()) != 1 {
		t.Error("Expected to match Googlebot")
	}

	// Should match Bingbot
	rs2 := MapResponseToRuleSet(response, "Bingbot", fetchTime)
	if len(rs2.DisallowRules()) != 1 {
		t.Error("Expected to match Bingbot")
	}

	// Should not match OtherBot
	rs3 := MapResponseToRuleSet(response, "OtherBot", fetchTime)
	if len(rs3.DisallowRules()) != 0 {
		t.Error("Expected not to match OtherBot")
	}
}

func TestMapResponseToRuleSet_UserAgentPrefixMatching(t *testing.T) {
	fetchTime := time.Now()
	response := RobotsResponse{
		Host: "example.com",
		UserAgents: []UserAgentGroup{
			{
				UserAgents: []string{"Googlebot"},
				Disallows:  []PathRule{{Path: "/no-google/"}},
			},
			{
				UserAgents: []string{"Googlebot-Image"},
				Disallows:  []PathRule{{Path: "/no-images/"}},
			},
		},
	}

	// Googlebot-Image should match Googlebot-Image (exact match, more specific)
	rs := MapResponseToRuleSet(response, "Googlebot-Image", fetchTime)
	disallows := rs.DisallowRules()
	if len(disallows) != 1 || !strings.Contains(disallows[0].Prefix(), "no-images") {
		t.Error("Expected Googlebot-Image to match its own group")
	}

	// Googlebot-News should match Googlebot (prefix match)
	rs2 := MapResponseToRuleSet(response, "Googlebot-News", fetchTime)
	disallows2 := rs2.DisallowRules()
	if len(disallows2) != 1 || !strings.Contains(disallows2[0].Prefix(), "no-google") {
		t.Error("Expected Googlebot-News to match Googlebot group via prefix")
	}
}
