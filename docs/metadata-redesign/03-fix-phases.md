# Metadata Package – Fix Phases and Tasks

## Overview

This document breaks the fix plan from `02-fix-plan.md` into sequential, independently verifiable phases. Each phase has a clear dependency on the previous one. Within a phase, tasks may be done in any order unless explicitly noted.

The goal is to reach a state where the `Recorder` produces a meaningful, complete event stream that a CLI/TUI can consume without additional transformation or inference.

---

## Phase 1 – Establish the New Data Model and Interface Contract

**Goal**: Define the typed event model and update the `MetadataSink` interface. This is a breaking change that must compile before any call sites can be fixed.

**Blocks all other phases.**

---

### Task 1.1 – Define Typed Event Structs in `data.go`

**Problem**  
`FetchEvent` exists but is a flat, unexported struct that is never populated. `ArtifactRecord` exists but is never passed to `RecordArtifact`. There are no types for pipeline stage events, skip events, or a sealed `Event` union.

**What to do**  
Replace the current `FetchEvent` and `crawlStats` structs and add the full set of event types. All new types are exported. All time fields are `time.Time`, not `time.Duration`.

Specifically, define or update:
- `FetchKind` (string enum: `KindPage`, `KindAsset`, `KindRobots`)
- `FetchEvent` — exported, includes `FetchedAt time.Time`, `Kind FetchKind`
- `ArtifactRecord` — add `RecordedAt time.Time`; all fields already present are kept
- `PipelineStage` (string enum: `StageExtract`, `StageSanitize`, `StageConvert`, `StageNormalize`)
- `PipelineEvent` — exported struct with `Stage`, `PageURL`, `Success bool`, `RecordedAt`, `LinksFound int`
- `SkipReason` (string enum: `SkipReasonRobotsDisallow`, `SkipReasonOutOfScope`, `SkipReasonAlreadyVisited`)
- `SkipEvent` — exported struct with `SkippedURL`, `Reason`, `RecordedAt`
- `CrawlStats` — replaces `crawlStats`; exported; adds `StartedAt time.Time`, `FinishedAt time.Time`
- `ErrorRecord` — exported struct with `packageName`, `action`, `cause ErrorCause`, `errorString`, `observedAt time.Time`, and `attrs []Attribute`; this is the typed struct accepted by `RecordError`, replacing flat args; follows the same constructor + accessor pattern as all other record types; the `attrs` slice is copied on construction to prevent external mutation
- `EventKind` (string enum: `EventKindFetch`, `EventKindArtifact`, `EventKindPipeline`, `EventKindSkip`, `EventKindError`, `EventKindStats`)
- `Event` — sealed discriminated union with a `Kind EventKind` field and one pointer field per event type; the `error` field is typed `*ErrorRecord`

**Acceptance Criteria**  
- All new types are in `internal/metadata/data.go`
- All new types are exported
- All new types have at least one test in `internal/metadata/data_test.go` verifying construction
- `crawlStats` (unexported) is removed and replaced by `CrawlStats` (exported)
- The existing `ArtifactKind` constants and `ErrorCause` constants are unchanged
- `go vet ./internal/metadata/...` passes

---

### Task 1.2 – Update the `MetadataSink` Interface and `CrawlFinalizer` Interface

**Problem**  
`MetadataSink` has flat argument lists (`RecordFetch(fetchUrl string, ...)`) and includes `RecordAssetFetch` as a separate method. `CrawlFinalizer` takes flat args instead of `CrawlStats`.

**What to do**  
Update `recorder.go`:

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

Remove `RecordAssetFetch` from the interface entirely. `RecordError` accepts the typed `ErrorRecord` struct rather than flat args — making it consistent with every other method on the interface.

**Acceptance Criteria**  
- `MetadataSink` no longer contains `RecordAssetFetch`
- `MetadataSink.RecordFetch` accepts `FetchEvent`, not flat args
- `MetadataSink.RecordArtifact` accepts `ArtifactRecord`, not flat args
- `MetadataSink.RecordError` accepts `ErrorRecord`, not flat args
- `MetadataSink` has `RecordPipelineStage` and `RecordSkip`
- `CrawlFinalizer.RecordFinalCrawlStats` accepts `CrawlStats`, not flat args
- The project does NOT compile at this point (call sites are broken) — this is expected

---

### Task 1.3 – Update `NoopSink` to Implement the New Interface

**Problem**  
`NoopSink` implements the old interface. It is the fastest path to restoring compilation.

**What to do**  
Update every method on `NoopSink` in `recorder.go` to match the new interface signatures. All bodies remain no-ops (`{}`).

**Acceptance Criteria**  
- `NoopSink` compiles against the new `MetadataSink` interface
- `NoopSink` implements `MetadataSink` — verify with `var _ MetadataSink = (*NoopSink)(nil)`
- `go build ./internal/metadata/...` passes (the metadata package itself compiles cleanly)

---

### Task 1.4 – Update the Attribute Key Catalogue

**Problem**  
`AttrField` is ambiguous. `AttrAssetURL` exists but is unused. `AttrContentHash`, `AttrURLHash`, `AttrPageURL`, `AttrStage` are missing.

**What to do**  
In `data.go`:
- Add `AttrContentHash`, `AttrURLHash`, `AttrPageURL`, `AttrStage`
- Add a code comment on `AttrField` marking it deprecated with the replacement instruction
- Do not remove `AttrField` yet — it is referenced by existing call sites that will be fixed in Phase 3

**Acceptance Criteria**  
- New attribute keys are present and exported
- `AttrField` has a deprecation comment
- `go vet ./internal/metadata/...` passes

---

## Phase 2 – Implement the Recorder

**Goal**: Make `Recorder` actually store and emit events. This phase is self-contained and does not require any call site changes.

**Blocked by: Phase 1**

---

### Task 2.1 – Implement the `Recorder` Append-Only Event Log

**Problem**  
All `Recorder` methods have empty bodies. No data is ever stored.

**What to do**  
In `recorder.go`, implement the `Recorder` struct with:
- An `events []Event` field (the append-only log)
- A `sync.RWMutex` for concurrent access safety
- A private `append(Event)` method that appends to the log under the write lock

Implement all `MetadataSink` methods to construct the appropriate `Event` and call `append`. Each method wraps its input in the correct event type and delegates to `append`.

Example for `RecordFetch`:
```go
func (r *Recorder) RecordFetch(event FetchEvent) {
    r.append(Event{Kind: EventKindFetch, Fetch: &event})
}
```

Implement `RecordFinalCrawlStats` to satisfy `CrawlFinalizer`.

Add a `Events() []Event` read method (snapshot):
```go
func (r *Recorder) Events() []Event {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make([]Event, len(r.events))
    copy(result, r.events)
    return result
}
```

**Acceptance Criteria**  
- All `Recorder` methods have non-empty bodies
- `Events()` returns a copy of the log (not the backing slice)
- A table-driven test in `data_test.go` or a new `recorder_test.go` verifies that calling each `Record*` method adds exactly one event to the log with the correct `Kind`
- `go test ./internal/metadata/...` passes

---

### Task 2.2 – Implement Streaming Subscribers on `Recorder`

**Problem**  
There is no mechanism for a live TUI to receive events as they are produced.

**What to do**  
Add a `Subscribe(ch chan<- Event)` method to `Recorder`:
```go
func (r *Recorder) Subscribe(ch chan<- Event) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.subs = append(r.subs, ch)
}
```

Update the private `append` method to forward to each subscriber channel in a non-blocking `select`:
```go
for _, ch := range r.subs {
    select {
    case ch <- e:
    default: // slow consumer: event dropped, crawl not blocked
    }
}
```

**Acceptance Criteria**  
- A subscriber registered before any events are recorded receives all subsequent events
- A slow subscriber (full channel) does not block `append`
- A subscriber registered after some events does NOT receive past events (forward-only)
- Tests cover: subscriber receives event, full channel does not block
- `go test ./internal/metadata/...` passes

---

## Phase 3 – Fix Call Sites in Pipeline Stages

**Goal**: Every pipeline stage emits correct, complete events using the new interface. The attribute key catalogue is used correctly.

**Blocked by: Phase 1 (interface must compile). Phase 2 is not a hard dependency but should be done first for a coherent implementation.**

Each task in this phase is independent of the others (different packages). They can be done in any order.

---

### Task 3.1 – Fix `fetcher/html.go`

**Problem**  
`RecordFetch` is called with the old flat signature. `fetchedAt` (absolute timestamp) is not passed. The call does not include `FetchKind`.

**What to do**  
- Update the `RecordFetch` call to pass a `FetchEvent{FetchedAt: startTime, Kind: KindPage, ...}` where `startTime` is the `time.Now()` captured at the top of `Fetch`.
- Remove the old `RecordAssetFetch` call site (the method no longer exists on the interface).
- Update the test mock in `fetcher/html_test.go` to implement the new `MetadataSink` interface.

**Acceptance Criteria**  
- `FetchEvent` passed to `RecordFetch` has `Kind == KindPage` and a non-zero `FetchedAt`
- The test mock compiles against the new interface
- `go test ./internal/fetcher/...` passes

---

### Task 3.2 – Fix `assets/resolver.go`

**Problem**  
- `RecordAssetFetch` is called (method no longer exists)
- Asset source URL is lost in `assetCallback` closure
- `AttrMessage` is used as an asset URL carrier instead of `AttrAssetURL`

**What to do**  
- Replace `RecordAssetFetch(...)` call with `RecordFetch(FetchEvent{Kind: KindAsset, ...})`
- Widen the `fetchEventCallback` function type to carry the asset URL explicitly, and update callers
- Widen the `assetCallback` function type from `func(localPath string)` to `func(localPath string, assetURL string)` and update all call sites within `resolve`
- In the widened `assetCallback`, call `RecordArtifact(ArtifactRecord{Kind: ArtifactAsset, WritePath: localPath, SourceURL: assetURL, RecordedAt: time.Now()})` 
- Replace `metadata.NewAttr(metadata.AttrMessage, urlStr)` with `metadata.NewAttr(metadata.AttrAssetURL, urlStr)` in all `RecordError` calls within this file
- Update the test mock in `assets/resolver_helper_test.go` to implement the new interface

**Acceptance Criteria**  
- No reference to `RecordAssetFetch` remains in `assets/resolver.go`
- `RecordArtifact` for assets includes a non-empty `SourceURL` matching the remote asset URL
- `RecordError` calls use `AttrAssetURL` (not `AttrMessage`) when referring to asset URLs
- `go test ./internal/assets/...` passes

---

### Task 3.3 – Fix `robots/robot.go` and `robots/fetcher.go`

**Problem**  
Successful robots.txt fetches emit no event. The `RecordError` call sites use the old flat signature (broken by Phase 1 only if RecordError signature changed — it did not, so only the mock needs updating).

**What to do**  
In `robots/robot.go` (`CachedRobot.Decide`):
- After a successful `fetcher.Fetch` call and before mapping to a rule set, emit:
  ```go
  r.metadataSink.RecordFetch(metadata.FetchEvent{
      Kind:       metadata.KindRobots,
      FetchURL:   robotsTxtURL,
      HTTPStatus: fetchResult.StatusCode,
      FetchedAt:  fetchResult.FetchedAt,
      Duration:   fetchResult.Duration,
  })
  ```
- Update test mocks to implement the new `MetadataSink` interface.

In `robots/fetcher.go`:
- Ensure the `RobotsFetcher` returns enough information from the HTTP response for the robot to populate `FetchEvent` above (status code, duration, fetched-at timestamp). If `FetchResult` doesn't carry these, extend it.

**Acceptance Criteria**  
- A successful robots.txt fetch produces exactly one `FetchEvent` with `Kind == KindRobots`
- The fetched-at timestamp is the time of the HTTP request, not a stub zero value
- `go test ./internal/robots/...` passes

---

### Task 3.4 – Fix `extractor/dom.go`

**Problem**  
`Extract` only records errors. Successful extractions are invisible. The `RecordError` call is already correct structurally (has `AttrURL`).

**What to do**  
- After a successful `d.extract(htmlByte)` call, emit:
  ```go
  d.metadataSink.RecordPipelineStage(metadata.PipelineEvent{
      Stage:      metadata.StageExtract,
      PageURL:    sourceUrl.String(),
      Success:    true,
      LinksFound: len(result.DiscoveredURLs), // if accessible
      RecordedAt: time.Now(),
  })
  ```
- Update test mock in `extractor/dom_test.go` to implement the new interface.

**Acceptance Criteria**  
- A successful extraction emits one `PipelineEvent` with `Stage == StageExtract` and `Success == true`
- `LinksFound` is non-negative
- `go test ./internal/extractor/...` passes

---

### Task 3.5 – Fix `sanitizer/html.go`

**Problem**  
`Sanitize` only records errors. Successful sanitization is invisible.

**What to do**  
- After a successful `sanitize(inputContentNode)` call, emit:
  ```go
  h.metadataSink.RecordPipelineStage(metadata.PipelineEvent{
      Stage:      metadata.StageSanitize,
      PageURL:    "",  // sanitizer does not receive the URL — see note below
      Success:    true,
      RecordedAt: time.Now(),
  })
  ```

> **Note on missing URL**: `Sanitize` does not currently receive the page URL. The `PipelineEvent` is still useful for counting stage throughput, even without the URL. As a future improvement, the `Sanitize` interface method can be extended to accept a `pageURL string` parameter. For this task, emit the event with an empty `PageURL` and add a TODO comment.

- Update test mock in `sanitizer/html_helper_test.go` to implement the new interface.

**Acceptance Criteria**  
- A successful sanitization emits one `PipelineEvent` with `Stage == StageSanitize` and `Success == true`
- A TODO comment documents the missing URL
- `go test ./internal/sanitizer/...` passes

---

### Task 3.6 – Fix `mdconvert/rules.go`

**Problem**  
Error events have empty `[]Attribute{}` — no URL context. Success is not recorded.

**What to do**  
- The `Convert` method signature does not currently receive the source URL. Add it:
  ```go
  Convert(sanitizedHTMLDoc sanitizer.SanitizedHTMLDoc, pageURL string) (ConversionResult, failure.ClassifiedError)
  ```
  Update the `ConvertRule` interface and all callers (scheduler).
- On error, emit `RecordError` with `metadata.NewAttr(metadata.AttrURL, pageURL)`.
- On success, emit:
  ```go
  s.metadataSink.RecordPipelineStage(metadata.PipelineEvent{
      Stage:      metadata.StageConvert,
      PageURL:    pageURL,
      Success:    true,
      RecordedAt: time.Now(),
  })
  ```
- Update test mock in `mdconvert/rules_test.go` to implement the new interface.

**Acceptance Criteria**  
- `ConvertRule.Convert` accepts `pageURL string` as second parameter
- Conversion error events include `AttrURL` with the page URL
- Successful conversion emits one `PipelineEvent` with `Stage == StageConvert`, `Success == true`, and a non-empty `PageURL`
- `go test ./internal/mdconvert/...` passes

---

### Task 3.7 – Fix `normalize/constraints.go`

**Problem**  
`Normalize` only records errors. Successful normalization is invisible.

**What to do**  
- After a successful `normalize(...)` call, emit:
  ```go
  m.metadataSink.RecordPipelineStage(metadata.PipelineEvent{
      Stage:      metadata.StageNormalize,
      PageURL:    fetchUrl.String(),
      Success:    true,
      RecordedAt: time.Now(),
  })
  ```
- Update test mock in `normalize/constraints_helper_test.go` to implement the new interface.

**Acceptance Criteria**  
- A successful normalization emits one `PipelineEvent` with `Stage == StageNormalize`, `Success == true`, and a non-empty `PageURL`
- `go test ./internal/normalize/...` passes

---

### Task 3.8 – Fix `storage/sink.go`

**Problem**  
`RecordArtifact` is called with the old flat signature and uses `AttrField` twice for `URLHash` and `ContentHash`.

**What to do**  
- Replace the current `RecordArtifact` call with:
  ```go
  s.metadataSink.RecordArtifact(metadata.ArtifactRecord{
      Kind:        metadata.ArtifactMarkdown,
      WritePath:   writeResult.Path(),
      SourceURL:   normalizedDoc.Frontmatter().SourceURL(),
      ContentHash: writeResult.ContentHash(),
      Bytes:       int64(len(normalizedDoc.Content())),
      RecordedAt:  time.Now(),
  })
  ```
- The `URLHash` field has no place in `ArtifactRecord` — it is an internal storage detail, not observable metadata. It should not be emitted. Remove the `AttrField` usage.
- Update test mock in `storage/sink_helper_test.go` to implement the new interface.

**Acceptance Criteria**  
- `RecordArtifact` is called with an `ArtifactRecord` struct
- No `AttrField` is used for `ContentHash` or `URLHash`
- `ArtifactRecord.SourceURL` contains the page URL
- `ArtifactRecord.Bytes` is the content length
- `go test ./internal/storage/...` passes

---

## Phase 4 – Fix the Scheduler Call Sites

**Goal**: The scheduler emits skip events for robots disallow decisions and passes complete `CrawlStats`.

**Blocked by: Phase 1 and Phase 3 (call sites in pipeline stages must compile before scheduler tests pass end-to-end).**

---

### Task 4.1 – Emit `SkipEvent` for Robots Disallow in `scheduler.go`

**Problem**  
When `robotsDecision.Allowed == false`, the scheduler returns nil with a TODO comment and no event is emitted.

**What to do**  
In `SubmitUrlForAdmission`, after the `!robotsDecision.Allowed` check, call:
```go
s.metadataSink.RecordSkip(metadata.SkipEvent{
    SkippedURL: canonicalURL.String(),
    Reason:     metadata.SkipReasonRobotsDisallow,
    RecordedAt: time.Now(),
})
```
Remove the TODO comment.

**Acceptance Criteria**  
- When a URL is disallowed by robots.txt, exactly one `SkipEvent` with `Reason == SkipReasonRobotsDisallow` is emitted
- The `SkippedURL` is the canonical form of the disallowed URL
- The existing scheduler tests for robots disallow behavior still pass
- `go test ./internal/scheduler/...` passes

---

### Task 4.2 – Pass Complete `CrawlStats` to `RecordFinalCrawlStats`

**Problem**  
`RecordFinalCrawlStats` receives flat args with no absolute timestamps. The crawl start time is computed locally in `ExecuteCrawlingWithState` but never forwarded.

**What to do**  
In `ExecuteCrawlingWithState`:
- Capture `execStartTime := time.Now()` (already done)
- In the deferred stats recording block, build:
  ```go
  stats := metadata.CrawlStats{
      StartedAt:             execStartTime,
      FinishedAt:            time.Now(),
      TotalPages:            s.frontier.VisitedCount(),
      TotalErrors:           totalErrors,
      TotalAssets:           totalAssets,
      ManualRetryQueueCount: s.failureJournal.Count(),
  }
  s.crawlFinalizer.RecordFinalCrawlStats(stats)
  ```
- Update `helper_finalizer_test.go` mock to implement the new `CrawlFinalizer` interface.

**Acceptance Criteria**  
- `CrawlStats.StartedAt` is non-zero and represents the beginning of the execution phase
- `CrawlStats.FinishedAt` is after `StartedAt`
- Existing scheduler stats tests still pass
- `go test ./internal/scheduler/...` passes

---

### Task 4.3 – Update All Scheduler Test Mocks

**Problem**  
The scheduler's test helpers (`helper_metadata_test.go`, `helper_finalizer_test.go`) implement the old interface signatures and will fail to compile against the new interface.

**What to do**  
- Update `errorRecordingSink` in `helper_metadata_test.go` to implement the full new `MetadataSink` interface (add `RecordFetch(FetchEvent)`, `RecordArtifact(ArtifactRecord)`, `RecordPipelineStage(PipelineEvent)`, `RecordSkip(SkipEvent)`)
- Update `mockFinalizer` in `helper_finalizer_test.go` to accept `CrawlStats` in `RecordFinalCrawlStats`
- Add compile-time interface checks:
  ```go
  var _ metadata.MetadataSink = (*errorRecordingSink)(nil)
  var _ metadata.CrawlFinalizer = (*mockFinalizer)(nil)
  ```

**Acceptance Criteria**  
- All scheduler tests compile
- `go test ./internal/scheduler/...` passes

---

## Phase 5 – End-to-End Verification

**Goal**: Verify the complete event stream is coherent, non-empty, and usable.

**Blocked by: All previous phases.**

---

### Task 5.1 – Verify Event Stream Completeness with an Integration Test

**Problem**  
There is no test that verifies the full event stream produced by a complete crawl pipeline execution contains the expected event sequence.

**What to do**  
Write a test in `internal/scheduler/` that:
1. Runs a full pipeline execution with a real (but minimal) `Recorder` instead of `NoopSink`
2. After execution, reads `recorder.Events()`
3. Asserts the following event types are present, in a sensible order:
   - At least one `EventKindFetch` with `Kind == KindRobots`
   - At least one `EventKindFetch` with `Kind == KindPage`
   - At least one `EventKindPipeline` for `StageExtract`
   - At least one `EventKindPipeline` for `StageSanitize`
   - At least one `EventKindPipeline` for `StageConvert`
   - At least one `EventKindPipeline` for `StageNormalize`
   - At least one `EventKindArtifact` with `Kind == ArtifactMarkdown`
   - Exactly one `EventKindStats`
4. Assert that the `CrawlStats` event has non-zero `StartedAt` and `FinishedAt`

**Acceptance Criteria**  
- The integration test passes with a real `Recorder`
- The event log contains at least one event per pipeline stage
- `CrawlStats` timestamps are both non-zero and `FinishedAt` is after `StartedAt`
- `go test ./internal/scheduler/...` passes

---

### Task 5.2 – Verify `SkipEvent` Is Emitted for Robots-Disallowed URLs

**Problem**  
No test verifies that the skip event path is exercised end-to-end through the scheduler.

**What to do**  
Write or extend a scheduler test that:
1. Configures a robots.txt response that disallows a specific path
2. Attempts to submit that URL for admission
3. Reads the event log from the `Recorder`
4. Asserts exactly one `EventKindSkip` with `Reason == SkipReasonRobotsDisallow` and `SkippedURL` matching the disallowed URL

**Acceptance Criteria**  
- A robots-disallowed URL produces exactly one `SkipEvent` in the recorder
- The `SkippedURL` is in canonical form
- `go test ./internal/scheduler/...` passes

---

## Phase Summary

| Phase | What It Delivers | Blocks |
|---|---|---|
| 1 – Data Model & Interface | Typed event structs, updated interface, NoopSink, attribute keys | Everything |
| 2 – Recorder Implementation | Working in-memory event log, streaming subscribers | Phase 3+ (logically) |
| 3 – Pipeline Call Sites | All stages emit correct, complete events | Phase 4 |
| 4 – Scheduler Call Sites | Skip events, complete CrawlStats, updated mocks | Phase 5 |
| 5 – End-to-End Verification | Proof of complete event stream | CLI/TUI integration |
