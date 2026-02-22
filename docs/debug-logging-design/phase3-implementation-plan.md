# Phase 3: Component Integration - Implementation Plan

## 1. Overview

This document provides a detailed implementation plan for integrating the debug logger into the retry handler, rate limiter, and all pipeline stages. It serves as a guide for developers and AI agents during implementation.

**Prerequisites:** Phase 1 (Core Infrastructure) and Phase 2 (Scheduler Integration) are complete. The `pkg/debug` package is fully implemented with `DebugLogger` interface, `NoOpLogger`, `SlogLogger`, and all supporting types.

## 2. Implementation Order

Components should be integrated in the following order to minimize dependencies:

1. **Retry Handler** - Foundation for retry logging, used by fetcher and asset resolver
2. **Rate Limiter** - Independent component, no dependencies on other components
3. **Fetcher** - First pipeline stage, uses retry handler
4. **Extractor** - Second pipeline stage
5. **Sanitizer** - Third pipeline stage
6. **Robots** - Admission checking before frontier
7. **Frontier** - URL queuing and deduplication
8. **Scheduler** - Update to pass logger to all components
9. **Other Stages** - Markdown converter, asset resolver, normalizer, storage

---

## 3. Retry Handler Integration

### 3.1 Target File
`pkg/retry/handler.go`

### 3.2 Current State
- Generic function `Retry[T any](retryParam RetryParam, fn func() (T, failure.ClassifiedError)) Result[T]`
- No debug logging capability
- Returns `Result[T]` with attempts count

### 3.3 Required Changes

#### 3.3.1 Function Signature Update
Add `debug.DebugLogger` as the second parameter:

```
Before: Retry[T any](retryParam RetryParam, fn func() (T, failure.ClassifiedError)) Result[T]
After:  Retry[T any](retryParam RetryParam, logger debug.DebugLogger, fn func() (T, failure.ClassifiedError)) Result[T]
```

#### 3.3.2 Logging Points

| Location | Step Name | Condition | Fields to Log |
|----------|-----------|-----------|---------------|
| Before retry loop | `retry_start` | Always | `max_attempts` |
| After failed attempt | N/A (use LogRetry) | Error and should retry | `attempt`, `max_attempts`, `backoff_ms`, `error` |
| After successful retry | N/A (use LogRetry) | Success | `attempt`, `max_attempts` |
| After exhausted | N/A (use LogRetry) | All attempts failed | `attempt`, `max_attempts`, `error` |

#### 3.3.3 Implementation Notes

1. **Early return optimization**: Check `logger.Enabled()` before any logging to avoid overhead
2. **Context**: Use `context.Background()` since retry doesn't have request context
3. **Backoff calculation**: Log the computed backoff delay before `time.Sleep()`

#### 3.3.4 Caller Updates Required

The following files call `retry.Retry()` and must be updated:
- `internal/fetcher/html.go` - `fetchWithRetry()` method
- `internal/assets/resolver.go` - asset download retry

### 3.4 Testing Strategy

1. Update existing tests in `pkg/retry/handler_test.go` to pass `NoOpLogger`
2. Add new test with `CaptureLogger` to verify log entries
3. Test with both successful and failed retry scenarios

---

## 4. Rate Limiter Integration

### 4.1 Target File
`pkg/limiter/rate.go`

### 4.2 Current State
- `ConcurrentRateLimiter` struct with mutex-protected fields
- `RateLimiter` interface without debug capability
- Methods: `SetBaseDelay`, `SetCrawlDelay`, `Backoff`, `ResetBackoff`, `ResolveDelay`

### 4.3 Required Changes

#### 4.3.1 Struct Modification
Add a `debugLogger` field to `ConcurrentRateLimiter`:

```
type ConcurrentRateLimiter struct {
    mu           sync.RWMutex
    rngMu        sync.Mutex
    baseDelay    time.Duration
    jitter       time.Duration
    hostTimings  map[string]hostTiming
    rng          *rand.Rand
    backoffParam timeutil.BackoffParam
    debugLogger  debug.DebugLogger  // NEW
}
```

#### 4.3.2 Interface Preservation
Do NOT modify the `RateLimiter` interface. Instead, add a `SetDebugLogger` method to `ConcurrentRateLimiter` only:

```
func (r *ConcurrentRateLimiter) SetDebugLogger(logger debug.DebugLogger)
```

This allows optional debug logging without breaking the interface.

#### 4.3.3 Constructor Update
Initialize with `NoOpLogger` in `NewConcurrentRateLimiter()`:

```
debugLogger: debug.NewNoOpLogger(),
```

#### 4.3.4 Logging Points

| Method | Step Name | Fields to Log |
|--------|-----------|---------------|
| `ResolveDelay()` | N/A (use LogRateLimit) | `host`, `delay_ms`, `rate_limit_reason`, `base_delay_ms`, `crawl_delay_ms`, `backoff_delay_ms`, `jitter_ms` |
| `Backoff()` | `backoff_triggered` | `host`, `backoff_count`, `backoff_delay_ms` |
| `SetCrawlDelay()` | `crawl_delay_set` | `host`, `crawl_delay_ms` |

#### 4.3.5 Rate Limit Reason Logic

In `ResolveDelay()`, determine the reason based on which delay factor is dominant:

| Condition | Reason |
|-----------|--------|
| `backoffDelay >= baseDelay && backoffDelay >= crawlDelay` | `RateLimitReasonBackoff` |
| `crawlDelay >= baseDelay && crawlDelay >= backoffDelay` | `RateLimitReasonCrawlDelay` |
| `baseDelay > 0` | `RateLimitReasonBaseDelay` |

#### 4.3.6 Thread Safety
The debug logger is read-only after initialization. No additional locking needed for logging calls.

### 4.4 Testing Strategy

1. Add `SetDebugLogger` to test setup
2. Verify log entries contain correct delay values
3. Test all three delay scenarios (base, crawl_delay, backoff)

---

## 5. Fetcher Integration

### 5.1 Target File
`internal/fetcher/html.go`

### 5.2 Current State
- `HtmlFetcher` struct with `metadataSink`, `httpClient`, `userAgent`
- `Fetch()` method performs HTTP requests
- Uses `retry.Retry()` internally via `fetchWithRetry()`

### 5.3 Required Changes

#### 5.3.1 Struct Modification
Add `debugLogger` field:

```
type HtmlFetcher struct {
    metadataSink metadata.MetadataSink
    httpClient   *http.Client
    userAgent    string
    debugLogger  debug.DebugLogger  // NEW
}
```

#### 5.3.2 Constructor Update
Initialize with `NoOpLogger` in `NewHtmlFetcher()`:

```
debugLogger: debug.NewNoOpLogger(),
```

#### 5.3.3 Setter Method
Add `SetDebugLogger()` method:

```
func (h *HtmlFetcher) SetDebugLogger(logger debug.DebugLogger)
```

#### 5.3.4 Logging Points

| Method | Step Name | Fields to Log |
|--------|-----------|---------------|
| `performFetch()` | `create_request` | `url`, `method`, `user_agent` |
| `performFetch()` | `response_received` | `status_code`, `content_type`, `content_length` |
| `performFetch()` | `body_read` | `body_size` |
| `performFetch()` | `redirect_detected` | `location`, `status_code` |

#### 5.3.5 Retry Handler Update
Update `fetchWithRetry()` to pass debug logger to `retry.Retry()`:

```
return retry.Retry(retryParam, h.debugLogger, fetchTask)
```

### 5.4 Testing Strategy

1. Mock debug logger in existing tests
2. Verify step sequence for successful fetch
3. Verify logging for error cases (4xx, 5xx, timeout)

---

## 6. Extractor Integration

### 6.1 Target File
`internal/extractor/dom.go`

### 6.2 Current State
- `DomExtractor` struct with `metadataSink`, `customSelectors`, `params`
- `Extract()` method processes HTML into DOM and extracts content
- Multi-layer extraction: semantic containers, known selectors, heuristic

### 6.3 Required Changes

#### 6.3.1 Struct Modification
Add `debugLogger` field:

```
type DomExtractor struct {
    metadataSink    metadata.MetadataSink
    customSelectors []string
    params          ExtractParam
    debugLogger     debug.DebugLogger  // NEW
}
```

#### 6.3.2 Constructor and Setter
Same pattern as fetcher.

#### 6.3.3 Logging Points

| Method | Step Name | Fields to Log |
|--------|-----------|---------------|
| `extract()` | `parse_html` | `input_size_bytes` |
| `extract()` | `layer_0_blacklist` | `selectors_count`, `removed_count` |
| `extract()` | `layer_1_semantic` | `found`, `selector` (main, article, role="main") |
| `extract()` | `layer_2_known` | `found`, `selector`, `match_count` |
| `extract()` | `layer_3_heuristic` | `found`, `content_score`, `candidate_tag` |
| `extract()` | `content_selected` | `final_layer`, `node_tag`, `has_h1` |

#### 6.3.4 Layer Result Logging
After each extraction layer, log whether content was found:

```
if contentNode != nil {
    h.debugLogger.LogStep(ctx, "extractor", "layer_1_semantic", debug.FieldMap{
        "found":    true,
        "selector": "main",
    })
}
```

### 6.4 Testing Strategy

1. Test with various HTML fixtures
2. Verify correct layer selection logging
3. Test edge cases (no content found, multiple candidates)

---

## 7. Sanitizer Integration

### 7.1 Target File
`internal/sanitizer/html.go`

### 7.2 Current State
- `HtmlSanitizer` struct with `metadataSink`
- `Sanitize()` method normalizes HTML structure
- Operations: heading normalization, chrome removal, URL extraction

### 7.3 Required Changes

#### 7.3.1 Struct Modification
Add `debugLogger` field:

```
type HtmlSanitizer struct {
    metadataSink metadata.MetadataSink
    debugLogger  debug.DebugLogger  // NEW
}
```

#### 7.3.2 Constructor and Setter
Same pattern as fetcher.

#### 7.3.3 Logging Points

| Method | Step Name | Fields to Log |
|--------|-----------|---------------|
| `sanitize()` | `validate_input` | `has_content`, `first_child_type` |
| `sanitize()` | `check_repairable` | `repairable`, `reason` |
| `normalizeHeadingLevels()` | `normalize_headings` | `headings_before_count`, `headings_after_count`, `renumbered_count` |
| `removePreH1Chrome()` | `remove_pre_h1_chrome` | `removed_count` |
| `removeDuplicateAndEmptyNode()` | `remove_empty_nodes` | `removed_count` |
| `removeDuplicateAndEmptyNode()` | `remove_duplicates` | `removed_count` |
| `extractUrl()` | `extract_urls` | `urls_found`, `skipped_fragment`, `skipped_invalid` |

### 7.4 Testing Strategy

1. Test with malformed HTML fixtures
2. Verify heading normalization logging
3. Test URL extraction logging

---

## 8. Robots Integration

### 8.1 Target File
`internal/robots/robot.go`

### 8.2 Current State
- `CachedRobot` struct with `metadataSink`, `fetcher`, `userAgent`
- `Decide()` method checks robots.txt rules for URL permission
- Uses internal `fetcher` for robots.txt retrieval

### 8.3 Required Changes

#### 8.3.1 Struct Modification
Add `debugLogger` field:

```
type CachedRobot struct {
    metadataSink metadata.MetadataSink
    fetcher      *RobotsFetcher
    userAgent    string
    debugLogger  debug.DebugLogger  // NEW
}
```

#### 8.3.2 Constructor and Setter
Same pattern as fetcher. Update both `NewCachedRobot()` and `Init()`.

#### 8.3.3 Logging Points

| Method | Step Name | Fields to Log |
|--------|-----------|---------------|
| `Decide()` | `robots_fetch` | `host`, `from_cache`, `http_status` |
| `Decide()` | `parse_rules` | `allow_rules_count`, `disallow_rules_count`, `crawl_delay_ms` |
| `decide()` | `match_rules` | `path`, `matched_rule`, `match_type` |
| `decide()` | `decision_made` | `allowed`, `reason`, `crawl_delay_ms` |

#### 8.3.4 Decision Reasons
Log the specific `DecisionReason` enum value:

| Reason | Description |
|--------|-------------|
| `EmptyRuleSet` | No rules found (404 or empty file) |
| `UserAgentNotMatched` | Rules exist but no match for our user-agent |
| `NoMatchingRules` | Rules exist but none match the path |
| `AllowedByRobots` | Explicitly allowed by rule |
| `DisallowedByRobots` | Explicitly disallowed by rule |

### 8.4 Testing Strategy

1. Test with mock robots.txt responses
2. Verify rule matching logging
3. Test cache hit vs cache miss logging

---

## 9. Frontier Integration

### 9.1 Target File
`internal/frontier/frontier.go`

### 9.2 Current State
- `CrawlFrontier` struct with queues, visited set, max depth/pages
- High-frequency operations: `Submit`, `Dequeue`
- Important invariant: Frontier MUST NOT influence crawl control flow

### 9.3 Required Changes

#### 9.3.1 Struct Modification
Add `debugLogger` field:

```
type CrawlFrontier struct {
    mu            sync.RWMutex
    queuesByDepth map[int]*collections.FIFOQueue[CrawlToken]
    visitedUrl    collections.Set[string]
    maxDepth      int
    currentDepth  int
    maxPages      int
    debugLogger   debug.DebugLogger  // NEW
}
```

#### 9.3.2 Selective Logging
**IMPORTANT**: Frontier is a high-frequency component. Only log:
- URLs that are **skipped** (depth exceeded, max pages, duplicate)
- **Depth transitions** when `currentDepth` changes

Do NOT log every successful submit/dequeue operation.

#### 9.3.3 Logging Points

| Method | Step Name | Condition | Fields to Log |
|--------|-----------|-----------|---------------|
| `Submit()` | `submit_skipped_depth` | `depth > maxDepth` | `url`, `depth`, `max_depth` |
| `Submit()` | `submit_skipped_max_pages` | `visitedUrl.Size() == maxPages` | `url`, `max_pages`, `visited_count` |
| `Submit()` | `submit_skipped_duplicate` | URL already visited | `url` |
| `Submit()` | `url_enqueued` | New URL added | `url`, `depth`, `visited_count` |
| `Dequeue()` | `depth_advanced` | Moving to new depth | `old_depth`, `new_depth`, `urls_at_depth` |

#### 9.3.4 Performance Consideration
Always check `debugLogger.Enabled()` before any logging:

```
if h.debugLogger.Enabled() {
    h.debugLogger.LogStep(...)
}
```

### 9.4 Testing Strategy

1. Test skip scenarios (depth, max pages, duplicate)
2. Verify depth transition logging
3. Verify no logging overhead when disabled

---

## 10. Scheduler Updates

### 10.1 Target File
`internal/scheduler/scheduler.go`

### 10.2 Current State
- Already has `debugLogger` field
- Already logs fetcher stage events
- Uses `NewSchedulerWithConfig()` which initializes debug logger

### 10.3 Required Changes

#### 10.3.1 Pass Debug Logger to Components

In `NewSchedulerWithConfig()` and `NewSchedulerWithDeps()`, call `SetDebugLogger()` on all components:

| Component | Setter Method |
|-----------|---------------|
| `htmlFetcher` | `SetDebugLogger(logger)` |
| `domExtractor` | `SetDebugLogger(logger)` |
| `htmlSanitizer` | `SetDebugLogger(logger)` |
| `robot` | `SetDebugLogger(logger)` |
| `frontier` | `SetDebugLogger(logger)` |
| `rateLimiter` | `SetDebugLogger(logger)` |
| `assetResolver` | `SetDebugLogger(logger)` |
| `markdownConstraint` | `SetDebugLogger(logger)` |
| `storageSink` | `SetDebugLogger(logger)` |

#### 10.3.2 Stage Event Logging

Add stage logging for all pipeline stages (currently only fetcher has it):

| Stage | Log Start | Log Complete |
|-------|-----------|--------------|
| extractor | Yes | Yes |
| sanitizer | Yes | Yes |
| mdconvert | Yes | Yes |
| assets | Yes | Yes |
| normalize | Yes | Yes |
| storage | Yes | Yes |

#### 10.3.3 Rate Limit Logging

Log rate limiting when applying delays:

```
delay := s.rateLimiter.ResolveDelay(s.currentHost)
if delay > 0 && s.debugLogger.Enabled() {
    s.debugLogger.LogRateLimit(ctx, s.currentHost, delay, reason)
}
s.sleeper.Sleep(delay)
```

### 10.4 Testing Strategy

1. Verify all components receive debug logger
2. Test full pipeline with debug logging enabled
3. Verify log output contains all expected stages

---

## 11. Other Pipeline Stages

### 11.1 Markdown Converter
**File**: `internal/mdconvert/rules.go`

**Logging Points**:
| Step Name | Fields |
|-----------|--------|
| `convert_elements` | `headings_count`, `paragraphs_count`, `code_blocks_count` |
| `build_markdown` | `output_size_bytes`, `links_count` |

### 11.2 Asset Resolver
**File**: `internal/assets/resolver.go`

**Logging Points**:
| Step Name | Fields |
|-----------|--------|
| `resolve_asset` | `asset_url`, `asset_type` (image, stylesheet, etc.) |
| `download_asset` | `bytes_downloaded`, `status_code` |
| `asset_cached` | `asset_url`, `hash` (if already downloaded) |

**Note**: Must also update retry call to pass debug logger.

### 11.3 Normalizer
**File**: `internal/normalize/constraints.go`

**Logging Points**:
| Step Name | Fields |
|-----------|--------|
| `apply_frontmatter` | `title`, `url`, `depth`, `fetched_at` |
| `normalize_content` | `input_size`, `output_size` |
| `compute_hash` | `algorithm`, `hash_value` |

### 11.4 Storage
**File**: `internal/storage/` (local sink and dry-run sink)

**Logging Points**:
| Step Name | Fields |
|-----------|--------|
| `write_file` | `file_path`, `size_bytes` |
| `create_directory` | `dir_path` |
| `file_skipped` | `file_path`, `reason` (already exists, etc.) |

---

## 12. Test Infrastructure Updates

### 12.1 Test Helpers Location
`pkg/debug/logger_test.go` already has test infrastructure. Extend it for integration tests.

### 12.2 Capture Logger Enhancement
The existing test helpers may need enhancement for integration testing:

```
type CaptureLogger struct {
    mu      sync.Mutex
    entries []LogEntry
}

// Add methods to filter entries by stage, step, etc.
func (c *CaptureLogger) EntriesByStage(stage string) []LogEntry
func (c *CaptureLogger) EntriesByStep(step string) []LogEntry
```

### 12.3 Integration Test Pattern

```
func TestFullPipelineDebugLogging(t *testing.T) {
    // Setup with capture logger
    logger := NewCaptureLogger()
    
    // Run pipeline with debug enabled
    cfg := config.WithDebug(true)
    scheduler := NewSchedulerWithConfig(cfg)
    scheduler.SetDebugLogger(logger)
    
    // Execute crawl
    scheduler.InitializeWithConfig(cfg)
    scheduler.ExecuteCrawlingWithState(init)
    
    // Verify log sequence
    entries := logger.EntriesByStage("fetcher")
    assertHasStep(entries, "create_request")
    assertHasStep(entries, "response_received")
    assertHasStep(entries, "body_read")
}
```

---

## 13. Migration Checklist

Before starting implementation, ensure:

- [ ] Phase 1 complete: `pkg/debug` package exists with all types
- [ ] Phase 2 complete: Scheduler has debug logger field
- [ ] All tests in `pkg/debug` pass
- [ ] CLI flags `--debug`, `--debug-file`, `--debug-format` work

Implementation order:

- [ ] 1. Retry Handler
- [ ] 2. Rate Limiter
- [ ] 3. Fetcher
- [ ] 4. Extractor
- [ ] 5. Sanitizer
- [ ] 6. Robots
- [ ] 7. Frontier
- [ ] 8. Scheduler updates
- [ ] 9. Markdown Converter
- [ ] 10. Asset Resolver
- [ ] 11. Normalizer
- [ ] 12. Storage
- [ ] 13. Integration tests
- [ ] 14. Documentation update

---

## 14. Common Patterns

### 14.1 Adding Debug Logger to a Component

1. Add `debugLogger debug.DebugLogger` field to struct
2. Initialize with `debug.NewNoOpLogger()` in constructor
3. Add `SetDebugLogger(logger debug.DebugLogger)` method
4. Add logging calls at key points with `Enabled()` check

### 14.2 Logging Pattern

```
if h.debugLogger.Enabled() {
    h.debugLogger.LogStep(ctx, "stage_name", "step_name", debug.FieldMap{
        "field1": value1,
        "field2": value2,
    })
}
```

### 14.3 Updating Retry Calls

When a component uses `retry.Retry()`:

```
// Before
result := retry.Retry(retryParam, fn)

// After
result := retry.Retry(retryParam, h.debugLogger, fn)
```

---

## 15. Troubleshooting

### 15.1 Debug logs not appearing
- Verify `--debug` flag is passed
- Check debug logger is passed to component via `SetDebugLogger()`
- Verify `Enabled()` check is not preventing logging

### 15.2 Performance impact
- Ensure `NoOpLogger` is used when debug is disabled
- Check `Enabled()` is called before expensive field construction
- Consider lazy evaluation for complex fields

### 15.3 Missing context
- Use `context.Background()` if no request context available
- Consider adding `context.Context` to component methods in future refactor

---

**Document Status**: Ready for Implementation  
**Author**: System Design Team  
**Created**: 2026-02-21  
**Last Updated**: 2026-02-21