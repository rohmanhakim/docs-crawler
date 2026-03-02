package debug

import (
	"context"
	"time"
)

// DebugLogger provides structured debug logging capabilities.
// All methods are no-ops when debug mode is disabled.
type DebugLogger interface {
	// Enabled returns true if debug logging is enabled.
	Enabled() bool

	// LogStage logs a pipeline stage event with structured fields.
	LogStage(ctx context.Context, stage string, event StageEvent)

	// LogRetry logs retry attempts with backoff information.
	LogRetry(ctx context.Context, attempt int, maxAttempts int, backoff time.Duration, err error)

	// LogRateLimit logs rate limiting decisions.
	LogRateLimit(ctx context.Context, host string, delay time.Duration, reason RateLimitReason)

	// LogStep logs a granular step within a pipeline stage.
	LogStep(ctx context.Context, stage string, step string, fields FieldMap)

	// LogError logs a debug-level error with context.
	LogError(ctx context.Context, stage string, err error, fields FieldMap)

	// WithFields returns a logger with pre-populated fields.
	WithFields(fields FieldMap) DebugLogger

	// Close flushes any buffered output and closes file handles.
	Close() error
}

// EventType represents the type of a pipeline stage event.
type EventType string

const (
	// EventTypeStart indicates a stage has started.
	EventTypeStart EventType = "start"
	// EventTypeProgress indicates progress within a stage.
	EventTypeProgress EventType = "progress"
	// EventTypeComplete indicates a stage has completed successfully.
	EventTypeComplete EventType = "complete"
	// EventTypeError indicates a stage has encountered an error.
	EventTypeError EventType = "error"
)

// StageEvent represents a pipeline stage event.
type StageEvent struct {
	// Type indicates the event type: start, progress, complete, error.
	Type EventType

	// URL being processed (if applicable).
	URL string

	// Duration of the stage (for complete events).
	Duration time.Duration

	// InputSummary is a truncated summary of input (for large inputs).
	InputSummary string

	// OutputSummary is a truncated summary of output (for large outputs).
	OutputSummary string

	// Additional structured fields.
	Fields FieldMap
}

// RateLimitReason describes why a rate limit was applied.
type RateLimitReason string

const (
	// RateLimitReasonBaseDelay indicates the base delay was applied.
	RateLimitReasonBaseDelay RateLimitReason = "base_delay"
	// RateLimitReasonCrawlDelay indicates crawl-delay from robots.txt was applied.
	RateLimitReasonCrawlDelay RateLimitReason = "crawl_delay"
	// RateLimitReasonBackoff indicates exponential backoff was applied.
	RateLimitReasonBackoff RateLimitReason = "backoff"
	// RateLimitReasonJitter indicates random jitter was added.
	RateLimitReasonJitter RateLimitReason = "jitter"
	// RateLimitReason429 indicates delay due to HTTP 429 response.
	RateLimitReason429 RateLimitReason = "http_429"
	// RateLimitReason5xx indicates delay due to HTTP 5xx response.
	RateLimitReason5xx RateLimitReason = "http_5xx"
)

// FieldMap is a map of structured field names to values.
type FieldMap map[string]any
