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
	// This is a non-recoverable error that indicates a pipeline invariant violation.
	ErrCauseBrokenH1Invariant NormalizationErrorCause = "broken H1 invariant"

	// ErrCauseEmptyContent indicates that the markdown content is empty after conversion.
	// Per design doc Section 9.2, documents must have content to be RAG-ready.
	// This is a non-recoverable error as empty documents provide no value for ingestion.
	ErrCauseEmptyContent NormalizationErrorCause = "empty markdown content"

	// ErrCauseSectionDerivationFailed indicates that the section field could not be derived
	// from the URL path. Per frontmatter.md Section 4, section must be mechanically derivable
	// from the first meaningful path segment after stripping allowedPathPrefix.
	// This is a non-recoverable error indicating a URL structure invariant violation.
	ErrCauseSectionDerivationFailed NormalizationErrorCause = "section derivation failed"

	// ErrCauseTitleExtractionFailed indicates that the title could not be extracted from
	// the first H1 heading. Per frontmatter.md Section 5, title must come from content.
	// This occurs when an H1 exists but contains no extractable text.
	// This is a non-recoverable error as title is required for indexing and display.
	ErrCauseTitleExtractionFailed NormalizationErrorCause = "title extraction failed"

	// ErrCauseHashComputationFailed indicates that doc_id or content_hash computation failed.
	// This can occur if the configured hash algorithm is unsupported or encounters an error.
	// Both hashes are critical for change detection and deduplication.
	// This is a non-recoverable error indicating a configuration or system issue.
	ErrCauseHashComputationFailed NormalizationErrorCause = "hash computation failed"

	// ErrCauseFrontmatterMarshalFailed indicates that YAML frontmatter serialization failed.
	// This can occur due to invalid characters, encoding issues, or marshal errors.
	// This is a non-recoverable error as the document cannot be written without valid frontmatter.
	ErrCauseFrontmatterMarshalFailed NormalizationErrorCause = "frontmatter marshal failed"

	// ErrCauseSkippedHeadingLevels indicates that heading levels were skipped
	// (e.g., H1 → H3 without H2). Per Invariant N3, levels must increase by at most +1.
	// This is a non-recoverable error indicating a structural contract violation.
	ErrCauseSkippedHeadingLevels NormalizationErrorCause = "skipped heading levels"

	// ErrCauseOrphanContent indicates content exists before the first H1 heading.
	// Per Invariant N4, all content must belong to the document rooted at H1.
	// This is a non-recoverable error indicating a structural contract violation.
	ErrCauseOrphanContent NormalizationErrorCause = "orphan content outside root hierarchy"

	// ErrCauseEmptySection indicates a heading has no content before the next
	// heading of same or higher level. Per Invariant N5, sections must have content.
	// This is a non-recoverable error indicating a structural contract violation.
	ErrCauseEmptySection NormalizationErrorCause = "empty section"

	// ErrCauseBrokenAtomicBlock indicates a heading appears inside a fenced code block,
	// table, or other atomic block. Per Invariant N6, atomic blocks must remain intact.
	// This is a non-recoverable error indicating a structural contract violation.
	ErrCauseBrokenAtomicBlock NormalizationErrorCause = "broken atomic block"
)

type NormalizationError struct {
	Message   string
	Retryable bool
	Cause     NormalizationErrorCause
}

func (e *NormalizationError) Error() string {
	return fmt.Sprintf("normalization error: %s", e.Cause)
}

func (e *NormalizationError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
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
