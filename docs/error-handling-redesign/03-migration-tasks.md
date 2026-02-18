# Error Handling Redesign: Migration Tasks

**Document Purpose**: Actionable, prioritized task breakdown for implementing the error handling redesign.

**Related Documents**:
- Analysis & Findings: `01-analysis-and-findings.md`
- Migration Proposal: `02-migration-proposal.md`

---

## Phase 1: Interface Extension (Foundation)

**Goal**: Extend `ClassifiedError` interface with new methods while maintaining backward compatibility.

**Duration**: 1–2 days
**Blocking**: All subsequent phases

---

### Task 1.1: Extend `pkg/failure/errors.go` Interface

**Technical Details**:
- Add `RetryPolicy()` method returning new `RetryPolicy` type
- Add `CrawlImpact()` method returning new `CrawlImpact` type
- Keep existing `Severity()` method for backward compatibility
- Define `RetryPolicy` constants: `Auto`, `Manual`, `Never`
- Define `CrawlImpact` constants: `Continue`, `Abort`
- Add `SeverityRetryExhausted` constant for observability

**Acceptance Criteria**:
- [ ] Interface compiles without errors
- [ ] Default implementations provided for backward compatibility
- [ ] Unit tests for new types and constants

**Files Modified**:
- `pkg/failure/errors.go`

**Files Created**:
- `pkg/failure/errors_test.go` (new unit tests)

---

### Task 1.2: Update `RetryError` with New Interface

**Technical Details**:
- Add `RetryPolicy()` returning `RetryPolicyManual` (exhausted = manual eligible)
- Add `CrawlImpact()` returning `ImpactContinue` (don't abort crawl)
- Preserve original error via `Unwrap()` method
- Update constructor to accept policy and impact parameters

**Acceptance Criteria**:
- [ ] `RetryError` implements full `ClassifiedError` interface
- [ ] `Unwrap()` method provides access to underlying error
- [ ] Unit tests verify classification and unwrapping

**Files Modified**:
- `pkg/retry/errors.go`

---

### Task 1.3: Add Default Implementations for All Stage Errors

**Technical Details**:
- Add `RetryPolicy()` method to all 8 stage error types
- Derive from existing `Retryable` field: `true` → `Auto`, `false` → `Manual`
- Add `CrawlImpact()` method returning `ImpactContinue` (conservative default)
- These are temporary — will be replaced with explicit values in Phase 2

**Stage Errors to Update**:
1. `internal/robots/errors.go` — `RobotsError`
2. `internal/fetcher/errors.go` — `FetchError`
3. `internal/extractor/errors.go` — `ExtractionError`
4. `internal/sanitizer/errors.go` — `SanitizationError`
5. `internal/mdconvert/errors.go` — `ConversionError`
6. `internal/assets/errors.go` — `AssetsError`
7. `internal/normalize/errors.go` — `NormalizationError`
8. `internal/storage/errors.go` — `StorageError`

**Acceptance Criteria**:
- [ ] All 8 error types compile with new interface
- [ ] No changes to existing behavior (purely additive)
- [ ] Integration tests pass without modification

**Files Modified**:
- All 8 error files listed above

---

## Phase 2: Retry Handler Migration

**Goal**: Update retry handler to use new interface methods.

**Duration**: 1 day
**Blocking**: Phase 3 (stage error refactoring)
**Blocked By**: Phase 1

---

### Task 2.1: Refactor `isErrorRetryable()` Function

**Technical Details**:
- Replace duck-typing with interface assertion on `ClassifiedError`
- Check `RetryPolicy() == RetryPolicyAuto` for retry decision
- Remove redundant interface checks (`hasRetryable`, `hasRetryableField`)
- Change default return from `true` to `false` (conservative)

**Before**:
```go
func isErrorRetryable(err failure.ClassifiedError) bool {
    type hasRetryable interface { IsRetryable() bool }
    if r, ok := err.(hasRetryable); ok { return r.IsRetryable() }
    type hasRetryableField interface { failure.ClassifiedError; IsRetryable() bool }
    if r, ok := err.(hasRetryableField); ok { return r.IsRetryable() }
    return true  // Dangerous default
}
```

**After**:
```go
func shouldAutoRetry(err failure.ClassifiedError) bool {
    return err.RetryPolicy() == failure.RetryPolicyAuto
}
```

**Acceptance Criteria**:
- [ ] Function simplified to single policy check
- [ ] Default changed to `false` (don't retry unknown errors)
- [ ] Unit tests verify retry decisions for each policy type
- [ ] Existing retry behavior unchanged (integration tests pass)

**Files Modified**:
- `pkg/retry/handler.go`

---

### Task 2.2: Update Retry Exhaustion Handling

**Technical Details**:
- When `RetryPolicyAuto` error exhausts attempts, return `RetryError` with:
  - `RetryPolicyManual` (eligible for manual retry)
  - `ImpactContinue` (don't abort crawl)
  - Wrapped original error (for debugging)
- Update `RetryError` severity derivation

**Acceptance Criteria**:
- [ ] Exhausted auto-retries return `RetryPolicyManual`
- [ ] Original error preserved via `Unwrap()`
- [ ] Unit test: verify exhausted 429 results in manual retry eligibility

**Files Modified**:
- `pkg/retry/handler.go`
- `pkg/retry/errors.go`

---

### Task 2.3: Remove `IsRetryable()` Method Dependencies

**Technical Details**:
- Remove `IsRetryable()` method from `FetchError` (temporary during transition)
- Remove `IsRetryable()` method from `AssetsError`
- Update any tests that directly call `IsRetryable()`

**Acceptance Criteria**:
- [ ] No `IsRetryable()` methods remain in codebase
- [ ] All tests updated to check `RetryPolicy()` instead
- [ ] No regression in test coverage

**Files Modified**:
- `internal/fetcher/errors.go`
- `internal/assets/errors.go`
- Test files referencing `IsRetryable()`

---

## Phase 3: Stage Error Refactoring (Per-Stage)

**Goal**: Explicitly classify each error cause with correct policy and impact.

**Duration**: 3–4 days (can be parallelized per stage)
**Blocking**: Phase 4 (scheduler update)
**Blocked By**: Phase 2

---

### Task 3.1: Refactor `internal/fetcher/errors.go`

**Technical Details**:

1. **Create Error Cause Classification Map**:
```go
var fetchErrorClassifications = map[FetchErrorCause]struct {
    Policy RetryPolicy
    Impact CrawlImpact
}{
    ErrCauseTimeout:               {RetryPolicyAuto, ImpactContinue},
    ErrCauseNetworkFailure:        {RetryPolicyAuto, ImpactContinue},
    ErrCauseRequest5xx:            {RetryPolicyAuto, ImpactContinue},
    ErrCauseRequestTooMany:        {RetryPolicyAuto, ImpactContinue},
    ErrCauseRequestPageForbidden:  {RetryPolicyManual, ImpactContinue},
    ErrCauseRepeated403:           {RetryPolicyNever, ImpactContinue},
    ErrCauseRedirectLimitExceeded: {RetryPolicyNever, ImpactContinue},
    ErrCauseContentTypeInvalid:    {RetryPolicyNever, ImpactContinue},
    ErrCauseReadResponseBodyError: {RetryPolicyAuto, ImpactContinue},
}
```

2. **Update Constructor Functions**:
```go
func NewFetchError(cause FetchErrorCause, message string) *FetchError {
    classification := fetchErrorClassifications[cause]
    return &FetchError{
        Message: message,
        Cause:   cause,
        policy:  classification.Policy,
        impact:  classification.Impact,
    }
}
```

3. **Implement Interface Methods**:
```go
func (e *FetchError) RetryPolicy() RetryPolicy { return e.policy }
func (e *FetchError) CrawlImpact() CrawlImpact { return e.impact }
```

4. **Update All Call Sites**:
- Replace `&FetchError{...}` direct construction with `NewFetchError()`
- Remove `Retryable` field from struct

**Acceptance Criteria**:
- [ ] All error causes have explicit classifications
- [ ] All call sites use constructor
- [ ] `Retryable` field removed from struct
- [ ] Unit tests for each error cause classification
- [ ] Integration tests pass

**Files Modified**:
- `internal/fetcher/errors.go`
- `internal/fetcher/html.go` (update error construction)
- `internal/fetcher/html_test.go` (update tests)

---

### Task 3.2: Refactor `internal/assets/errors.go`

**Technical Details**:
- Similar pattern to Task 3.1
- Special attention to `ErrCauseDiskFull` → `RetryPolicyManual`

**Classification Mapping**:
| Cause | Policy | Impact |
|-------|--------|--------|
| ErrCauseTimeout | Auto | Continue |
| ErrCauseNetworkFailure | Auto | Continue |
| ErrCauseRequest5xx | Auto | Continue |
| ErrCauseRequestTooMany | Auto | Continue |
| ErrCauseRequestPageForbidden | Manual | Continue |
| ErrCauseRepeated403 | Never | Continue |
| ErrCauseAssetTooLarge | Never | Continue |
| ErrCauseDiskFull | Manual | Continue |
| ErrCauseWriteFailure | Never | Continue |
| ErrCausePathError | Never | Continue |
| ErrCauseHashError | Never | Continue |

**Acceptance Criteria**:
- [ ] Same as Task 3.1

**Files Modified**:
- `internal/assets/errors.go`
- `internal/assets/resolver.go` (update error construction)
- `internal/assets/resolver_test.go`

---

### Task 3.3: Refactor `internal/robots/errors.go`

**Technical Details**:
- Special case: `ErrCauseDisallowRoot` is not an error per-se, it's a policy decision
- Robots errors generally shouldn't abort crawl (continue without robots if fetch fails)

**Classification Mapping**:
| Cause | Policy | Impact |
|-------|--------|--------|
| ErrCauseHttpTooManyRequests | Auto | Continue |
| ErrCauseHttpServerError | Auto | Continue |
| ErrCauseDisallowRoot | Never | Continue |
| ErrCauseParseError | Never | Continue |
| ErrCauseInvalidRobotsUrl | Never | Continue |
| ErrCausePreFetchFailure | Never | Continue |
| ErrCauseHttpFetchFailure | Auto | Continue |
| ErrCauseHttpTooManyRedirects | Never | Continue |
| ErrCauseHttpUnexpectedStatus | Never | Continue |

**Acceptance Criteria**:
- [ ] Same as Task 3.1

**Files Modified**:
- `internal/robots/errors.go`
- `internal/robots/fetcher.go`
- `internal/robots/robot.go`

---

### Task 3.4: Refactor `internal/storage/errors.go`

**Technical Details**:
- Storage errors generally don't abort crawl (one file failure ≠ all files)
- Exception: If storage is completely unavailable, might need systemic handling (future)

**Classification Mapping**:
| Cause | Policy | Impact |
|-------|--------|--------|
| ErrCauseDiskFull | Manual | Continue |
| ErrCauseWriteFailure | Never | Continue |
| ErrCauseHashComputationFailed | Never | Continue |
| ErrCausePathError | Never | Continue |

**Acceptance Criteria**:
- [ ] Same as Task 3.1

**Files Modified**:
- `internal/storage/errors.go`
- `internal/storage/sink.go`

---

### Task 3.5: Refactor Content Processing Errors

**Technical Details**:
- extractor, sanitizer, mdconvert, normalize errors
- All content errors are deterministic — retrying same content yields same error
- Policy: `Never` for all (no benefit from retry)

**Stages**:
1. `internal/extractor/errors.go`
2. `internal/sanitizer/errors.go`
3. `internal/mdconvert/errors.go`
4. `internal/normalize/errors.go`

**Uniform Classification**:
- All causes: `RetryPolicyNever`, `ImpactContinue`

**Acceptance Criteria**:
- [ ] All 4 stages refactored
- [ ] Uniform classification applied
- [ ] Tests updated

**Files Modified**:
- All 4 error files
- Corresponding implementation files
- Test files

---

## Phase 4: Scheduler and Frontier Updates

**Goal**: Update scheduler to use `CrawlImpact` for control flow and implement manual retry book-keeping.

**Duration**: 2–3 days
**Blocking**: Phase 5 (integration testing)
**Blocked By**: Phase 3

---

### Task 4.1: Update Scheduler Error Handling Logic

**Technical Details**:

**Before**:
```go
if err.Severity() == failure.SeverityFatal {
    return CrawlingExecution{}, err  // Abort
}
totalErrors++
continue
```

**After**:
```go
if err.CrawlImpact() == failure.ImpactAbort {
    return CrawlingExecution{}, err  // Systemic failure — abort
}

if err.RetryPolicy() == failure.RetryPolicyManual {
    s.frontier.BookKeepForRetry(nextCrawlToken.URL(), err)
}

totalErrors++
continue
```

**Acceptance Criteria**:
- [ ] Scheduler checks `CrawlImpact()` instead of `Severity()`
- [ ] Manual retry book-keeping integrated
- [ ] No regression in abort behavior for config errors
- [ ] HTTP 403/404 no longer abort crawl (integration test)

**Files Modified**:
- `internal/scheduler/scheduler.go`
- `internal/scheduler/scheduler_*_test.go` (multiple test files)

---

### Task 4.2: Implement Frontier Retry Queue

**Technical Details**:

1. **Add Data Structure to Frontier**:
```go
// internal/frontier/data.go
type RetryEntry struct {
    URL       url.URL
    Reason    string
    Timestamp time.Time
    Attempts  int
}

type CrawlFrontier struct {
    // ... existing fields ...
    retryQueue []RetryEntry
    retrySet   map[string]bool // URL deduplication for retry queue
}
```

2. **Implement Methods**:
```go
func (f *CrawlFrontier) BookKeepForRetry(url url.URL, reason failure.ClassifiedError) {
    key := url.String()
    if f.retrySet[key] {
        return // Already tracked
    }
    f.retrySet[key] = true
    f.retryQueue = append(f.retryQueue, RetryEntry{
        URL:       url,
        Reason:    reason.Error(),
        Timestamp: time.Now(),
        Attempts:  1,
    })
}

func (f *CrawlFrontier) GetRetryCandidates() []url.URL {
    candidates := make([]url.URL, len(f.retryQueue))
    for i, entry := range f.retryQueue {
        candidates[i] = entry.URL
    }
    return candidates
}

func (f *CrawlFrontier) ClearRetryQueue(processed []url.URL) {
    // Remove processed URLs from retry queue
    // Rebuild retrySet for remaining URLs
}
```

3. **Add Persistence** (Optional for Phase 4, can be Phase 5):
```go
func (f *CrawlFrontier) PersistRetryQueue(path string) error
func (f *CrawlFrontier) LoadRetryQueue(path string) error
```

**Acceptance Criteria**:
- [ ] `BookKeepForRetry()` deduplicates URLs
- [ ] `GetRetryCandidates()` returns all tracked URLs
- [ ] `ClearRetryQueue()` removes successfully processed URLs
- [ ] Unit tests for all methods
- [ ] Thread-safe implementation (if frontier is concurrent)

**Files Modified**:
- `internal/frontier/data.go`
- `internal/frontier/frontier.go`
- New test file: `internal/frontier/retry_queue_test.go`

---

### Task 4.3: Add Retry Queue Metrics

**Technical Details**:
- Track count of URLs in manual retry queue
- Record reasons for retry (for observability)
- Add to crawl finalization stats

**Acceptance Criteria**:
- [ ] `RetryQueueSize()` method added to frontier
- [ ] Stats include "URLs awaiting manual retry" count
- [ ] Metadata recording for retry queue entries

**Files Modified**:
- `internal/frontier/frontier.go`
- `internal/scheduler/scheduler.go` (stats collection)
- `internal/metadata/recorder.go` (if new metadata types needed)

---

## Phase 5: Integration and Testing

**Goal**: End-to-end validation of the redesigned error handling.

**Duration**: 2 days
**Blocking**: Phase 6 (cleanup)
**Blocked By**: Phase 4

---

### Task 5.1: Create Integration Test Suite

**Test Scenarios**:

1. **HTTP 403 Handling**:
   - Setup: Mock server returning 403 for specific URL
   - Execute: Crawl with that URL in scope
   - Verify: Crawl continues, URL tracked in retry queue, other URLs processed

2. **Retry Exhaustion**:
   - Setup: URL that returns 429 three times, then 200
   - Configure: maxAttempts=2
   - Execute: Crawl
   - Verify: URL in retry queue, not marked as visited

3. **Disk Full Simulation**:
   - Setup: Mock storage that fails with disk full
   - Execute: Crawl
   - Verify: URL in retry queue, crawl continues

4. **Systemic Failure (Config)**:
   - Setup: Invalid configuration
   - Execute: InitializeCrawling
   - Verify: Immediate abort, no URLs processed

5. **Mixed Success/Failure**:
   - Setup: Some URLs succeed, some 403, some 404, some timeout then succeed
   - Execute: Full crawl
   - Verify: Correct categorization, retry queue populated, stats accurate

**Acceptance Criteria**:
- [ ] All 5 scenarios have passing tests
- [ ] Tests use mock implementations (no external dependencies)
- [ ] Tests verify both behavior and state (retry queue contents)

**Files Created**:
- `internal/scheduler/scheduler_error_handling_test.go` (new comprehensive test file)

---

### Task 5.2: Update Existing Tests

**Technical Details**:
- Update all tests that check `Severity()` for control flow
- Update tests that use `IsRetryable()` method
- Update tests that construct errors with `Retryable` field

**Files to Review**:
- `internal/scheduler/*_test.go` (all scheduler tests)
- `pkg/retry/handler_test.go`
- All stage-specific `*_test.go` files

**Acceptance Criteria**:
- [ ] All existing tests pass
- [ ] No references to old `Retryable` field in tests
- [ ] No references to `IsRetryable()` in tests
- [ ] Test coverage maintained or improved

---

### Task 5.3: Add Error Classification Unit Tests

**Technical Details**:
- For each stage, test every error cause has correct classification
- Use table-driven tests for maintainability

**Example Test Structure**:
```go
func TestFetchError_Classifications(t *testing.T) {
    tests := []struct {
        name       string
        cause      FetchErrorCause
        wantPolicy failure.RetryPolicy
        wantImpact failure.CrawlImpact
    }{
        {"5xx auto-retry", ErrCauseRequest5xx, failure.RetryPolicyAuto, failure.ImpactContinue},
        {"403 manual-retry", ErrCauseRequestPageForbidden, failure.RetryPolicyManual, failure.ImpactContinue},
        {"404 never-retry", ErrCauseContentTypeInvalid, failure.RetryPolicyNever, failure.ImpactContinue},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := NewFetchError(tt.cause, "test message")
            assert.Equal(t, tt.wantPolicy, err.RetryPolicy())
            assert.Equal(t, tt.wantImpact, err.CrawlImpact())
        })
    }
}
```

**Acceptance Criteria**:
- [ ] Unit tests exist for all 8 stage error types
- [ ] Every error cause is tested (100% classification coverage)
- [ ] Tests fail if classification changes (prevent accidental changes)

---

## Phase 6: Cleanup and Documentation

**Goal**: Remove legacy code and finalize documentation.

**Duration**: 1 day
**Blocking**: None (final phase)
**Blocked By**: Phase 5

---

### Task 6.1: Remove Legacy `Retryable` Field

**Technical Details**:
- Remove `Retryable` field from all stage error structs
- Remove `Severity()` derivation from `Retryable` (now derives from explicit policy/impact)
- Search for any remaining references to `Retryable` field

**Stage Errors to Clean**:
1. `internal/robots/errors.go`
2. `internal/fetcher/errors.go`
3. `internal/extractor/errors.go`
4. `internal/sanitizer/errors.go`
5. `internal/mdconvert/errors.go`
6. `internal/assets/errors.go`
7. `internal/normalize/errors.go`
8. `internal/storage/errors.go`
9. `pkg/retry/errors.go` (`RetryError`)

**Acceptance Criteria**:
- [ ] No `Retryable` fields remain
- [ ] No `IsRetryable()` methods remain
- [ ] All tests pass after removal

---

### Task 6.2: Update Design Documents

**Technical Details**:
- Update `docs/technical_design.md` Section 16 (Error Handling Philosophy)
- Update `docs/design_document.md` Section 5.2 (Error Handling)
- Add ADR (Architecture Decision Record) for the redesign

**Acceptance Criteria**:
- [ ] Technical design reflects new two-dimensional classification
- [ ] ADR documents rationale and migration path
- [ ] Cross-references to error handling redesign documents added

**Files Modified**:
- `docs/technical_design.md`
- `docs/design_document.md`
- New: `docs/adr/001-error-handling-redesign.md`

---

### Task 6.3: Create Runbook for Manual Retry Feature

**Technical Details**:
- Document how users can resume failed URLs
- Provide CLI examples for checkpoint/resume
- Explain retry queue inspection

**Content**:
```markdown
# Manual Retry Runbook

## Viewing Failed URLs
./crawler --show-retry-queue --checkpoint-file=crawl.checkpoint

## Resuming Failed URLs
./crawler --config=config.json --resume-from=crawl.checkpoint

## Clearing Retry Queue
./crawler --clear-retry-queue --checkpoint-file=crawl.checkpoint
```

**Acceptance Criteria**:
- [ ] Runbook document created
- [ ] CLI flags documented
- [ ] Examples provided

**Files Created**:
- `docs/runbooks/manual-retry.md`

---

## Summary Timeline

| Phase | Duration | Dependencies | Deliverables |
|-------|----------|--------------|--------------|
| Phase 1 | 1–2 days | None | Extended interface, default implementations |
| Phase 2 | 1 day | Phase 1 | Refactored retry handler, removed `IsRetryable()` |
| Phase 3 | 3–4 days | Phase 2 | All 8 stage errors refactored with explicit classifications |
| Phase 4 | 2–3 days | Phase 3 | Scheduler uses `CrawlImpact`, retry queue implemented |
| Phase 5 | 2 days | Phase 4 | Integration tests, updated unit tests, 100% classification coverage |
| Phase 6 | 1 day | Phase 5 | Legacy code removal, documentation updates |
| **Total** | **10–13 days** | | |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Breaking changes in production | Phase 1 provides backward-compatible defaults; extensive integration testing in Phase 5 |
| Misclassification of errors | Task 5.3 provides 100% classification coverage; table-driven tests prevent accidental changes |
| Concurrent access to retry queue | Task 4.2 implements thread-safe operations; use mutex if frontier is concurrent |
| Performance regression | Benchmark retry queue operations; complexity is O(1) for dedup with map |

---

## Success Metrics

- [ ] **Functional**: HTTP 403/404 errors do NOT abort crawl (verified by integration test)
- [ ] **Functional**: Exhausted auto-retries are tracked in manual retry queue
- [ ] **Functional**: Config errors DO abort crawl immediately
- [ ] **Code Quality**: 100% classification coverage (every error cause has explicit test)
- [ ] **Code Quality**: Zero `Retryable` field references
- [ ] **Code Quality**: Zero `IsRetryable()` method references
- [ ] **Maintainability**: All error classifications in declarative maps (easy to modify)
- [ ] **Observability**: Manual retry queue metrics available in crawl stats

---

**End of Document**
