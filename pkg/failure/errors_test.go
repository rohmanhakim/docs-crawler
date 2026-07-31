package failure

import (
	"testing"

	gopipeline "github.com/rohmanhakim/gopipeline"
	"github.com/rohmanhakim/retrier"
)

// TestRetryPolicyValues verifies that RetryPolicy constants have expected values
func TestRetryPolicyValues(t *testing.T) {
	tests := []struct {
		name     string
		policy   RetryPolicy
		wantZero bool
	}{
		{"RetryPolicyAuto is 0", RetryPolicyAuto, true},
		{"RetryPolicyManual is 1", RetryPolicyManual, false},
		{"RetryPolicyNever is 2", RetryPolicyNever, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantZero && tt.policy != 0 {
				t.Errorf("expected %s to be 0, got %d", tt.name, tt.policy)
			}
			if !tt.wantZero && tt.policy == 0 {
				t.Errorf("expected %s to not be 0", tt.name)
			}
		})
	}
}

// TestImpactLevelValues verifies that ImpactLevel constants have expected values
func TestImpactLevelValues(t *testing.T) {
	tests := []struct {
		name     string
		impact   ImpactLevel
		wantZero bool
	}{
		{"ImpactLevelContinue is 0", ImpactLevelContinue, true},
		{"ImpactLevelAbort is 1", ImpactLevelAbort, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantZero && tt.impact != 0 {
				t.Errorf("expected %s to be 0, got %d", tt.name, tt.impact)
			}
			if !tt.wantZero && tt.impact == 0 {
				t.Errorf("expected %s to not be 0", tt.name)
			}
		})
	}
}

// TestSeverityValues verifies Severity string constants
func TestSeverityValues(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     Severity
	}{
		{"SeverityOK", SeverityOK, "ok"},
		{"SeverityRecoverable", SeverityRecoverable, "recoverable"},
		{"SeverityFatal", SeverityFatal, "fatal"},
		{"SeverityRetryExhausted", SeverityRetryExhausted, "retry_exhausted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.severity != tt.want {
				t.Errorf("expected %s, got %s", tt.want, tt.severity)
			}
		})
	}
}

// mockError implements ClassifiedError for testing
type mockError struct {
	err         string
	retryPolicy RetryPolicy
	impactLevel ImpactLevel
	severity    Severity
}

func (e *mockError) Error() string {
	return e.err
}

func (e *mockError) RetryPolicy() RetryPolicy {
	return e.retryPolicy
}

func (e *mockError) Impact() ImpactLevel {
	return e.impactLevel
}

func (e *mockError) Severity() Severity {
	return e.severity
}

// TestClassifiedErrorInterface verifies that mockError implements ClassifiedError
func TestClassifiedErrorInterface(t *testing.T) {
	var _ ClassifiedError = &mockError{}
}

// TestClassifiedErrorSatisfiesStageError verifies ClassifiedError satisfies gopipeline.StageError.
func TestClassifiedErrorSatisfiesStageError(t *testing.T) {
	// Compile-time interface check: ClassifiedError must satisfy gopipeline.StageError.
	// gopipeline.StageError is just `interface{ error }`, so any error type satisfies it.
	// This test documents and locks in that guarantee.
	var _ gopipeline.StageError = &mockError{}
}

// TestAsRetryableErrorSatisfiesGopipelineRetryableError verifies the adapter
// satisfies gopipeline.RetryableError (which is retrier.RetryableError).
func TestAsRetryableErrorSatisfiesGopipelineRetryableError(t *testing.T) {
	err := &mockError{err: "test", retryPolicy: RetryPolicyAuto, impactLevel: ImpactLevelContinue}
	adapter := AsRetryableError(err)

	// Compile-time interface check
	var _ gopipeline.RetryableError = adapter
}

// TestRetryableErrorAdapterNilReturnsNil verifies nil safety.
func TestRetryableErrorAdapterNilReturnsNil(t *testing.T) {
	adapter := AsRetryableError(nil)
	if adapter != nil {
		t.Error("AsRetryableError(nil) should return nil")
	}
}

// TestRetryableErrorAdapterRetryPolicyMapping verifies failure.RetryPolicy maps correctly
// to retrier.RetryPolicy (and by extension gopipeline.RetryableError).
func TestRetryableErrorAdapterRetryPolicyMapping(t *testing.T) {
	tests := []struct {
		name           string
		failurePolicy  RetryPolicy
		wantRetrier    retrier.RetryPolicy
		wantGopipeline gopipeline.RetryPolicy
	}{
		{
			name:           "Auto maps to retrier Auto and gopipeline Auto",
			failurePolicy:  RetryPolicyAuto,
			wantRetrier:    retrier.RetryPolicyAuto,
			wantGopipeline: gopipeline.RetryPolicyAuto,
		},
		{
			name:           "Manual maps to retrier Manual and gopipeline Manual",
			failurePolicy:  RetryPolicyManual,
			wantRetrier:    retrier.RetryPolicyManual,
			wantGopipeline: gopipeline.RetryPolicyManual,
		},
		{
			name:           "Never maps to retrier Never and gopipeline Never",
			failurePolicy:  RetryPolicyNever,
			wantRetrier:    retrier.RetryPolicyNever,
			wantGopipeline: gopipeline.RetryPolicyNever,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &mockError{
				err:         "test",
				retryPolicy: tt.failurePolicy,
				impactLevel: ImpactLevelContinue,
			}
			adapter := AsRetryableError(err)

			// Verify retrier.RetryPolicy mapping
			if got := adapter.RetryPolicy(); got != tt.wantRetrier {
				t.Errorf("RetryPolicy() = %v, want %v", got, tt.wantRetrier)
			}

			// Verify gopipeline.RetryPolicy mapping (same underlying type)
			if got := gopipeline.RetryPolicy(adapter.RetryPolicy()); got != tt.wantGopipeline {
				t.Errorf("gopipeline.RetryPolicy() = %v, want %v", got, tt.wantGopipeline)
			}
		})
	}
}

// TestRetryableErrorAdapterErrorAndUnwrap verifies Error() and Unwrap() delegation.
func TestRetryableErrorAdapterErrorAndUnwrap(t *testing.T) {
	original := &mockError{err: "original error", retryPolicy: RetryPolicyNever, impactLevel: ImpactLevelAbort}
	adapter := AsRetryableError(original)

	if adapter.Error() != "original error" {
		t.Errorf("Error() = %q, want %q", adapter.Error(), "original error")
	}

	if adapter.Unwrap() != original {
		t.Error("Unwrap() should return the original ClassifiedError")
	}
}

// TestClassifiedErrorImplementations tests various implementations of ClassifiedError
func TestClassifiedErrorImplementations(t *testing.T) {
	tests := []struct {
		name       string
		err        ClassifiedError
		wantPolicy RetryPolicy
		wantImpact ImpactLevel
	}{
		{
			name:       "auto-retry error",
			err:        &mockError{err: "auto-retry", retryPolicy: RetryPolicyAuto, impactLevel: ImpactLevelContinue},
			wantPolicy: RetryPolicyAuto,
			wantImpact: ImpactLevelContinue,
		},
		{
			name:       "manual retry error",
			err:        &mockError{err: "manual-retry", retryPolicy: RetryPolicyManual, impactLevel: ImpactLevelContinue},
			wantPolicy: RetryPolicyManual,
			wantImpact: ImpactLevelContinue,
		},
		{
			name:       "never retry error",
			err:        &mockError{err: "never-retry", retryPolicy: RetryPolicyNever, impactLevel: ImpactLevelAbort},
			wantPolicy: RetryPolicyNever,
			wantImpact: ImpactLevelAbort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.RetryPolicy(); got != tt.wantPolicy {
				t.Errorf("RetryPolicy() = %v, want %v", got, tt.wantPolicy)
			}
			if got := tt.err.Impact(); got != tt.wantImpact {
				t.Errorf("Impact() = %v, want %v", got, tt.wantImpact)
			}
		})
	}
}
