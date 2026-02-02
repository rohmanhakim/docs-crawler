package robots

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
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

func (e *RobotsError) Severity() internal.Severity {
	if e.Retryable {
		return internal.SeverityRecoverable
	}
	return internal.SeverityFatal
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
