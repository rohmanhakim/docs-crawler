package storage

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type StorageErrorCause string

const (
	ErrCauseDiskFull              StorageErrorCause = "disk is full"
	ErrCauseWriteFailure          StorageErrorCause = "write failed"
	ErrCauseHashComputationFailed StorageErrorCause = "hash computation failed"
	ErrCausePathError             StorageErrorCause = "path error"
)

// storageErrorClassifications provides explicit retry policy and crawl impact
// for each StorageErrorCause. This replaces the old Retryable boolean field
// with explicit two-dimensional classification.
//
// Classification Rationale:
// - DiskFull: Manual retry - user can clean disk and retry later
// - WriteFailure: Never retry - permissions or I/O issue, permanent
// - HashComputationFailed: Never retry - algorithm issue, permanent
// - PathError: Never retry - path configuration issue, permanent
var storageErrorClassifications = map[StorageErrorCause]struct {
	Policy failure.RetryPolicy
	Impact failure.CrawlImpact
}{
	ErrCauseDiskFull:              {failure.RetryPolicyManual, failure.ImpactContinue},
	ErrCauseWriteFailure:          {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCauseHashComputationFailed: {failure.RetryPolicyNever, failure.ImpactContinue},
	ErrCausePathError:             {failure.RetryPolicyNever, failure.ImpactContinue},
}

// StorageError represents an error that occurred during storage operations.
// It implements failure.ClassifiedError interface with explicit retry policy
// and crawl impact based on the error cause.
type StorageError struct {
	Message string
	Cause   StorageErrorCause
	Path    string
	policy  failure.RetryPolicy
	impact  failure.CrawlImpact
}

// NewStorageError creates a new StorageError with explicit classification based on cause.
// The retry policy and crawl impact are determined by the error cause classification map.
func NewStorageError(cause StorageErrorCause, message string, path string) *StorageError {
	classification := storageErrorClassifications[cause]
	return &StorageError{
		Message: message,
		Cause:   cause,
		Path:    path,
		policy:  classification.Policy,
		impact:  classification.Impact,
	}
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("storage error: %s", e.Cause)
}

func (e *StorageError) Severity() failure.Severity {
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
// This is now explicitly set based on the error cause, not derived from a boolean.
func (e *StorageError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

// CrawlImpact returns how the scheduler should respond to this error.
// Storage errors never abort the crawl - they are per-URL failures.
func (e *StorageError) CrawlImpact() failure.CrawlImpact {
	return e.impact
}

// mapStorageErrorToMetadataCause maps storage-local error semantics
// to the canonical metadata.ErrorCause table.
//
// This mapping is observational only and MUST NOT be used
// to derive control-flow decisions.
func mapStorageErrorToMetadataCause(err *StorageError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseDiskFull:
		return metadata.CauseStorageFailure
	case ErrCauseWriteFailure:
		return metadata.CauseStorageFailure
	case ErrCausePathError:
		return metadata.CauseStorageFailure
	case ErrCauseHashComputationFailed:
		return metadata.CauseInvariantViolation
	default:
		return metadata.CauseUnknown
	}
}
