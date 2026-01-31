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
}

type DecisionReason string

const (
	AllowedByRobots     DecisionReason = "allowed_by_robots"
	DisallowedByRobots  DecisionReason = "disallowed_by_robots"
	NoRobotsFile        DecisionReason = "no_robots_file"
	UserAgentNotMatched DecisionReason = "user_agent_not_matched"
)

type Decision struct {
	Url url.URL

	Allowed bool

	// Why this decision was made (for logging/debugging)
	Reason DecisionReason

	// Optional delay override (robots crawl-delay)
	CrawlDelay *time.Duration
}
