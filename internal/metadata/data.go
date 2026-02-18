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
	fetchedAt   time.Time
	fetchURL    string
	httpStatus  int
	duration    time.Duration
	contentType string
	retryCount  int
	crawlDepth  int
	kind        FetchKind
}

// NewFetchEvent constructs an immutable FetchEvent.
func NewFetchEvent(
	fetchedAt time.Time,
	fetchURL string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
	kind FetchKind,
) FetchEvent {
	return FetchEvent{
		fetchedAt:   fetchedAt,
		fetchURL:    fetchURL,
		httpStatus:  httpStatus,
		duration:    duration,
		contentType: contentType,
		retryCount:  retryCount,
		crawlDepth:  crawlDepth,
		kind:        kind,
	}
}

func (f FetchEvent) FetchedAt() time.Time    { return f.fetchedAt }
func (f FetchEvent) FetchURL() string        { return f.fetchURL }
func (f FetchEvent) HTTPStatus() int         { return f.httpStatus }
func (f FetchEvent) Duration() time.Duration { return f.duration }
func (f FetchEvent) ContentType() string     { return f.contentType }
func (f FetchEvent) RetryCount() int         { return f.retryCount }
func (f FetchEvent) CrawlDepth() int         { return f.crawlDepth }
func (f FetchEvent) Kind() FetchKind         { return f.kind }

/*
CrawlStats represents a terminal, derived summary of a completed crawl.
  - Contains only aggregate counts and timestamps.
  - Is computed by the scheduler after crawl termination.
  - Is recorded exactly once.
  - Must not influence scheduling, retries, or crawl termination.
  - Must be constructed without reading metadata.
*/
type CrawlStats struct {
	startedAt             time.Time
	finishedAt            time.Time
	totalPages            int
	totalErrors           int
	totalAssets           int
	manualRetryQueueCount int // URLs in manual retry queue at crawl completion
}

// NewCrawlStats constructs an immutable CrawlStats.
func NewCrawlStats(
	startedAt time.Time,
	finishedAt time.Time,
	totalPages int,
	totalErrors int,
	totalAssets int,
	manualRetryQueueCount int,
) CrawlStats {
	return CrawlStats{
		startedAt:             startedAt,
		finishedAt:            finishedAt,
		totalPages:            totalPages,
		totalErrors:           totalErrors,
		totalAssets:           totalAssets,
		manualRetryQueueCount: manualRetryQueueCount,
	}
}

func (c CrawlStats) StartedAt() time.Time       { return c.startedAt }
func (c CrawlStats) FinishedAt() time.Time      { return c.finishedAt }
func (c CrawlStats) TotalPages() int            { return c.totalPages }
func (c CrawlStats) TotalErrors() int           { return c.totalErrors }
func (c CrawlStats) TotalAssets() int           { return c.totalAssets }
func (c CrawlStats) ManualRetryQueueCount() int { return c.manualRetryQueueCount }

type ArtifactKind string

const (
	ArtifactMarkdown ArtifactKind = "markdown"
	ArtifactAsset    ArtifactKind = "asset"
)

type ArtifactRecord struct {
	kind        ArtifactKind
	writePath   string
	sourceURL   string
	contentHash string
	overwrite   bool
	bytes       int64
	recordedAt  time.Time
}

// NewArtifactRecord constructs an immutable ArtifactRecord.
func NewArtifactRecord(
	kind ArtifactKind,
	writePath string,
	sourceURL string,
	contentHash string,
	overwrite bool,
	bytes int64,
	recordedAt time.Time,
) ArtifactRecord {
	return ArtifactRecord{
		kind:        kind,
		writePath:   writePath,
		sourceURL:   sourceURL,
		contentHash: contentHash,
		overwrite:   overwrite,
		bytes:       bytes,
		recordedAt:  recordedAt,
	}
}

func (a ArtifactRecord) Kind() ArtifactKind    { return a.kind }
func (a ArtifactRecord) WritePath() string     { return a.writePath }
func (a ArtifactRecord) SourceURL() string     { return a.sourceURL }
func (a ArtifactRecord) ContentHash() string   { return a.contentHash }
func (a ArtifactRecord) Overwrite() bool       { return a.overwrite }
func (a ArtifactRecord) Bytes() int64          { return a.bytes }
func (a ArtifactRecord) RecordedAt() time.Time { return a.recordedAt }

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
	stage      PipelineStage
	pageURL    string
	success    bool
	recordedAt time.Time
	// linksFound is populated on success for StageExtract only.
	linksFound int
}

// NewPipelineEvent constructs an immutable PipelineEvent.
func NewPipelineEvent(
	stage PipelineStage,
	pageURL string,
	success bool,
	recordedAt time.Time,
	linksFound int,
) PipelineEvent {
	return PipelineEvent{
		stage:      stage,
		pageURL:    pageURL,
		success:    success,
		recordedAt: recordedAt,
		linksFound: linksFound,
	}
}

func (p PipelineEvent) Stage() PipelineStage  { return p.stage }
func (p PipelineEvent) PageURL() string       { return p.pageURL }
func (p PipelineEvent) Success() bool         { return p.success }
func (p PipelineEvent) RecordedAt() time.Time { return p.recordedAt }
func (p PipelineEvent) LinksFound() int       { return p.linksFound }

// SkipReason classifies why a URL was not crawled.
type SkipReason string

const (
	SkipReasonRobotsDisallow SkipReason = "robots_disallow"
	SkipReasonOutOfScope     SkipReason = "out_of_scope"
	SkipReasonAlreadyVisited SkipReason = "already_visited"
)

// SkipEvent records that a URL was admitted to the frontier but not crawled.
type SkipEvent struct {
	skippedURL string
	reason     SkipReason
	recordedAt time.Time
}

// NewSkipEvent constructs an immutable SkipEvent.
func NewSkipEvent(skippedURL string, reason SkipReason, recordedAt time.Time) SkipEvent {
	return SkipEvent{
		skippedURL: skippedURL,
		reason:     reason,
		recordedAt: recordedAt,
	}
}

func (s SkipEvent) SkippedURL() string    { return s.skippedURL }
func (s SkipEvent) Reason() SkipReason    { return s.reason }
func (s SkipEvent) RecordedAt() time.Time { return s.recordedAt }

// ErrorRecord is the typed struct accepted by RecordError. It follows the same
// constructor + accessor pattern as FetchEvent, ArtifactRecord, PipelineEvent,
// and SkipEvent, making RecordError consistent with every other MetadataSink method.
//
// errorString carries the raw err.Error() string from the call site.
// attrs holds any ad-hoc contextual key/value pairs (e.g. AttrURL, AttrAssetURL).
type ErrorRecord struct {
	packageName string
	action      string
	cause       ErrorCause
	errorString string
	observedAt  time.Time
	attrs       []Attribute
}

// NewErrorRecord constructs an immutable ErrorRecord.
// attrs is copied to prevent external mutation.
func NewErrorRecord(
	observedAt time.Time,
	packageName string,
	action string,
	cause ErrorCause,
	errorString string,
	attrs []Attribute,
) ErrorRecord {
	cp := make([]Attribute, len(attrs))
	copy(cp, attrs)
	return ErrorRecord{
		observedAt:  observedAt,
		packageName: packageName,
		action:      action,
		cause:       cause,
		errorString: errorString,
		attrs:       cp,
	}
}

func (e ErrorRecord) ObservedAt() time.Time { return e.observedAt }
func (e ErrorRecord) PackageName() string   { return e.packageName }
func (e ErrorRecord) Action() string        { return e.action }
func (e ErrorRecord) Cause() ErrorCause     { return e.cause }
func (e ErrorRecord) ErrorString() string   { return e.errorString }

// Attrs returns a copy of the attribute slice to prevent external mutation.
func (e ErrorRecord) Attrs() []Attribute {
	cp := make([]Attribute, len(e.attrs))
	copy(cp, e.attrs)
	return cp
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
	kind     EventKind
	fetch    *FetchEvent
	artifact *ArtifactRecord
	pipeline *PipelineEvent
	skip     *SkipEvent
	error    *ErrorRecord
	stats    *CrawlStats
}

func (e Event) Kind() EventKind           { return e.kind }
func (e Event) Fetch() *FetchEvent        { return e.fetch }
func (e Event) Artifact() *ArtifactRecord { return e.artifact }
func (e Event) Pipeline() *PipelineEvent  { return e.pipeline }
func (e Event) Skip() *SkipEvent          { return e.skip }
func (e Event) Error() *ErrorRecord       { return e.error }
func (e Event) Stats() *CrawlStats        { return e.stats }

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
	key   AttributeKey
	value string
}

func NewAttr(key AttributeKey, val string) Attribute {
	return Attribute{
		key:   key,
		value: val,
	}
}

func (a Attribute) Key() AttributeKey { return a.key }
func (a Attribute) Value() string     { return a.value }

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
