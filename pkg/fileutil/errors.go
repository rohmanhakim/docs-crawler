package fileutil

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type FileErrorCause string

const (
	ErrCausePathError FileErrorCause = "path error"
)

// fileErrorClassifications provides explicit retry policy and crawl impact
// for each FileErrorCause. This replaces the old Retryable boolean field
// with explicit two-dimensional classification.
var fileErrorClassifications = map[FileErrorCause]struct {
	Policy failure.RetryPolicy
	Impact failure.CrawlImpact
}{
	ErrCausePathError: {
		Policy: failure.RetryPolicyNever,
		Impact: failure.ImpactContinue,
	},
}

type FileError struct {
	Message string
	Cause   FileErrorCause
	policy  failure.RetryPolicy
	impact  failure.CrawlImpact
}

// NewFileError creates a new FileError with explicit classification
func NewFileError(cause FileErrorCause, message string) *FileError {
	classification, ok := fileErrorClassifications[cause]
	if !ok {
		// Default classification for unknown causes
		return &FileError{
			Message: message,
			Cause:   cause,
			policy:  failure.RetryPolicyNever,
			impact:  failure.ImpactContinue,
		}
	}
	return &FileError{
		Message: message,
		Cause:   cause,
		policy:  classification.Policy,
		impact:  classification.Impact,
	}
}

func (e *FileError) Error() string {
	return fmt.Sprintf("file error: %s: %s", e.Cause, e.Message)
}

func (e *FileError) Severity() failure.Severity {
	if e.impact == failure.ImpactAbort {
		return failure.SeverityFatal
	}
	if e.policy == failure.RetryPolicyNever {
		return failure.SeverityRecoverable
	}
	if e.policy == failure.RetryPolicyManual {
		return failure.SeverityRetryExhausted
	}
	return failure.SeverityRecoverable
}

func (e *FileError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

func (e *FileError) CrawlImpact() failure.CrawlImpact {
	return e.impact
}
