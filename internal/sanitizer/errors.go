package sanitizer

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type SanitizationErrorCause string

const (
	// ErrCauseUnparseableHTML:
	// - Severity: Fatal
	// - Examples:
	//   - HTML parser fails catastrophically
	//   - DOM tree cannot be constructed at all
	//   - Content is effectively binary garbage
	//   - The extracted content node is not a tree (e.g. nil root)
	//
	// - Example HTML-like payload:
	// ```
	// <\x00\xff\xfe\x01\x02>
	// ````
	// or truncated content that defeats even tolerant parsers.
	ErrCauseUnparseableHTML = "unparseable html"

	// ErrCauseCompetingRoots: Multiple competing document roots found (S3 invariant violation).
	ErrCauseCompetingRoots = "competing document roots"

	// ErrCauseNoStructuralAnchor: Document has no headings and no structural anchors (H3 invariant violation).
	ErrCauseNoStructuralAnchor = "no structural anchor"

	// ErrCauseMultipleH1NoRoot: Multiple H1 elements without a provable primary root (H2 invariant violation).
	ErrCauseMultipleH1NoRoot = "multiple h1 without primary root"

	// ErrCauseImpliedMultipleDocs: Document implies multiple documents (S5 invariant violation).
	ErrCauseImpliedMultipleDocs = "implied multiple documents"

	// ErrCauseAmbiguousDOM: Document has structurally ambiguous DOM (E1 invariant violation).
	ErrCauseAmbiguousDOM = "ambiguous dom structure"
)

// sanitizationErrorClassifications provides explicit retry policy and crawl impact
// for each SanitizationErrorCause. Content processing errors are deterministic -
// retrying the same content yields the same error.
//
// Classification Rationale:
// - All causes: Never retry - content processing errors are deterministic and permanent
var sanitizationErrorClassifications = map[SanitizationErrorCause]struct {
	Policy failure.RetryPolicy
	Impact failure.CrawlImpact
}{
	ErrCauseUnparseableHTML:     {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseCompetingRoots:      {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseNoStructuralAnchor:  {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseMultipleH1NoRoot:    {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseImpliedMultipleDocs: {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseAmbiguousDOM:        {failure.RetryPolicyNever, failure.ImpactContinue},
}

// SanitizationError represents an error that occurred during HTML sanitization.
// It implements failure.ClassifiedError interface with explicit retry policy
// and crawl impact based on the error cause.
type SanitizationError struct {
	Message string
	Cause   SanitizationErrorCause
	policy  failure.RetryPolicy
	impact  failure.CrawlImpact
}

// NewSanitizationError creates a new SanitizationError with explicit classification based on cause.
// The retry policy and crawl impact are determined by the error cause classification map.
func NewSanitizationError(cause SanitizationErrorCause, message string) *SanitizationError {
	classification := sanitizationErrorClassifications[cause]
	return &SanitizationError{
		Message: message,
		Cause:   cause,
		policy:  classification.Policy,
		impact:  classification.Impact,
	}
}

func (e *SanitizationError) Error() string {
	return fmt.Sprintf("sanitization error: %s", e.Cause)
}

func (e *SanitizationError) Severity() failure.Severity {
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
func (e *SanitizationError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

// CrawlImpact returns how the scheduler should respond to this error.
// Sanitization errors never abort the crawl - they are per-URL failures.
func (e *SanitizationError) CrawlImpact() failure.CrawlImpact {
	return e.impact
}

// mapSanitizationErrorToMetadataCause maps sanitizer-local error semantics
// to the canonical metadata.ErrorCause table.
//
// This mapping is observational only and MUST NOT be used
// to derive control-flow decisions.
func mapSanitizationErrorToMetadataCause(err SanitizationError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseUnparseableHTML:
		return metadata.CauseContentInvalid
	case ErrCauseCompetingRoots,
		ErrCauseNoStructuralAnchor,
		ErrCauseMultipleH1NoRoot,
		ErrCauseImpliedMultipleDocs,
		ErrCauseAmbiguousDOM:
		return metadata.CauseInvariantViolation
	default:
		return metadata.CauseUnknown
	}
}
