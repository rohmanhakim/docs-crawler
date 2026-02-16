package fetcher

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

// TestFetchError_Classifications tests that all FetchErrorCause values
// have the correct RetryPolicy and CrawlImpact classification.
// This ensures the two-dimensional error classification is correctly applied.
func TestFetchError_Classifications(t *testing.T) {
	tests := []struct {
		name         string
		cause        FetchErrorCause
		wantPolicy   failure.RetryPolicy
		wantImpact   failure.CrawlImpact
		wantSeverity failure.Severity
	}{
		// ErrCauseTimeout - transient network issue, should auto-retry
		{
			name:         "ErrCauseTimeout should be RetryPolicyAuto",
			cause:        ErrCauseTimeout,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		// ErrCauseNetworkFailure - transient network issue, should auto-retry
		{
			name:         "ErrCauseNetworkFailure should be RetryPolicyAuto",
			cause:        ErrCauseNetworkFailure,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		// ErrCauseReadResponseBodyError - transient read issue, should auto-retry
		{
			name:         "ErrCauseReadResponseBodyError should be RetryPolicyAuto",
			cause:        ErrCauseReadResponseBodyError,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		// ErrCauseContentTypeInvalid - non-recoverable content issue, manual retry eligible
		{
			name:         "ErrCauseContentTypeInvalid should be RetryPolicyManual",
			cause:        ErrCauseContentTypeInvalid,
			wantPolicy:   failure.RetryPolicyManual,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRetryExhausted,
		},
		// ErrCauseRedirectLimitExceeded - permanent config issue, never retry
		{
			name:         "ErrCauseRedirectLimitExceeded should be RetryPolicyNever",
			cause:        ErrCauseRedirectLimitExceeded,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		// ErrCauseRequestPageForbidden - auth issue, manual retry eligible after user fixes auth
		{
			name:         "ErrCauseRequestPageForbidden should be RetryPolicyManual",
			cause:        ErrCauseRequestPageForbidden,
			wantPolicy:   failure.RetryPolicyManual,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRetryExhausted,
		},
		// ErrCauseRequestTooMany - rate limit, should auto-retry with backoff
		{
			name:         "ErrCauseRequestTooMany should be RetryPolicyAuto",
			cause:        ErrCauseRequestTooMany,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		// ErrCauseRequest5xx - server error, should auto-retry
		{
			name:         "ErrCauseRequest5xx should be RetryPolicyAuto",
			cause:        ErrCauseRequest5xx,
			wantPolicy:   failure.RetryPolicyAuto,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
		// ErrCauseRepeated403 - repeated auth failures, never retry
		{
			name:         "ErrCauseRepeated403 should be RetryPolicyNever",
			cause:        ErrCauseRepeated403,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewFetchError(tt.cause, "test message")

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

// TestFetchError_AllCausesCovered verifies that all FetchErrorCause constants
// are covered by the classification map. This is a safety check to ensure
// no causes are accidentally omitted.
func TestFetchError_AllCausesCovered(t *testing.T) {
	// List of all known causes
	allCauses := []FetchErrorCause{
		ErrCauseTimeout,
		ErrCauseNetworkFailure,
		ErrCauseReadResponseBodyError,
		ErrCauseContentTypeInvalid,
		ErrCauseRedirectLimitExceeded,
		ErrCauseRequestPageForbidden,
		ErrCauseRequestTooMany,
		ErrCauseRequest5xx,
		ErrCauseRepeated403,
	}

	for _, cause := range allCauses {
		t.Run(string(cause), func(t *testing.T) {
			if _, ok := fetchErrorClassifications[cause]; !ok {
				t.Errorf("cause %q not found in fetchErrorClassifications map", cause)
			}
		})
	}
}

// TestFetchError_NewFetchErrorVariations tests that NewFetchError correctly
// handles the error message parameter.
func TestFetchError_NewFetchErrorVariations(t *testing.T) {
	tests := []struct {
		name    string
		cause   FetchErrorCause
		message string
	}{
		{"custom message", ErrCauseNetworkFailure, "connection refused"},
		{"empty message", ErrCauseNetworkFailure, ""},
		{"long message", ErrCauseRequest5xx, "server returned 500 with body: internal server error occurred while processing request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewFetchError(tt.cause, tt.message)

			if err.Cause != tt.cause {
				t.Errorf("Cause = %v, want %v", err.Cause, tt.cause)
			}

			// Error() should return the cause string
			if err.Error() == "" {
				t.Error("Error() should not be empty")
			}
		})
	}
}
