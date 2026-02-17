package mdconvert

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

// TestConversionError_Classifications tests that all ConversionErrorCause values
// have the correct RetryPolicy and ImpactLevel classification.
// This ensures the two-dimensional error classification is correctly applied.
func TestConversionError_Classifications(t *testing.T) {
	tests := []struct {
		name         string
		cause        ConversionErrorCause
		wantPolicy   failure.RetryPolicy
		wantImpact   failure.ImpactLevel
		wantSeverity failure.Severity
	}{
		// Never retry: content processing errors are deterministic
		{
			name:         "ErrCauseConversionFailure should be RetryPolicyNever",
			cause:        ErrCauseConversionFailure,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewConversionError(tt.cause, "test message")

			if err.RetryPolicy() != tt.wantPolicy {
				t.Errorf("RetryPolicy() = %v, want %v", err.RetryPolicy(), tt.wantPolicy)
			}

			if err.Impact() != tt.wantImpact {
				t.Errorf("ImpactLevel() = %v, want %v", err.Impact(), tt.wantImpact)
			}

			if err.Severity() != tt.wantSeverity {
				t.Errorf("Severity() = %v, want %v", err.Severity(), tt.wantSeverity)
			}
		})
	}
}

// TestConversionError_AllCausesCovered verifies that all ConversionErrorCause constants
// are covered by the classification map. This is a safety check to ensure
// no causes are accidentally omitted.
func TestConversionError_AllCausesCovered(t *testing.T) {
	allCauses := []ConversionErrorCause{
		ErrCauseConversionFailure,
	}

	for _, cause := range allCauses {
		t.Run(string(cause), func(t *testing.T) {
			if _, ok := conversionErrorClassifications[cause]; !ok {
				t.Errorf("cause %q not found in conversionErrorClassifications map", cause)
			}
		})
	}
}

// TestConversionError_NewConversionErrorVariations tests that NewConversionError correctly
// handles the error message parameter.
func TestConversionError_NewConversionErrorVariations(t *testing.T) {
	tests := []struct {
		name    string
		cause   ConversionErrorCause
		message string
	}{
		{"custom message", ErrCauseConversionFailure, "pandoc failed with exit code 4"},
		{"empty message", ErrCauseConversionFailure, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewConversionError(tt.cause, tt.message)

			if err.Cause != tt.cause {
				t.Errorf("Cause = %v, want %v", err.Cause, tt.cause)
			}

			// Error() should not be empty
			if err.Error() == "" {
				t.Error("Error() should not be empty")
			}
		})
	}
}
