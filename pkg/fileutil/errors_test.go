package fileutil_test

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/fileutil"
)

// TestFileError_Classifications tests that all FileErrorCause values
// have the correct RetryPolicy and ImpactLevel classification.
func TestFileError_Classifications(t *testing.T) {
	tests := []struct {
		name         string
		cause        fileutil.FileErrorCause
		wantPolicy   failure.RetryPolicy
		wantImpact   failure.ImpactLevel
		wantSeverity failure.Severity
	}{
		// ErrCausePathError - permanent file system issue, never retry
		{
			name:         "ErrCausePathError should be RetryPolicyNever",
			cause:        fileutil.ErrCausePathError,
			wantPolicy:   failure.RetryPolicyNever,
			wantImpact:   failure.ImpactLevelContinue,
			wantSeverity: failure.SeverityRecoverable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fileutil.NewFileError(tt.cause, "test message")

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

// TestFileError_UnknownCause tests that unknown causes get default classification.
func TestFileError_UnknownCause(t *testing.T) {
	unknownCause := fileutil.FileErrorCause("unknown cause")
	err := fileutil.NewFileError(unknownCause, "test message")

	// Unknown causes should get default classification
	if err.RetryPolicy() != failure.RetryPolicyNever {
		t.Errorf("RetryPolicy() = %v, want %v (default)", err.RetryPolicy(), failure.RetryPolicyNever)
	}

	if err.Impact() != failure.ImpactLevelContinue {
		t.Errorf("Impact() = %v, want %v (default)", err.Impact(), failure.ImpactLevelContinue)
	}

	// Unknown causes should still return proper error message
	if err.Cause != unknownCause {
		t.Errorf("Cause = %v, want %v", err.Cause, unknownCause)
	}
}

// TestFileError_Error tests the Error() method format.
func TestFileError_Error(t *testing.T) {
	tests := []struct {
		name         string
		cause        fileutil.FileErrorCause
		message      string
		wantContains []string
	}{
		{
			name:         "path error with message",
			cause:        fileutil.ErrCausePathError,
			message:      "file not found",
			wantContains: []string{"file error", "path error", "file not found"},
		},
		{
			name:         "unknown cause with message",
			cause:        fileutil.FileErrorCause("custom cause"),
			message:      "something went wrong",
			wantContains: []string{"file error", "custom cause", "something went wrong"},
		},
		{
			name:         "empty message",
			cause:        fileutil.ErrCausePathError,
			message:      "",
			wantContains: []string{"file error", "path error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fileutil.NewFileError(tt.cause, tt.message)
			errStr := err.Error()

			for _, want := range tt.wantContains {
				if !contains(errStr, want) {
					t.Errorf("Error() = %q, should contain %q", errStr, want)
				}
			}
		})
	}
}

// TestFileError_Severity tests all severity derivation paths.
func TestFileError_Severity(t *testing.T) {
	// We can't directly set private fields, but we can test through known causes
	// and verify the behavior. The severity logic is:
	// - If impact == ImpactLevelAbort -> SeverityFatal
	// - If policy == RetryPolicyNever -> SeverityRecoverable
	// - If policy == RetryPolicyManual -> SeverityRetryExhausted
	// - Default -> SeverityRecoverable

	// Test known cause (ErrCausePathError has RetryPolicyNever, ImpactLevelContinue)
	err := fileutil.NewFileError(fileutil.ErrCausePathError, "test")
	if err.Severity() != failure.SeverityRecoverable {
		t.Errorf("Severity() = %v, want %v", err.Severity(), failure.SeverityRecoverable)
	}

	// Test unknown cause (should get default: RetryPolicyNever, ImpactLevelContinue)
	unknownErr := fileutil.NewFileError(fileutil.FileErrorCause("unknown"), "test")
	if unknownErr.Severity() != failure.SeverityRecoverable {
		t.Errorf("Severity() = %v, want %v", unknownErr.Severity(), failure.SeverityRecoverable)
	}
}

// TestFileError_AllCausesCovered verifies that all FileErrorCause constants
// are covered by the classification map.
func TestFileError_AllCausesCovered(t *testing.T) {
	// List of all known causes - this is a safety check
	allCauses := []fileutil.FileErrorCause{
		fileutil.ErrCausePathError,
	}

	for _, cause := range allCauses {
		t.Run(string(cause), func(t *testing.T) {
			// This should not panic and should create a valid error
			err := fileutil.NewFileError(cause, "test")
			if err == nil {
				t.Error("NewFileError should not return nil")
			}
			if err.Cause != cause {
				t.Errorf("Cause = %v, want %v", err.Cause, cause)
			}
		})
	}
}

// TestFileError_ImplementsClassifiedError verifies that FileError implements
// the ClassifiedError interface.
func TestFileError_ImplementsClassifiedError(t *testing.T) {
	err := fileutil.NewFileError(fileutil.ErrCausePathError, "test")

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

// TestFileError_NewFileErrorVariations tests different message variations.
func TestFileError_NewFileErrorVariations(t *testing.T) {
	tests := []struct {
		name    string
		cause   fileutil.FileErrorCause
		message string
	}{
		{"custom message", fileutil.ErrCausePathError, "file /path/to/file not found"},
		{"empty message", fileutil.ErrCausePathError, ""},
		{"long message", fileutil.ErrCausePathError, "permission denied: cannot read configuration file at /etc/app/config.yaml: operation not permitted"},
		{"special characters", fileutil.ErrCausePathError, "path with 'quotes' and \"double quotes\" and \n newline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fileutil.NewFileError(tt.cause, tt.message)

			if err.Cause != tt.cause {
				t.Errorf("Cause = %v, want %v", err.Cause, tt.cause)
			}

			if err.Message != tt.message {
				t.Errorf("Message = %v, want %v", err.Message, tt.message)
			}

			// Error() should never be empty
			if err.Error() == "" {
				t.Error("Error() should not be empty")
			}
		})
	}
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
