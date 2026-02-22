package debug

import (
	"context"
	"log/slog"
	"time"
)

// SlogLogger wraps slog.Logger and implements the DebugLogger interface.
// It provides structured logging with domain-specific methods for pipeline stages,
// retry attempts, and rate limiting decisions.
type SlogLogger struct {
	logger   *slog.Logger
	enabled  bool
	preAttrs FieldMap
	closer   func() error
}

// NewSlogLogger creates a new SlogLogger with the given configuration.
// If config.Enabled is false, a NoOpLogger is returned instead.
func NewSlogLogger(config DebugConfig) (DebugLogger, error) {
	if !config.Enabled {
		return NewNoOpLogger(), nil
	}

	// Create the writer (stdout + optional file)
	writer, err := NewMultiWriter(config.OutputFile)
	if err != nil {
		return nil, err
	}

	// Create the appropriate handler based on format
	var handler slog.Handler
	switch config.Format {
	case FormatText:
		handler = NewTextHandler(writer, &TextHandlerOptions{})
	default:
		handler = NewLogstashHandler(writer, &LogstashHandlerOptions{
			IncludeFields: config.IncludeFields,
			ExcludeFields: config.ExcludeFields,
		})
	}

	return &SlogLogger{
		logger:  slog.New(handler),
		enabled: true,
		closer:  writer.Close,
	}, nil
}

// Enabled returns true if debug logging is enabled.
func (s *SlogLogger) Enabled() bool { return s.enabled }

// LogStage logs a pipeline stage event with structured fields.
func (s *SlogLogger) LogStage(ctx context.Context, stage string, event StageEvent) {
	attrs := []slog.Attr{
		slog.String("stage", stage),
		slog.String("event_type", string(event.Type)),
	}

	if event.URL != "" {
		attrs = append(attrs, slog.String("url", event.URL))
	}

	if event.Duration > 0 {
		attrs = append(attrs, slog.Int64("duration_ms", event.Duration.Milliseconds()))
	}

	if event.InputSummary != "" {
		attrs = append(attrs, slog.String("input_summary", event.InputSummary))
	}

	if event.OutputSummary != "" {
		attrs = append(attrs, slog.String("output_summary", event.OutputSummary))
	}

	// Add pre-populated fields
	for k, v := range s.preAttrs {
		attrs = append(attrs, slog.Any(k, v))
	}

	// Add event fields
	for k, v := range event.Fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	s.logger.LogAttrs(ctx, slog.LevelDebug, stageMessage(event.Type, stage), attrs...)
}

// LogRetry logs retry attempts with backoff information.
func (s *SlogLogger) LogRetry(ctx context.Context, attempt int, maxAttempts int, backoff time.Duration, err error) {
	attrs := []slog.Attr{
		slog.Int("attempt", attempt),
		slog.Int("max_attempts", maxAttempts),
		slog.Int64("backoff_ms", backoff.Milliseconds()),
	}

	// Add pre-populated fields
	for k, v := range s.preAttrs {
		attrs = append(attrs, slog.Any(k, v))
	}

	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		s.logger.LogAttrs(ctx, slog.LevelDebug, "Retry attempt failed", attrs...)
	} else {
		s.logger.LogAttrs(ctx, slog.LevelDebug, "Retry attempt succeeded", attrs...)
	}
}

// LogRateLimit logs rate limiting decisions.
func (s *SlogLogger) LogRateLimit(ctx context.Context, host string, delay time.Duration, reason RateLimitReason) {
	attrs := []slog.Attr{
		slog.String("host", host),
		slog.Int64("delay_ms", delay.Milliseconds()),
		slog.String("rate_limit_reason", string(reason)),
	}

	// Add pre-populated fields
	for k, v := range s.preAttrs {
		attrs = append(attrs, slog.Any(k, v))
	}

	s.logger.LogAttrs(ctx, slog.LevelDebug, "Rate limit applied", attrs...)
}

// LogStep logs a granular step within a pipeline stage.
func (s *SlogLogger) LogStep(ctx context.Context, stage string, step string, fields FieldMap) {
	attrs := []slog.Attr{
		slog.String("stage", stage),
		slog.String("step", step),
	}

	// Add pre-populated fields
	for k, v := range s.preAttrs {
		attrs = append(attrs, slog.Any(k, v))
	}

	// Add step fields
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	s.logger.LogAttrs(ctx, slog.LevelDebug, "Step executed", attrs...)
}

// LogError logs a debug-level error with context.
func (s *SlogLogger) LogError(ctx context.Context, stage string, err error, fields FieldMap) {
	attrs := []slog.Attr{
		slog.String("stage", stage),
		slog.String("error", err.Error()),
	}

	// Add pre-populated fields
	for k, v := range s.preAttrs {
		attrs = append(attrs, slog.Any(k, v))
	}

	// Add error fields
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	s.logger.LogAttrs(ctx, slog.LevelDebug, "Error occurred", attrs...)
}

// WithFields returns a logger with pre-populated fields.
func (s *SlogLogger) WithFields(fields FieldMap) DebugLogger {
	merged := make(FieldMap)
	for k, v := range s.preAttrs {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}

	return &SlogLogger{
		logger:   s.logger,
		enabled:  s.enabled,
		preAttrs: merged,
		closer:   s.closer,
	}
}

// Close flushes any buffered output and closes file handles.
func (s *SlogLogger) Close() error {
	if s.closer != nil {
		return s.closer()
	}
	return nil
}

// stageMessage returns a human-readable message for a stage event.
func stageMessage(eventType EventType, stage string) string {
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
