package robots

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots/cache"
)

/*
Responsibilities

- Fetch robots.txt per host
- Cache rules for crawl duration
- Enforce allow/disallow rules before enqueue

robots.txt checks occur before a URL enters the frontier.
TODO:
Split robots API into:
- Decision (admission)
- Error (infrastructure)
*/

// Robot handles robots.txt fetching and decision making for URL crawling permissions.
type Robot interface {
	Init(userAgent string, httpClient *http.Client)
	Decide(targetURL url.URL) (Decision, *RobotsError)
}

type CachedRobot struct {
	metadataSink metadata.MetadataSink
	fetcher      *RobotsFetcher
	userAgent    string
}

// NewCachedRobot creates a new empty Robot instance.
// Use Init() to initialize the robot with a user-agent.
func NewCachedRobot(metadataSink metadata.MetadataSink) CachedRobot {
	return CachedRobot{
		metadataSink: metadataSink,
	}
}

// Init initializes the Robot with a user-agent and HTTP client.
// The RobotsFetcher is created internally with a memory cache.
func (r *CachedRobot) Init(userAgent string, httpClient *http.Client) {
	// Create an in-memory cache for robots.txt files
	memCache := cache.NewMemoryCache()

	// Create the fetcher internally with the cache
	fetcher := NewRobotsFetcher(httpClient, r.metadataSink, userAgent, memCache)

	r.fetcher = fetcher
	r.userAgent = userAgent
}

// InitWithCache initializes the Robot with the given user-agent, HTTP client, and a custom cache implementation.
// This is useful for testing with mock caches.
func (r *CachedRobot) InitWithCache(userAgent string, httpClient *http.Client, cacheImpl cache.Cache) {
	fetcher := NewRobotsFetcher(httpClient, r.metadataSink, userAgent, cacheImpl)

	r.fetcher = fetcher
	r.userAgent = userAgent
}

// Decide determines whether a URL is allowed to be crawled based on robots.txt rules.
// It fetches the robots.txt for the URL's host, maps it to a ruleSet, and makes a decision.
// This is the exported entry point that handles error classification and metadata recording.
func (r *CachedRobot) Decide(targetURL url.URL) (Decision, *RobotsError) {
	// Fetch robots.txt for the host
	ctx := context.Background()
	fetchResult, err := r.fetcher.Fetch(ctx, targetURL.Scheme, targetURL.Host)
	if err != nil {
		r.metadataSink.RecordError(
			time.Now(),
			"robots",
			"Robot.Decide",
			mapRobotsErrorToMetadataCause(err),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, targetURL.String()),
				metadata.NewAttr(metadata.AttrHost, targetURL.Host),
			},
		)
		return Decision{}, err
	}

	// Map the fetch result to a ruleSet for decision making
	rs := MapResponseToRuleSet(fetchResult.Response, r.userAgent, fetchResult.FetchedAt)

	// Make the decision using the private decide function
	decision, decideErr := decide(rs, targetURL)
	if decideErr != nil {
		var robotsError *RobotsError
		if errors.As(decideErr, &robotsError) {
			r.metadataSink.RecordError(
				time.Now(),
				"robots",
				"Robot.Decide",
				mapRobotsErrorToMetadataCause(robotsError),
				robotsError.Error(),
				[]metadata.Attribute{
					metadata.NewAttr(metadata.AttrURL, targetURL.String()),
					metadata.NewAttr(metadata.AttrHost, targetURL.Host),
					metadata.NewAttr(metadata.AttrPath, targetURL.Path),
				},
			)
			return Decision{}, robotsError
		}
		// Unexpected error type
		return Decision{}, &RobotsError{
			Message:   fmt.Sprintf("unexpected error during decision: %v", decideErr),
			Retryable: false,
			Cause:     ErrCauseParseError,
		}
	}

	return decision, nil
}

// decide determines whether a URL is allowed based on the provided ruleSet.
// This is the internal decision-making logic that works with ruleSet directly.
// It implements the robots.txt matching algorithm according to the spec:
// - The most specific (longest) matching rule takes precedence
// - Allow rules take precedence over disallow rules of the same length
// - Wildcards (*) match any sequence of characters
// - The $ wildcard indicates the end of the URL
func decide(rs ruleSet, targetURL url.URL) (Decision, *RobotsError) {
	// Check if there are no rules at all (empty ruleSet means allow all)
	if len(rs.allowRules) == 0 && len(rs.disallowRules) == 0 {
		// Distinguish between:
		// - EmptyRuleSet: no groups at all (404) OR matched group has no rules
		// - UserAgentNotMatched: groups exist but none matched our user-agent
		reason := EmptyRuleSet
		if rs.hasGroups && !rs.matchedGroup {
			reason = UserAgentNotMatched
		}
		return Decision{
			Url:        targetURL,
			Allowed:    true,
			Reason:     reason,
			CrawlDelay: rs.CrawlDelay(),
		}, nil
	}

	path := targetURL.Path
	if path == "" {
		path = "/"
	}

	// Find the best matching rule
	// According to robots.txt spec:
	// 1. Allow rules take precedence over disallow rules of the same length
	// 2. More specific (longer) rules take precedence over less specific ones
	// 3. The $ wildcard indicates end of URL
	// 4. The * wildcard matches any sequence of characters

	bestMatch := matchRule{}
	hasMatch := false

	// Check allow rules first
	for _, rule := range rs.allowRules {
		matchType, length := matchesRule(path, rule.prefix)
		if matchType == noMatch {
			continue
		}

		// Allow rules have higher precedence than disallow of same length
		priority := float64(length)
		if matchType == exactMatch {
			priority += 1000 // Boost exact matches
		}

		if !hasMatch || priority > bestMatch.priority {
			bestMatch = matchRule{
				isAllow:  true,
				length:   length,
				priority: priority,
			}
			hasMatch = true
		}
	}

	// Check disallow rules
	for _, rule := range rs.disallowRules {
		matchType, length := matchesRule(path, rule.prefix)
		if matchType == noMatch {
			continue
		}

		// Disallow rules have lower precedence than allow of same length
		priority := float64(length)
		if matchType == exactMatch {
			priority += 1000 // Boost exact matches
		}
		// Disallow gets a slight penalty compared to allow of same length
		priority -= 0.5

		if !hasMatch || priority > bestMatch.priority {
			bestMatch = matchRule{
				isAllow:  false,
				length:   length,
				priority: priority,
			}
			hasMatch = true
		}
	}

	// If no rules matched, the URL is allowed (default allow)
	if !hasMatch {
		return Decision{
			Url:        targetURL,
			Allowed:    true,
			Reason:     NoMatchingRules,
			CrawlDelay: rs.CrawlDelay(),
		}, nil
	}

	// Determine the reason
	var reason DecisionReason
	if bestMatch.isAllow {
		reason = AllowedByRobots
	} else {
		reason = DisallowedByRobots
	}

	return Decision{
		Url:        targetURL,
		Allowed:    bestMatch.isAllow,
		Reason:     reason,
		CrawlDelay: rs.CrawlDelay(),
	}, nil
}

// matchType represents the type of match found
type matchType int

const (
	noMatch matchType = iota
	prefixMatch
	exactMatch
)

// matchRule represents a matching rule with its priority
type matchRule struct {
	isAllow  bool
	length   int
	priority float64
}

// matchesRule checks if a path matches a rule pattern.
// Returns the match type and the match length (for priority calculation).
// The length represents how specific the match is (longer = more specific).
func matchesRule(path, pattern string) (matchType, int) {
	// Handle exact match with $ suffix
	if strings.HasSuffix(pattern, "$") {
		patternWithoutSuffix := pattern[:len(pattern)-1]
		if path == patternWithoutSuffix {
			return exactMatch, len(patternWithoutSuffix)
		}
		// Check if pattern has wildcards that could match exactly
		if matchesWildcard(path, patternWithoutSuffix) {
			return exactMatch, len(patternWithoutSuffix)
		}
		return noMatch, 0
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		if matchesWildcard(path, pattern) {
			// Calculate match length as the literal parts of the pattern
			literalParts := strings.ReplaceAll(pattern, "*", "")
			return prefixMatch, len(literalParts)
		}
		return noMatch, 0
	}

	// Simple prefix match (most common case)
	// According to robots.txt spec, paths match as prefixes by default
	if strings.HasPrefix(path, pattern) {
		// Check if this is an exact match
		if path == pattern {
			return exactMatch, len(pattern)
		}
		return prefixMatch, len(pattern)
	}

	return noMatch, 0
}

// matchesWildcard checks if a path matches a pattern containing * wildcards.
// The * wildcard matches any sequence of characters (including empty).
func matchesWildcard(path, pattern string) bool {
	// Split pattern by *
	parts := strings.Split(pattern, "*")

	// Empty pattern matches empty path
	if len(parts) == 0 {
		return path == ""
	}

	// Track current position in path
	pos := 0

	for i, part := range parts {
		if part == "" {
			// Consecutive * or * at start/end
			continue
		}

		if i == 0 {
			// First part must match at the beginning
			if !strings.HasPrefix(path, part) {
				return false
			}
			pos = len(part)
		} else {
			// Subsequent parts must appear somewhere after current position
			idx := strings.Index(path[pos:], part)
			if idx == -1 {
				return false
			}
			pos += idx + len(part)
		}
	}

	// If pattern doesn't end with *, the last part must be at the end
	if !strings.HasSuffix(pattern, "*") && pos < len(path) {
		// Check if the remaining path just contains the last part
		lastPart := parts[len(parts)-1]
		if lastPart != "" && !strings.HasSuffix(path, lastPart) {
			return false
		}
	}

	return true
}
