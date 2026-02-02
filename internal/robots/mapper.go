package robots

import (
	"strings"
	"time"
)

// MapResponseToRuleSet converts a RobotsResponse to an immutable ruleSet.
// This function selects the most specific user agent group matching the provided
// user agent string and creates a ruleSet from it.
func MapResponseToRuleSet(response RobotsResponse, targetUserAgent string, fetchedAt time.Time) ruleSet {
	rs := ruleSet{
		host:      response.Host,
		userAgent: targetUserAgent,
		fetchedAt: fetchedAt,
		sourceURL: "https://" + response.Host + "/robots.txt",
	}

	// Track if there are any groups in the response
	rs.hasGroups = len(response.UserAgents) > 0

	// Find the most specific matching group for the target user agent
	group := findBestMatchingGroup(response.UserAgents, targetUserAgent)

	if group != nil {
		// Mark that we found a matching group
		rs.matchedGroup = true

		// Map allow rules
		rs.allowRules = make([]pathRule, 0, len(group.Allows))
		for _, allow := range group.Allows {
			if allow.Path != "" {
				rs.allowRules = append(rs.allowRules, pathRule{
					prefix: normalizePath(allow.Path),
				})
			}
		}

		// Map disallow rules
		rs.disallowRules = make([]pathRule, 0, len(group.Disallows))
		for _, disallow := range group.Disallows {
			if disallow.Path != "" {
				rs.disallowRules = append(rs.disallowRules, pathRule{
					prefix: normalizePath(disallow.Path),
				})
			}
		}

		// Map crawl delay
		if group.CrawlDelay != nil {
			delay := *group.CrawlDelay
			rs.crawlDelay = &delay
		}
	}

	return rs
}

// findBestMatchingGroup finds the most specific user agent group matching the target.
// According to the spec:
// 1. Exact matches take precedence over wildcard matches
// 2. More specific user-agent strings take precedence over less specific ones
// 3. The wildcard (*) matches all user agents
func findBestMatchingGroup(groups []UserAgentGroup, targetUserAgent string) *UserAgentGroup {
	var bestMatch *UserAgentGroup
	targetLower := strings.ToLower(targetUserAgent)
	bestMatchLength := 0

	for i := range groups {
		group := &groups[i]

		for _, ua := range group.UserAgents {
			uaLower := strings.ToLower(ua)

			// Check for exact match (case-insensitive)
			if uaLower == targetLower {
				return group // Exact match is the best possible
			}

			// Check for wildcard
			if ua == "*" {
				if bestMatch == nil {
					bestMatch = group
				}
				continue
			}

			// Check if this user agent string matches the beginning of the target
			// e.g., "Googlebot" matches "Googlebot-Image" (case-insensitive)
			if strings.HasPrefix(targetLower, uaLower) {
				if len(uaLower) > bestMatchLength {
					bestMatch = group
					bestMatchLength = len(uaLower)
				}
			}
		}
	}

	return bestMatch
}

// normalizePath ensures the path starts with "/" and handles special cases.
func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// ruleSet getters for immutability

// Host returns the host this ruleSet applies to.
func (r ruleSet) Host() string {
	return r.host
}

// UserAgent returns the user agent string these rules apply to.
func (r ruleSet) UserAgent() string {
	return r.userAgent
}

// FetchedAt returns when this ruleSet was fetched.
func (r ruleSet) FetchedAt() time.Time {
	return r.fetchedAt
}

// SourceURL returns the URL of the robots.txt file.
func (r ruleSet) SourceURL() string {
	return r.sourceURL
}

// CrawlDelay returns the crawl delay if specified, or nil.
func (r ruleSet) CrawlDelay() *time.Duration {
	if r.crawlDelay == nil {
		return nil
	}
	delay := *r.crawlDelay
	return &delay
}

// AllowRules returns a copy of the allow rules.
func (r ruleSet) AllowRules() []pathRule {
	result := make([]pathRule, len(r.allowRules))
	copy(result, r.allowRules)
	return result
}

// DisallowRules returns a copy of the disallow rules.
func (r ruleSet) DisallowRules() []pathRule {
	result := make([]pathRule, len(r.disallowRules))
	copy(result, r.disallowRules)
	return result
}

// Prefix returns the path prefix for this rule.
func (p pathRule) Prefix() string {
	return p.prefix
}
