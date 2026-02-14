package fileutil

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type FileErrorCause string

const (
	ErrCausePathError FileErrorCause = "path error"
)

type FileError struct {
	Message   string
	Retryable bool
	Cause     FileErrorCause
}

func (e *FileError) Error() string {
	return fmt.Sprintf("storage error: %s", e.Cause)
}

func (e *FileError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
}
