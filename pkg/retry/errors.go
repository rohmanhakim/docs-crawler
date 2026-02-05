package retry

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type RetryErrorCause string

const (
	ErrZeroAttempt       = "zero attempt"
	ErrExhaustedAttempts = "exhausted attempt"
)

type RetryError struct {
	Message   string
	Retryable bool
	Cause     RetryErrorCause
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("retry error: %s, %s", e.Cause, e.Message)
}

func (e *RetryError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
}

func (e *RetryError) IsRetryable() bool {
	return e.Retryable
}

// Is allows errors.Is to match RetryError types
func (e *RetryError) Is(target error) bool {
	_, ok := target.(*RetryError)
	return ok
}
