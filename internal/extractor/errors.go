package extractor

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type ExtractionErrorCause string

const (
	ErrCauseNoContent ExtractionErrorCause = "no content"
	ErrCauseNotHTML   ExtractionErrorCause = "not an HTML content"
)

// extractionErrorClassifications provides explicit retry policy and crawl impact
// for each ExtractionErrorCause. Content processing errors are deterministic -
// retrying the same content yields the same error.
//
// Classification Rationale:
// - NoContent: Never retry - no content to extract, retrying won't help
// - NotHTML: Never retry - content type mismatch, retrying won't help
var extractionErrorClassifications = map[ExtractionErrorCause]struct {
	Policy failure.RetryPolicy
	Impact failure.CrawlImpact
}{
	ErrCauseNoContent: {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseNotHTML:   {failure.RetryPolicyNever, failure.ImpactContinue},
}

// ExtractionError represents an error that occurred during content extraction.
// It implements failure.ClassifiedError interface with explicit retry policy
// and crawl impact based on the error cause.
type ExtractionError struct {
	Message string
	Cause   ExtractionErrorCause
	policy  failure.RetryPolicy
	impact  failure.CrawlImpact
}

// NewExtractionError creates a new ExtractionError with explicit classification based on cause.
// The retry policy and crawl impact are determined by the error cause classification map.
func NewExtractionError(cause ExtractionErrorCause, message string) *ExtractionError {
	classification := extractionErrorClassifications[cause]
	return &ExtractionError{
		Message: message,
		Cause:   cause,
		policy:  classification.Policy,
		impact:  classification.Impact,
	}
}

func (e *ExtractionError) Error() string {
	return fmt.Sprintf("extraction error: %s", e.Cause)
}

func (e *ExtractionError) Severity() failure.Severity {
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
func (e *ExtractionError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

// CrawlImpact returns how the scheduler should respond to this error.
// Extraction errors never abort the crawl - they are per-URL failures.
func (e *ExtractionError) CrawlImpact() failure.CrawlImpact {
	return e.impact
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
