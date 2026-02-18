package scheduler_test

import "github.com/rohmanhakim/docs-crawler/pkg/failure"

// mockClassifiedError is a mock implementation of failure.ClassifiedError for testing
type mockClassifiedError struct {
	msg         string
	severity    failure.Severity
	retryPolicy failure.RetryPolicy
	impactLevel failure.ImpactLevel
}

func (e *mockClassifiedError) Error() string {
	return e.msg
}

func (e *mockClassifiedError) Severity() failure.Severity {
	return e.severity
}

func (e *mockClassifiedError) RetryPolicy() failure.RetryPolicy {
	return e.retryPolicy
}

func (e *mockClassifiedError) Impact() failure.ImpactLevel {
	return e.impactLevel
}
