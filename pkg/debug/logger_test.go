package debug

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNoOpLogger(t *testing.T) {
	logger := NewNoOpLogger()

	t.Run("Enabled returns false", func(t *testing.T) {
		if logger.Enabled() {
			t.Error("NoOpLogger.Enabled() should return false")
		}
	})

	t.Run("LogStage is no-op", func(t *testing.T) {
		// Should not panic
		logger.LogStage(context.Background(), "fetcher", StageEvent{
			Type: EventTypeStart,
			URL:  "https://example.com",
		})
	})

	t.Run("LogRetry is no-op", func(t *testing.T) {
		// Should not panic
		logger.LogRetry(context.Background(), 1, 10, 100*time.Millisecond, 0, errors.New("test"))
	})

	t.Run("LogRateLimit is no-op", func(t *testing.T) {
		// Should not panic
		logger.LogRateLimit(context.Background(), "example.com", 1*time.Second, RateLimitReasonBaseDelay)
	})

	t.Run("LogStep is no-op", func(t *testing.T) {
		// Should not panic
		logger.LogStep(context.Background(), "fetcher", "http_request", FieldMap{"url": "https://example.com"})
	})

	t.Run("LogError is no-op", func(t *testing.T) {
		// Should not panic
		logger.LogError(context.Background(), "fetcher", errors.New("test"), nil)
	})

	t.Run("WithFields returns same instance", func(t *testing.T) {
		result := logger.WithFields(FieldMap{"key": "value"})
		if result != logger {
			t.Error("NoOpLogger.WithFields() should return the same instance")
		}
	})

	t.Run("Close returns nil", func(t *testing.T) {
		if err := logger.Close(); err != nil {
			t.Errorf("NoOpLogger.Close() should return nil, got: %v", err)
		}
	})
}

func TestNewDebugConfig(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		outputFile  string
		format      string
		wantErr     bool
		wantFormat  Format
		wantEnabled bool
	}{
		{
			name:        "default json format",
			enabled:     true,
			outputFile:  "",
			format:      "",
			wantErr:     false,
			wantFormat:  FormatJSON,
			wantEnabled: true,
		},
		{
			name:        "explicit json format",
			enabled:     true,
			outputFile:  "/tmp/debug.log",
			format:      "json",
			wantErr:     false,
			wantFormat:  FormatJSON,
			wantEnabled: true,
		},
		{
			name:        "text format",
			enabled:     false,
			outputFile:  "",
			format:      "text",
			wantErr:     false,
			wantFormat:  FormatText,
			wantEnabled: false,
		},
		{
			name:       "invalid format",
			enabled:    true,
			outputFile: "",
			format:     "invalid",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewDebugConfig(tt.enabled, tt.outputFile, tt.format)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if cfg.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", cfg.Enabled, tt.wantEnabled)
			}
			if cfg.Format != tt.wantFormat {
				t.Errorf("Format = %v, want %v", cfg.Format, tt.wantFormat)
			}
			if cfg.OutputFile != tt.outputFile {
				t.Errorf("OutputFile = %v, want %v", cfg.OutputFile, tt.outputFile)
			}
		})
	}
}

func TestLogstashHandler(t *testing.T) {
	tests := []struct {
		name         string
		event        StageEvent
		wantFields   map[string]any
		excludeField string
	}{
		{
			name: "start event",
			event: StageEvent{
				Type: EventTypeStart,
				URL:  "https://example.com/docs",
			},
			wantFields: map[string]any{
				"stage":       "fetcher",
				"event_type":  "start",
				"url":         "https://example.com/docs",
				"@timestamp":  nil, // will be checked separately
				"@version":    "1",
				"level":       "DEBUG",
				"logger_name": "docs-crawler",
			},
		},
		{
			name: "complete event with duration",
			event: StageEvent{
				Type:     EventTypeComplete,
				URL:      "https://example.com/docs",
				Duration: 150 * time.Millisecond,
				Fields: FieldMap{
					"status_code": 200,
				},
			},
			wantFields: map[string]any{
				"stage":       "fetcher",
				"event_type":  "complete",
				"url":         "https://example.com/docs",
				"duration_ms": float64(150),
				"status_code": float64(200),
				"@version":    "1",
				"level":       "DEBUG",
				"logger_name": "docs-crawler",
			},
		},
		{
			name: "exclude field",
			event: StageEvent{
				Type: EventTypeStart,
				URL:  "https://example.com",
			},
			excludeField: "url",
			wantFields: map[string]any{
				"stage":       "fetcher",
				"event_type":  "start",
				"@version":    "1",
				"level":       "DEBUG",
				"logger_name": "docs-crawler",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := &LogstashHandlerOptions{}
			if tt.excludeField != "" {
				opts.ExcludeFields = []string{tt.excludeField}
			}
			handler := NewLogstashHandler(&buf, opts)

			logger := SlogLogger{logger: slog.New(handler), enabled: true}
			logger.LogStage(context.Background(), "fetcher", tt.event)

			// Parse output
			var result map[string]any
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse JSON output: %v", err)
			}

			// Check expected fields
			for key, wantValue := range tt.wantFields {
				if key == "@timestamp" {
					if _, ok := result[key]; !ok {
						t.Error("missing @timestamp field")
					}
					continue
				}
				got, ok := result[key]
				if !ok {
					t.Errorf("missing field: %s", key)
					continue
				}
				if wantValue != nil && got != wantValue {
					t.Errorf("field %s = %v, want %v", key, got, wantValue)
				}
			}

			// Verify excluded field is not present
			if tt.excludeField != "" {
				if _, ok := result[tt.excludeField]; ok {
					t.Errorf("excluded field %s should not be present", tt.excludeField)
				}
			}
		})
	}
}

func TestTextHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTextHandler(&buf, nil)

	logger := SlogLogger{logger: slog.New(handler), enabled: true}
	logger.LogStage(context.Background(), "fetcher", StageEvent{
		Type: EventTypeStart,
		URL:  "https://example.com/docs",
	})

	output := buf.String()

	// Check format: timestamp [level] fields...
	if !strings.Contains(output, "[DEBUG]") {
		t.Errorf("output should contain [DEBUG], got: %s", output)
	}
	if !strings.Contains(output, "stage=fetcher") {
		t.Errorf("output should contain stage=fetcher, got: %s", output)
	}
	if !strings.Contains(output, "event_type=start") {
		t.Errorf("output should contain event_type=start, got: %s", output)
	}
}

func TestSlogLogger_AllMethods(t *testing.T) {
	t.Run("LogRetry", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := SlogLogger{logger: slog.New(handler), enabled: true}

		logger.LogRetry(context.Background(), 1, 10, 100*time.Millisecond, 50*time.Millisecond, errors.New("timeout"))

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["attempt"] != float64(1) {
			t.Errorf("attempt = %v, want 1", result["attempt"])
		}
		if result["max_attempts"] != float64(10) {
			t.Errorf("max_attempts = %v, want 10", result["max_attempts"])
		}
		if result["backoff_ms"] != float64(100) {
			t.Errorf("backoff_ms = %v, want 100", result["backoff_ms"])
		}
		if result["error"] != "timeout" {
			t.Errorf("error = %v, want timeout", result["error"])
		}
	})

	t.Run("LogRateLimit", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := SlogLogger{logger: slog.New(handler), enabled: true}

		logger.LogRateLimit(context.Background(), "example.com", 1500*time.Millisecond, RateLimitReasonBaseDelay)

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["host"] != "example.com" {
			t.Errorf("host = %v, want example.com", result["host"])
		}
		if result["delay_ms"] != float64(1500) {
			t.Errorf("delay_ms = %v, want 1500", result["delay_ms"])
		}
		if result["rate_limit_reason"] != "base_delay" {
			t.Errorf("rate_limit_reason = %v, want base_delay", result["rate_limit_reason"])
		}
	})

	t.Run("LogStep", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := SlogLogger{logger: slog.New(handler), enabled: true}

		logger.LogStep(context.Background(), "fetcher", "http_request", FieldMap{
			"method": "GET",
			"url":    "https://example.com",
		})

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["stage"] != "fetcher" {
			t.Errorf("stage = %v, want fetcher", result["stage"])
		}
		if result["step"] != "http_request" {
			t.Errorf("step = %v, want http_request", result["step"])
		}
		if result["method"] != "GET" {
			t.Errorf("method = %v, want GET", result["method"])
		}
	})

	t.Run("LogError", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := SlogLogger{logger: slog.New(handler), enabled: true}

		logger.LogError(context.Background(), "fetcher", errors.New("connection refused"), FieldMap{
			"retry_count": 3,
		})

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["stage"] != "fetcher" {
			t.Errorf("stage = %v, want fetcher", result["stage"])
		}
		if result["error"] != "connection refused" {
			t.Errorf("error = %v, want connection refused", result["error"])
		}
		if result["retry_count"] != float64(3) {
			t.Errorf("retry_count = %v, want 3", result["retry_count"])
		}
	})

	t.Run("WithFields", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := SlogLogger{logger: slog.New(handler), enabled: true}

		loggerWithFields := logger.WithFields(FieldMap{
			"crawl_id": "crawl-123",
		})
		loggerWithFields.LogStage(context.Background(), "fetcher", StageEvent{
			Type: EventTypeStart,
		})

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["crawl_id"] != "crawl-123" {
			t.Errorf("crawl_id = %v, want crawl-123", result["crawl_id"])
		}
	})
}

func TestNewSlogLogger_Disabled(t *testing.T) {
	cfg := DebugConfig{
		Enabled: false,
	}

	logger, err := NewSlogLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return NoOpLogger when disabled
	if logger.Enabled() {
		t.Error("logger should be disabled")
	}

	// Should be NoOpLogger type
	if _, ok := logger.(*NoOpLogger); !ok {
		t.Error("expected NoOpLogger when disabled")
	}
}

func TestMultiWriter(t *testing.T) {
	t.Run("stdout only", func(t *testing.T) {
		// This test just verifies creation without file
		writer, err := NewMultiWriter("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if writer == nil {
			t.Error("writer should not be nil")
		}
		if err := writer.Close(); err != nil {
			t.Errorf("unexpected close error: %v", err)
		}
	})
}

func TestStageMessage(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      string
	}{
		{EventTypeStart, "Pipeline stage started"},
		{EventTypeProgress, "Pipeline stage progress"},
		{EventTypeComplete, "Pipeline stage completed"},
		{EventTypeError, "Pipeline stage error"},
		{EventType("unknown"), "Pipeline stage event"},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			got := stageMessage(tt.eventType, "test")
			if got != tt.want {
				t.Errorf("stageMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRateLimitReasonConstants(t *testing.T) {
	// Verify all constants match design document
	tests := []struct {
		reason RateLimitReason
		want   string
	}{
		{RateLimitReasonBaseDelay, "base_delay"},
		{RateLimitReasonCrawlDelay, "crawl_delay"},
		{RateLimitReasonBackoff, "backoff"},
		{RateLimitReasonJitter, "jitter"},
		{RateLimitReason429, "http_429"},
		{RateLimitReason5xx, "http_5xx"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.reason) != tt.want {
				t.Errorf("RateLimitReason = %v, want %v", tt.reason, tt.want)
			}
		})
	}
}

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      string
	}{
		{EventTypeStart, "start"},
		{EventTypeProgress, "progress"},
		{EventTypeComplete, "complete"},
		{EventTypeError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.eventType) != tt.want {
				t.Errorf("EventType = %v, want %v", tt.eventType, tt.want)
			}
		})
	}
}
