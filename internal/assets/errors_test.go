package assets

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

// TestAssetsError_Classifications tests that all AssetsErrorCause values
// have the correct RetryPolicy and CrawlImpact classification.
// This ensures the two-dimensional error classification is correctly applied.
func TestAssetsError_Classifications(t *testing.T) {
	tests := []struct {
		name         string
		cause        AssetsErrorCause
		wantPolicy   failure.RetryPolicy
		wantImpact   failure.ImpactLevel
		wantSeverity failure.Severity
	}{
		// Auto-retryable: transient network/server errors
		{
			name:         "ErrCauseTimeout should be RetryPolicyAuto",
			cause:        ErrCauseTimeout,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseNetworkFailure should be RetryPolicyAuto",
			cause:        ErrCauseNetworkFailure,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseReadResponseBodyError should be RetryPolicyAuto",
			cause:        ErrCauseReadResponseBodyError,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseRequest5xx should be RetryPolicyAuto",
			cause:        ErrCauseRequest5xx,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseRequestTooMany should be RetryPolicyAuto",
			cause:        ErrCauseRequestTooMany,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		// Manual retry: user-fixable errors
		{
			name:         "ErrCauseRequestPageForbidden should be RetryPolicyManual",
			cause:        ErrCauseRequestPageForbidden,
			wantPolicy:   failure.RetryPolicyManual,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRetryExhausted,
		},
		{
			name:         "ErrCauseDiskFull should be RetryPolicyManual",
			cause:        ErrCauseDiskFull,
			wantPolicy:   failure.RetryPolicyManual,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRetryExhausted,
		},
		// Never retry: permanent failures
		{
			name:         "ErrCauseRepeated403 should be RetryPolicyNever",
			cause:        ErrCauseRepeated403,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseAssetTooLarge should be RetryPolicyNever",
			cause:        ErrCauseAssetTooLarge,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseRedirectLimitExceeded should be RetryPolicyNever",
			cause:        ErrCauseRedirectLimitExceeded,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseContentTypeInvalid should be RetryPolicyNever",
			cause:        ErrCauseContentTypeInvalid,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseWriteFailure should be RetryPolicyNever",
			cause:        ErrCauseWriteFailure,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCausePathError should be RetryPolicyNever",
			cause:        ErrCausePathError,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		{
			name:         "ErrCauseHashError should be RetryPolicyNever",
			cause:        ErrCauseHashError,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAssetsError(tt.cause, "test message")

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

// TestAssetsError_AllCausesCovered verifies that all AssetsErrorCause constants
// are covered by the classification map. This is a safety check to ensure
// no causes are accidentally omitted.
func TestAssetsError_AllCausesCovered(t *testing.T) {
	allCauses := []AssetsErrorCause{
		ErrCausePathError,
		ErrCauseDiskFull,
		ErrCauseWriteFailure,
		ErrCauseTimeout,
		ErrCauseRequestTooMany,
		ErrCauseNetworkFailure,
		ErrCauseRepeated403,
		ErrCauseReadResponseBodyError,
		ErrCauseContentTypeInvalid,
		ErrCauseRedirectLimitExceeded,
		ErrCauseRequestPageForbidden,
		ErrCauseRequest5xx,
		ErrCauseAssetTooLarge,
		ErrCauseHashError,
	}

	for _, cause := range allCauses {
		t.Run(string(cause), func(t *testing.T) {
			if _, ok := assetsErrorClassifications[cause]; !ok {
				t.Errorf("cause %q not found in assetsErrorClassifications map", cause)
			}
		})
	}
}

// TestAssetsError_NewAssetsErrorVariations tests that NewAssetsError correctly
// handles the error message parameter.
func TestAssetsError_NewAssetsErrorVariations(t *testing.T) {
	tests := []struct {
		name    string
		cause   AssetsErrorCause
		message string
	}{
		{"custom message", ErrCauseNetworkFailure, "connection refused"},
		{"empty message", ErrCauseNetworkFailure, ""},
		{"long message", ErrCauseRequest5xx, "server returned 500 with body: internal server error occurred while processing request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAssetsError(tt.cause, tt.message)

			if err.Cause != tt.cause {
				t.Errorf("Cause = %v, want %v", err.Cause, tt.cause)
			}

			if err.Message != tt.message {
				t.Errorf("Message = %v, want %v", err.Message, tt.message)
			}

			// Error() should not be empty
			if err.Error() == "" {
				t.Error("Error() should not be empty")
			}
		})
	}
}

// TestMapAssetsErrorToMetadataCause tests the mapping from assets errors
// to the canonical metadata.ErrorCause table.
func TestMapAssetsErrorToMetadataCause(t *testing.T) {
	tests := []struct {
		name      string
		err       *AssetsError
		wantCause metadata.ErrorCause
	}{
		{
			name:      "ErrCausePathError maps to CauseStorageFailure",
			err:       NewAssetsError(ErrCausePathError, "test"),
			wantCause: metadata.CauseStorageFailure,
		},
		{
			name:      "ErrCauseDiskFull maps to CauseStorageFailure",
			err:       NewAssetsError(ErrCauseDiskFull, "test"),
			wantCause: metadata.CauseStorageFailure,
		},
		{
			name:      "ErrCauseWriteFailure maps to CauseStorageFailure",
			err:       NewAssetsError(ErrCauseWriteFailure, "test"),
			wantCause: metadata.CauseStorageFailure,
		},
		{
			name:      "ErrCauseTimeout maps to CauseNetworkFailure",
			err:       NewAssetsError(ErrCauseTimeout, "test"),
			wantCause: metadata.CauseNetworkFailure,
		},
		{
			name:      "ErrCauseRequestTooMany maps to CausePolicyDisallow",
			err:       NewAssetsError(ErrCauseRequestTooMany, "test"),
			wantCause: metadata.CausePolicyDisallow,
		},
		{
			name:      "ErrCauseRepeated403 maps to CausePolicyDisallow",
			err:       NewAssetsError(ErrCauseRepeated403, "test"),
			wantCause: metadata.CausePolicyDisallow,
		},
		{
			name:      "ErrCauseReadResponseBodyError maps to CauseContentInvalid",
			err:       NewAssetsError(ErrCauseReadResponseBodyError, "test"),
			wantCause: metadata.CauseContentInvalid,
		},
		{
			name:      "ErrCauseContentTypeInvalid maps to CauseContentInvalid",
			err:       NewAssetsError(ErrCauseContentTypeInvalid, "test"),
			wantCause: metadata.CauseContentInvalid,
		},
		{
			name:      "ErrCauseRedirectLimitExceeded maps to CauseUnknown",
			err:       NewAssetsError(ErrCauseRedirectLimitExceeded, "test"),
			wantCause: metadata.CauseUnknown,
		},
		{
			name:      "ErrCauseRequestPageForbidden maps to CausePolicyDisallow",
			err:       NewAssetsError(ErrCauseRequestPageForbidden, "test"),
			wantCause: metadata.CausePolicyDisallow,
		},
		{
			name:      "ErrCauseRequest5xx maps to CauseUnknown",
			err:       NewAssetsError(ErrCauseRequest5xx, "test"),
			wantCause: metadata.CauseUnknown,
		},
		{
			name:      "ErrCauseAssetTooLarge maps to CausePolicyDisallow",
			err:       NewAssetsError(ErrCauseAssetTooLarge, "test"),
			wantCause: metadata.CausePolicyDisallow,
		},
		{
			name:      "ErrCauseHashError maps to CauseContentInvalid",
			err:       NewAssetsError(ErrCauseHashError, "test"),
			wantCause: metadata.CauseContentInvalid,
		},
		{
			name:      "unknown cause maps to CauseUnknown",
			err:       &AssetsError{Cause: "unknown cause"},
			wantCause: metadata.CauseUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAssetsErrorToMetadataCause(*tt.err)
			if got != tt.wantCause {
				t.Errorf("mapAssetsErrorToMetadataCause() = %v, want %v", got, tt.wantCause)
			}
		})
	}
}

// TestAssetsError_SeverityEdgeCases tests edge cases for the Severity method.
func TestAssetsError_SeverityEdgeCases(t *testing.T) {
	// Test that Severity() doesn't panic and returns valid values
	err := NewAssetsError(ErrCauseNetworkFailure, "test")

	// Verify Severity doesn't panic and returns valid value
	_ = err.Severity()

	// Verify Error() format
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error() should not be empty")
	}
}
