package scheduler_test

import "github.com/rohmanhakim/docs-crawler/pkg/failure"

// mockClassifiedError is a mock implementation of failure.ClassifiedError for testing
type mockClassifiedError struct {
	msg      string
	severity failure.Severity
}

func (e *mockClassifiedError) Error() string {
	return e.msg
}

func (e *mockClassifiedError) Severity() failure.Severity {
	return e.severity
}
