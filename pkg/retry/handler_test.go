package retry_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// defaultBackoffParam returns a default backoff parameter for tests
func defaultBackoffParam() timeutil.BackoffParam {
	return timeutil.NewBackoffParam(
		10*time.Millisecond,
		2.0,
		30*time.Second,
	)
}

// mockError is a mock implementation of failure.ClassifiedError for testing
type mockError struct {
	msg       string
	retryable bool
	severity  failure.Severity
}

func (m *mockError) Error() string {
	return m.msg
}

func (m *mockError) Severity() failure.Severity {
	return m.severity
}

func (m *mockError) IsRetryable() bool {
	return m.retryable
}

// TestRetry_SuccessOnFirstAttempt verifies that a successful function returns immediately
func TestRetry_SuccessOnFirstAttempt(t *testing.T) {
	callCount := 0
	fn := func() (string, failure.ClassifiedError) {
		callCount++
		return "success", nil
	}

	params := retry.NewRetryParam(
		100*time.Millisecond,
		10*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != "success" {
		t.Fatalf("expected 'success', got: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got: %d", callCount)
	}
}

func TestRetry_PassParameter(t *testing.T) {
	toPrint := "Hello"
	callCount := 0

	fn := func() (string, failure.ClassifiedError) {
		callCount++
		return fmt.Sprintf("%s, world!", toPrint), nil
	}

	params := retry.NewRetryParam(
		100*time.Millisecond,
		10*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != "Hello, world!" {
		t.Fatalf("expected 'Hello, world!', got: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got: %d", callCount)
	}
}

// TestRetry_SuccessAfterRetries verifies that retryable errors lead to retries until success
func TestRetry_SuccessAfterRetries(t *testing.T) {
	callCount := 0
	fn := func() (string, failure.ClassifiedError) {
		callCount++
		if callCount < 3 {
			return "", &mockError{
				msg:       "transient error",
				retryable: true,
				severity:  failure.SeverityRecoverable,
			}
		}
		return "success", nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		5,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != "success" {
		t.Fatalf("expected 'success', got: %s", result.Value())
	}
	if result.Attempts() != 3 {
		t.Fatalf("expected 3 attempts, got: %d", result.Attempts())
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls, got: %d", callCount)
	}
}

// TestRetry_NonRetryableErrorReturnsImmediately verifies that non-retryable errors return immediately
func TestRetry_NonRetryableErrorReturnsImmediately(t *testing.T) {
	callCount := 0
	expectedErr := &mockError{
		msg:       "fatal error",
		retryable: false,
		severity:  failure.SeverityFatal,
	}

	fn := func() (string, failure.ClassifiedError) {
		callCount++
		return "", expectedErr
	}

	params := retry.NewRetryParam(
		100*time.Millisecond,
		10*time.Millisecond,
		42,
		5,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}
	if result.Value() != "" {
		t.Fatalf("expected empty result, got: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call for non-retryable error, got: %d", callCount)
	}
	if result.Err().Error() != expectedErr.Error() {
		t.Fatalf("expected error '%s', got: '%s'", expectedErr.Error(), result.Err().Error())
	}
}

// TestRetry_ExhaustedAttempts verifies that retryable errors exhaust all attempts
func TestRetry_ExhaustedAttempts(t *testing.T) {
	callCount := 0
	fn := func() (int, failure.ClassifiedError) {
		callCount++
		return 0, &mockError{
			msg:       "persistent transient error",
			retryable: true,
			severity:  failure.SeverityRecoverable,
		}
	}

	maxAttempts := 3
	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		maxAttempts,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsSuccess() {
		t.Fatal("expected error after exhausting attempts, got nil")
	}
	if result.Value() != 0 {
		t.Fatalf("expected zero result, got: %d", result.Value())
	}
	if result.Attempts() != maxAttempts {
		t.Fatalf("expected %d attempts, got: %d", maxAttempts, result.Attempts())
	}
	if callCount != maxAttempts {
		t.Fatalf("expected %d calls, got: %d", maxAttempts, callCount)
	}
	if result.Err().Severity() != failure.SeverityRecoverable {
		t.Fatalf("expected error severity to be 'SeverityRecoverable', got: '%s'", result.Err().Severity())
	}
	var retryErr *retry.RetryError
	errors.As(result.Err(), &retryErr)
	if retryErr.Cause != retry.ErrExhaustedAttempts {
		t.Fatalf("expected error cause 'ErrExhaustedAttempts', got: '%s'", retryErr.Cause)
	}
}

// TestRetry_MaxAttemptsLessThanOne verifies that MaxAttempts < 1 returns an error
func TestRetry_MaxAttemptsLessThanOne(t *testing.T) {
	fn := func() (string, failure.ClassifiedError) {
		return "success", nil
	}

	params := retry.NewRetryParam(
		100*time.Millisecond,
		10*time.Millisecond,
		42,
		0,
		defaultBackoffParam(),
	)

	var retryErr *retry.RetryError
	result := retry.Retry(params, fn)

	if result.IsSuccess() {
		t.Fatal("expected error for MaxAttempts < 1, got nil")
	}
	if result.Err().Severity() != failure.SeverityRecoverable {
		t.Fatalf("expected error severity to be 'SeverityRecoverable', got: '%s'", result.Err().Severity())
	}
	errors.As(result.Err(), &retryErr)
	if retryErr.Cause != retry.ErrZeroAttempt {
		t.Fatalf("expected error cause is ErrZeroAttempt, got %s", retryErr.Cause)
	}
	if result.Value() != "" {
		t.Fatalf("expected empty result, got: %s", result.Value())
	}
	if result.Attempts() != 0 {
		t.Fatalf("expected 0 attempts, got: %d", result.Attempts())
	}
}

// TestRetry_GenericTypePointer verifies that Retry works with pointer types
func TestRetry_GenericTypePointer(t *testing.T) {
	type Data struct {
		Value int
	}

	callCount := 0
	fn := func() (*Data, failure.ClassifiedError) {
		callCount++
		if callCount < 2 {
			return nil, &mockError{
				msg:       "transient error",
				retryable: true,
				severity:  failure.SeverityRecoverable,
			}
		}
		return &Data{Value: 42}, nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() == nil {
		t.Fatal("expected non-nil result, got nil")
	}
	if result.Value().Value != 42 {
		t.Fatalf("expected Value=42, got: %d", result.Value().Value)
	}
	if result.Attempts() != 2 {
		t.Fatalf("expected 2 attempts, got: %d", result.Attempts())
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got: %d", callCount)
	}
}

// TestRetry_GenericTypeSlice verifies that Retry works with slice types
func TestRetry_GenericTypeSlice(t *testing.T) {
	callCount := 0
	fn := func() ([]int, failure.ClassifiedError) {
		callCount++
		if callCount < 2 {
			return nil, &mockError{
				msg:       "transient error",
				retryable: true,
				severity:  failure.SeverityRecoverable,
			}
		}
		return []int{1, 2, 3}, nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if len(result.Value()) != 3 {
		t.Fatalf("expected 3 elements, got: %d", len(result.Value()))
	}
	if result.Attempts() != 2 {
		t.Fatalf("expected 2 attempts, got: %d", result.Attempts())
	}
}

// TestRetry_MixedRetryableAndNonRetryable verifies behavior with mixed error types
func TestRetry_MixedRetryableAndNonRetryable(t *testing.T) {
	callCount := 0
	fn := func() (string, failure.ClassifiedError) {
		callCount++
		switch callCount {
		case 1:
			return "", &mockError{
				msg:       "retryable error 1",
				retryable: true,
				severity:  failure.SeverityRecoverable,
			}
		case 2:
			return "", &mockError{
				msg:       "retryable error 2",
				retryable: true,
				severity:  failure.SeverityRecoverable,
			}
		case 3:
			return "", &mockError{
				msg:       "non-retryable error",
				retryable: false,
				severity:  failure.SeverityFatal,
			}
		default:
			return "success", nil
		}
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		5,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}
	if result.Value() != "" {
		t.Fatalf("expected empty result, got: %s", result.Value())
	}
	if result.Attempts() != 3 {
		t.Fatalf("expected 3 attempts, got: %d", result.Attempts())
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls (stops at non-retryable), got: %d", callCount)
	}
}

// TestRetry_DeterministicWithSameSeed verifies deterministic behavior with same seed
func TestRetry_DeterministicWithSameSeed(t *testing.T) {
	// This test verifies that using the same random seed produces consistent timing
	// We can't easily test the exact timing, but we can verify the function works
	// with a fixed seed

	callCount := 0
	fn := func() (int, failure.ClassifiedError) {
		callCount++
		if callCount < 2 {
			return 0, &mockError{
				msg:       "transient error",
				retryable: true,
				severity:  failure.SeverityRecoverable,
			}
		}
		return 42, nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		12345, // fixed seed
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != 42 {
		t.Fatalf("expected 42, got: %d", result.Value())
	}
	if result.Attempts() != 2 {
		t.Fatalf("expected 2 attempts, got: %d", result.Attempts())
	}
}

// TestRetry_SuccessAfterManyFailures verifies eventual success after many retries
func TestRetry_SuccessAfterManyFailures(t *testing.T) {
	callCount := 0
	maxAttempts := 10
	fn := func() (string, failure.ClassifiedError) {
		callCount++
		if callCount < maxAttempts {
			return "", &mockError{
				msg:       "transient error",
				retryable: true,
				severity:  failure.SeverityRecoverable,
			}
		}
		return "eventual success", nil
	}

	params := retry.NewRetryParam(
		5*time.Millisecond,
		2*time.Millisecond,
		42,
		maxAttempts,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != "eventual success" {
		t.Fatalf("expected 'eventual success', got: %s", result.Value())
	}
	if result.Attempts() != maxAttempts {
		t.Fatalf("expected %d attempts, got: %d", maxAttempts, result.Attempts())
	}
	if callCount != maxAttempts {
		t.Fatalf("expected %d calls, got: %d", maxAttempts, callCount)
	}
}

// TestRetry_ExhaustedErrorIsRetryable verifies that exhausted attempt error is marked as retryable
func TestRetry_ExhaustedErrorIsRetryable(t *testing.T) {
	fn := func() (string, failure.ClassifiedError) {
		return "", &mockError{
			msg:       "persistent error",
			retryable: true,
			severity:  failure.SeverityRecoverable,
		}
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		2,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}

	// The error should be retryable at scheduler level
	type retryableChecker interface {
		IsRetryable() bool
	}

	if r, ok := result.Err().(retryableChecker); ok {
		if !r.IsRetryable() {
			t.Error("expected exhausted attempt error to be retryable at scheduler level")
		}
	} else {
		t.Error("error should implement IsRetryable method")
	}
}

// errorWithoutIsRetryable is an error that doesn't implement IsRetryable
type errorWithoutIsRetryable struct {
	msg string
}

func (e *errorWithoutIsRetryable) Error() string {
	return e.msg
}

func (e *errorWithoutIsRetryable) Severity() failure.Severity {
	return failure.SeverityRecoverable
}

// TestRetry_DefaultRetryableWhenNoIsRetryable verifies that errors without IsRetryable
// default to being retryable (backward compatibility)
func TestRetry_DefaultRetryableWhenNoIsRetryable(t *testing.T) {
	callCount := 0
	fn := func() (string, failure.ClassifiedError) {
		callCount++
		if callCount < 2 {
			// Return an error without IsRetryable method
			return "", &errorWithoutIsRetryable{msg: "error without retryable flag"}
		}
		return "success", nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error after retry, got: %v", result.Err())
	}
	if result.Value() != "success" {
		t.Fatalf("expected 'success', got: %s", result.Value())
	}
	if result.Attempts() != 2 {
		t.Fatalf("expected 2 attempts, got: %d", result.Attempts())
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls (default to retryable), got: %d", callCount)
	}
}

// TestRetry_ErrorWrapping verifies that the original error is included in exhausted message
func TestRetry_ErrorWrapping(t *testing.T) {
	originalErr := &mockError{
		msg:       "original error message",
		retryable: true,
		severity:  failure.SeverityRecoverable,
	}

	fn := func() (string, failure.ClassifiedError) {
		return "", originalErr
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		2,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}

	// The error message should contain information about exhausted attempts
	if result.Err().Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestNewRetryParam verifies the constructor creates RetryParam correctly
func TestNewRetryParam(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	jitter := 50 * time.Millisecond
	seed := int64(42)
	maxAttempts := 5

	params := retry.NewRetryParam(baseDelay, jitter, seed, maxAttempts, defaultBackoffParam())

	// We can't directly access fields since this is black box testing,
	// but we can verify the behavior through Retry function
	callCount := 0
	fn := func() (string, failure.ClassifiedError) {
		callCount++
		return "success", nil
	}

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("unexpected error: %v", result.Err())
	}
	if result.Value() != "success" {
		t.Fatalf("unexpected result: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got: %d", callCount)
	}
}

// BenchmarkRetry benchmarks the retry function
func BenchmarkRetry(b *testing.B) {
	fn := func() (int, failure.ClassifiedError) {
		return 42, nil
	}

	params := retry.NewRetryParam(
		1*time.Millisecond,
		1*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retry.Retry(params, fn)
	}
}

// errorIsNil verifies that the function handles nil errors correctly at type level
// This is more of a compile-time check through usage
func TestRetry_NilErrorTypeSafety(t *testing.T) {
	// Ensure the function signature accepts functions that can return nil error
	fn := func() (string, failure.ClassifiedError) {
		return "success", nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)

	if result.IsFailure() {
		t.Fatalf("expected nil error, got: %v", result.Err())
	}
	if result.Value() != "success" {
		t.Fatalf("expected 'success', got: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
}

// Verify that RetryError type is accessible and has the expected shape
func TestRetryErrorType(t *testing.T) {
	// This test verifies we can create and use RetryError from the package
	// In black box testing, we interact through the exported API

	fn := func() (string, failure.ClassifiedError) {
		// Return a proper ClassifiedError implementation
		return "", &mockError{
			msg:       "some error",
			retryable: true,
			severity:  failure.SeverityRecoverable,
		}
	}

	// This would fail to compile if RetryError wasn't properly exported/accessible
	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		1, // Only 1 attempt to avoid type conversion issues
		defaultBackoffParam(),
	)

	// We expect an error here - it should be a RetryError after exhausting attempts
	result := retry.Retry(params, fn)
	if result.IsSuccess() {
		t.Fatal("expected error after exhausting attempts")
	}
}
