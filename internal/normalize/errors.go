package normalize

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type NormalizationErrorCause string

const (
	// ErrCauseBrokenH1Invariant indicates that the document does not have exactly one H1 heading.
	// The sanitizer should enforce this invariant, but normalization verifies it before title extraction.
	ErrCauseBrokenH1Invariant NormalizationErrorCause = "broken H1 invariant"

	// ErrCauseEmptyContent indicates that the markdown content is empty after conversion.
	// Per design doc Section 9.2, documents must have content to be RAG-ready.
	ErrCauseEmptyContent NormalizationErrorCause = "empty markdown content"

	// ErrCauseSectionDerivationFailed indicates that the section field could not be derived
	// from the URL path. Per frontmatter.md Section 4, section must be mechanically derivable.
	ErrCauseSectionDerivationFailed NormalizationErrorCause = "section derivation failed"

	// ErrCauseTitleExtractionFailed indicates that the title could not be extracted from
	// the first H1 heading. Per frontmatter.md Section 5, title must come from content.
	ErrCauseTitleExtractionFailed NormalizationErrorCause = "title extraction failed"

	// ErrCauseHashComputationFailed indicates that doc_id or content_hash computation failed.
	// This can occur if the configured hash algorithm is unsupported or encounters an error.
	ErrCauseHashComputationFailed NormalizationErrorCause = "hash computation failed"

	// ErrCauseFrontmatterMarshalFailed indicates that YAML frontmatter serialization failed.
	// This can occur due to invalid characters, encoding issues, or marshal errors.
	ErrCauseFrontmatterMarshalFailed NormalizationErrorCause = "frontmatter marshal failed"

	// ErrCauseSkippedHeadingLevels indicates that heading levels were skipped
	// (e.g., H1 -> H3 without H2). Per Invariant N3, levels must increase by at most +1.
	ErrCauseSkippedHeadingLevels NormalizationErrorCause = "skipped heading levels"

	// ErrCauseOrphanContent indicates content exists before the first H1 heading.
	// Per Invariant N4, all content must belong to the document rooted at H1.
	ErrCauseOrphanContent NormalizationErrorCause = "orphan content outside root hierarchy"

	// ErrCauseEmptySection indicates a heading has no content before the next
	// heading of same or higher level. Per Invariant N5, sections must have content.
	ErrCauseEmptySection NormalizationErrorCause = "empty section"

	// ErrCauseBrokenAtomicBlock indicates a heading appears inside a fenced code block,
	// table, or other atomic block. Per Invariant N6, atomic blocks must remain intact.
	ErrCauseBrokenAtomicBlock NormalizationErrorCause = "broken atomic block"
)

// normalizationErrorClassifications provides explicit retry policy and impact level
// for each NormalizationErrorCause. Content processing errors are deterministic -
// retrying the same content yields the same error.
//
// Classification Rationale:
// - All causes: Never retry - content processing errors are deterministic and permanent
var normalizationErrorClassifications = map[NormalizationErrorCause]struct {
	Policy failure.RetryPolicy
	Impact failure.ImpactLevel
}{
	ErrCauseBrokenH1Invariant:        {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseEmptyContent:             {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseSectionDerivationFailed:  {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseTitleExtractionFailed:    {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseHashComputationFailed:    {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseFrontmatterMarshalFailed: {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseSkippedHeadingLevels:     {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseOrphanContent:            {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseEmptySection:             {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseBrokenAtomicBlock:        {failure.RetryPolicyNever, failure.ImpactLevelContinue},
}

// NormalizationError represents an error that occurred during markdown normalization.
// It implements failure.ClassifiedError interface with explicit retry policy
// and impact level based on the error cause.
type NormalizationError struct {
	Message string
	Cause   NormalizationErrorCause
	policy  failure.RetryPolicy
	impact  failure.ImpactLevel
}

// NewNormalizationError creates a new NormalizationError with explicit classification based on cause.
// The retry policy and crawl impact are determined by the error cause classification map.
func NewNormalizationError(cause NormalizationErrorCause, message string) *NormalizationError {
	classification := normalizationErrorClassifications[cause]
	return &NormalizationError{
		Message: message,
		Cause:   cause,
		policy:  classification.Policy,
		impact:  classification.Impact,
	}
}

func (e *NormalizationError) Error() string {
	return fmt.Sprintf("normalization error: %s", e.Cause)
}

func (e *NormalizationError) Severity() failure.Severity {
	if e.impact == failure.ImpactLevelAbort {
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
func (e *NormalizationError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

// Impact returns how the scheduler should respond to this error.
// Normalization errors never abort the crawl - they are per-URL failures.
func (e *NormalizationError) Impact() failure.ImpactLevel {
	return e.impact
}

// mapNormalizationErrorToMetadataCause maps normalize-local error semantics
// to the canonical metadata.ErrorCause table.
//
// This mapping is observational only and MUST NOT be used
// to derive control-flow decisions.
func mapNormalizationErrorToMetadataCause(err NormalizationError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseBrokenH1Invariant,
		ErrCauseSkippedHeadingLevels,
		ErrCauseOrphanContent,
		ErrCauseEmptySection,
		ErrCauseBrokenAtomicBlock,
		ErrCauseSectionDerivationFailed,
		ErrCauseTitleExtractionFailed,
		ErrCauseHashComputationFailed,
		ErrCauseFrontmatterMarshalFailed:
		return metadata.CauseInvariantViolation
	case ErrCauseEmptyContent:
		return metadata.CauseContentInvalid
	default:
		return metadata.CauseUnknown
	}
}
