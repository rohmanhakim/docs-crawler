package metadata

import (
	"time"
)

type FetchEvent struct {
	fetchUrl    string
	httpStatus  int
	duration    time.Duration
	contentType string
	retryCount  int
	crawlDepth  int
}

/*
crawlStats
  - Represents a terminal, derived summary of a completed crawl
  - Contains only aggregate counts and durations
  - Is computed by the scheduler after crawl termination
  - Is recorded exactly once
  - Must not influence scheduling, retries, or crawl termination
  - Must be constructed without reading metadata
*/
type crawlStats struct {
	totalPages  int
	totalErrors int
	totalAssets int
	durationMs  int64
}

type ArtifactRecord struct {
	paths string
}

/*
	ErrorCause is a closed, canonical classification used exclusively for
	observability (logging, metrics, reporting).

	Rules:
	 - ErrorCause is for observability only.
	 - It must never be used to derive retry, continuation, or abort decisions.
	 - Any use of metadata.ErrorCause outside logging, metrics, or reporting is a design violation.
	 - ErrorCause MUST NOT influence control flow.
	 - ErrorCause MUST NOT be used for retry, continuation, or abort decisions.
	 - ErrorCause values MUST have stable, package-agnostic semantics.
	 - Pipeline packages MAY map their local errors to ErrorCause,
	   but MUST NOT invent new meanings.
	Non-goals:
	 - ErrorCause does not encode severity.
	 - ErrorCause does not imply retryability.
	 - ErrorCause does not imply crawl termination.
	 - ErrorCause does not imply correctness of downstream behavior.

If a failure does not clearly match a defined cause, CauseUnknown MUST be used.
*/
type ErrorCause int

/*
Canonical ErrorCause Table

# CauseUnknown

Meaning:
  - The failure does not map cleanly to any known category.
  - Used as a safe fallback.

Examples:
  - Unexpected internal errors
  - Unclassified third-party library failures

# CauseNetworkFailure

Meaning:
  - Failure caused by network transport or remote availability.

Examples:
  - TCP timeouts
  - DNS resolution failures
  - Connection resets
  - robots.txt fetch timeout

# CausePolicyDisallow

Meaning:
  - Crawling was disallowed by an explicit policy or rule.

Examples:
  - robots.txt disallow
  - HTTP 403 / 401 interpreted as access denial
  - rate-limit enforcement

# CauseContentInvalid

Meaning:
  - Content was fetched but could not be processed meaningfully.

Examples:
  - Non-HTML responses
  - Empty or unextractable document bodies
  - Broken DOM preventing extraction

# CauseStorageFailure

Meaning:
  - Failure while persisting crawl artifacts.

Examples:
  - Disk full
  - Write permission errors
  - Filesystem I/O failures

# CauseInvariantViolation

Meaning:
  - A system-level invariant was violated.

Examples:
  - Multiple H1s in a document
  - Impossible crawl depth
  - Internal consistency checks failing
*/
const (
	CauseUnknown = iota
	CauseNetworkFailure
	CausePolicyDisallow
	CauseContentInvalid
	CauseStorageFailure
	CauseInvariantViolation
)

type ErrorRecord struct {
	packageName string
	action      string
	cause       ErrorCause
	errorString string
	observedAt  time.Time
	attrs       []Attribute
}

type Attribute struct {
	Key   AttributeKey
	Value string
}

func NewAttr(key AttributeKey, val string) Attribute {
	return Attribute{
		Key:   key,
		Value: val,
	}
}

type AttributeKey string

const (
	AttrTime       AttributeKey = "time"
	AttrURL        AttributeKey = "url"
	AttrHost       AttributeKey = "host"
	AttrPath       AttributeKey = "path"
	AttrDepth      AttributeKey = "depth"
	AttrField      AttributeKey = "field"
	AttrHTTPStatus AttributeKey = "http_status"
	AttrAssetURL   AttributeKey = "asset_url"
	AttrWritePath  AttributeKey = "write_path"
)
