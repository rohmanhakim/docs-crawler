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

// RetryPolicy returns the automatic retry behavior for this error.
// During transition, this derives from the existing Retryable field:
// - Retryable: true  -> RetryPolicyAuto
// - Retryable: false -> RetryPolicyManual (conservative default)
func (e *FileError) RetryPolicy() failure.RetryPolicy {
	if e.Retryable {
		return failure.RetryPolicyAuto
	}
	return failure.RetryPolicyManual
}

// CrawlImpact returns how the scheduler should respond to this error.
// During transition, this always returns ImpactContinue (conservative default).
// Only config/scheduler errors should abort the crawl.
func (e *FileError) CrawlImpact() failure.CrawlImpact {
	return failure.ImpactContinue
}
