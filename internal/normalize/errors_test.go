package normalize

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

// TestNormalizationError_Classifications tests that all NormalizationErrorCause values
// have the correct RetryPolicy and CrawlImpact classification.
// This ensures the two-dimensional error classification is correctly applied.
func TestNormalizationError_Classifications(t *testing.T) {
	tests := []struct {
		name         string
		cause        NormalizationErrorCause
		wantPolicy   failure.RetryPolicy
		wantImpact   failure.ImpactLevel
		wantSeverity failure.Severity
	}{
		// Never retry: content processing errors are deterministic
		{
			name:         "ErrCauseBrokenH1Invariant should be RetryPolicyNever",
			cause:        ErrCauseBrokenH1Invariant,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseEmptyContent should be RetryPolicyNever",
			cause:        ErrCauseEmptyContent,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseSectionDerivationFailed should be RetryPolicyNever",
			cause:        ErrCauseSectionDerivationFailed,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseTitleExtractionFailed should be RetryPolicyNever",
			cause:        ErrCauseTitleExtractionFailed,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseHashComputationFailed should be RetryPolicyNever",
			cause:        ErrCauseHashComputationFailed,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseFrontmatterMarshalFailed should be RetryPolicyNever",
			cause:        ErrCauseFrontmatterMarshalFailed,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseSkippedHeadingLevels should be RetryPolicyNever",
			cause:        ErrCauseSkippedHeadingLevels,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseOrphanContent should be RetryPolicyNever",
			cause:        ErrCauseOrphanContent,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseEmptySection should be RetryPolicyNever",
			cause:        ErrCauseEmptySection,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseBrokenAtomicBlock should be RetryPolicyNever",
			cause:        ErrCauseBrokenAtomicBlock,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewNormalizationError(tt.cause, "test message")

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

// TestNormalizationError_AllCausesCovered verifies that all NormalizationErrorCause constants
// are covered by the classification map. This is a safety check to ensure
// no causes are accidentally omitted.
func TestNormalizationError_AllCausesCovered(t *testing.T) {
	allCauses := []NormalizationErrorCause{
		ErrCauseBrokenH1Invariant,
		ErrCauseEmptyContent,
		ErrCauseSectionDerivationFailed,
		ErrCauseTitleExtractionFailed,
		ErrCauseHashComputationFailed,
		ErrCauseFrontmatterMarshalFailed,
		ErrCauseSkippedHeadingLevels,
		ErrCauseOrphanContent,
		ErrCauseEmptySection,
		ErrCauseBrokenAtomicBlock,
	}

	for _, cause := range allCauses {
		t.Run(string(cause), func(t *testing.T) {
			if _, ok := normalizationErrorClassifications[cause]; !ok {
				t.Errorf("cause %q not found in normalizationErrorClassifications map", cause)
			}
		})
	}
}

// TestNormalizationError_NewNormalizationErrorVariations tests that NewNormalizationError correctly
// handles the error message parameter.
func TestNormalizationError_NewNormalizationErrorVariations(t *testing.T) {
	tests := []struct {
		name    string
		cause   NormalizationErrorCause
		message string
	}{
		{"custom message", ErrCauseBrokenH1Invariant, "found 0 H1 elements"},
		{"empty message", ErrCauseEmptyContent, ""},
		{"section derivation", ErrCauseSectionDerivationFailed, "cannot derive section from URL"},
		{"title extraction", ErrCauseTitleExtractionFailed, "no H1 found in document"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewNormalizationError(tt.cause, tt.message)

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
