package metadata

import (
	"time"
)

// FetchKind discriminates the type of resource that was fetched.
type FetchKind string

const (
	KindPage   FetchKind = "page"
	KindAsset  FetchKind = "asset"
	KindRobots FetchKind = "robots"
)

// FetchEvent represents a completed HTTP fetch for any resource kind.
type FetchEvent struct {
	FetchedAt   time.Time
	FetchURL    string
	HTTPStatus  int
	Duration    time.Duration
	ContentType string
	RetryCount  int
	CrawlDepth  int
	Kind        FetchKind
}

/*
CrawlStats represents a terminal, derived summary of a completed crawl.
  - Contains only aggregate counts and timestamps.
  - Is computed by the scheduler after crawl termination.
  - Is recorded exactly once.
  - Must not influence scheduling, retries, or crawl termination.
  - Must be constructed without reading metadata.
*/
type CrawlStats struct {
	StartedAt             time.Time
	FinishedAt            time.Time
	TotalPages            int
	TotalErrors           int
	TotalAssets           int
	ManualRetryQueueCount int // URLs in manual retry queue at crawl completion
}

type ArtifactKind string

const (
	ArtifactMarkdown ArtifactKind = "markdown"
	ArtifactAsset    ArtifactKind = "asset"
)

type ArtifactRecord struct {
	Kind        ArtifactKind
	WritePath   string
	SourceURL   string
	ContentHash string
	Overwrite   bool
	Bytes       int64
	RecordedAt  time.Time
}

// PipelineStage identifies a processing stage in the crawl pipeline.
type PipelineStage string

const (
	StageExtract   PipelineStage = "extract"
	StageSanitize  PipelineStage = "sanitize"
	StageConvert   PipelineStage = "convert"
	StageNormalize PipelineStage = "normalize"
)

// PipelineEvent represents the outcome of a single pipeline stage for a given page.
type PipelineEvent struct {
	Stage      PipelineStage
	PageURL    string
	Success    bool
	RecordedAt time.Time
	// LinksFound is populated on success for StageExtract only.
	LinksFound int
}

// SkipReason classifies why a URL was not crawled.
type SkipReason string

const (
	SkipReasonRobotsDisallow SkipReason = "robots_disallow"
	SkipReasonOutOfScope     SkipReason = "out_of_scope"
	SkipReasonAlreadyVisited SkipReason = "already_visited"
)

// SkipEvent records that a URL was admitted to the frontier but not crawled.
type SkipEvent struct {
	SkippedURL string
	Reason     SkipReason
	RecordedAt time.Time
}

// ErrorEvent wraps the parameters of RecordError for inclusion in the sealed Event log.
type ErrorEvent struct {
	ObservedAt  time.Time
	PackageName string
	Action      string
	Cause       ErrorCause
	Details     string
	Attrs       []Attribute
}

// EventKind discriminates the concrete payload type stored in an Event.
type EventKind string

const (
	EventKindFetch    EventKind = "fetch"
	EventKindArtifact EventKind = "artifact"
	EventKindPipeline EventKind = "pipeline"
	EventKindSkip     EventKind = "skip"
	EventKindError    EventKind = "error"
	EventKindStats    EventKind = "stats"
)

// Event is a sealed discriminated union of all event types recorded by the Recorder.
// Only the pointer field matching Kind is non-nil.
type Event struct {
	Kind     EventKind
	Fetch    *FetchEvent
	Artifact *ArtifactRecord
	Pipeline *PipelineEvent
	Skip     *SkipEvent
	Error    *ErrorEvent
	Stats    *CrawlStats
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
	CauseRetryFailure
)

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
	AttrHTTPStatus AttributeKey = "http_status"
	AttrAssetURL   AttributeKey = "asset_url"
	AttrWritePath  AttributeKey = "write_path"
	AttrMessage    AttributeKey = "message"

	// AttrContentHash is the hash of the written file's content.
	AttrContentHash AttributeKey = "content_hash"
	// AttrURLHash is the hash derived from the page URL, used as a storage filename.
	AttrURLHash AttributeKey = "url_hash"
	// AttrPageURL is the source page URL providing context for asset and pipeline events.
	AttrPageURL AttributeKey = "page_url"
	// AttrStage is the pipeline stage name (extract, sanitize, convert, normalize).
	AttrStage AttributeKey = "stage"

	// Deprecated: AttrField is ambiguous — two call sites used it for both URLHash
	// and ContentHash, making events uninterpretable. Use AttrContentHash or
	// AttrURLHash instead. AttrField must not be used in new code.
	AttrField AttributeKey = "field"
)
