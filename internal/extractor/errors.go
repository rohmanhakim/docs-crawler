package extractor

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type ExtractionErrorCause string

const (
	ErrCauseNoContent = "no content"
	ErrCauseNotHTML   = "not an HTML content"
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

// RetryPolicy returns the automatic retry behavior for this error.
// During transition, this derives from the existing Retryable field:
// - Retryable: true  -> RetryPolicyAuto
// - Retryable: false -> RetryPolicyManual (conservative default)
func (e *ExtractionError) RetryPolicy() failure.RetryPolicy {
	if e.Retryable {
		return failure.RetryPolicyAuto
	}
	return failure.RetryPolicyManual
}

// CrawlImpact returns how the scheduler should respond to this error.
// During transition, this always returns ImpactContinue (conservative default).
// Only config/scheduler errors should abort the crawl.
func (e *ExtractionError) CrawlImpact() failure.CrawlImpact {
	return failure.ImpactContinue
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
