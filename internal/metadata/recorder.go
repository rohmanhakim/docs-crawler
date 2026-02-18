package metadata

import (
	"sync"
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
Recorder captures structured crawl events in an in-memory, append-only log.

It must not:
- perform I/O decisions
- affect control flow
- impose a logging backend

Ordering guarantees:
- Events are recorded synchronously in the order they are received by a single worker.
- No global ordering across workers is guaranteed.
- Consumers MUST NOT assume total ordering across the crawl.
- Ordering is provided for debuggability, not causality.

Concurrency:
- All public methods are safe for concurrent use.
- The event log is protected by a read/write mutex.
- Reading via Events() never blocks recording.
*/

type MetadataSink interface {
	RecordError(
		observedAt time.Time,
		packageName string,
		action string,
		cause ErrorCause,
		details string,
		attrs []Attribute,
	)
	RecordFetch(event FetchEvent)
	RecordArtifact(record ArtifactRecord)
	RecordPipelineStage(event PipelineEvent)
	RecordSkip(event SkipEvent)
}

type CrawlFinalizer interface {
	RecordFinalCrawlStats(stats CrawlStats)
}

type Recorder struct {
	workerId string
	mu       sync.RWMutex
	events   []Event
	subs     []chan<- Event // streaming subscribers; forwarded on every append
}

func NewRecorder(workerId string) Recorder {
	return Recorder{
		workerId: workerId,
	}
}

// Subscribe registers ch as a streaming subscriber. After this call returns,
// every subsequent event appended to the log is forwarded to ch in a
// non-blocking send. Events recorded before Subscribe was called are NOT
// delivered — subscribers receive only future events (forward-only).
//
// ch must be a buffered channel. A zero-capacity channel will cause every
// event to be dropped silently (the crawl is never blocked).
func (r *Recorder) Subscribe(ch chan<- Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs = append(r.subs, ch)
}

// append is the single internal write path. It acquires the write lock,
// appends the event to the log, then forwards the event to each registered
// subscriber in a non-blocking select. A slow or full subscriber channel
// causes the event to be dropped for that subscriber; the crawl is never
// blocked.
func (r *Recorder) append(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	for _, ch := range r.subs {
		select {
		case ch <- e:
		default: // slow consumer: event dropped, crawl not blocked
		}
	}
}

// Events returns a snapshot copy of the event log.
// The returned slice is independent of the recorder's internal state;
// mutations to it do not affect the recorder.
func (r *Recorder) Events() []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Event, len(r.events))
	copy(result, r.events)
	return result
}

func (r *Recorder) RecordFetch(event FetchEvent) {
	r.append(Event{kind: EventKindFetch, fetch: &event})
}

func (r *Recorder) RecordArtifact(record ArtifactRecord) {
	r.append(Event{kind: EventKindArtifact, artifact: &record})
}

func (r *Recorder) RecordPipelineStage(event PipelineEvent) {
	r.append(Event{kind: EventKindPipeline, pipeline: &event})
}

func (r *Recorder) RecordSkip(event SkipEvent) {
	r.append(Event{kind: EventKindSkip, skip: &event})
}

func (r *Recorder) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause ErrorCause,
	details string,
	attrs []Attribute,
) {
	ee := NewErrorEvent(observedAt, packageName, action, cause, details, attrs)
	r.append(Event{kind: EventKindError, error: &ee})
}

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
func (r *Recorder) RecordFinalCrawlStats(stats CrawlStats) {
	r.append(Event{kind: EventKindStats, stats: &stats})
}
