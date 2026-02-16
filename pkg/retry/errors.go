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

// RetryError represents an error that occurred during retry attempts.
// It stores the original error for debugging and implements the ClassifiedError interface.
type RetryError struct {
	Message string
	Cause   RetryErrorCause
	wrapped error               // Original error that caused the retry failure
	policy  failure.RetryPolicy // Cached policy for interface method
	impact  failure.CrawlImpact // Cached impact for interface method
}

// NewRetryError creates a new RetryError with explicit classification.
// Parameters:
//   - cause: The error cause (ErrZeroAttempt or ErrExhaustedAttempts)
//   - message: Human-readable error message
//   - policy: failure.RetryPolicyAuto, failure.RetryPolicyManual, or failure.RetryPolicyNever
//   - impact: failure.ImpactContinue or failure.ImpactAbort
//   - wrapped: The original error that caused the retry failure (may be nil)
func NewRetryError(cause RetryErrorCause, message string, policy failure.RetryPolicy, impact failure.CrawlImpact, wrapped error) *RetryError {
	return &RetryError{
		Message: message,
		Cause:   cause,
		wrapped: wrapped,
		policy:  policy,
		impact:  impact,
	}
}

// Error returns the error message implementing the error interface.
func (e *RetryError) Error() string {
	if e.wrapped != nil {
		return fmt.Sprintf("retry error: %s, %s: %v", e.Cause, e.Message, e.wrapped)
	}
	return fmt.Sprintf("retry error: %s, %s", e.Cause, e.Message)
}

// Unwrap returns the wrapped error for error chain support.
func (e *RetryError) Unwrap() error {
	return e.wrapped
}

// Severity returns the severity for observability.
// Derives from policy and impact for backward compatibility.
func (e *RetryError) Severity() failure.Severity {
	if e.impact == failure.ImpactAbort {
		return failure.SeverityFatal
	}
	switch e.policy {
	case failure.RetryPolicyAuto:
		return failure.SeverityRecoverable
	case failure.RetryPolicyManual:
		return failure.SeverityRetryExhausted
	case failure.RetryPolicyNever:
		return failure.SeverityFatal
	default:
		return failure.SeverityRecoverable
	}
}

// IsRetryable returns whether this error is retryable.
// Deprecated: Use RetryPolicy() instead.
func (e *RetryError) IsRetryable() bool {
	return e.policy == failure.RetryPolicyAuto
}

// RetryPolicy returns the automatic retry behavior for this error.
// When RetryError is returned (exhausted attempts), it returns the cached policy.
func (e *RetryError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

// CrawlImpact returns how the scheduler should respond to this error.
// RetryError should never abort the crawl. It returns the cached impact.
func (e *RetryError) CrawlImpact() failure.CrawlImpact {
	return e.impact
}

// Is allows errors.Is to match RetryError types
func (e *RetryError) Is(target error) bool {
	_, ok := target.(*RetryError)
	return ok
}
