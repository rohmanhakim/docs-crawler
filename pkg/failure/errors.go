package failure

import (
	"github.com/rohmanhakim/retrier"
)

// RetryPolicy defines automatic retry behavior
// This controls whether retrier.Retry() will attempt exponential backoff
type RetryPolicy int

const (
	RetryPolicyAuto   RetryPolicy = iota // Retry immediately with exponential backoff
	RetryPolicyManual                    // Do not auto-retry, but eligible for manual retry queue
	RetryPolicyNever                     // Permanent failure, do not track for retry
)

// ImpactLevel defines how the scheduler should respond
// This controls processing lifecycle decisions
type ImpactLevel int

const (
	ImpactLevelContinue ImpactLevel = iota // Continue to next item (default)
	ImpactLevelAbort                       // Abort entire operation (systemic failure)
)

// Severity provides observability and legacy compatibility
type Severity string

const (
	SeverityOK             Severity = "ok"
	SeverityRecoverable    Severity = "recoverable"
	SeverityFatal          Severity = "fatal"
	SeverityRetryExhausted Severity = "retry_exhausted" // Signals manual retry needed
)

// ClassifiedError is the primary error interface for the entire pipeline
type ClassifiedError interface {
	error

	// RetryPolicy controls automatic retry behavior
	// Used by retry handler
	RetryPolicy() RetryPolicy

	// Impact controls processing continuation/abortion
	// Used by scheduler
	Impact() ImpactLevel

	// Severity provides observability and legacy compatibility
	// Used by: metadata recording, logging, monitoring
	Severity() Severity
}

// RetryableErrorAdapter wraps ClassifiedError to implement retrier.RetryableError.
// This allows ClassifiedError types to be used with the external retry package.
type RetryableErrorAdapter struct {
	ClassifiedError
}

// AsRetryableError wraps a ClassifiedError to implement retrier.RetryableError.
// Returns nil if the error is nil.
func AsRetryableError(err ClassifiedError) *RetryableErrorAdapter {
	if err == nil {
		return nil
	}
	return &RetryableErrorAdapter{ClassifiedError: err}
}

// Error implements the error interface.
func (a *RetryableErrorAdapter) Error() string {
	return a.ClassifiedError.Error()
}

// RetryPolicy converts failure.RetryPolicy to retrier.RetryPolicy.
func (a *RetryableErrorAdapter) RetryPolicy() retrier.RetryPolicy {
	switch a.ClassifiedError.RetryPolicy() {
	case RetryPolicyAuto:
		return retrier.RetryPolicyAuto
	case RetryPolicyManual:
		return retrier.RetryPolicyManual
	case RetryPolicyNever:
		return retrier.RetryPolicyNever
	default:
		return retrier.RetryPolicyNever
	}
}

// Unwrap returns the underlying ClassifiedError for error chain support.
func (a *RetryableErrorAdapter) Unwrap() error {
	return a.ClassifiedError
}

// RetryExhaustedError wraps retrier.RetryError to implement ClassifiedError
// while preserving the full error chain for errors.As compatibility.
// This allows both errors.As(err, &retryErr) and errors.As(err, &classifiedErr) to work.
type RetryExhaustedError struct {
	RetryError *retrier.RetryError
	Classified ClassifiedError
}

// Error implements the error interface.
func (e *RetryExhaustedError) Error() string {
	return e.RetryError.Error()
}

// Unwrap returns the RetryError to preserve the error chain for errors.As.
func (e *RetryExhaustedError) Unwrap() error {
	return e.RetryError
}

// As implements the errors.As interface to allow errors.As to find the underlying ClassifiedError.
// This allows both errors.As(err, &retryErr) and errors.As(err, &fetchErr) to work.
func (e *RetryExhaustedError) As(target interface{}) bool {
	// First, try to match the Classified error (e.g., FetchError)
	if e.Classified != nil {
		if targetPtr, ok := target.(*ClassifiedError); ok {
			*targetPtr = e.Classified
			return true
		}
		// Try direct type assertion for concrete types
		switch t := target.(type) {
		case *ClassifiedError:
			*t = e.Classified
			return true
		}
	}
	return false
}

// RetryPolicy returns the retry policy from the underlying ClassifiedError.
func (e *RetryExhaustedError) RetryPolicy() RetryPolicy {
	return e.Classified.RetryPolicy()
}

// Impact returns the impact level from the underlying ClassifiedError.
func (e *RetryExhaustedError) Impact() ImpactLevel {
	return e.Classified.Impact()
}

// Severity returns SeverityRetryExhausted to signal that retries have been exhausted.
func (e *RetryExhaustedError) Severity() Severity {
	return SeverityRetryExhausted
}

// AsRetryExhaustedError wraps a retrier.RetryError and ClassifiedError into a RetryExhaustedError.
// Returns nil if either argument is nil.
func AsRetryExhaustedError(retryErr *retrier.RetryError, classified ClassifiedError) *RetryExhaustedError {
	if retryErr == nil || classified == nil {
		return nil
	}
	return &RetryExhaustedError{
		RetryError: retryErr,
		Classified: classified,
	}
}
