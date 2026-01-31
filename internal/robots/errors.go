package robots

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

type RobotsErrorCause string

const (
	ErrCauseRepeatedFetchFailure = "repeated fetch failure"
	ErrCauseDisallowRoot         = "roots disallowed to be crawled"
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
	case ErrCauseRepeatedFetchFailure:
		return metadata.CauseNetworkFailure
	case ErrCauseDisallowRoot:
		return metadata.CausePolicyDisallow
	default:
		return metadata.CauseUnknown
	}
}
