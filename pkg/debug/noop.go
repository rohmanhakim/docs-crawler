package debug

import (
	"context"
	"time"
)

// NoOpLogger is a no-operation implementation of DebugLogger.
// It provides zero overhead when debug mode is disabled.
// All methods are empty and Enabled() always returns false.
type NoOpLogger struct{}

// NewNoOpLogger creates a new NoOpLogger instance.
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

// Enabled returns false - debug logging is disabled.
func (n *NoOpLogger) Enabled() bool { return false }

// LogStage is a no-op.
func (n *NoOpLogger) LogStage(_ context.Context, _ string, _ StageEvent) {}

// LogRetry is a no-op.
func (n *NoOpLogger) LogRetry(_ context.Context, _ int, _ int, _ time.Duration, _ error) {
}

// LogRateLimit is a no-op.
func (n *NoOpLogger) LogRateLimit(_ context.Context, _ string, _ time.Duration, _ RateLimitReason) {
}

// LogStep is a no-op.
func (n *NoOpLogger) LogStep(_ context.Context, _ string, _ string, _ FieldMap) {}

// LogError is a no-op.
func (n *NoOpLogger) LogError(_ context.Context, _ string, _ error, _ FieldMap) {}

// WithFields returns the same NoOpLogger instance.
func (n *NoOpLogger) WithFields(_ FieldMap) DebugLogger { return n }

// Close returns nil - no resources to release.
func (n *NoOpLogger) Close() error { return nil }
