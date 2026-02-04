package sanitizer

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type SanitizationErrorCause string

const (
	ErrCauseBrokenDOM = "broken dom"
)

type SanitizationError struct {
	Message   string
	Retryable bool
	Cause     SanitizationErrorCause
}

func (e *SanitizationError) Error() string {
	return fmt.Sprintf("sanitization error: %s", e.Cause)
}

func (e *SanitizationError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
}

// mapSanitizationErrorToMetadataCause maps sanitizer-local error semantics
// to the canonical metadata.ErrorCause table.
//
// This mapping is observational only and MUST NOT be used
// to derive control-flow decisions.
func mapSanitizationErrorToMetadataCause(err SanitizationError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseBrokenDOM:
		return metadata.CauseContentInvalid
	default:
		return metadata.CauseUnknown
	}
}
