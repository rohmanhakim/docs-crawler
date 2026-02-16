package robots

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type RobotsErrorCause string

const (
	ErrCauseDisallowRoot         = "root disallowed to be crawled"
	ErrCauseInvalidRobotsUrl     = "invalid robots.txt URL"
	ErrCausePreFetchFailure      = "failed before making fetch"
	ErrCauseHttpFetchFailure     = "failed to fetch"
	ErrCauseHttpTooManyRequests  = "too many requests"
	ErrCauseHttpTooManyRedirects = "too many redirects"
	ErrCauseHttpServerError      = "http server error"
	ErrCauseHttpUnexpectedStatus = "unexpected http status"
	ErrCauseParseError           = "failed to parse robots.txt"
)

// robotsErrorClassifications provides explicit retry policy and crawl impact
// for each RobotsErrorCause. This replaces the old Retryable boolean field
// with explicit two-dimensional classification.
//
// Classification Rationale:
// - HttpTooManyRequests (429): Auto-retry with backoff, transient rate limiting
// - HttpServerError (5xx): Auto-retry, transient server issues
// - HttpFetchFailure: Auto-retry, network issues are usually transient
// - DisallowRoot: Never retry, policy decision, not an error
// - ParseError: Never retry, malformed robots.txt won't become valid
// - InvalidRobotsUrl: Never retry, URL configuration issue
// - PreFetchFailure: Never retry, configuration/internal error
// - HttpTooManyRedirects: Never retry, redirect loop won't resolve
// - HttpUnexpectedStatus: Never retry, unknown status, likely permanent
var robotsErrorClassifications = map[RobotsErrorCause]struct {
	Policy failure.RetryPolicy
	Impact failure.CrawlImpact
}{
	ErrCauseHttpTooManyRequests:  {failure.RetryPolicyAuto, failure.ImpactContinue},
	ErrCauseHttpServerError:      {failure.RetryPolicyAuto, failure.ImpactContinue},
	ErrCauseHttpFetchFailure:     {failure.RetryPolicyAuto, failure.ImpactContinue},
	ErrCauseDisallowRoot:         {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseParseError:           {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseInvalidRobotsUrl:     {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCausePreFetchFailure:      {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseHttpTooManyRedirects: {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseHttpUnexpectedStatus: {failure.RetryPolicyNever, failure.ImpactContinue},
}

// RobotsError represents an error that occurred during robots.txt processing.
// It implements failure.ClassifiedError interface with explicit retry policy
// and crawl impact based on the error cause.
type RobotsError struct {
	Message string
	Cause   RobotsErrorCause
	policy  failure.RetryPolicy
	impact  failure.CrawlImpact
}

// NewRobotsError creates a new RobotsError with explicit classification based on cause.
// The retry policy and crawl impact are determined by the error cause classification map.
func NewRobotsError(cause RobotsErrorCause, message string) *RobotsError {
	classification := robotsErrorClassifications[cause]
	return &RobotsError{
		Message: message,
		Cause:   cause,
		policy:  classification.Policy,
		impact:  classification.Impact,
	}
}

func (e *RobotsError) Error() string {
	return fmt.Sprintf("robots error: %s", e.Cause)
}

func (e *RobotsError) Severity() failure.Severity {
	if e.impact == failure.ImpactAbort {
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
func (e *RobotsError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

// CrawlImpact returns how the scheduler should respond to this error.
// Robots errors never abort the crawl - they are per-URL failures.
func (e *RobotsError) CrawlImpact() failure.CrawlImpact {
	return e.impact
}

// mapRobotsErrorToMetadataCause maps robots-local error semantics
// to the canonical metadata.ErrorCause table.
//
// This mapping is observational only and MUST NOT be used
// to derive control-flow decisions.
func mapRobotsErrorToMetadataCause(err *RobotsError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseDisallowRoot:
		return metadata.CausePolicyDisallow
	case ErrCauseInvalidRobotsUrl:
		return metadata.CauseInvariantViolation
	case ErrCausePreFetchFailure:
		return metadata.CauseUnknown
	case ErrCauseHttpFetchFailure:
		return metadata.CauseNetworkFailure
	case ErrCauseHttpTooManyRequests:
		return metadata.CauseNetworkFailure
	case ErrCauseHttpTooManyRedirects:
		return metadata.CauseNetworkFailure
	case ErrCauseHttpServerError:
		return metadata.CauseNetworkFailure
	case ErrCauseHttpUnexpectedStatus:
		return metadata.CauseNetworkFailure
	case ErrCauseParseError:
		return metadata.CauseContentInvalid
	default:
		return metadata.CauseUnknown
	}
}
