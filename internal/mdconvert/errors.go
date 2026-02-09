package mdconvert

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type ConversionErrorCause string

const (
	ErrCauseConversionFailure = "conversion failed"
)

type ConversionError struct {
	Message   string
	Retryable bool
	Cause     ConversionErrorCause
}

func (e *ConversionError) Error() string {
	return fmt.Sprintf("sanitization error: %s", e.Cause)
}

func (e *ConversionError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
}

func mapConversionErrorToMetadataCause(err ConversionError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseConversionFailure:
		return metadata.CauseContentInvalid
	default:
		return metadata.CauseUnknown
	}
}
