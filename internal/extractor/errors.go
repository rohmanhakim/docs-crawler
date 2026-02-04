package extractor

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type ExtractionErrorCause string

const (
	ErrCauseNoContent = "no content"
)

type ExtractionError struct {
	Message   string
	Retryable bool
	Cause     ExtractionErrorCause
}

func (e *ExtractionError) Error() string {
	return fmt.Sprintf("extraction error: %s", e.Cause)
}

func (e *ExtractionError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
}

// mapExtractionErrorToMetadataCause maps extractor-local error semantics
// to the canonical metadata.ErrorCause table.
//
// This mapping is observational only and MUST NOT be used
// to derive control-flow decisions.
func mapExtractionErrorToMetadataCause(err *ExtractionError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseNoContent:
		return metadata.CauseContentInvalid
	default:
		return metadata.CauseUnknown
	}
}
