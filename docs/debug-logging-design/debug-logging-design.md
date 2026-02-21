# Debug Logging Design Document

## 1. Overview

This document describes the design for implementing a `--debug` flag that enables granular debug logging throughout the docs-crawler application. The debug logging system will provide visibility into internal operations, pipeline stages, retry mechanisms, and rate limiting decisions.

## 2. Goals

- Provide a centralized debug logging package that can be used across all components
- Enable granular visibility into pipeline stage execution with step-by-step details
- Surface retry backoff timing and remaining attempt counts
- Expose rate limiting decisions and delay computations
- Support structured output format compatible with popular tracing platforms (Logstash, Elasticsearch, Jaeger, etc.)
- Allow configuration of output destination (stdout, file, or both)
- Maintain zero overhead when debug mode is disabled

## 3. Non-Goals

- Distributed tracing across multiple crawler instances
- Real-time streaming to external services
- Log aggregation or log rotation (use external tools like logrotate)
- Performance profiling or metrics collection
- Replacing the existing metadata event system

## 4. Architecture

### 4.1 Package Structure

```
pkg/
  debug/
    logger.go       # Core logger interface and implementation
    config.go       # Configuration for debug logging
    formatter.go    # Output formatters (JSON, text)
    writer.go       # Multi-output writer (stdout, file)
    context.go      # Context helpers for debug logging
    noop.go         # NoOp implementation for zero overhead
```

### 4.2 Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI Layer                                │
│  --debug flag → DebugConfig → DebugLogger                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Scheduler                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │   Fetcher   │→ │  Extractor  │→ │  Sanitizer  │ → ...      │
│  │  (debug)    │  │  (debug)    │  │  (debug)    │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│        │                │                │                      │
│        └────────────────┴────────────────┘                      │
│                         │                                       │
│                         ▼                                       │
│              ┌─────────────────────┐                           │
│              │    DebugLogger      │                           │
│              │  (JSON/Text output) │                           │
│              └─────────────────────┘                           │
│                         │                                       │
│            ┌────────────┴────────────┐                         │
│            ▼                         ▼                         │
│     ┌───────────┐            ┌───────────┐                    │
│     │  stdout   │            │   file    │                    │
│     └───────────┘            └───────────┘                    │
└─────────────────────────────────────────────────────────────────┘
```

## 5. Debug Logger Interface

### 5.1 Core Interface

```go
// DebugLogger provides structured debug logging capabilities.
// All methods are no-ops when debug mode is disabled.
type DebugLogger interface {
    // Enabled returns true if debug logging is enabled.
    Enabled() bool

    // LogStage logs a pipeline stage event with structured fields.
    LogStage(ctx context.Context, stage string, event StageEvent)

    // LogRetry logs retry attempts with backoff information.
    LogRetry(ctx context.Context, attempt int, maxAttempts int, backoff time.Duration, remaining time.Duration, err error)

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
```

### 5.2 Stage Event Types

```go
// StageEvent represents a pipeline stage event.
type StageEvent struct {
    // Type indicates the event type: start, progress, complete, error
    Type EventType
    
    // URL being processed (if applicable)
    URL string
    
    // Duration of the stage (for complete events)
    Duration time.Duration
    
    // Input summary (truncated for large inputs)
    InputSummary string
    
    // Output summary (truncated for large outputs)
    OutputSummary string
    
    // Additional structured fields
    Fields FieldMap
}

type EventType string

const (
    EventTypeStart    EventType = "start"
    EventTypeProgress EventType = "progress"
    EventTypeComplete EventType = "complete"
    EventTypeError    EventType = "error"
)
```

### 5.3 Rate Limit Reason Types

```go
// RateLimitReason describes why a rate limit was applied.
type RateLimitReason string

const (
    RateLimitReasonBaseDelay  RateLimitReason = "base_delay"
    RateLimitReasonCrawlDelay RateLimitReason = "crawl_delay"
    RateLimitReasonBackoff    RateLimitReason = "backoff"
    RateLimitReasonJitter     RateLimitReason = "jitter"
    RateLimitReason429        RateLimitReason = "http_429"
    RateLimitReason5xx        RateLimitReason = "http_5xx"
)
```

## 6. Output Format

### 6.1 JSON Structure (Logstash/Elasticsearch Compatible)

Each log entry follows this JSON structure:

```json
{
  "@timestamp": "2026-02-21T13:50:00.123456789Z",
  "@version": "1",
  "level": "DEBUG",
  "logger_name": "docs-crawler",
  "message": "Pipeline stage completed",
  "thread_name": "main",
  "stage": "fetcher",
  "event_type": "complete",
  "url": "https://example.com/docs/page",
  "duration_ms": 245,
  "attempt": 1,
  "max_attempts": 10,
  "backoff_ms": 100,
  "remaining_ms": 0,
  "host": "example.com",
  "delay_ms": 1500,
  "rate_limit_reason": "base_delay",
  "step": "http_request",
  "crawl_id": "crawl-abc123",
  "worker_id": "worker-1",
  "fields": {
    "status_code": 200,
    "content_type": "text/html",
    "content_length": 15234
  },
  "error": null,
  "stack_trace": null
}
```

### 6.2 Field Naming Conventions

All field names follow these conventions for compatibility with common tracing platforms:

| Field | Type | Description |
|-------|------|-------------|
| `@timestamp` | string | ISO 8601 timestamp with nanoseconds |
| `@version` | string | Log format version (always "1") |
| `level` | string | Log level (DEBUG, INFO, WARN, ERROR) |
| `logger_name` | string | Logger identifier |
| `message` | string | Human-readable message |
| `stage` | string | Pipeline stage name |
| `event_type` | string | Event type within stage |
| `url` | string | URL being processed |
| `duration_ms` | int64 | Duration in milliseconds |
| `attempt` | int | Current retry attempt |
| `max_attempts` | int | Maximum retry attempts |
| `backoff_ms` | int64 | Backoff duration in milliseconds |
| `remaining_ms` | int64 | Remaining wait time in milliseconds |
| `host` | string | Target hostname |
| `delay_ms` | int64 | Rate limit delay in milliseconds |
| `rate_limit_reason` | string | Reason for rate limit |
| `step` | string | Granular step within stage |
| `crawl_id` | string | Unique crawl identifier |
| `worker_id` | string | Worker identifier |
| `fields` | object | Additional structured fields |
| `error` | string | Error message (if applicable) |
| `stack_trace` | string | Stack trace (if applicable) |

### 6.3 Text Format (Human-Readable)

For local debugging, a human-readable text format is also supported:

```
2026-02-21T13:50:00.123Z [DEBUG] [fetcher] [start] url=https://example.com/docs/page
2026-02-21T13:50:00.245Z [DEBUG] [fetcher] [step:http_request] status_code=200 duration=122ms
2026-02-21T13:50:00.246Z [DEBUG] [fetcher] [complete] url=https://example.com/docs/page duration=123ms
2026-02-21T13:50:00.247Z [DEBUG] [retry] attempt=1/10 backoff=100ms remaining=0ms error=network timeout
2026-02-21T13:50:01.350Z [DEBUG] [rate_limit] host=example.com delay=1500ms reason=base_delay
```

## 7. Configuration

### 7.1 CLI Flags

```go
// In internal/cli/root.go
var (
    debug       bool   // --debug flag
    debugFile   string // --debug-file flag (optional, for file output)
    debugFormat string // --debug-format flag (json or text, default: json)
)
```

### 7.2 Configuration File Support

```json
{
  "debug": {
    "enabled": true,
    "outputFile": "/var/log/docs-crawler/debug.jsonl",
    "format": "json",
    "includeFields": ["url", "stage", "duration_ms"],
    "excludeFields": ["stack_trace"]
  }
}
```

### 7.3 Debug Config Structure

```go
// DebugConfig holds configuration for debug logging.
type DebugConfig struct {
    // Enabled controls whether debug logging is active.
    Enabled bool
    
    // OutputFile is the path to write debug logs.
    // Empty means stdout only.
    OutputFile string
    
    // Format controls output format: "json" or "text".
    Format string
    
    // IncludeFields filters fields to include (empty = all).
    IncludeFields []string
    
    // ExcludeFields filters fields to exclude.
    ExcludeFields []string
}

// NewDebugConfig creates a DebugConfig from CLI flags.
func NewDebugConfig(enabled bool, outputFile string, format string) DebugConfig {
    if format == "" {
        format = "json"
    }
    return DebugConfig{
        Enabled:      enabled,
        OutputFile:   outputFile,
        Format:       format,
    }
}
```

## 8. Integration Points

### 8.1 Scheduler Integration

The scheduler will receive a `DebugLogger` instance and pass it to pipeline stages:

```go
type Scheduler struct {
    // ... existing fields
    debugLogger debug.DebugLogger
}

func (s *Scheduler) ExecuteCrawlingWithState(init *CrawlInitialization) (CrawlingExecution, error) {
    // Log pipeline start
    s.debugLogger.LogStage(s.ctx, "pipeline", debug.StageEvent{
        Type: debug.EventTypeStart,
        URL:  nextCrawlToken.URL().String(),
    })
    
    // ... existing code
    
    // Log fetcher stage
    s.debugLogger.LogStage(s.ctx, "fetcher", debug.StageEvent{
        Type:      debug.EventTypeStart,
        URL:       nextCrawlToken.URL().String(),
    })
    
    fetchResult, err := s.htmlFetcher.Fetch(...)
    
    s.debugLogger.LogStage(s.ctx, "fetcher", debug.StageEvent{
        Type:     debug.EventTypeComplete,
        URL:      nextCrawlToken.URL().String(),
        Duration: time.Since(startTime),
        Fields: debug.FieldMap{
            "status_code": fetchResult.Code(),
        },
    })
}
```

### 8.2 Retry Handler Integration

The retry package will accept an optional `DebugLogger`:

```go
// Retry executes the provided function with retry logic and debug logging.
func Retry[T any](retryParam RetryParam, logger debug.DebugLogger, fn func() (T, failure.ClassifiedError)) Result[T] {
    // ... existing code
    
    for attempt := 1; attempt <= retryParam.MaxAttempts; attempt++ {
        result, err := fn()
        
        if err != nil && shouldAutoRetry(err) && attempt < retryParam.MaxAttempts {
            backoffDelay := timeutil.ExponentialBackoffDelay(...)
            
            // Log retry attempt
            if logger.Enabled() {
                logger.LogRetry(context.Background(), attempt, retryParam.MaxAttempts, 
                    backoffDelay, 0, err)
            }
            
            time.Sleep(backoffDelay)
        }
    }
}
```

### 8.3 Rate Limiter Integration

The rate limiter will log delay decisions:

```go
func (r *ConcurrentRateLimiter) ResolveDelay(host string) time.Duration {
    // ... existing code
    
    // Log rate limit decision if debug enabled
    if r.debugLogger.Enabled() {
        r.debugLogger.LogRateLimit(context.Background(), host, finalDelay, reason)
    }
    
    return finalDelay
}
```

### 8.4 Pipeline Stage Integration

Each pipeline stage can log granular steps:

```go
// In fetcher
func (h *HtmlFetcher) Fetch(...) {
    h.debugLogger.LogStep(ctx, "fetcher", "create_request", debug.FieldMap{
        "method": "GET",
        "url":    fetchUrl.String(),
    })
    
    resp, err := h.httpClient.Do(req)
    
    h.debugLogger.LogStep(ctx, "fetcher", "response_received", debug.FieldMap{
        "status_code":  resp.StatusCode,
        "content_type": resp.Header.Get("Content-Type"),
    })
    
    body, err := io.ReadAll(resp.Body)
    
    h.debugLogger.LogStep(ctx, "fetcher", "body_read", debug.FieldMap{
        "content_length": len(body),
    })
}
```

## 9. Example Debug Output

### 9.1 Full Pipeline Stage Debug Log

```json
{"@timestamp":"2026-02-21T13:50:00.100Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Pipeline stage started","stage":"fetcher","event_type":"start","url":"https://docs.example.com/getting-started","crawl_id":"crawl-20260221-135000","worker_id":"worker-1"}
{"@timestamp":"2026-02-21T13:50:00.101Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Step executed","stage":"fetcher","step":"create_request","fields":{"method":"GET","url":"https://docs.example.com/getting-started"}}
{"@timestamp":"2026-02-21T13:50:00.250Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Step executed","stage":"fetcher","step":"response_received","fields":{"status_code":200,"content_type":"text/html; charset=utf-8"}}
{"@timestamp":"2026-02-21T13:50:00.255Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Step executed","stage":"fetcher","step":"body_read","fields":{"content_length":15234}}
{"@timestamp":"2026-02-21T13:50:00.256Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Pipeline stage completed","stage":"fetcher","event_type":"complete","url":"https://docs.example.com/getting-started","duration_ms":156}
{"@timestamp":"2026-02-21T13:50:00.257Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Pipeline stage started","stage":"extractor","event_type":"start","url":"https://docs.example.com/getting-started"}
{"@timestamp":"2026-02-21T13:50:00.260Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Step executed","stage":"extractor","step":"parse_html","fields":{"input_size":15234}}
{"@timestamp":"2026-02-21T13:50:00.275Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Step executed","stage":"extractor","step":"find_main_content","fields":{"selector":"main","found":true}}
{"@timestamp":"2026-02-21T13:50:00.280Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Pipeline stage completed","stage":"extractor","event_type":"complete","url":"https://docs.example.com/getting-started","duration_ms":23}
```

### 9.2 Retry with Backoff Debug Log

```json
{"@timestamp":"2026-02-21T13:50:05.000Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Retry attempt failed","stage":"fetcher","attempt":1,"max_attempts":10,"backoff_ms":100,"remaining_ms":100,"error":"network timeout"}
{"@timestamp":"2026-02-21T13:50:05.100Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Retry attempt failed","stage":"fetcher","attempt":2,"max_attempts":10,"backoff_ms":200,"remaining_ms":200,"error":"network timeout"}
{"@timestamp":"2026-02-21T13:50:05.300Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Retry attempt failed","stage":"fetcher","attempt":3,"max_attempts":10,"backoff_ms":400,"remaining_ms":400,"error":"network timeout"}
{"@timestamp":"2026-02-21T13:50:05.700Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Retry attempt succeeded","stage":"fetcher","attempt":4,"max_attempts":10}
```

### 9.3 Rate Limiting Debug Log

```json
{"@timestamp":"2026-02-21T13:50:06.000Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Rate limit applied","host":"docs.example.com","delay_ms":1500,"rate_limit_reason":"base_delay","fields":{"base_delay_ms":1000,"jitter_ms":500}}
{"@timestamp":"2026-02-21T13:50:07.500Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Rate limit applied","host":"docs.example.com","delay_ms":2000,"rate_limit_reason":"crawl_delay","fields":{"crawl_delay_ms":2000}}
{"@timestamp":"2026-02-21T13:50:09.500Z","@version":"1","level":"DEBUG","logger_name":"docs-crawler","message":"Rate limit applied","host":"docs.example.com","delay_ms":4000,"rate_limit_reason":"backoff","fields":{"backoff_count":2}}
```

## 10. Performance Considerations

### 10.1 Zero Overhead When Disabled

When debug mode is disabled (`--debug` not passed), the `NoOpLogger` implementation is used:

```go
type NoOpLogger struct{}

func (n *NoOpLogger) Enabled() bool { return false }
func (n *NoOpLogger) LogStage(_ context.Context, _ string, _ StageEvent) {}
func (n *NoOpLogger) LogRetry(_ context.Context, _ int, _ int, _ time.Duration, _ time.Duration, _ error) {}
func (n *NoOpLogger) LogRateLimit(_ context.Context, _ string, _ time.Duration, _ RateLimitReason) {}
func (n *NoOpLogger) LogStep(_ context.Context, _ string, _ string, _ FieldMap) {}
func (n *NoOpLogger) LogError(_ context.Context, _ string, _ error, _ FieldMap) {}
func (n *NoOpLogger) WithFields(_ FieldMap) DebugLogger { return n }
func (n *NoOpLogger) Close() error { return nil }
```

### 10.2 Buffered File Output

When writing to a file, use buffered I/O to minimize performance impact:

```go
type FileOutput struct {
    mu     sync.Mutex
    file   *os.File
    writer *bufio.Writer
}

func (f *FileOutput) Write(p []byte) (n int, err error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    
    n, err = f.writer.Write(p)
    if err != nil {
        return n, err
    }
    
    // Flush after each log entry to ensure durability
    return n, f.writer.Flush()
}
```

### 10.3 Async Logging (Future Enhancement)

For high-throughput scenarios, consider async logging with a channel:

```go
type AsyncLogger struct {
    entries chan LogEntry
    logger  DebugLogger
    wg      sync.WaitGroup
}

func (a *AsyncLogger) start() {
    a.wg.Add(1)
    go func() {
        defer a.wg.Done()
        for entry := range a.entries {
            a.logger.logEntry(entry)
        }
    }()
}
```

## 11. Testing Strategy

### 11.1 Unit Tests

- Test each formatter (JSON, text) independently
- Test field filtering logic
- Test NoOpLogger returns immediately without side effects
- Test buffered file output with sync.MockFile

### 11.2 Integration Tests

- Test debug logging through full pipeline execution
- Verify log output contains expected fields
- Test file output creates valid JSONL files

### 11.3 Test Helpers

```go
// CaptureLogger captures all log entries for testing.
type CaptureLogger struct {
    mu      sync.Mutex
    entries []LogEntry
}

func (c *CaptureLogger) LogStage(_ context.Context, stage string, event StageEvent) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.entries = append(c.entries, LogEntry{
        Stage: stage,
        Event: event,
    })
}

func (c *CaptureLogger) Entries() []LogEntry {
    c.mu.Lock()
    defer c.mu.Unlock()
    return append([]LogEntry{}, c.entries...)
}
```

## 12. Migration Path

### 12.1 Phase 1: Core Infrastructure

1. Create `pkg/debug` package with interfaces and types
2. Implement `NoOpLogger` and `JSONLogger`
3. Add `--debug` and `--debug-file` flags to CLI
4. Add `DebugConfig` to config package

### 12.2 Phase 2: Scheduler Integration

1. Add `DebugLogger` field to `Scheduler`
2. Pass logger to `NewSchedulerWithConfig`
3. Log pipeline stage start/complete events

### 12.3 Phase 3: Component Integration

1. Integrate with retry handler
2. Integrate with rate limiter
3. Add granular step logging to each pipeline stage

### 12.4 Phase 4: Documentation and Examples

1. Document CLI flags in README
2. Add examples of debug output format
3. Document Logstash/Elasticsearch integration

## 13. Backward Compatibility

- The `--debug` flag is optional and defaults to `false`
- No changes to existing behavior when disabled
- No breaking changes to existing APIs
- Debug logging is additive to existing metadata system

## 14. Security Considerations

- Sanitize URLs to avoid logging sensitive query parameters
- Avoid logging full response bodies (use truncated summaries)
- File output should respect file permissions (0600)
- Consider log file path traversal prevention

## 15. Appendix: Logstash Pipeline Configuration

Example Logstash pipeline configuration for ingesting debug logs:

```ruby
input {
  file {
    path => "/var/log/docs-crawler/debug.jsonl"
    codec => json
    start_position => "beginning"
    sincedb_path => "/dev/null"
  }
}

filter {
  date {
    match => ["@timestamp", "ISO8601"]
  }
  
  # Add geo-ip for host field
  dns {
    reverse => ["host"]
    action => "replace"
  }
}

output {
  elasticsearch {
    hosts => ["localhost:9200"]
    index => "docs-crawler-debug-%{+YYYY.MM.dd}"
  }
}
```

## 16. Appendix: Jaeger/OpenTelemetry Integration (Future)

For distributed tracing, the debug logger can be extended to emit OpenTelemetry spans:

```go
// TelemetryLogger extends DebugLogger with OpenTelemetry support.
type TelemetryLogger struct {
    DebugLogger
    tracer trace.Tracer
}

func (t *TelemetryLogger) LogStage(ctx context.Context, stage string, event StageEvent) {
    ctx, span := t.tracer.Start(ctx, stage)
    defer span.End()
    
    span.SetAttributes(
        attribute.String("url", event.URL),
        attribute.Int64("duration_ms", event.Duration.Milliseconds()),
    )
    
    t.DebugLogger.LogStage(ctx, stage, event)
}
```

---

**Document Status**: Draft  
**Author**: System Design Team  
**Created**: 2026-02-21  
**Last Updated**: 2026-02-21