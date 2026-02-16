package mdconvert

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type ConversionErrorCause string

const (
	ErrCauseConversionFailure ConversionErrorCause = "conversion failed"
)

// conversionErrorClassifications provides explicit retry policy and crawl impact
// for each ConversionErrorCause. Content processing errors are deterministic -
// retrying the same content yields the same error.
//
// Classification Rationale:
// - ConversionFailure: Never retry - conversion errors are deterministic and permanent
var conversionErrorClassifications = map[ConversionErrorCause]struct {
	Policy failure.RetryPolicy
	Impact failure.CrawlImpact
}{
	ErrCauseConversionFailure: {failure.RetryPolicyNever, failure.ImpactContinue},
}

// ConversionError represents an error that occurred during markdown conversion.
// It implements failure.ClassifiedError interface with explicit retry policy
// and crawl impact based on the error cause.
type ConversionError struct {
	Message string
	Cause   ConversionErrorCause
	policy  failure.RetryPolicy
	impact  failure.CrawlImpact
}

// NewConversionError creates a new ConversionError with explicit classification based on cause.
// The retry policy and crawl impact are determined by the error cause classification map.
func NewConversionError(cause ConversionErrorCause, message string) *ConversionError {
	classification := conversionErrorClassifications[cause]
	return &ConversionError{
		Message: message,
		Cause:   cause,
		policy:  classification.Policy,
		impact:  classification.Impact,
	}
}

func (e *ConversionError) Error() string {
	return fmt.Sprintf("conversion error: %s", e.Cause)
}

func (e *ConversionError) Severity() failure.Severity {
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
// Content processing errors are deterministic and never benefit from retry.
func (e *ConversionError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

// CrawlImpact returns how the scheduler should respond to this error.
// Conversion errors never abort the crawl - they are per-URL failures.
func (e *ConversionError) CrawlImpact() failure.CrawlImpact {
	return e.impact
}

func mapConversionErrorToMetadataCause(err ConversionError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseConversionFailure:
		return metadata.CauseContentInvalid
	default:
		return metadata.CauseUnknown
	}
}
