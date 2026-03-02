package debug

import (
	"context"
	"time"

	"github.com/rohmanhakim/dlog"
	"github.com/rohmanhakim/retrier"
)

// DomainLogger provides domain-specific logging methods for the crawler.
// It wraps dlog.DebugLogger and translates domain types to FieldMap.
type DomainLogger struct {
	logger dlog.DebugLogger
}

// NewDomainLogger creates a new DomainLogger wrapping a dlog.DebugLogger.
// If logger is nil, returns a logger wrapping dlog.NewNoOpLogger().
func NewDomainLogger(logger dlog.DebugLogger) *DomainLogger {
	if logger == nil {
		return &DomainLogger{logger: dlog.NewNoOpLogger()}
	}
	return &DomainLogger{logger: logger}
}

// Enabled returns true if debug logging is enabled.
func (d *DomainLogger) Enabled() bool { return d.logger.Enabled() }

// LogStage logs a pipeline stage event with structured fields.
func (d *DomainLogger) LogStage(ctx context.Context, stage string, event StageEvent) {
	fields := dlog.FieldMap{
		"stage":      stage,
		"event_type": string(event.Type),
	}

	if event.URL != "" {
		fields["url"] = event.URL
	}
	if event.Duration > 0 {
		fields["duration_ms"] = event.Duration.Milliseconds()
	}
	if event.InputSummary != "" {
		fields["input_summary"] = event.InputSummary
	}
	if event.OutputSummary != "" {
		fields["output_summary"] = event.OutputSummary
	}

	// Merge event fields
	if len(event.Fields) > 0 {
		for k, v := range event.Fields {
			fields[k] = v
		}
	}

	d.logger.LogDebug(ctx, stageMessage(event.Type), fields)
}

// LogRetry logs retry attempts with backoff information.
// The attrs parameter allows passing additional context like URL and stage.
func (d *DomainLogger) LogRetry(ctx context.Context, attempt int, maxAttempts int, backoff time.Duration, err error, attrs FieldMap) {
	fields := dlog.FieldMap{
		"attempt":      attempt,
		"max_attempts": maxAttempts,
		"backoff_ms":   backoff.Milliseconds(),
	}

	// Merge additional attributes
	if len(attrs) > 0 {
		for k, v := range attrs {
			fields[k] = v
		}
	}

	if err != nil {
		d.logger.LogDebug(ctx, "Retry attempt failed", fields)
	} else {
		d.logger.LogDebug(ctx, "Retry attempt succeeded", fields)
	}
}

// LogRateLimit logs rate limiting decisions.
func (d *DomainLogger) LogRateLimit(ctx context.Context, host string, delay time.Duration, reason RateLimitReason) {
	fields := dlog.FieldMap{
		"host":              host,
		"delay_ms":          delay.Milliseconds(),
		"rate_limit_reason": string(reason),
	}

	d.logger.LogDebug(ctx, "Rate limit applied", fields)
}

// LogStep logs a granular step within a pipeline stage.
func (d *DomainLogger) LogStep(ctx context.Context, stage string, step string, fields FieldMap) {
	dlogFields := dlog.FieldMap{
		"stage": stage,
		"step":  step,
	}

	// Merge step fields
	if len(fields) > 0 {
		for k, v := range fields {
			dlogFields[k] = v
		}
	}

	d.logger.LogDebug(ctx, "Step executed", dlogFields)
}

// LogError logs a debug-level error with context.
func (d *DomainLogger) LogError(ctx context.Context, stage string, err error, fields FieldMap) {
	dlogFields := dlog.FieldMap{
		"stage": stage,
	}

	// Merge error fields
	if len(fields) > 0 {
		for k, v := range fields {
			dlogFields[k] = v
		}
	}

	d.logger.LogError(ctx, "Error occurred", err, dlogFields)
}

// WithFields returns a logger with pre-populated fields.
func (d *DomainLogger) WithFields(fields FieldMap) DebugLogger {
	// Convert to dlog.FieldMap
	dlogFields := dlog.FieldMap(fields)
	return &DomainLogger{logger: d.logger.WithFields(dlogFields)}
}

// Close flushes any buffered output and closes file handles.
func (d *DomainLogger) Close() error { return d.logger.Close() }

// stageMessage returns a human-readable message for a stage event.
func stageMessage(eventType EventType) string {
	switch eventType {
	case EventTypeStart:
		return "Pipeline stage started"
	case EventTypeProgress:
		return "Pipeline stage progress"
	case EventTypeComplete:
		return "Pipeline stage completed"
	case EventTypeError:
		return "Pipeline stage error"
	default:
		return "Pipeline stage event"
	}
}

// Ensure DomainLogger implements DebugLogger at compile time.
var _ DebugLogger = (*DomainLogger)(nil)

// RetryLoggerAdapter wraps DebugLogger to implement retrier.DebugLogger.
// This allows DebugLogger types to be used with the external retry package.
type RetryLoggerAdapter struct {
	DebugLogger
}

// AsRetryLogger wraps a DebugLogger to implement retrier.DebugLogger.
// Returns a NoOpRetryLogger if the logger is nil.
func AsRetryLogger(logger DebugLogger) *RetryLoggerAdapter {
	if logger == nil {
		return &RetryLoggerAdapter{NewDomainLogger(nil)}
	}
	return &RetryLoggerAdapter{DebugLogger: logger}
}

// LogRetry implements retrier.DebugLogger.LogRetry by delegating to DebugLogger.LogRetry.
// The attrs parameter is converted to FieldMap for the underlying logger.
func (a *RetryLoggerAdapter) LogRetry(ctx context.Context, attempt int, maxAttempts int, backoff time.Duration, err error, attrs ...any) {
	// Convert attrs to FieldMap
	fieldMap := FieldMap{}
	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			if key, ok := attrs[i].(string); ok {
				fieldMap[key] = attrs[i+1]
			}
		}
	}
	a.DebugLogger.LogRetry(ctx, attempt, maxAttempts, backoff, err, fieldMap)
}

// Ensure RetryLoggerAdapter implements retrier.DebugLogger at compile time.
var _ retrier.DebugLogger = (*RetryLoggerAdapter)(nil)
