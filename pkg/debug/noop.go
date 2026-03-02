package debug

import (
	"github.com/rohmanhakim/dlog"
)

// NewNoOpLogger creates a no-operation logger that discards all log output.
// It wraps dlog's NoOpLogger with DomainLogger for domain-specific methods.
func NewNoOpLogger() DebugLogger {
	return NewDomainLogger(dlog.NewNoOpLogger())
}
