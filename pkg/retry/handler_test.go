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

func (m *mockError) RetryPolicy() failure.RetryPolicy {
	if m.retryable {
		return failure.RetryPolicyAuto
	}
	return failure.RetryPolicyManual
}

func (m *mockError) CrawlImpact() failure.CrawlImpact {
	return failure.ImpactContinue
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
	// New behavior: exhausted attempts should return SeverityRetryExhausted
	if result.Err().Severity() != failure.SeverityRetryExhausted {
		t.Fatalf("expected error severity to be 'SeverityRetryExhausted', got: '%s'", result.Err().Severity())
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
	// New behavior: zero attempt is a config error -> SeverityFatal
	if result.Err().Severity() != failure.SeverityFatal {
		t.Fatalf("expected error severity to be 'SeverityFatal', got: '%s'", result.Err().Severity())
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
		12345,
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

// TestRetry_ExhaustedErrorIsRetryable verifies that exhausted attempt error has RetryPolicyManual
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

	// New behavior: exhausted error should have RetryPolicyManual (eligible for manual retry)
	// Check using RetryPolicy() method
	if result.Err().RetryPolicy() != failure.RetryPolicyManual {
		t.Errorf("expected exhausted error to have RetryPolicyManual, got: %v", result.Err().RetryPolicy())
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

func (e *errorWithoutIsRetryable) RetryPolicy() failure.RetryPolicy {
	return failure.RetryPolicyAuto
}

func (e *errorWithoutIsRetryable) CrawlImpact() failure.CrawlImpact {
	return failure.ImpactContinue
}

// TestRetry_DefaultRetryableWhenNoIsRetryable verifies backward compatibility
func TestRetry_DefaultRetryableWhenNoIsRetryable(t *testing.T) {
	callCount := 0
	fn := func() (string, failure.ClassifiedError) {
		callCount++
		if callCount < 2 {
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

// TestRetry_ErrorWrapping verifies that the original error is included
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

	if result.Err().Error() == "" {
		t.Error("expected non-empty error message")
	}

	// Verify the wrapped error can be accessed via Unwrap
	var retryErr *retry.RetryError
	if errors.As(result.Err(), &retryErr) {
		// Unwrap should return the original error (the mockError wrapped in RetryError)
		unwrapped := retryErr.Unwrap()
		if unwrapped == nil {
			t.Error("expected wrapped error to be accessible via Unwrap")
		}
	}
}

// TestNewRetryParam verifies the constructor creates RetryParam correctly
func TestNewRetryParam(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	jitter := 50 * time.Millisecond
	seed := int64(42)
	maxAttempts := 5

	params := retry.NewRetryParam(baseDelay, jitter, seed, maxAttempts, defaultBackoffParam())

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

func TestRetry_NilErrorTypeSafety(t *testing.T) {
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

func TestRetryErrorType(t *testing.T) {
	fn := func() (string, failure.ClassifiedError) {
		return "", &mockError{
			msg:       "some error",
			retryable: true,
			severity:  failure.SeverityRecoverable,
		}
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		1,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, fn)
	if result.IsSuccess() {
		t.Fatal("expected error after exhausting attempts")
	}
}
