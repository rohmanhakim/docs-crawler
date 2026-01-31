package normalize

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

type NormalizationErrorCause string

const (
	ErrCauseBrokenH1Invariant = "broken H1 invariant"
)

type NormalizationError struct {
	Message   string
	Retryable bool
	Cause     NormalizationErrorCause
}

func (e *NormalizationError) Error() string {
	return fmt.Sprintf("normalization error: %s", e.Cause)
}

func (e *NormalizationError) Severity() internal.Severity {
	if e.Retryable {
		return internal.SeverityRecoverable
	}
	return internal.SeverityFatal
}

// mapNormalizationErrorToMetadataCause maps normalize-local error semantics
// to the canonical metadata.ErrorCause table.
//
// This mapping is observational only and MUST NOT be used
// to derive control-flow decisions.
func mapNormalizationErrorToMetadataCause(err NormalizationError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseBrokenH1Invariant:
		return metadata.CauseInvariantViolation
	default:
		return metadata.CauseUnknown
	}
}
