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

// RetryPolicy returns the automatic retry behavior for this error.
// When RetryError is returned (exhausted attempts), it should be RetryPolicyManual
// since auto-retry is exhausted but manual retry may be possible.
func (e *RetryError) RetryPolicy() failure.RetryPolicy {
	if e.Retryable {
		return failure.RetryPolicyManual
	}
	return failure.RetryPolicyNever
}

// CrawlImpact returns how the scheduler should respond to this error.
// RetryError should never abort the crawl. It's a per-URL failure.
func (e *RetryError) CrawlImpact() failure.CrawlImpact {
	return failure.ImpactContinue
}

// Is allows errors.Is to match RetryError types
func (e *RetryError) Is(target error) bool {
	_, ok := target.(*RetryError)
	return ok
}
