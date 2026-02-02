package robots

import (
	"strings"
	"time"
)

// RobotsResponse represents the parsed content of a robots.txt file.
// This struct is used for parsing the fetch response and should not be
// used directly for decision making - instead, map it to ruleSet.
type RobotsResponse struct {
	// The host this robots.txt applies to
	Host string

	// List of sitemap URLs found in the robots.txt
	Sitemaps []string

	// User agent groups, each containing rules for specific user agents
	UserAgents []UserAgentGroup
}

// UserAgentGroup represents a set of rules for one or more user agents.
type UserAgentGroup struct {
	// List of user agent strings this group applies to
	UserAgents []string

	// Allow rules (paths that may be crawled)
	Allows []PathRule

	// Disallow rules (paths that may not be crawled)
	Disallows []PathRule

	// Optional crawl delay
	CrawlDelay *time.Duration
}

// PathRule represents a single allow or disallow rule.
type PathRule struct {
	// The path pattern (may include wildcards * and $)
	Path string
}

// IsEmpty returns true if the response contains no rules or sitemaps.
func (r RobotsResponse) IsEmpty() bool {
	if len(r.Sitemaps) > 0 {
		return false
	}
	for _, group := range r.UserAgents {
		if len(group.Allows) > 0 || len(group.Disallows) > 0 {
			return false
		}
	}
	return true
}

// GetGroupForUserAgent returns the most specific user agent group for the given user agent.
// Returns nil if no matching group is found.
// Matching is case-insensitive as per robots.txt spec.
func (r RobotsResponse) GetGroupForUserAgent(userAgent string) *UserAgentGroup {
	userAgentLower := strings.ToLower(userAgent)

	// Look for exact match first (case-insensitive)
	for i, group := range r.UserAgents {
		for _, ua := range group.UserAgents {
			if strings.ToLower(ua) == userAgentLower {
				return &r.UserAgents[i]
			}
		}
	}

	// Look for prefix match (e.g., "Googlebot" matches "Googlebot-Image")
	var bestMatch *UserAgentGroup
	bestMatchLength := 0

	for i, group := range r.UserAgents {
		for _, ua := range group.UserAgents {
			uaLower := strings.ToLower(ua)

			if ua == "*" {
				if bestMatch == nil {
					bestMatch = &r.UserAgents[i]
				}
				continue
			}

			if strings.HasPrefix(userAgentLower, uaLower) {
				if len(uaLower) > bestMatchLength {
					bestMatch = &r.UserAgents[i]
					bestMatchLength = len(uaLower)
				}
			}
		}
	}

	return bestMatch
}
