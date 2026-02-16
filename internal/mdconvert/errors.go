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

// RetryPolicy returns the automatic retry behavior for this error.
// During transition, this derives from the existing Retryable field:
// - Retryable: true  -> RetryPolicyAuto
// - Retryable: false -> RetryPolicyManual (conservative default)
func (e *ConversionError) RetryPolicy() failure.RetryPolicy {
	if e.Retryable {
		return failure.RetryPolicyAuto
	}
	return failure.RetryPolicyManual
}

// CrawlImpact returns how the scheduler should respond to this error.
// During transition, this always returns ImpactContinue (conservative default).
// Only config/scheduler errors should abort the crawl.
func (e *ConversionError) CrawlImpact() failure.CrawlImpact {
	return failure.ImpactContinue
}

func mapConversionErrorToMetadataCause(err ConversionError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseConversionFailure:
		return metadata.CauseContentInvalid
	default:
		return metadata.CauseUnknown
	}
}
