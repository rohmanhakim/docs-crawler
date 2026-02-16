package failure

import (
	"testing"
)

// TestRetryPolicyValues verifies that RetryPolicy constants have expected values
func TestRetryPolicyValues(t *testing.T) {
	tests := []struct {
		name     string
		policy   RetryPolicy
		wantZero bool
	}{
		{"RetryPolicyAuto is 0", RetryPolicyAuto, true},
		{"RetryPolicyManual is 1", RetryPolicyManual, false},
		{"RetryPolicyNever is 2", RetryPolicyNever, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantZero && tt.policy != 0 {
				t.Errorf("expected %s to be 0, got %d", tt.name, tt.policy)
			}
			if !tt.wantZero && tt.policy == 0 {
				t.Errorf("expected %s to not be 0", tt.name)
			}
		})
	}
}

// TestCrawlImpactValues verifies that CrawlImpact constants have expected values
func TestCrawlImpactValues(t *testing.T) {
	tests := []struct {
		name     string
		impact   CrawlImpact
		wantZero bool
	}{
		{"ImpactContinue is 0", ImpactContinue, true},
		{"ImpactAbort is 1", ImpactAbort, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantZero && tt.impact != 0 {
				t.Errorf("expected %s to be 0, got %d", tt.name, tt.impact)
			}
			if !tt.wantZero && tt.impact == 0 {
				t.Errorf("expected %s to not be 0", tt.name)
			}
		})
	}
}

// TestSeverityValues verifies Severity string constants
func TestSeverityValues(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     Severity
	}{
		{"SeverityOK", SeverityOK, "ok"},
		{"SeverityRecoverable", SeverityRecoverable, "recoverable"},
		{"SeverityFatal", SeverityFatal, "fatal"},
		{"SeverityRetryExhausted", SeverityRetryExhausted, "retry_exhausted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.severity != tt.want {
				t.Errorf("expected %s, got %s", tt.want, tt.severity)
			}
		})
	}
}

// mockError implements ClassifiedError for testing
type mockError struct {
	err         string
	retryPolicy RetryPolicy
	crawlImpact CrawlImpact
	severity    Severity
}

func (e *mockError) Error() string {
	return e.err
}

func (e *mockError) RetryPolicy() RetryPolicy {
	return e.retryPolicy
}

func (e *mockError) CrawlImpact() CrawlImpact {
	return e.crawlImpact
}

func (e *mockError) Severity() Severity {
	return e.severity
}

// TestClassifiedErrorInterface verifies that mockError implements ClassifiedError
func TestClassifiedErrorInterface(t *testing.T) {
	var _ ClassifiedError = &mockError{}
}

// TestClassifiedErrorImplementations tests various implementations of ClassifiedError
func TestClassifiedErrorImplementations(t *testing.T) {
	tests := []struct {
		name       string
		err        ClassifiedError
		wantPolicy RetryPolicy
		wantImpact CrawlImpact
	}{
		{
			name:       "auto-retry error",
			err:        &mockError{err: "auto-retry", retryPolicy: RetryPolicyAuto, crawlImpact: ImpactContinue},
			wantPolicy: RetryPolicyAuto,
			wantImpact: ImpactContinue,
		},
		{
			name:       "manual retry error",
			err:        &mockError{err: "manual-retry", retryPolicy: RetryPolicyManual, crawlImpact: ImpactContinue},
			wantPolicy: RetryPolicyManual,
			wantImpact: ImpactContinue,
		},
		{
			name:       "never retry error",
			err:        &mockError{err: "never-retry", retryPolicy: RetryPolicyNever, crawlImpact: ImpactAbort},
			wantPolicy: RetryPolicyNever,
			wantImpact: ImpactAbort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.RetryPolicy(); got != tt.wantPolicy {
				t.Errorf("RetryPolicy() = %v, want %v", got, tt.wantPolicy)
			}
			if got := tt.err.CrawlImpact(); got != tt.wantImpact {
				t.Errorf("CrawlImpact() = %v, want %v", got, tt.wantImpact)
			}
		})
	}
}
