# Error Handling Redesign: Migration Proposal

**Document Purpose**: Detailed design proposal for refactoring the error handling system to fix semantic overload and support manual retry workflows.

**Related Documents**:
- Analysis & Findings: `01-analysis-and-findings.md`
- Migration Tasks: `03-migration-tasks.md`

---

## Goals

1. **Separate Concerns**: Distinguish automatic retry policy from crawl-level impact
2. **Support Manual Retry**: Enable "resume failed URLs later" functionality
3. **Correct Abort Behavior**: Only abort crawl on systemic failures, not per-URL errors
4. **Explicit Contracts**: Replace duck-typing with clear interface requirements
5. **Backward Compatibility**: Migrate incrementally without breaking existing functionality

---

## Proposed Design

### 1. New Error Classification Interface

Replace the current `ClassifiedError` interface with an expanded contract:

```go
// pkg/failure/errors.go

// RetryPolicy defines automatic retry behavior
// This controls whether retry.Retry() will attempt exponential backoff
type RetryPolicy int

const (
    RetryPolicyAuto   RetryPolicy = iota // Retry immediately with exponential backoff
    RetryPolicyManual                    // Do not auto-retry, but eligible for manual retry queue
    RetryPolicyNever                     // Permanent failure, do not track for retry
)

// CrawlImpact defines how the scheduler should respond
// This controls crawl lifecycle decisions
type CrawlImpact int

const (
    ImpactContinue CrawlImpact = iota // Continue to next URL (default)
    ImpactAbort                       // Abort entire crawl (systemic failure)
)

// Severity provides observability and legacy compatibility
// Deprecated: Use CrawlImpact for control flow decisions
type Severity string

const (
    SeverityOK              Severity = "ok"
    SeverityRecoverable     Severity = "recoverable"
    SeverityFatal           Severity = "fatal"
    SeverityRetryExhausted  Severity = "retry_exhausted" // New: signals manual retry needed
)

// ClassifiedError is the primary error interface for the entire pipeline
type ClassifiedError interface {
    error
    
    // RetryPolicy controls automatic retry behavior
    // Used by: pkg/retry/handler.go
    RetryPolicy() RetryPolicy
    
    // CrawlImpact controls scheduler continuation/abortion
    // Used by: internal/scheduler/scheduler.go
    CrawlImpact() CrawlImpact
    
    // Severity provides observability and legacy compatibility
    // Used by: metadata recording, logging, monitoring
    Severity() Severity
}
```

### 2. Stage Error Refactoring Pattern

All stage errors should follow this unified structure:

```go
// internal/fetcher/errors.go (example)

type FetchError struct {
    Message  string
    Cause    FetchErrorCause
    policy   RetryPolicy   // Private, set via constructor
    impact   CrawlImpact   // Private, set via constructor
}

// Constructor-based initialization ensures explicit classification
func NewFetchError(cause FetchErrorCause, message string, policy RetryPolicy, impact CrawlImpact) *FetchError {
    return &FetchError{
        Message: message,
        Cause:   cause,
        policy:  policy,
        impact:  impact,
    }
}

// Interface implementations
func (e *FetchError) Error() string {
    return fmt.Sprintf("fetcher error: %s: %s", e.Cause, e.Message)
}

func (e *FetchError) RetryPolicy() RetryPolicy {
    return e.policy
}

func (e *FetchError) CrawlImpact() CrawlImpact {
    return e.impact
}

func (e *FetchError) Severity() Severity {
    // Derive severity from impact and policy for observability
    if e.impact == ImpactAbort {
        return SeverityFatal
    }
    if e.policy == RetryPolicyNever {
        return SeverityRecoverable
    }
    if e.policy == RetryPolicyManual {
        return SeverityRetryExhausted
    }
    return SeverityRecoverable
}
```

### 3. Retry Handler Redesign

Update `pkg/retry/handler.go` to use the new interface:

```go
// pkg/retry/handler.go

func Retry[T any](retryParam RetryParam, fn func() (T, failure.ClassifiedError)) Result[T] {
    var lastErr failure.ClassifiedError
    var zero T

    if retryParam.MaxAttempts < 1 {
        return Result[T]{
            value: zero,
            err: &RetryError{
                Message:   "max attempt cannot be 0",
                Cause:     ErrZeroAttempt,
                policy:    RetryPolicyNever,
                impact:    ImpactContinue,
            },
            attempts: 0,
        }
    }

    rng := rand.New(rand.NewSource(retryParam.RandomSeed))

    for attempt := 1; attempt <= retryParam.MaxAttempts; attempt++ {
        result, err := fn()

        if err == nil {
            return NewSuccessResult(result, attempt)
        }

        lastErr = err

        // Only auto-retry if policy is RetryPolicyAuto
        if err.RetryPolicy() != RetryPolicyAuto {
            // Not auto-retryable — return immediately with original error
            return Result[T]{
                value:    zero,
                err:      err,
                attempts: attempt,
            }
        }

        // Skip backoff on final attempt
        if attempt == retryParam.MaxAttempts {
            break
        }

        backoffDelay := timeutil.ExponentialBackoffDelay(
            attempt,
            retryParam.Jitter,
            *rng,
            retryParam.BackoffParam,
        )
        time.Sleep(backoffDelay)
    }

    // All attempts exhausted — return RetryError with manual retry policy
    return Result[T]{
        value: zero,
        err: &RetryError{
            Message:   fmt.Sprintf("exhausted %d attempts. Last error: %v", retryParam.MaxAttempts, lastErr),
            Cause:     ErrExhaustedAttempts,
            wrapped:   lastErr,          // Preserve original error
            policy:    RetryPolicyManual, // Exhausted auto-retry → manual retry eligible
            impact:    ImpactContinue,   // Don't abort crawl
        },
        attempts: retryParam.MaxAttempts,
    }
}
```

### 4. Scheduler Logic Update

Simplify scheduler to use `CrawlImpact` for control flow:

```go
// internal/scheduler/scheduler.go

func (s *Scheduler) processURL(token CrawlToken) {
    // ... pipeline stages ...
    
    result, err := s.htmlFetcher.Fetch(s.ctx, token.Depth(), token.URL(), retryParam)
    if err != nil {
        // New logic: Check CrawlImpact for control flow
        if err.CrawlImpact() == failure.ImpactAbort {
            // Systemic failure — abort entire crawl
            return CrawlingExecution{}, err
        }
        
        // Page-level failure — track for manual retry if eligible
        if err.RetryPolicy() == failure.RetryPolicyManual {
            s.frontier.BookKeepForRetry(token.URL(), err)
        }
        
        // Continue to next URL
        totalErrors++
        return
    }
    
    // ... rest of pipeline ...
}
```

### 5. Frontier Extension for Manual Retry

Add book-keeping capability to frontier:

```go
// internal/frontier/frontier.go

// RetryQueue tracks URLs eligible for manual retry
type RetryQueue interface {
    // BookKeepForRetry adds a URL to the manual retry queue
    // Called when auto-retry is exhausted or manual retry is indicated
    BookKeepForRetry(url url.URL, reason failure.ClassifiedError)
    
    // GetRetryCandidates returns URLs that should be retried
    // Called on crawl resume after user intervention
    GetRetryCandidates() []url.URL
    
    // ClearRetryQueue removes successfully processed URLs from retry queue
    ClearRetryQueue(processed []url.URL)
    
    // Persist/Load for checkpointing
    PersistRetryQueue(path string) error
    LoadRetryQueue(path string) error
}
```

### 6. Error Classification Mapping

#### Fetch Error Classifications

| Cause | RetryPolicy | CrawlImpact | Severity | Rationale |
|-------|-------------|-------------|----------|-----------|
| ErrCauseTimeout | Auto | Continue | Recoverable | Network blips are transient |
| ErrCauseNetworkFailure | Auto | Continue | Recoverable | Network issues usually resolve |
| ErrCauseRequest5xx | Auto | Continue | Recoverable | Server errors are often transient |
| ErrCauseRequestTooMany (429) | Auto | Continue | Recoverable | Rate limits backoff and retry |
| ErrCauseRequestPageForbidden (403) | Manual | Continue | RetryExhausted | Auth may be fixed later |
| ErrCauseRedirectLimitExceeded | Never | Continue | Recoverable | Configuration issue, permanent |
| ErrCauseContentTypeInvalid | Never | Continue | Recoverable | Not HTML, skip permanently |
| ErrCauseRepeated403 | Never | Continue | Recoverable | Auth definitely failing |

#### Storage Error Classifications

| Cause | RetryPolicy | CrawlImpact | Severity | Rationale |
|-------|-------------|-------------|----------|-----------|
| ErrCauseDiskFull | Manual | Continue | RetryExhausted | User can clean disk and resume |
| ErrCauseWriteFailure | Never | Continue | Recoverable | Permissions issue, permanent for this file |
| ErrCausePathError | Never | Continue | Recoverable | Path issue, permanent |
| ErrCauseHashComputationFailed | Never | Continue | Recoverable | Hash algo issue, permanent |

#### Asset Error Classifications

| Cause | RetryPolicy | CrawlImpact | Severity | Rationale |
|-------|-------------|-------------|----------|-----------|
| ErrCauseTimeout | Auto | Continue | Recoverable | Network transient |
| ErrCauseNetworkFailure | Auto | Continue | Recoverable | Network transient |
| ErrCauseRequest5xx | Auto | Continue | Recoverable | Server transient |
| ErrCauseRequestTooMany (429) | Auto | Continue | Recoverable | Rate limit transient |
| ErrCauseRequestPageForbidden (403) | Manual | Continue | RetryExhausted | May be auth fixable |
| ErrCauseRepeated403 | Never | Continue | Recoverable | Auth definitely failing |
| ErrCauseAssetTooLarge | Never | Continue | Recoverable | Policy violation, permanent |
| ErrCauseDiskFull | Manual | Continue | RetryExhausted | Clean disk and retry |

#### Robots Error Classifications

| Cause | RetryPolicy | CrawlImpact | Severity | Rationale |
|-------|-------------|-------------|----------|-----------|
| ErrCauseHttpTooManyRequests (429) | Auto | Continue | Recoverable | Backoff and retry robots.txt |
| ErrCauseHttpServerError (5xx) | Auto | Continue | Recoverable | Server transient |
| ErrCauseDisallowRoot | Never | Continue | Recoverable | Policy decision, not error |
| ErrCauseParseError | Never | Continue | Recoverable | Parse failure, continue without robots |
| ErrCauseInvalidRobotsUrl | Never | Continue | Recoverable | URL issue, skip robots check |

#### Sanitizer/Extractor/MDConvert/Normalize Errors

All content processing errors:
- **RetryPolicy**: Never (content processing doesn't benefit from retry)
- **CrawlImpact**: Continue (content errors are per-URL)
- **Severity**: Recoverable (skip URL, continue crawl)

Rationale: Content errors (invalid HTML, missing structure) are deterministic — retrying the same content will produce the same error.

### 7. Systemic Abort Conditions

These errors should have `CrawlImpact: ImpactAbort`:

| Stage | Cause | Condition | Rationale |
|-------|-------|-----------|-----------|
| config | Invalid configuration | Startup validation fails | Cannot proceed with invalid config |
| scheduler | 100% seed URL failure | All seed URLs failed admission | Cannot begin crawl |
| scheduler | Repeated robots 5xx | Robots.txt consistently unavailable | Safety — unknown crawl permission |

---

## Migration Strategy

### Phase 1: Interface Extension (Non-Breaking)

1. Add `RetryPolicy()` and `CrawlImpact()` to `ClassifiedError` interface
2. Provide default implementations that derive from existing `Retryable` field
3. No changes to stage errors yet — maintains backward compatibility

### Phase 2: Stage-by-Stage Refactoring

1. Update each stage error to use constructor-based initialization
2. Explicitly set `RetryPolicy` and `CrawlImpact` per error cause
3. Remove `IsRetryable()` methods (replaced by `RetryPolicy()`)
4. Update unit tests to verify new classifications

### Phase 3: Scheduler and Retry Handler Updates

1. Update `pkg/retry/handler.go` to check `RetryPolicy()` instead of `IsRetryable()`
2. Update `internal/scheduler/scheduler.go` to check `CrawlImpact()` for abort decisions
3. Implement frontier book-keeping for `RetryPolicyManual` errors
4. Add metrics/observability for manual retry queue

### Phase 4: Cleanup

1. Remove deprecated `Severity` derivation from `Retryable` field
2. Remove `IsRetryable()` method from all errors (dead code)
3. Update documentation and ADRs

---

## Backward Compatibility

### For Stage Errors

During migration, provide backward-compatible default implementations:

```go
// Temporary default implementation
func (e *FetchError) RetryPolicy() RetryPolicy {
    // Derive from existing Retryable field during transition
    if e.Retryable {
        return RetryPolicyAuto
    }
    // Conservative default: if not auto-retryable, assume manual retry eligible
    return RetryPolicyManual
}

func (e *FetchError) CrawlImpact() CrawlImpact {
    // During transition, never abort based on fetch errors
    // Only config/scheduler errors should abort
    return ImpactContinue
}
```

### For Retry Handler

Support both old and new interfaces during transition:

```go
func shouldAutoRetry(err error) bool {
    // Check new interface first
    if classified, ok := err.(ClassifiedError); ok {
        return classified.RetryPolicy() == RetryPolicyAuto
    }
    // Fall back to old interface during transition
    if oldStyle, ok := err.(interface{ IsRetryable() bool }); ok {
        return oldStyle.IsRetryable()
    }
    // Conservative default: don't auto-retry unknown errors
    return false
}
```

---

## Testing Strategy

### Unit Tests

Each stage error should have classification tests:

```go
func TestFetchError_Classification(t *testing.T) {
    tests := []struct {
        name     string
        cause    FetchErrorCause
        wantPolicy RetryPolicy
        wantImpact CrawlImpact
    }{
        {"5xx is auto-retryable", ErrCauseRequest5xx, RetryPolicyAuto, ImpactContinue},
        {"403 is manual-retry", ErrCauseRequestPageForbidden, RetryPolicyManual, ImpactContinue},
        {"invalid content is permanent", ErrCauseContentTypeInvalid, RetryPolicyNever, ImpactContinue},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := NewFetchError(tt.cause, "test", tt.wantPolicy, tt.wantImpact)
            assert.Equal(t, tt.wantPolicy, err.RetryPolicy())
            assert.Equal(t, tt.wantImpact, err.CrawlImpact())
        })
    }
}
```

### Integration Tests

Test end-to-end retry and abort behavior:

```go
func TestScheduler_RetryExhaustion_BooksForManualRetry(t *testing.T) {
    // Setup: URL that always returns 429
    // Execute: Crawl with max 2 attempts
    // Verify: URL is in manual retry queue, crawl continues, other URLs processed
}

func TestScheduler_SystemicFailure_AbortsCrawl(t *testing.T) {
    // Setup: Invalid configuration
    // Execute: InitializeCrawling
    // Verify: Immediate abort, no URLs processed
}
```

---

## Rollback Plan

If issues are discovered post-migration:

1. **Interface Level**: Revert to checking `Retryable` field in scheduler/retry handler
2. **Stage Level**: Temporarily revert constructor calls to direct struct initialization
3. **Feature Level**: Disable manual retry queue (continue behavior as before)

All changes are additive (new methods) rather than destructive, allowing gradual rollback.

---

## Success Criteria

The migration is successful when:

1. ✓ All stage errors implement `RetryPolicy()` and `CrawlImpact()` explicitly
2. ✓ HTTP 403/404 errors do NOT abort crawl (verified by integration test)
3. ✓ Exhausted auto-retries are tracked in manual retry queue
4. ✓ Systemic failures (config error) DO abort crawl immediately
5. ✓ No regression in existing retry behavior (network errors still retry)
6. ✓ Unit tests exist for every error cause classification

---

**End of Document**
