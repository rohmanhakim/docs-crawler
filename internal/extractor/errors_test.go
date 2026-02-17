package extractor

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

// TestExtractionError_Classifications tests that all ExtractionErrorCause values
// have the correct RetryPolicy and CrawlImpact classification.
// This ensures the two-dimensional error classification is correctly applied.
func TestExtractionError_Classifications(t *testing.T) {
	tests := []struct {
		name         string
		cause        ExtractionErrorCause
		wantPolicy   failure.RetryPolicy
		wantImpact   failure.ImpactLevel
		wantSeverity failure.Severity
	}{
		// Never retry: content processing errors are deterministic
		{
			name:         "ErrCauseNoContent should be RetryPolicyNever",
			cause:        ErrCauseNoContent,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseNotHTML should be RetryPolicyNever",
			cause:        ErrCauseNotHTML,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewExtractionError(tt.cause, "test message")

			if err.RetryPolicy() != tt.wantPolicy {
				t.Errorf("RetryPolicy() = %v, want %v", err.RetryPolicy(), tt.wantPolicy)
			}

			if err.Impact() != tt.wantImpact {
				t.Errorf("CrawlImpact() = %v, want %v", err.Impact(), tt.wantImpact)
			}

			if err.Severity() != tt.wantSeverity {
				t.Errorf("Severity() = %v, want %v", err.Severity(), tt.wantSeverity)
			}
		})
	}
}

// TestExtractionError_AllCausesCovered verifies that all ExtractionErrorCause constants
// are covered by the classification map. This is a safety check to ensure
// no causes are accidentally omitted.
func TestExtractionError_AllCausesCovered(t *testing.T) {
	allCauses := []ExtractionErrorCause{
		ErrCauseNoContent,
		ErrCauseNotHTML,
	}

	for _, cause := range allCauses {
		t.Run(string(cause), func(t *testing.T) {
			if _, ok := extractionErrorClassifications[cause]; !ok {
				t.Errorf("cause %q not found in extractionErrorClassifications map", cause)
			}
		})
	}
}

// TestExtractionError_NewExtractionErrorVariations tests that NewExtractionError correctly
// handles the error message parameter.
func TestExtractionError_NewExtractionErrorVariations(t *testing.T) {
	tests := []struct {
		name    string
		cause   ExtractionErrorCause
		message string
	}{
		{"custom message", ErrCauseNoContent, "page returned empty body"},
		{"empty message", ErrCauseNoContent, ""},
		{"not HTML", ErrCauseNotHTML, "content-type is application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewExtractionError(tt.cause, tt.message)

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
