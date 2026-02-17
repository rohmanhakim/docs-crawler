package sanitizer

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

// TestSanitizationError_Classifications tests that all SanitizationErrorCause values
// have the correct RetryPolicy and CrawlImpact classification.
// This ensures the two-dimensional error classification is correctly applied.
func TestSanitizationError_Classifications(t *testing.T) {
	tests := []struct {
		name         string
		cause        SanitizationErrorCause
		wantPolicy   failure.RetryPolicy
		wantImpact   failure.CrawlImpact
		wantSeverity failure.Severity
	}{
		// Never retry: content processing errors are deterministic
		{
			name:         "ErrCauseUnparseableHTML should be RetryPolicyNever",
			cause:        ErrCauseUnparseableHTML,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseCompetingRoots should be RetryPolicyNever",
			cause:        ErrCauseCompetingRoots,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseNoStructuralAnchor should be RetryPolicyNever",
			cause:        ErrCauseNoStructuralAnchor,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseMultipleH1NoRoot should be RetryPolicyNever",
			cause:        ErrCauseMultipleH1NoRoot,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseImpliedMultipleDocs should be RetryPolicyNever",
			cause:        ErrCauseImpliedMultipleDocs,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseAmbiguousDOM should be RetryPolicyNever",
			cause:        ErrCauseAmbiguousDOM,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewSanitizationError(tt.cause, "test message")

			if err.RetryPolicy() != tt.wantPolicy {
				t.Errorf("RetryPolicy() = %v, want %v", err.RetryPolicy(), tt.wantPolicy)
			}

			if err.CrawlImpact() != tt.wantImpact {
				t.Errorf("CrawlImpact() = %v, want %v", err.CrawlImpact(), tt.wantImpact)
			}

			if err.Severity() != tt.wantSeverity {
				t.Errorf("Severity() = %v, want %v", err.Severity(), tt.wantSeverity)
			}
		})
	}
}

// TestSanitizationError_AllCausesCovered verifies that all SanitizationErrorCause constants
// are covered by the classification map. This is a safety check to ensure
// no causes are accidentally omitted.
func TestSanitizationError_AllCausesCovered(t *testing.T) {
	allCauses := []SanitizationErrorCause{
		ErrCauseUnparseableHTML,
		ErrCauseCompetingRoots,
		ErrCauseNoStructuralAnchor,
		ErrCauseMultipleH1NoRoot,
		ErrCauseImpliedMultipleDocs,
		ErrCauseAmbiguousDOM,
	}

	for _, cause := range allCauses {
		t.Run(string(cause), func(t *testing.T) {
			if _, ok := sanitizationErrorClassifications[cause]; !ok {
				t.Errorf("cause %q not found in sanitizationErrorClassifications map", cause)
			}
		})
	}
}

// TestSanitizationError_NewSanitizationErrorVariations tests that NewSanitizationError correctly
// handles the error message parameter.
func TestSanitizationError_NewSanitizationErrorVariations(t *testing.T) {
	tests := []struct {
		name    string
		cause   SanitizationErrorCause
		message string
	}{
		{"custom message", ErrCauseUnparseableHTML, "HTML parser failed with error"},
		{"empty message", ErrCauseCompetingRoots, ""},
		{"no structural anchor", ErrCauseNoStructuralAnchor, "document has no headings"},
		{"multiple H1", ErrCauseMultipleH1NoRoot, "found 3 H1 elements"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewSanitizationError(tt.cause, tt.message)

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
