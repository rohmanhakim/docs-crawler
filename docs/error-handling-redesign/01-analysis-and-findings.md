# Error Handling Design: Analysis and Findings

**Document Purpose**: Provide context, background, and deep analysis of the current error handling design flaws across the pipeline stages.

**Related Documents**:
- Migration Proposal: `02-migration-proposal.md`
- Actionable Tasks: `03-migration-tasks.md`

---

## Executive Summary

The current error handling design conflates multiple orthogonal concerns into a single `Retryable` boolean field. This design flaw creates semantic ambiguity between **automatic retry decisions** (exponential backoff at stage level) and **manual retry eligibility** (user-initiated resume after crawl completion). Additionally, the `Severity()` method derived from `Retryable` incorrectly conflates "immediate retry worthiness" with "crawl-level impact", causing per-URL failures (like HTTP 403) to potentially abort the entire crawl.

**Impact Assessment**: HIGH — Current design risks aborting crawls on recoverable per-URL errors and creates confusion about which errors qualify for manual retry queues.

---

## Current Design Architecture

### Error Structure Across Pipeline Stages

All pipeline stages implement a consistent pattern:

```go
// internal/fetcher/errors.go (representative example)
type FetchError struct {
    Message   string
    Retryable bool           // Determines BOTH auto-retry AND severity
    Cause     FetchErrorCause
}

func (e *FetchError) Severity() failure.Severity {
    if e.Retryable {
        return failure.SeverityRecoverable  // ← Derived from same field
    }
    return failure.SeverityFatal
}
```

### Component Responsibilities

| Component | Uses `Retryable` For | Current Behavior |
|-----------|---------------------|------------------|
| `pkg/retry/handler.go` | Auto-retry decision with exponential backoff | Checks `IsRetryable()` method or struct field |
| `internal/scheduler/scheduler.go` | Crawl continuation vs. abort decision | Calls `Severity()`, derives from `Retryable` |
| Stage errors (8 files) | Both purposes simultaneously | Single boolean serves dual masters |

### Severity Classification

```go
// pkg/failure/errors.go
type Severity string
const (
    SeverityFatal       = "fatal"       // Aborts crawl
    SeverityRecoverable = "recoverable" // Continues to next URL
)
```

---

## Critical Design Flaws

### 1. Semantic Overload: The `Retryable` Field Duality

**The Problem**: The `Retryable` field serves **two incompatible purposes**:

| Purpose | Consumer | Question Being Answered |
|---------|----------|------------------------|
| Auto-retry trigger | `pkg/retry/handler.go` | "Should I retry this operation immediately with exponential backoff?" |
| Severity derivation | `internal/scheduler/scheduler.go` | "Should I abort the entire crawl or continue?" |

**Production Risk Example**:

```go
// HTTP 403 Forbidden
return &FetchError{
    Message:   "access forbidden (403)",
    Retryable: false,  // Don't auto-retry (correct)
    Cause:     ErrCauseRequestPageForbidden,
}
// → Severity() returns SeverityFatal
// → Scheduler aborts ENTIRE crawl
// → BUG: 403 is a per-URL permanent failure, not systemic
```

**Correct Behavior**: HTTP 403 should:
- NOT trigger auto-retry (correct: `Retryable: false`)
- NOT abort crawl (wrong: currently aborts due to `SeverityFatal`)
- Be tracked for manual retry if user fixes auth later

### 2. Missing Distinction: Transient vs. Recoverable vs. Permanent

Current design collapses a **three-dimensional error taxonomy** into a single boolean:

```
Error Classification Matrix (What We Need):
┌─────────────────────────┬──────────────┬─────────────────┬────────────────┐
│ Error Type              │ Auto-Retry?  │ Manual Retry?   │ Crawl Impact   │
├─────────────────────────┼──────────────┼─────────────────┼────────────────┤
│ HTTP 429 (rate limit)   │ Yes (backoff)│ Yes             │ Skip URL       │
│ HTTP 503 (unavailable)  │ Yes (backoff)│ Yes             │ Skip URL       │
│ Network timeout         │ Yes (backoff)│ Yes             │ Skip URL       │
│ HTTP 403 (forbidden)    │ No           │ Yes (if auth)   │ Skip URL       │
│ Disk full (storage)     │ No           │ Yes (clean disk)│ Skip URL       │
│ HTTP 404 (not found)    │ No           │ No              │ Skip URL       │
│ Invalid HTML            │ No           │ No              │ Skip URL       │
│ Config error            │ No           │ No              │ ABORT          │
│ 100% seed failure       │ No           │ Maybe           │ ABORT          │
└─────────────────────────┴──────────────┴─────────────────┴────────────────┘
```

Current boolean design cannot express:
- "Don't auto-retry, but manual retry possible" (disk full, 403, exhausted attempts)
- "Permanent failure for this URL only" (404, invalid content)
- "Systemic failure requiring abort" (config error)

### 3. Ambiguous `RetryError` Semantics

When all retry attempts are exhausted, `pkg/retry/handler.go` returns:

```go
return &RetryError{
    Message:   fmt.Sprintf("exhausted %d attempts...", retryParam.MaxAttempts),
    Cause:     ErrExhaustedAttempts,
    Retryable: true,  // ← Ambiguous: what does this mean?
}
```

**Questions Raised**:
- Does `Retryable: true` on a `RetryError` mean "retry the HTTP request again" (circular)?
- Or "eligible for manual retry queue" (different semantics)?
- Why is exhaustion considered "recoverable" when the operation definitively failed?

**Correct Interpretation**: `RetryError` with exhausted attempts should signal:
- Auto-retry policy: `Never` (we already tried)
- Manual retry eligibility: `Yes` (user can retry later)
- Severity: `PageRecoverable` (don't abort crawl, just track this URL)

### 4. Inconsistent Interface Implementation

Only `FetchError` and `AssetsError` implement `IsRetryable()` method. The retry handler has complex fallback logic:

```go
func isErrorRetryable(err failure.ClassifiedError) bool {
    // Redundant checks for the same method
    type hasRetryable interface { IsRetryable() bool }
    if r, ok := err.(hasRetryable); ok { return r.IsRetryable() }
    
    type hasRetryableField interface {
        failure.ClassifiedError
        IsRetryable() bool  // Same method!
    }
    if r, ok := err.(hasRetryableField); ok { return r.IsRetryable() }
    
    return true  // ← Dangerous default
}
```

**Problems**:
- `hasRetryable` and `hasRetryableField` check for the same method signature
- Default `return true` means unknown errors get retried indefinitely
- Inconsistent interfaces across stages create maintenance burden
- Duck-typing breaks when new error types are added

### 5. Severity Derivation from Retryability is Wrong

Current derivation logic:

```go
func (e *SomeError) Severity() failure.Severity {
    if e.Retryable {
        return failure.SeverityRecoverable
    }
    return failure.SeverityFatal
}
```

**Why This Is Incorrect**:

| Error | `Retryable` | Current Severity | Should Be Severity | Reason |
|-------|-------------|------------------|-------------------|---------|
| HTTP 429 | `true` | `Recoverable` | `Recoverable` | ✓ Correct |
| HTTP 403 | `false` | `Fatal` | `Recoverable` | Per-URL, not systemic |
| HTTP 404 | `false` | `Fatal` | `Recoverable` | Per-URL, not systemic |
| Invalid HTML | `false` | `Fatal` | `Recoverable` | Per-URL, not systemic |
| Disk full | `false` | `Fatal` | `Recoverable` | Per-URL (can clean disk, resume) |
| Config error | `false` | `Fatal` | `Fatal` | ✓ Systemic, can't proceed |

**Root Cause**: `SeverityFatal` should mean "system cannot make forward progress", not "this operation failed and shouldn't be retried immediately".

---

## Understanding User Intent (Clarified)

Based on clarification, the intended behavior is:

1. **Manual Retry After Crawl**: User wants to book-keep failed URLs for later manual retry. Examples:
   - Disk full → clean disk → resume failed URLs
   - Exhausted attempts on rate-limited URLs → retry next day
   - Auth issues → fix credentials → retry

2. **Per-URL Retry Limits**: Each URL has its own retry budget (configurable attempts). No global retry limit.

3. **Abort Conditions**: Only abort crawl when system cannot make forward progress:
   - Configuration errors (invalid config file)
   - 100% seed URL failure (can't even begin)
   - Not: individual URL failures (403, 404, etc.)

4. **RetryError Purpose**: When a retryable error exhausts all attempts, the system should:
   - NOT auto-retry further (immediate retry exhausted)
   - Track the URL for manual retry later
   - Continue crawling (don't abort)

---

## Impact on Production Systems

### Scenario 1: False Positive Abort

**Situation**: Crawling documentation site, one page returns 403 (requires auth that wasn't configured).

**Current Behavior**:
1. Fetcher returns `FetchError{Retryable: false, Cause: ErrCauseRequestPageForbidden}`
2. `Severity()` returns `SeverityFatal`
3. Scheduler sees `SeverityFatal`, aborts entire crawl
4. Hundreds of valid pages never processed due to one 403

**Desired Behavior**:
1. Fetcher classifies as "permanent failure for this URL, but crawl continues"
2. Scheduler skips URL, increments error count, continues
3. URL tracked in "manual retry queue" (user can fix auth and retry later)
4. Crawl completes with partial success

### Scenario 2: Retry Exhaustion Ambiguity

**Situation**: Rate-limited URL (429) fails after 3 retry attempts.

**Current Behavior**:
1. Retry handler returns `RetryError{Retryable: true}`
2. Scheduler sees `SeverityRecoverable`, continues crawl
3. User sees "recoverable" and assumes URL succeeded or will be auto-retried
4. URL lost — not tracked for manual retry because it was "recoverable"

**Desired Behavior**:
1. Retry handler exhausts attempts
2. Returns classification: "auto-retry exhausted, manual retry possible"
3. Scheduler tracks URL in "manual retry queue"
4. User can see list of URLs that need retry and resume them later

### Scenario 3: Disk Full Handling

**Situation**: Storage fills up during asset downloading.

**Current Behavior**:
1. Storage returns `StorageError{Retryable: true, Cause: ErrCauseDiskFull}`
2. Wait — is disk full really auto-retryable? Currently some stages mark it `Retryable: true`
3. If `Retryable: true`, retry handler will backoff and retry (wastes time, disk still full)
4. If `Retryable: false`, scheduler may abort (wrong if other URLs could succeed)

**Desired Behavior**:
1. Disk full detected: "not auto-retryable, but manual retry possible"
2. Scheduler: don't auto-retry, track URL for manual resume
3. User cleans disk, restarts with "resume from checkpoint"
4. Previously failed URLs are retried first

---

## Conclusion

The current error handling design has a **fundamental semantic flaw**: the `Retryable` field attempts to serve two orthogonal concerns (auto-retry vs. crawl impact) that have incompatible semantics. This causes:

1. **Incorrect abort decisions** — per-URL failures abort entire crawls
2. **Lost retry opportunities** — exhausted auto-retries aren't tracked for manual retry
3. **Wasted resources** — some errors marked auto-retryable when they shouldn't be
4. **Maintenance burden** — inconsistent interfaces and duck-typing across stages

The migration to a **two-dimensional classification** (separating auto-retry policy from crawl impact) is required to support the intended "manual retry after crawl" feature and ensure correct crawl lifecycle management.

---

## Appendix: Error Type Inventory

### Current Stage Errors

| Stage | Error Type | Has `IsRetryable()` | `Retryable` Field |
|-------|------------|---------------------|-------------------|
| robots | `RobotsError` | No | Yes |
| fetcher | `FetchError` | Yes | Yes |
| extractor | `ExtractionError` | No | Yes |
| sanitizer | `SanitizationError` | No | Yes |
| mdconvert | `ConversionError` | No | Yes |
| assets | `AssetsError` | Yes | Yes |
| normalize | `NormalizationError` | No | Yes |
| storage | `StorageError` | No | Yes |
| retry | `RetryError` | Yes | Yes |

### Current Severity Mappings (All Stages)

```go
// Universal pattern across all stages
func (e *XxxError) Severity() failure.Severity {
    if e.Retryable {
        return failure.SeverityRecoverable
    }
    return failure.SeverityFatal
}
```

This uniformity is the problem — different error types need different severity mappings.

---

**End of Document**
