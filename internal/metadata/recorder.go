package metadata

import (
	"time"
)

/*
Metadata Collected
- Fetch timestamps
- HTTP status codes
- Content hashes
- Crawl depth

Logging Goals
- Debuggable crawl behavior
- Post-run auditability
- Failure diagnostics

Structured logging is preferred.

Allowed:
- Primitive values
- Timestamps
- URLs (as values, not objects with behavior)
- Hashes
- Status codes
- Durations
- Identifiers (page ID, crawl ID)

Determinism guarantees:
 - Metadata does not affect control flow
 - Errors do not reorder the frontier
 - Jitter is seed-controlled
 - Output is stable given identical inputs

Metadata is write-only.
No component may read metadata to influence crawl decisions.
*/

/*
Recorder captures structured crawl events.
It must not:
- perform I/O decisions
- affect control flow
- impose a logging backend
Ordering guarantees:
- Events are recorded synchronously in the order they are received by a single worker.
- No global ordering across workers is guaranteed.
- Consumers MUST NOT assume total ordering across the crawl.
- Ordering is provided for debuggability, not causality.
*/
type Recorder struct {
	workerId string
}

func NewRecorder(workerId string) Recorder {
	return Recorder{
		workerId: workerId,
	}
}

func (r *Recorder) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause ErrorCause,
	errorString string,
	attrs []Attribute,
) {

}

func (r *Recorder) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
}

func (r *Recorder) RecordArtifact(kind ArtifactKind, path string, attrs []Attribute) {}

/*
RecordFinalCrawlStats records a terminal, derived summary of a completed crawl.

Contract:
  - MUST be called exactly once per crawl execution.
  - MUST be called only after crawl termination
    (frontier exhausted or scheduler abort).
  - MUST NOT be called during active crawling.
  - The provided CrawlStats MUST be derived from scheduler state,
    not accumulated incrementally via the recorder.
  - Recorded stats MUST NOT influence control flow or scheduling.
*/
func (r *Recorder) RecordFinalCrawlStats(
	totalPages int,
	totalErrors int,
	totalAssets int,
	duration time.Duration,
) {
	stats := crawlStats{
		totalPages:  totalPages,
		totalErrors: totalErrors,
		totalAssets: totalAssets,
		durationMs:  duration.Milliseconds(),
	}

	r.append(stats)
}

func (r *Recorder) append(crawlStats) {}

type MetadataSink interface {
	RecordError(
		observedAt time.Time,
		packageName string,
		action string,
		cause ErrorCause,
		details string,
		attrs []Attribute,
	)

	RecordFetch(
		fetchUrl string,
		httpStatus int,
		duration time.Duration,
		contentType string,
		retryCount int,
		crawlDepth int,
	)
	RecordArtifact(kind ArtifactKind, path string, attrs []Attribute)
}

type CrawlFinalizer interface {
	RecordFinalCrawlStats(
		totalPages int,
		totalErrors int,
		totalAssets int,
		duration time.Duration,
	)
}

// NoopSink, struct that implements metadata.Sink but does nothing
// Scheduler (or Test) can decide whether to inject Recorder or NoopSink
// Purpose is to make metadata orthogonal

type NoopSink struct{}

func (n *NoopSink) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause ErrorCause,
	errorString string,
	attrs []Attribute,
) {

}

func (n *NoopSink) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
}

func (n *NoopSink) RecordArtifact(kind ArtifactKind, path string, attrs []Attribute) {}
