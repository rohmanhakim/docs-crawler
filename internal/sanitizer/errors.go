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
	// - Retryable: No
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
	// - Severity: Fatal
	// - Retryable: No
	ErrCauseCompetingRoots = "competing document roots"

	// ErrCauseNoStructuralAnchor: Document has no headings and no structural anchors (H3 invariant violation).
	// - Severity: Fatal
	// - Retryable: No
	ErrCauseNoStructuralAnchor = "no structural anchor"

	// ErrCauseMultipleH1NoRoot: Multiple H1 elements without a provable primary root (H2 invariant violation).
	// - Severity: Fatal
	// - Retryable: No
	ErrCauseMultipleH1NoRoot = "multiple h1 without primary root"

	// ErrCauseImpliedMultipleDocs: Document implies multiple documents (S5 invariant violation).
	// - Severity: Fatal
	// - Retryable: No
	ErrCauseImpliedMultipleDocs = "implied multiple documents"

	// ErrCauseAmbiguousDOM: Document has structurally ambiguous DOM (E1 invariant violation).
	// - Severity: Fatal
	// - Retryable: No
	ErrCauseAmbiguousDOM = "ambiguous dom structure"
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
