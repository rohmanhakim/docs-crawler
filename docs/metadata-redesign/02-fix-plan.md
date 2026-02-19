# Metadata Package – Fix Plan

## 1. Purpose of This Document

This document provides the design guide for fixing the `internal/metadata` package so it becomes a meaningful, usable foundation for CLI/TUI integration, post-run audit reports, and failure diagnostics. It addresses every problem identified in `01-analysis-and-findings.md` and defines the target design for the corrected system.

The plan respects the existing architectural constraints:
- Metadata is **write-only** and must **never influence control flow**
- `ErrorCause` must **never be used for retry, continuation, or abort decisions**
- Determinism guarantees must be preserved
- Package boundaries must not be violated

---

## 2. Guiding Principles for the Fix

### 2.1 Events Are Typed, Not Stringly-Typed

The current system passes structured data through a generic `[]Attribute` slice, using `AttributeKey` strings as the only type discriminator. The fix moves toward structured event types as first-class citizens of the `metadata` package, so callers pass typed structs and receivers get typed data.

### 2.2 Every Pipeline Stage Is Observable

After the fix, every stage emits at minimum one event on both success and failure paths. The event stream must be complete enough for a TUI to render a per-page pipeline progress indicator without gaps.

### 2.3 The Attribute Key Catalogue Must Be Exhaustive and Non-Ambiguous

Every distinct piece of data must have its own `AttributeKey`. No two distinct data fields may share a key. Keys must be semantically self-describing.

### 2.4 Absolute Timestamps Are Required for Timeline Views

All events that represent a point in time (fetch start, crawl start, crawl end) must carry an absolute `time.Time`, not just a `time.Duration`.

### 2.5 URL Is Mandatory Context for All Error Events

Every `RecordError` call must include the source URL that was being processed when the error occurred. This is already the practice for most stages but must be enforced universally.

---

## 3. Fix Area 1: Implement the Recorder

### Problem
Every `Recorder` method is a no-op. Nothing is ever stored or emitted.

### Target Design
The `Recorder` must maintain an in-memory, append-only log of events. Each event is a discriminated type (not a plain string). The log is bounded by crawl duration and is never consulted by the crawl control path.

The recorder must support two consumption modes:
1. **Streaming**: a consumer registers a callback and receives events as they are appended (for live TUI rendering)
2. **Snapshot**: a consumer reads the full event log after the crawl completes (for post-run reports and audit output)

The `Recorder` internal structure should look like:

```go
type Recorder struct {
    workerId string
    mu       sync.Mutex
    events   []Event           // append-only log
    subs     []chan<- Event     // optional streaming subscribers
}
```

Where `Event` is a sealed discriminated union of all event types (see Fix Area 2).

The `append` method becomes the single internal write path:
```go
func (r *Recorder) append(e Event) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.events = append(r.events, e)
    for _, ch := range r.subs {
        select {
        case ch <- e:
        default: // non-blocking: slow consumers don't block the crawl
        }
    }
}
```

### Invariants
- `Recorder` is the only `MetadataSink` that stores data. `NoopSink` remains for testing.
- Reading the event log must never block the crawl.
- The event log must not be consulted by any component that can affect crawl decisions.

---

## 4. Fix Area 2: Define a Typed Event Model

### Problem
All event data flows through loosely typed `[]Attribute` slices. The `ArtifactRecord` struct is defined but never used. There is no sealed event type.

### Target Design
Define a closed set of event types in `data.go`. The `MetadataSink` interface methods accept these typed structs directly, not flat argument lists.

#### 4.1 Replace flat `RecordFetch` with `FetchEvent`

Current signature (flat args):
```go
RecordFetch(fetchUrl string, httpStatus int, duration time.Duration,
    contentType string, retryCount int, crawlDepth int)
```

Target signature (typed struct):
```go
type FetchEvent struct {
    FetchedAt   time.Time
    FetchURL    string
    HTTPStatus  int
    Duration    time.Duration
    ContentType string
    RetryCount  int
    CrawlDepth  int
    Kind        FetchKind  // KindPage | KindAsset | KindRobots
}

RecordFetch(event FetchEvent)
```

The `FetchKind` discriminator unifies page fetches, asset fetches, and robots.txt fetches into a single event type — resolving the `RecordFetch` vs `RecordAssetFetch` asymmetry.

#### 4.2 Activate `ArtifactRecord`

Replace the current `RecordArtifact(kind ArtifactKind, path string, attrs []Attribute)` with:

```go
RecordArtifact(record ArtifactRecord)
```

The existing `ArtifactRecord` struct needs two additional fields:
```go
type ArtifactRecord struct {
    Kind        ArtifactKind
    WritePath   string
    SourceURL   string  // remote URL for assets, page URL for markdown
    ContentHash string
    Overwrite   bool
    Bytes       int64
    RecordedAt  time.Time
}
```

#### 4.3 Add `PipelineEvent` for Mid-Stage Observability

```go
type PipelineStage string

const (
    StageExtract   PipelineStage = "extract"
    StageSanitize  PipelineStage = "sanitize"
    StageConvert   PipelineStage = "convert"
    StageNormalize PipelineStage = "normalize"
)

type PipelineEvent struct {
    Stage      PipelineStage
    PageURL    string
    Success    bool
    RecordedAt time.Time
    // populated on success only:
    LinksFound int    // for extract stage
    // populated on failure: use RecordError instead
}

RecordPipelineStage(event PipelineEvent)
```

#### 4.4 Add `SkipEvent` for Robots Disallow

```go
type SkipReason string

const (
    SkipReasonRobotsDisallow  SkipReason = "robots_disallow"
    SkipReasonOutOfScope      SkipReason = "out_of_scope"
    SkipReasonAlreadyVisited  SkipReason = "already_visited"
)

type SkipEvent struct {
    SkippedURL string
    Reason     SkipReason
    RecordedAt time.Time
}

RecordSkip(event SkipEvent)
```

#### 4.5 Add absolute timestamps to `CrawlStats`

```go
type CrawlStats struct {
    StartedAt             time.Time
    FinishedAt            time.Time
    TotalPages            int
    TotalErrors           int
    TotalAssets           int
    ManualRetryQueueCount int
}

RecordFinalCrawlStats(stats CrawlStats)
```

#### 4.6 Seal the `Event` type for the log

`ErrorRecord` is the canonical typed struct for error events — there is no separate `ErrorEvent` wrapper. It follows the same constructor + accessor pattern as all other record types.

```go
type EventKind string

const (
    EventKindFetch    EventKind = "fetch"
    EventKindArtifact EventKind = "artifact"
    EventKindPipeline EventKind = "pipeline"
    EventKindSkip     EventKind = "skip"
    EventKindError    EventKind = "error"
    EventKindStats    EventKind = "stats"
)

type Event struct {
    Kind      EventKind
    Fetch     *FetchEvent
    Artifact  *ArtifactRecord
    Pipeline  *PipelineEvent
    Skip      *SkipEvent
    Error     *ErrorRecord
    Stats     *CrawlStats
}
```

---

## 5. Fix Area 3: Repair the Attribute Key Catalogue

### Problem
`AttrField` is reused ambiguously. `AttrAssetURL` exists but is not used. Keys for URL hash and content hash are missing.

### Target Design
Add the missing keys and deprecate `AttrField` in favor of specific keys:

```go
const (
    // existing keys — kept
    AttrTime       AttributeKey = "time"
    AttrURL        AttributeKey = "url"
    AttrHost       AttributeKey = "host"
    AttrPath       AttributeKey = "path"
    AttrDepth      AttributeKey = "depth"
    AttrHTTPStatus AttributeKey = "http_status"
    AttrAssetURL   AttributeKey = "asset_url"   // was defined, now actually used
    AttrWritePath  AttributeKey = "write_path"
    AttrMessage    AttributeKey = "message"

    // new keys
    AttrContentHash AttributeKey = "content_hash"
    AttrURLHash     AttributeKey = "url_hash"
    AttrPageURL     AttributeKey = "page_url"    // page context for asset events
    AttrStage       AttributeKey = "stage"       // pipeline stage name

    // deprecated — must not be used in new code
    // AttrField — ambiguous, replaced by specific keys above
)
```

With typed event structs (Fix Area 2), most of these keys become fields on the struct rather than entries in an `attrs` slice. The key catalogue primarily serves `RecordError` which retains its `[]Attribute` variadic for genuinely ad-hoc contextual data.

---

## 6. Fix Area 4: Complete the Event Emission at Each Call Site

### 6.1 `fetcher/html.go`

**Current**: `RecordFetch` is called but `fetchedAt` is not passed (only `duration`).

**Fix**: Pass `FetchEvent` with `FetchedAt: startTime` (the captured `time.Now()`) and `Kind: KindPage`.

### 6.2 `assets/resolver.go`

**Current**: `RecordAssetFetch` is a separate method with incompatible signature. Asset source URL is lost in `assetCallback`. `AttrMessage` is used instead of `AttrAssetURL`.

**Fix**:
- Remove `RecordAssetFetch`; use `RecordFetch` with `Kind: KindAsset`.
- Widen the `assetCallback` signature to accept both `localPath string` and `assetURL string` so `RecordArtifact` can carry the true source URL.
- Use `AttrAssetURL` for asset URLs in `RecordError` calls.

### 6.3 `robots/robot.go` and `robots/fetcher.go`

**Current**: Successful robots.txt fetches emit no event.

**Fix**:
- After a successful fetch, emit `RecordFetch` with `Kind: KindRobots`.
- After a disallow decision, the scheduler must call `RecordSkip` with `Reason: SkipReasonRobotsDisallow`.

### 6.4 `scheduler/scheduler.go`

**Current**: The `TODO` for recording disallowed URLs is unaddressed. `RecordFinalCrawlStats` does not include `startedAt`.

**Fix**:
- Resolve the TODO: call `sink.RecordSkip(...)` in `SubmitUrlForAdmission` when `!robotsDecision.Allowed`.
- Pass `CrawlStats{StartedAt: execStartTime, FinishedAt: time.Now(), ...}` to `RecordFinalCrawlStats`.

### 6.5 `extractor/dom.go`, `sanitizer/html.go`, `normalize/constraints.go`

**Current**: Only errors are recorded. Successes are invisible.

**Fix**: After a successful operation, emit `RecordPipelineStage(PipelineEvent{Stage: ..., PageURL: ..., Success: true})`. For extractor, include `LinksFound` in the success event.

### 6.6 `mdconvert/rules.go`

**Current**: Error is recorded with empty `[]Attribute{}` — no URL context.

**Fix**:
- Pass the source page URL into `Convert` (or propagate it via `SanitizedHTMLDoc`).
- Emit `RecordError` with `AttrURL` populated.
- Emit `RecordPipelineStage` on success.

### 6.7 `storage/sink.go`

**Current**: `RecordArtifact` is called with two `AttrField` attributes for URL hash and content hash.

**Fix**: Call `RecordArtifact(ArtifactRecord{..., ContentHash: ..., WritePath: ..., SourceURL: ...})` using the activated struct.

---

## 7. Fix Area 5: Add Page Correlation ID

### Problem
Events from different stages for the same page cannot be correlated in a stream.

### Target Design
A `pageURL` field on every event type serves as the natural correlation key for the single-worker case. This is simpler and sufficient for the current single-worker architecture.

For future multi-worker support, a `CrawlToken` ID (derived from the frontier's crawl token for a given URL) should be added to events as an explicit `PageID string` field. The frontier already tracks pages with depth information; the token can provide a stable identifier. This is a future extension and does not need to be implemented now, but the event struct fields should be reserved.

---

## 8. Updated Interface Contract

After all fixes, the `MetadataSink` interface becomes:

```go
type MetadataSink interface {
    RecordFetch(event FetchEvent)
    RecordArtifact(record ArtifactRecord)
    RecordPipelineStage(event PipelineEvent)
    RecordSkip(event SkipEvent)
    RecordError(record ErrorRecord)
}

type CrawlFinalizer interface {
    RecordFinalCrawlStats(stats CrawlStats)
}
```

`RecordError` accepts a typed `ErrorRecord` struct — consistent with every other method on the interface. Call sites construct it via `NewErrorRecord(observedAt, packageName, action, cause, errorString, attrs)`.

`NoopSink` must implement the updated interface. All test mocks must be updated accordingly.

---

## 9. What Does Not Change

The following constraints from the original design are **preserved unchanged**:

- Metadata is **write-only**. No pipeline stage may read from the recorder to influence decisions.
- `ErrorCause` must **never be used for retry, continuation, or abort decisions**.
- `NoopSink` remains available for test injection.
- `CrawlFinalizer` remains a separate interface so components that only need to finalize stats are not forced to implement the full sink.
- The `Recorder` is injected via interfaces, not imported directly by pipeline stages.
- `workerId` remains on `Recorder` for future multi-worker support.

---

## 10. Migration Approach

The changes touch the `MetadataSink` interface, which is a breaking change for all callers and all test mocks. The recommended migration order is:

1. Fix the data model and interface first (`internal/metadata/data.go`, `internal/metadata/recorder.go`) — this defines the new contract.
2. Update `NoopSink` to implement the new interface — this is the simplest implementation and unblocks compilation.
3. Implement the `Recorder` body — storage and streaming.
4. Update each pipeline stage call site one at a time — each stage is independent.
5. Update all test mocks last — they are mechanical adaptations.

See `03-fix-phases.md` for the full task breakdown.
