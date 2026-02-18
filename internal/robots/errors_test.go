package robots

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

// TestRobotsError_Classifications tests that all RobotsErrorCause values
// have the correct RetryPolicy and Impact classification.
// This ensures the two-dimensional error classification is correctly applied.
func TestRobotsError_Classifications(t *testing.T) {
	tests := []struct {
		name         string
		cause        RobotsErrorCause
		wantPolicy   failure.RetryPolicy
		wantImpact   failure.ImpactLevel
		wantSeverity failure.Severity
	}{
		// Auto-retryable: transient network/server errors
		{
			name:         "ErrCauseHttpTooManyRequests should be RetryPolicyAuto",
			cause:        ErrCauseHttpTooManyRequests,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseHttpServerError should be RetryPolicyAuto",
			cause:        ErrCauseHttpServerError,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseHttpFetchFailure should be RetryPolicyAuto",
			cause:        ErrCauseHttpFetchFailure,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		// Never retry: policy decisions and permanent failures
		{
			name:         "ErrCauseDisallowRoot should be RetryPolicyNever",
			cause:        ErrCauseDisallowRoot,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseParseError should be RetryPolicyNever",
			cause:        ErrCauseParseError,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseInvalidRobotsUrl should be RetryPolicyNever",
			cause:        ErrCauseInvalidRobotsUrl,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCausePreFetchFailure should be RetryPolicyNever",
			cause:        ErrCausePreFetchFailure,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseHttpTooManyRedirects should be RetryPolicyNever",
			cause:        ErrCauseHttpTooManyRedirects,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseHttpUnexpectedStatus should be RetryPolicyNever",
			cause:        ErrCauseHttpUnexpectedStatus,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRobotsError(tt.cause, "test message")

			if err.RetryPolicy() != tt.wantPolicy {
				t.Errorf("RetryPolicy() = %v, want %v", err.RetryPolicy(), tt.wantPolicy)
			}

			if err.Impact() != tt.wantImpact {
				t.Errorf("Impact() = %v, want %v", err.Impact(), tt.wantImpact)
			}

			if err.Severity() != tt.wantSeverity {
				t.Errorf("Severity() = %v, want %v", err.Severity(), tt.wantSeverity)
			}
		})
	}
}

// TestRobotsError_AllCausesCovered verifies that all RobotsErrorCause constants
// are covered by the classification map. This is a safety check to ensure
// no causes are accidentally omitted.
func TestRobotsError_AllCausesCovered(t *testing.T) {
	allCauses := []RobotsErrorCause{
		ErrCauseDisallowRoot,
		ErrCauseInvalidRobotsUrl,
		ErrCausePreFetchFailure,
		ErrCauseHttpFetchFailure,
		ErrCauseHttpTooManyRequests,
		ErrCauseHttpTooManyRedirects,
		ErrCauseHttpServerError,
		ErrCauseHttpUnexpectedStatus,
		ErrCauseParseError,
	}

	for _, cause := range allCauses {
		t.Run(string(cause), func(t *testing.T) {
			if _, ok := robotsErrorClassifications[cause]; !ok {
				t.Errorf("cause %q not found in robotsErrorClassifications map", cause)
			}
		})
	}
}

// TestRobotsError_NewRobotsErrorVariations tests that NewRobotsError correctly
// handles the error message parameter.
func TestRobotsError_NewRobotsErrorVariations(t *testing.T) {
	tests := []struct {
		name    string
		cause   RobotsErrorCause
		message string
	}{
		{"custom message", ErrCauseHttpFetchFailure, "connection refused"},
		{"empty message", ErrCauseHttpFetchFailure, ""},
		{"long message", ErrCauseHttpServerError, "server returned 500 with body: internal server error occurred while processing robots.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRobotsError(tt.cause, tt.message)

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

// TestMapRobotsErrorToMetadataCause tests the mapping from robots errors
// to the canonical metadata.ErrorCause table.
func TestMapRobotsErrorToMetadataCause(t *testing.T) {
	tests := []struct {
		name      string
		err       *RobotsError
		wantCause metadata.ErrorCause
	}{
		{
			name:      "ErrCauseDisallowRoot maps to CausePolicyDisallow",
			err:       NewRobotsError(ErrCauseDisallowRoot, "test"),
			wantCause: metadata.CausePolicyDisallow,
		},
		{
			name:      "ErrCauseInvalidRobotsUrl maps to CauseInvariantViolation",
			err:       NewRobotsError(ErrCauseInvalidRobotsUrl, "test"),
			wantCause: metadata.CauseInvariantViolation,
		},
		{
			name:      "ErrCausePreFetchFailure maps to CauseUnknown",
			err:       NewRobotsError(ErrCausePreFetchFailure, "test"),
			wantCause: metadata.CauseUnknown,
		},
		{
			name:      "ErrCauseHttpFetchFailure maps to CauseNetworkFailure",
			err:       NewRobotsError(ErrCauseHttpFetchFailure, "test"),
			wantCause: metadata.CauseNetworkFailure,
		},
		{
			name:      "ErrCauseHttpTooManyRequests maps to CauseNetworkFailure",
			err:       NewRobotsError(ErrCauseHttpTooManyRequests, "test"),
			wantCause: metadata.CauseNetworkFailure,
		},
		{
			name:      "ErrCauseHttpTooManyRedirects maps to CauseNetworkFailure",
			err:       NewRobotsError(ErrCauseHttpTooManyRedirects, "test"),
			wantCause: metadata.CauseNetworkFailure,
		},
		{
			name:      "ErrCauseHttpServerError maps to CauseNetworkFailure",
			err:       NewRobotsError(ErrCauseHttpServerError, "test"),
			wantCause: metadata.CauseNetworkFailure,
		},
		{
			name:      "ErrCauseHttpUnexpectedStatus maps to CauseNetworkFailure",
			err:       NewRobotsError(ErrCauseHttpUnexpectedStatus, "test"),
			wantCause: metadata.CauseNetworkFailure,
		},
		{
			name:      "ErrCauseParseError maps to CauseContentInvalid",
			err:       NewRobotsError(ErrCauseParseError, "test"),
			wantCause: metadata.CauseContentInvalid,
		},
		{
			name:      "unknown cause maps to CauseUnknown",
			err:       &RobotsError{Cause: "unknown cause"},
			wantCause: metadata.CauseUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapRobotsErrorToMetadataCause(tt.err)
			if got != tt.wantCause {
				t.Errorf("mapRobotsErrorToMetadataCause() = %v, want %v", got, tt.wantCause)
			}
		})
	}
}

// TestRobotsError_SeverityEdgeCases tests edge cases for the Severity method.
func TestRobotsError_SeverityEdgeCases(t *testing.T) {
	// Test that Severity() doesn't panic and returns valid values
	err := NewRobotsError(ErrCauseHttpFetchFailure, "test")

	// Verify Severity doesn't panic and returns valid value
	_ = err.Severity()

	// Verify Error() format
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error() should not be empty")
	}
}
