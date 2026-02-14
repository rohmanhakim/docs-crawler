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
