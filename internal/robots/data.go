package robots

import (
	"net/url"
	"time"
)

// Permission modeling

type pathRule struct {
	prefix string
}

type ruleSet struct {
	host string

	// The user-agent these rules apply to (resolved, not raw)
	userAgent string

	// Path-based rules, evaluated in order of precedence
	allowRules    []pathRule
	disallowRules []pathRule

	// Optional crawl delay from robots.txt
	crawlDelay *time.Duration

	// Metadata / observability
	fetchedAt time.Time
	sourceURL string

	// matchedGroup indicates if a user-agent group was matched in robots.txt
	// This is false when no group matches (not even wildcard *)
	matchedGroup bool

	// hasGroups indicates if the robots.txt file had any user-agent groups at all
	// This is false when the response had no groups (e.g., 404 or empty file)
	hasGroups bool
}

type DecisionReason string

const (
	AllowedByRobots     DecisionReason = "allowed_by_robots"
	DisallowedByRobots  DecisionReason = "disallowed_by_robots"
	UserAgentNotMatched DecisionReason = "user_agent_not_matched"
	EmptyRuleSet        DecisionReason = "empty_rule_set"
	NoMatchingRules     DecisionReason = "no_matching_rules"
)

type Decision struct {
	Url url.URL

	Allowed bool

	// Why this decision was made (for logging/debugging)
	Reason DecisionReason

	// Optional delay override (robots crawl-delay)
	CrawlDelay *time.Duration
}
