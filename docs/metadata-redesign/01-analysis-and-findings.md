# Metadata Package – Analysis and Findings

## 1. Purpose of This Document

This document records the findings from a systematic review of the `internal/metadata` package and how it is used across all pipeline stages. It identifies the structural problems that prevent metadata from being useful for CLI/TUI integration, post-run audit reports, and failure diagnostics.

The findings are organized from most foundational to most specific.

---

## 2. The Foundational Problem: The Recorder Is Entirely a No-Op

Every method on `Recorder` has an empty body:

```go
func (r *Recorder) RecordFetch(...) {}
func (r *Recorder) RecordAssetFetch(...) {}
func (r *Recorder) RecordArtifact(...) {}
func (r *Recorder) RecordError(...) {}
func (r *Recorder) append(crawlStats) {}
```

The data types exist (`FetchEvent`, `crawlStats`, `ArtifactRecord`), the interfaces are wired into every pipeline stage via dependency injection, and the `MetadataSink` interface is broadly implemented — but nothing is ever stored or emitted. The metadata infrastructure is fully plumbed and entirely inert.

All of the problems described below are compounded by this: even if callers pass the right data, nothing is captured.

---

## 3. Structural Inconsistency Problems

### 3.1 `ArtifactRecord` Struct Is Dead Code

`data.go` defines a rich, typed struct:

```go
type ArtifactRecord struct {
    Kind        ArtifactKind
    WritePath   string
    SourceURL   string
    ContentHash string
    Overwrite   bool
    Bytes       int64
}
```

But `RecordArtifact` in the `MetadataSink` interface takes `(kind ArtifactKind, path string, attrs []Attribute)`. The `ArtifactRecord` struct is never constructed or passed anywhere. Callers are forced to smuggle structured fields through the generic `attrs []Attribute` slice. Type safety and intent are lost entirely.

### 3.2 Ambiguous `AttrField` Reuse in `storage/sink.go`

`LocalSink.Write` passes two attributes with the same key:

```go
metadata.NewAttr(metadata.AttrField, writeResult.URLHash()),
metadata.NewAttr(metadata.AttrField, writeResult.ContentHash()),
```

Both attributes use the key `"field"`. A consumer receiving these events has two indistinguishable attributes with no way to tell which is the URL hash and which is the content hash. The attribute key catalogue (`data.go`) is missing `AttrContentHash` and `AttrURLHash`.

### 3.3 `AttrAssetURL` Defined but Never Used

`data.go` defines:

```go
AttrAssetURL AttributeKey = "asset_url"
```

But in `assets/resolver.go`, asset URLs are carried via `AttrMessage` instead:

```go
metadata.NewAttr(metadata.AttrMessage, urlStr),        // asset URL, wrong key
metadata.NewAttr(metadata.AttrURL, pageUrl.String()),  // page URL, correct key
```

`AttrMessage` is not a URL carrier — its semantics are blurred. The dedicated `AttrAssetURL` key goes unused.

### 3.4 Asset Source URL Lost in `RecordArtifact`

In `assets/resolver.go`, `RecordArtifact` is called via a callback that only receives the local write path:

```go
assetCallback := func(localPath string) {
    r.metadataSink.RecordArtifact(
        metadata.ArtifactAsset,
        localPath,
        []metadata.Attribute{
            metadata.NewAttr(metadata.AttrURL, pageUrl.String()), // ← page URL, not asset URL
        },
    )
}
```

The callback cannot carry the asset's own source URL because the closure only captures `localPath`. A consumer cannot reconstruct "which remote asset URL was downloaded to which local path" from this event alone.

### 3.5 `mdconvert` Error Events Carry No URL Context

`StrictConversionRule.Convert` records errors with empty attributes:

```go
s.metadataSink.RecordError(
    time.Now(),
    "mdconvert",
    "StrictConversionRule.Convert",
    mapConversionErrorToMetadataCause(*conversionError),
    err.Error(),
    []metadata.Attribute{},  // ← no URL
)
```

Unlike every other pipeline stage, conversion errors have no source URL. The `Convert` method doesn't receive the page URL, so it cannot include it. A consumer cannot identify which page caused a conversion failure.

---

## 4. Missing Event Problems

### 4.1 Robots Disallow Decisions Are Silently Dropped

When `robots.txt` disallows a URL, the scheduler has an explicit TODO:

```go
// TODO: record to metadataSink that robots explicitly disallowed the URL
return nil
```

This is one of the most user-visible events. A TUI "skipped URLs" list is completely empty even when `robots.txt` is actively filtering dozens of URLs. There is no `CausePolicyDisallow` event emitted for disallowed URLs.

### 4.2 Robots.txt Fetch Events Are Not Recorded

`CachedRobot.Decide` and `RobotsFetcher` call `RecordError` on failure, but a successful robots.txt fetch for a host produces no fetch event. There is no `RecordFetch` call after a successful robots.txt retrieval. Per-host network activity is incomplete.

### 4.3 Mid-Pipeline Stages Are Observation Dead Zones

Only the first stage (fetch) and last stage (storage write) emit success events. All intermediate stages are silent on success:

| Stage | Success event emitted? | Error event emitted? |
|---|---|---|
| `fetcher.Fetch` | ✅ `RecordFetch` | ✅ `RecordError` |
| `extractor.Extract` | ❌ | ✅ `RecordError` |
| `sanitizer.Sanitize` | ❌ | ✅ `RecordError` |
| `mdconvert.Convert` | ❌ | ✅ `RecordError` (no URL) |
| `normalize.Normalize` | ❌ | ✅ `RecordError` |
| `storage.Write` | ✅ `RecordArtifact` | ✅ `RecordError` |

For a TUI rendering live per-page pipeline progress, there is no way to show a page moving through stages. The only observable facts are: a fetch happened, and eventually either an error was recorded or an artifact was written. All intermediate steps are invisible.

---

## 5. Semantic Asymmetry Problems

### 5.1 `RecordFetch` and `RecordAssetFetch` Cannot Be Unified

`RecordFetch` carries `contentType` and `crawlDepth`:

```go
RecordFetch(fetchUrl string, httpStatus int, duration time.Duration,
    contentType string, retryCount int, crawlDepth int)
```

`RecordAssetFetch` has neither:

```go
RecordAssetFetch(fetchUrl string, httpStatus int, duration time.Duration, retryCount int)
```

A TUI rendering a unified network activity log or per-host request timeline must treat these as incompatible event types rather than two variants of the same fetch concept.

### 5.2 No Absolute Timestamp on `RecordFetch`

`HtmlFetcher.Fetch` captures `startTime := time.Now()` and computes `duration`, but only `duration` is passed to `RecordFetch`. The absolute start timestamp is not forwarded. `FetchResult` carries a `fetchedAt` field but it never reaches the sink.

A TUI timeline ("pages crawled over time") or a per-page fetch timestamp in a report requires absolute timestamps, not just durations.

### 5.3 `RecordFinalCrawlStats` Loses the Crawl Start Time

The scheduler computes `execStartTime := time.Now()` and derives `execDuration`, but only `execDuration` is passed:

```go
s.crawlFinalizer.RecordFinalCrawlStats(
    totalPages, totalErrors, totalAssets, execDuration, retryQueueCount,
)
```

The absolute start time is discarded. A post-run report showing "Crawl started at HH:MM:SS, completed at HH:MM:SS" cannot be produced from recorded data.

---

## 6. Correlation Gap

### 6.1 No Page-Level Identifier Linking Events Across Stages

When `RecordFetch` is emitted for `https://docs.example.com/guide/intro` and later `RecordArtifact` is emitted for `docs/a3f7b2c41d.md`, there is no shared identifier between the two events. A TUI consuming a stream of events cannot say "this artifact came from that fetch". The URL string is the only potential link, but it is present in `RecordFetch` and absent from `RecordArtifact`'s attributes in the storage case — the URL only appears in the error path.

When concurrency is introduced (multiple workers), the absence of a page correlation ID makes the event stream impossible to interpret per-page.

---

## 7. Summary

The metadata recording system has the following categories of problems:

| Category | Issues |
|---|---|
| **Foundational** | Recorder is entirely no-op; no data is ever stored or emitted |
| **Structural** | `ArtifactRecord` struct is dead code; ambiguous attribute keys; wrong attribute keys used |
| **Missing events** | Robots disallow decisions; robots.txt fetch successes; all mid-pipeline success states |
| **Semantic asymmetry** | `RecordFetch` vs `RecordAssetFetch` incompatibility; missing absolute timestamps |
| **Correlation** | No page-level identifier linking fetch → extraction → artifact events |

For CLI/TUI integration, the highest-impact problems to fix first are:
1. The recorder being a no-op (nothing is stored at all)
2. The missing robots disallow events (most visible to users)
3. The absence of mid-pipeline success events (cannot show live progress)
4. The missing page correlation ID (cannot build per-page views with multiple workers)
