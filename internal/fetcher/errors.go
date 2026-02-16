package fetcher

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type FetchErrorCause string

const (
	ErrCauseTimeout               = "timeout"
	ErrCauseNetworkFailure        = "network issues"
	ErrCauseReadResponseBodyError = "failed to read response body"
	ErrCauseContentTypeInvalid    = "non-HTML content"
	ErrCauseRedirectLimitExceeded = "reached redirect limit"
	ErrCauseRequestPageForbidden  = "forbidden"
	ErrCauseRequestTooMany        = "too many requests"
	ErrCauseRequest5xx            = "5xx"
	ErrCauseRepeated403           = "repeated 403s"
)

type FetchError struct {
	Message   string
	Retryable bool
	Cause     FetchErrorCause
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("fetcher error: %s", e.Cause)
}

func (e *FetchError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
}

// RetryPolicy returns the automatic retry behavior for this error.
// During transition, this derives from the existing Retryable field:
// - Retryable: true  -> RetryPolicyAuto
// - Retryable: false -> RetryPolicyManual (conservative default)
func (e *FetchError) RetryPolicy() failure.RetryPolicy {
	if e.Retryable {
		return failure.RetryPolicyAuto
	}
	return failure.RetryPolicyManual
}

// CrawlImpact returns how the scheduler should respond to this error.
// During transition, this always returns ImpactContinue (conservative default).
// Only config/scheduler errors should abort the crawl.
func (e *FetchError) CrawlImpact() failure.CrawlImpact {
	return failure.ImpactContinue
}

// mapFetchErrorToMetadataCause maps fetcher-local error semantics
// to the canonical metadata.ErrorCause table.
//
// This mapping is observational only and MUST NOT be used
// to derive control-flow decisions.
func mapFetchErrorToMetadataCause(err *FetchError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseTimeout:
		return metadata.CauseNetworkFailure
	case ErrCauseRequestTooMany:
		return metadata.CausePolicyDisallow
	case ErrCauseRepeated403:
		return metadata.CausePolicyDisallow
	default:
		return metadata.CauseUnknown
	}
}
