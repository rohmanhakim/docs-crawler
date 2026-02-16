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

type StorageError struct {
	Message   string
	Retryable bool
	Cause     StorageErrorCause
	Path      string
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("storage error: %s", e.Cause)
}

func (e *StorageError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
}

// RetryPolicy returns the automatic retry behavior for this error.
// During transition, this derives from the existing Retryable field:
// - Retryable: true  -> RetryPolicyAuto
// - Retryable: false -> RetryPolicyManual (conservative default)
func (e *StorageError) RetryPolicy() failure.RetryPolicy {
	if e.Retryable {
		return failure.RetryPolicyAuto
	}
	return failure.RetryPolicyManual
}

// CrawlImpact returns how the scheduler should respond to this error.
// During transition, this always returns ImpactContinue (conservative default).
// Only config/scheduler errors should abort the crawl.
func (e *StorageError) CrawlImpact() failure.CrawlImpact {
	return failure.ImpactContinue
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
