package robots

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type RobotsErrorCause string

const (
	// ErrCauseRepeatedFetchFailure = "repeated fetch failure"
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

type RobotsError struct {
	Message   string
	Retryable bool
	Cause     RobotsErrorCause
}

func (e *RobotsError) Error() string {
	return fmt.Sprintf("robots error: %s", e.Cause)
}

func (e *RobotsError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
}

// RetryPolicy returns the automatic retry behavior for this error.
// During transition, this derives from the existing Retryable field:
// - Retryable: true  -> RetryPolicyAuto
// - Retryable: false -> RetryPolicyManual (conservative default)
func (e *RobotsError) RetryPolicy() failure.RetryPolicy {
	if e.Retryable {
		return failure.RetryPolicyAuto
	}
	return failure.RetryPolicyManual
}

// CrawlImpact returns how the scheduler should respond to this error.
// During transition, this always returns ImpactContinue (conservative default).
// Only config/scheduler errors should abort the crawl.
func (e *RobotsError) CrawlImpact() failure.CrawlImpact {
	return failure.ImpactContinue
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
