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

// fetchErrorClassifications provides explicit retry policy and impact level
// for each FetchErrorCause. This replaces the old Retryable boolean field
// with explicit two-dimensional classification.
var fetchErrorClassifications = map[FetchErrorCause]struct {
	Policy failure.RetryPolicy
	Impact failure.ImpactLevel
}{
	ErrCauseTimeout:               {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseNetworkFailure:        {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseReadResponseBodyError: {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseContentTypeInvalid:    {failure.RetryPolicyManual, failure.ImpactLevelContinue},
	ErrCauseRedirectLimitExceeded: {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseRequestPageForbidden:  {failure.RetryPolicyManual, failure.ImpactLevelContinue},
	ErrCauseRequestTooMany:        {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseRequest5xx:            {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseRepeated403:           {failure.RetryPolicyNever, failure.ImpactLevelContinue},
}

// FetchError represents an error that occurred during HTTP fetch operations.
// It implements failure.ClassifiedError interface with explicit retry policy
// and impact level based on the error cause.
type FetchError struct {
	Message string
	Cause   FetchErrorCause
	policy  failure.RetryPolicy
	impact  failure.ImpactLevel
}

// NewFetchError creates a new FetchError with explicit classification based on cause.
// The retry policy and crawl impact are determined by the error cause classification map.
func NewFetchError(cause FetchErrorCause, message string) *FetchError {
	classification := fetchErrorClassifications[cause]
	return &FetchError{
		Message: message,
		Cause:   cause,
		policy:  classification.Policy,
		impact:  classification.Impact,
	}
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("fetcher error: %s", e.Cause)
}

func (e *FetchError) Severity() failure.Severity {
	if e.impact == failure.ImpactLevelAbort {
		return failure.SeverityFatal
	}
	switch e.policy {
	case failure.RetryPolicyAuto:
		return failure.SeverityRecoverable
	case failure.RetryPolicyManual:
		return failure.SeverityRetryExhausted
	case failure.RetryPolicyNever:
		return failure.SeverityRecoverable
	default:
		return failure.SeverityRecoverable
	}
}

// RetryPolicy returns the automatic retry behavior for this error.
// This is now explicitly set based on the error cause, not derived from a boolean.
func (e *FetchError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

// Impact returns how the scheduler should respond to this error.
// Fetch errors never abort the crawl - they are per-URL failures.
func (e *FetchError) Impact() failure.ImpactLevel {
	return e.impact
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
