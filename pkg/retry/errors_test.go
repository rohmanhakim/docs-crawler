package retry_test

import (
	"errors"
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
)

// TestRetryError_NewRetryError tests the creation of RetryError with various parameters.
func TestRetryError_NewRetryError(t *testing.T) {
	innerErr := errors.New("original error")

	tests := []struct {
		name        string
		cause       retry.RetryErrorCause
		message     string
		policy      failure.RetryPolicy
		impact      failure.ImpactLevel
		wrapped     error
		wantCause   retry.RetryErrorCause
		wantMessage string
		wantPolicy  failure.RetryPolicy
		wantImpact  failure.ImpactLevel
		wantWrapped bool
	}{
		{
			name:        "ErrZeroAttempt with auto policy",
			cause:       retry.ErrZeroAttempt,
			message:     "cannot retry with zero attempts",
			policy:      failure.RetryPolicyAuto,
			impact:      failure.ImpactLevelContinue,
			wrapped:     innerErr,
			wantCause:   retry.ErrZeroAttempt,
			wantMessage: "cannot retry with zero attempts",
			wantPolicy:  failure.RetryPolicyAuto,
			wantImpact:  failure.ImpactLevelContinue,
			wantWrapped: true,
		},
		{
			name:        "ErrExhaustedAttempts with manual policy",
			cause:       retry.ErrExhaustedAttempts,
			message:     "max retries exceeded",
			policy:      failure.RetryPolicyManual,
			impact:      failure.ImpactLevelContinue,
			wrapped:     innerErr,
			wantCause:   retry.ErrExhaustedAttempts,
			wantMessage: "max retries exceeded",
			wantPolicy:  failure.RetryPolicyManual,
			wantImpact:  failure.ImpactLevelContinue,
			wantWrapped: true,
		},
		{
			name:        "ErrExhaustedAttempts with never policy and abort impact",
			cause:       retry.ErrExhaustedAttempts,
			message:     "permanent failure",
			policy:      failure.RetryPolicyNever,
			impact:      failure.ImpactLevelAbort,
			wrapped:     nil,
			wantCause:   retry.ErrExhaustedAttempts,
			wantMessage: "permanent failure",
			wantPolicy:  failure.RetryPolicyNever,
			wantImpact:  failure.ImpactLevelAbort,
			wantWrapped: false,
		},
		{
			name:        "nil wrapped error",
			cause:       retry.ErrZeroAttempt,
			message:     "test",
			policy:      failure.RetryPolicyNever,
			impact:      failure.ImpactLevelContinue,
			wrapped:     nil,
			wantCause:   retry.ErrZeroAttempt,
			wantMessage: "test",
			wantPolicy:  failure.RetryPolicyNever,
			wantImpact:  failure.ImpactLevelContinue,
			wantWrapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := retry.NewRetryError(tt.cause, tt.message, tt.policy, tt.impact, tt.wrapped)

			if err.Cause != tt.wantCause {
				t.Errorf("Cause = %v, want %v", err.Cause, tt.wantCause)
			}

			if err.Message != tt.wantMessage {
				t.Errorf("Message = %v, want %v", err.Message, tt.wantMessage)
			}

			if err.RetryPolicy() != tt.wantPolicy {
				t.Errorf("RetryPolicy() = %v, want %v", err.RetryPolicy(), tt.wantPolicy)
			}

			if err.Impact() != tt.wantImpact {
				t.Errorf("Impact() = %v, want %v", err.Impact(), tt.wantImpact)
			}

			gotWrapped := err.Unwrap() != nil
			if gotWrapped != tt.wantWrapped {
				t.Errorf("Unwrap() nil = %v, want %v", gotWrapped, tt.wantWrapped)
			}

			if tt.wantWrapped && err.Unwrap() != tt.wrapped {
				t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), tt.wrapped)
			}
		})
	}
}

// TestRetryError_Error tests the Error() method format.
func TestRetryError_Error(t *testing.T) {
	innerErr := errors.New("original error")

	tests := []struct {
		name         string
		cause        retry.RetryErrorCause
		message      string
		wrapped      error
		wantContains []string
	}{
		{
			name:         "with wrapped error",
			cause:        retry.ErrExhaustedAttempts,
			message:      "max retries exceeded",
			wrapped:      innerErr,
			wantContains: []string{"retry error", "exhausted attempt", "max retries exceeded", "original error"},
		},
		{
			name:         "without wrapped error",
			cause:        retry.ErrZeroAttempt,
			message:      "zero attempts not allowed",
			wrapped:      nil,
			wantContains: []string{"retry error", "zero attempt", "zero attempts not allowed"},
		},
		{
			name:         "empty message with wrapped",
			cause:        retry.ErrExhaustedAttempts,
			message:      "",
			wrapped:      innerErr,
			wantContains: []string{"retry error", "exhausted attempt"},
		},
		{
			name:         "empty message without wrapped",
			cause:        retry.ErrZeroAttempt,
			message:      "",
			wrapped:      nil,
			wantContains: []string{"retry error", "zero attempt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := retry.NewRetryError(tt.cause, tt.message, failure.RetryPolicyAuto, failure.ImpactLevelContinue, tt.wrapped)
			errStr := err.Error()

			for _, want := range tt.wantContains {
				if !containsString(errStr, want) {
					t.Errorf("Error() = %q, should contain %q", errStr, want)
				}
			}
		})
	}
}

// TestRetryError_Unwrap tests the Unwrap() method for error chain support.
func TestRetryError_Unwrap(t *testing.T) {
	innerErr := errors.New("network error")

	// Test with wrapped error
	err := retry.NewRetryError(retry.ErrExhaustedAttempts, "max retries", failure.RetryPolicyManual, failure.ImpactLevelContinue, innerErr)
	if err.Unwrap() != innerErr {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), innerErr)
	}

	// Test with nil wrapped error
	errNil := retry.NewRetryError(retry.ErrZeroAttempt, "test", failure.RetryPolicyNever, failure.ImpactLevelContinue, nil)
	if errNil.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil", errNil.Unwrap())
	}
}

// TestRetryError_Severity tests all severity derivation paths.
func TestRetryError_Severity(t *testing.T) {
	tests := []struct {
		name         string
		policy       failure.RetryPolicy
		impact       failure.ImpactLevel
		wantSeverity failure.Severity
	}{
		// ImpactLevelAbort takes precedence
		{
			name:         "abort impact with auto policy",
			policy:       failure.RetryPolicyAuto,
			impact:       failure.ImpactLevelAbort,
			wantSeverity: failure.SeverityFatal,
		},
		{
			name:         "abort impact with manual policy",
			policy:       failure.RetryPolicyManual,
			impact:       failure.ImpactLevelAbort,
			wantSeverity: failure.SeverityFatal,
		},
		{
			name:         "abort impact with never policy",
			policy:       failure.RetryPolicyNever,
			impact:       failure.ImpactLevelAbort,
			wantSeverity: failure.SeverityFatal,
		},
		// ImpactLevelContinue with various policies
		{
			name:         "continue with auto policy",
			policy:       failure.RetryPolicyAuto,
			impact:       failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "continue with manual policy",
			policy:       failure.RetryPolicyManual,
			impact:       failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRetryExhausted,
		},
		{
			name:         "continue with never policy",
			policy:       failure.RetryPolicyNever,
			impact:       failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityFatal,
		},
		// Unknown policy (should default to Recoverable)
		{
			name:         "unknown policy",
			policy:       failure.RetryPolicy(99),
			impact:       failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := retry.NewRetryError(retry.ErrExhaustedAttempts, "test", tt.policy, tt.impact, nil)
			if err.Severity() != tt.wantSeverity {
				t.Errorf("Severity() = %v, want %v", err.Severity(), tt.wantSeverity)
			}
		})
	}
}

// TestRetryError_RetryPolicy tests that RetryPolicy() returns the cached policy.
func TestRetryError_RetryPolicy(t *testing.T) {
	policies := []failure.RetryPolicy{
		failure.RetryPolicyAuto,
		failure.RetryPolicyManual,
		failure.RetryPolicyNever,
	}

	for _, policy := range policies {
		t.Run("policy", func(t *testing.T) {
			err := retry.NewRetryError(retry.ErrExhaustedAttempts, "test", policy, failure.ImpactLevelContinue, nil)
			if err.RetryPolicy() != policy {
				t.Errorf("RetryPolicy() = %v, want %v", err.RetryPolicy(), policy)
			}
		})
	}
}

// TestRetryError_Impact tests that Impact() returns the cached impact.
func TestRetryError_Impact(t *testing.T) {
	impacts := []failure.ImpactLevel{
		failure.ImpactLevelContinue,
		failure.ImpactLevelAbort,
	}

	for _, impact := range impacts {
		t.Run("impact", func(t *testing.T) {
			err := retry.NewRetryError(retry.ErrExhaustedAttempts, "test", failure.RetryPolicyAuto, impact, nil)
			if err.Impact() != impact {
				t.Errorf("Impact() = %v, want %v", err.Impact(), impact)
			}
		})
	}
}

// TestRetryError_Is tests the Is() method for errors.Is support.
func TestRetryError_Is(t *testing.T) {
	err := retry.NewRetryError(retry.ErrExhaustedAttempts, "test", failure.RetryPolicyManual, failure.ImpactLevelContinue, nil)

	// Should match RetryError
	if !err.Is(&retry.RetryError{}) {
		t.Error("Is() should return true for RetryError target")
	}

	// Should not match other error types
	if err.Is(errors.New("other error")) {
		t.Error("Is() should return false for non-RetryError target")
	}

	// Should not match nil
	if err.Is(nil) {
		t.Error("Is() should return false for nil target")
	}
}

// TestRetryError_ImplementsClassifiedError verifies that RetryError implements
// the ClassifiedError interface.
func TestRetryError_ImplementsClassifiedError(t *testing.T) {
	err := retry.NewRetryError(retry.ErrZeroAttempt, "test", failure.RetryPolicyAuto, failure.ImpactLevelContinue, nil)

	// Verify all interface methods exist and return valid values
	var _ failure.ClassifiedError = err

	// Verify basic interface contract
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
	if err.RetryPolicy() < 0 {
		t.Error("RetryPolicy() should return valid policy")
	}
	if err.Impact() < 0 {
		t.Error("Impact() should return valid impact")
	}
	if err.Severity() == "" {
		t.Error("Severity() should not be empty")
	}
}

// TestRetryError_Causes tests both RetryErrorCause constants.
func TestRetryError_Causes(t *testing.T) {
	causes := []retry.RetryErrorCause{
		retry.ErrZeroAttempt,
		retry.ErrExhaustedAttempts,
	}

	for _, cause := range causes {
		t.Run(string(cause), func(t *testing.T) {
			err := retry.NewRetryError(cause, "test", failure.RetryPolicyAuto, failure.ImpactLevelContinue, nil)
			if err.Cause != cause {
				t.Errorf("Cause = %v, want %v", err.Cause, cause)
			}
		})
	}
}

// TestRetryError_ErrorChain tests errors.Is and errors.As support.
func TestRetryError_ErrorChain(t *testing.T) {
	innerErr := errors.New("original error")
	err := retry.NewRetryError(retry.ErrExhaustedAttempts, "max retries", failure.RetryPolicyManual, failure.ImpactLevelContinue, innerErr)

	// Test errors.Is
	if !errors.Is(err, innerErr) {
		t.Error("errors.Is should find wrapped error")
	}

	// Test errors.As
	var retryErr *retry.RetryError
	if !errors.As(err, &retryErr) {
		t.Error("errors.As should find RetryError")
	}
}

// containsString is a helper to check if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
