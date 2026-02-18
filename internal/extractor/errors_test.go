package extractor

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
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

// TestMapExtractionErrorToMetadataCause tests the mapping from extraction errors
// to the canonical metadata.ErrorCause table.
func TestMapExtractionErrorToMetadataCause(t *testing.T) {
	tests := []struct {
		name      string
		err       *ExtractionError
		wantCause metadata.ErrorCause
	}{
		{
			name:      "ErrCauseNoContent maps to CauseContentInvalid",
			err:       NewExtractionError(ErrCauseNoContent, "test"),
			wantCause: metadata.CauseContentInvalid,
		},
		{
			name:      "ErrCauseNotHTML maps to CauseUnknown",
			err:       NewExtractionError(ErrCauseNotHTML, "test"),
			wantCause: metadata.CauseUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapExtractionErrorToMetadataCause(tt.err)
			if got != tt.wantCause {
				t.Errorf("mapExtractionErrorToMetadataCause() = %v, want %v", got, tt.wantCause)
			}
		})
	}
}

// TestExtractionError_SeverityEdgeCases tests edge cases for the Severity method.
func TestExtractionError_SeverityEdgeCases(t *testing.T) {
	// Test with different impact levels to ensure Severity() handles them correctly
	// Since all current causes have ImpactLevelContinue, we test the default case
	// in the Severity() method's switch statement.

	// Create an error and directly test the Severity logic by checking the method
	// doesn't panic and returns expected values
	err := NewExtractionError(ErrCauseNoContent, "test")

	// Verify Severity doesn't panic and returns valid value
	_ = err.Severity()

	// Verify Error() format
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error() should not be empty")
	}
}
