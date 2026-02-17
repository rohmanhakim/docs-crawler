package storage

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

// TestStorageError_Classifications tests that all StorageErrorCause values
// have the correct RetryPolicy and CrawlImpact classification.
// This ensures the two-dimensional error classification is correctly applied.
func TestStorageError_Classifications(t *testing.T) {
	tests := []struct {
		name         string
		cause        StorageErrorCause
		wantPolicy   failure.RetryPolicy
		wantImpact   failure.ImpactLevel
		wantSeverity failure.Severity
	}{
		// Manual retry: user-fixable errors
		{
			name:         "ErrCauseDiskFull should be RetryPolicyManual",
			cause:        ErrCauseDiskFull,
			wantPolicy:   failure.RetryPolicyManual,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRetryExhausted,
		},
		// Never retry: permanent failures
		{
			name:         "ErrCauseWriteFailure should be RetryPolicyNever",
			cause:        ErrCauseWriteFailure,
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
			name:         "ErrCausePathError should be RetryPolicyNever",
			cause:        ErrCausePathError,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewStorageError(tt.cause, "test message", "/test/path")

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

// TestStorageError_AllCausesCovered verifies that all StorageErrorCause constants
// are covered by the classification map. This is a safety check to ensure
// no causes are accidentally omitted.
func TestStorageError_AllCausesCovered(t *testing.T) {
	allCauses := []StorageErrorCause{
		ErrCauseDiskFull,
		ErrCauseWriteFailure,
		ErrCauseHashComputationFailed,
		ErrCausePathError,
	}

	for _, cause := range allCauses {
		t.Run(string(cause), func(t *testing.T) {
			if _, ok := storageErrorClassifications[cause]; !ok {
				t.Errorf("cause %q not found in storageErrorClassifications map", cause)
			}
		})
	}
}

// TestStorageError_NewStorageErrorVariations tests that NewStorageError correctly
// handles the error message and path parameters.
func TestStorageError_NewStorageErrorVariations(t *testing.T) {
	tests := []struct {
		name    string
		cause   StorageErrorCause
		message string
		path    string
	}{
		{"custom message and path", ErrCauseWriteFailure, "permission denied", "/var/data/file.md"},
		{"empty message", ErrCauseWriteFailure, "", "/tmp/test.md"},
		{"empty path", ErrCausePathError, "invalid path", ""},
		{"disk full with path", ErrCauseDiskFull, "no space left on device", "/data/output/docs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewStorageError(tt.cause, tt.message, tt.path)

			if err.Cause != tt.cause {
				t.Errorf("Cause = %v, want %v", err.Cause, tt.cause)
			}

			if err.Message != tt.message {
				t.Errorf("Message = %v, want %v", err.Message, tt.message)
			}

			if err.Path != tt.path {
				t.Errorf("Path = %v, want %v", err.Path, tt.path)
			}

			// Error() should not be empty
			if err.Error() == "" {
				t.Error("Error() should not be empty")
			}
		})
	}
}
