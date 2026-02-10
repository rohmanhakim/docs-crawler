package retry

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// Retry executes the provided function with retry logic.
// It will retry the function up to MaxAttempts times, applying exponential backoff
// with jitter between attempts. Only retryable errors will trigger a retry.
//
// Type parameter T represents the return type of the function being retried.
// Returns a Result containing the value (if successful), error (if failed),
// and the number of attempts made.
func Retry[T any](retryParam RetryParam, fn func() (T, failure.ClassifiedError)) Result[T] {
	var lastErr failure.ClassifiedError
	var zero T

	if retryParam.MaxAttempts < 1 {
		return Result[T]{
			value: zero,
			err: &RetryError{
				Message:   "max attempt cannot be 0",
				Cause:     ErrZeroAttempt,
				Retryable: true,
			},
			attempts: 0,
		}
	}

	// Initialize random number generator with the provided seed
	rng := rand.New(rand.NewSource(retryParam.RandomSeed))

	for attempt := 1; attempt <= retryParam.MaxAttempts; attempt++ {
		result, err := fn()

		// Success case: no error
		if err == nil {
			return NewSuccessResult(result, attempt)
		}

		lastErr = err

		// Check if the error is retryable
		// We check if the error implements the retryable interface or has Retryable field
		shouldRetry := isErrorRetryable(err)

		// If not retryable, return immediately
		if !shouldRetry {
			return Result[T]{
				value:    zero,
				err:      err,
				attempts: attempt,
			}
		}

		// If this was the last attempt, break and return exhausted error
		if attempt == retryParam.MaxAttempts {
			break
		}

		// Compute delay for the next retry using exponential backoff with jitter
		backoffDelay := timeutil.ExponentialBackoffDelay(
			attempt,
			retryParam.Jitter,
			*rng,
			retryParam.BackoffParam,
		)

		// Sleep for the computed delay
		time.Sleep(backoffDelay)
	}

	// Return failure result when max attempts are exhausted
	return Result[T]{
		value: zero,
		err: &RetryError{
			Message:   fmt.Sprintf("exhausted %d attempts. Last error: %v", retryParam.MaxAttempts, lastErr),
			Cause:     ErrExhaustedAttempts,
			Retryable: true, // This is recoverable at scheduler level
		},
		attempts: retryParam.MaxAttempts,
	}
}

// isErrorRetryable checks if an error should be retried.
// It uses type assertion to check for the Retryable property.
func isErrorRetryable(err failure.ClassifiedError) bool {
	// Type assert to check if the error has a Retryable field/method
	type hasRetryable interface {
		IsRetryable() bool
	}

	if r, ok := err.(hasRetryable); ok {
		return r.IsRetryable()
	}

	// Check for struct with Retryable field via reflection-like interface
	// This handles errors like RetryError that have a Retryable field
	type hasRetryableField interface {
		failure.ClassifiedError
		IsRetryable() bool
	}

	if r, ok := err.(hasRetryableField); ok {
		return r.IsRetryable()
	}

	// Default to retryable if we can't determine
	// This maintains backward compatibility
	return true
}
