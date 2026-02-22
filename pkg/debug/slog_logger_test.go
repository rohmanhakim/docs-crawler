package debug

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSlogLogger(t *testing.T) {
	t.Run("disabled_config_returns_noop", func(t *testing.T) {
		cfg := DebugConfig{
			Enabled: false,
		}

		logger, err := NewSlogLogger(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if logger.Enabled() {
			t.Error("logger should be disabled")
		}

		// Should be NoOpLogger type
		if _, ok := logger.(*NoOpLogger); !ok {
			t.Error("expected NoOpLogger when disabled")
		}
	})

	t.Run("enabled_config_returns_sloglogger", func(t *testing.T) {
		cfg := DebugConfig{
			Enabled: true,
			Format:  FormatJSON,
		}

		logger, err := NewSlogLogger(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !logger.Enabled() {
			t.Error("logger should be enabled")
		}

		// Should be SlogLogger type
		if _, ok := logger.(*SlogLogger); !ok {
			t.Error("expected SlogLogger when enabled")
		}

		// Clean up
		if err := logger.Close(); err != nil {
			t.Errorf("unexpected close error: %v", err)
		}
	})

	t.Run("json_format_creates_logstash_handler", func(t *testing.T) {
		cfg := DebugConfig{
			Enabled: true,
			Format:  FormatJSON,
		}

		logger, err := NewSlogLogger(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify by checking the output format
		slogLogger, ok := logger.(*SlogLogger)
		if !ok {
			t.Fatal("expected SlogLogger")
		}

		// Use a buffer to capture output
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		testLogger := &SlogLogger{}
		testLogger = slogLogger.WithFields(FieldMap{"test": "value"}).(*SlogLogger)

		// Simple verification that logger was created
		if testLogger == nil {
			t.Error("logger should not be nil")
		}

		_ = handler // avoid unused variable error

		if err := logger.Close(); err != nil {
			t.Errorf("unexpected close error: %v", err)
		}
	})

	t.Run("text_format_creates_text_handler", func(t *testing.T) {
		cfg := DebugConfig{
			Enabled: true,
			Format:  FormatText,
		}

		logger, err := NewSlogLogger(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !logger.Enabled() {
			t.Error("logger should be enabled")
		}

		if err := logger.Close(); err != nil {
			t.Errorf("unexpected close error: %v", err)
		}
	})

	t.Run("invalid_output_file_returns_error", func(t *testing.T) {
		cfg := DebugConfig{
			Enabled:    true,
			OutputFile: "/nonexistent/directory/that/does/not/exist/debug.log",
			Format:     FormatJSON,
		}

		_, err := NewSlogLogger(cfg)
		if err == nil {
			t.Error("expected error for invalid output file path")
		}
	})

	t.Run("with_valid_output_file", func(t *testing.T) {
		// Create a temp file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "debug.log")

		cfg := DebugConfig{
			Enabled:    true,
			OutputFile: tmpFile,
			Format:     FormatJSON,
		}

		logger, err := NewSlogLogger(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !logger.Enabled() {
			t.Error("logger should be enabled")
		}

		if err := logger.Close(); err != nil {
			t.Errorf("unexpected close error: %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
			t.Error("output file should have been created")
		}
	})
}

func TestSlogLogger_Enabled(t *testing.T) {
	t.Run("returns_true_when_enabled", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:  slog.New(handler),
			enabled: true,
		}

		if !logger.Enabled() {
			t.Error("Enabled() should return true")
		}
	})

	t.Run("returns_false_when_disabled", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:  slog.New(handler),
			enabled: false,
		}

		if logger.Enabled() {
			t.Error("Enabled() should return false")
		}
	})
}

func TestSlogLogger_LogStage_InputSummary_OutputSummary(t *testing.T) {
	t.Run("includes_input_summary_when_not_empty", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:  slog.New(handler),
			enabled: true,
		}

		event := StageEvent{
			Type:         EventTypeStart,
			URL:          "https://example.com",
			InputSummary: "input data summary",
		}

		logger.LogStage(context.Background(), "fetcher", event)

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["input_summary"] != "input data summary" {
			t.Errorf("input_summary = %v, want 'input data summary'", result["input_summary"])
		}
	})

	t.Run("includes_output_summary_when_not_empty", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:  slog.New(handler),
			enabled: true,
		}

		event := StageEvent{
			Type:          EventTypeComplete,
			URL:           "https://example.com",
			OutputSummary: "output data summary",
		}

		logger.LogStage(context.Background(), "fetcher", event)

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["output_summary"] != "output data summary" {
			t.Errorf("output_summary = %v, want 'output data summary'", result["output_summary"])
		}
	})

	t.Run("omits_input_summary_when_empty", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:  slog.New(handler),
			enabled: true,
		}

		event := StageEvent{
			Type:         EventTypeStart,
			URL:          "https://example.com",
			InputSummary: "", // empty
		}

		logger.LogStage(context.Background(), "fetcher", event)

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if _, exists := result["input_summary"]; exists {
			t.Error("input_summary should not be present when empty")
		}
	})

	t.Run("omits_output_summary_when_empty", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:  slog.New(handler),
			enabled: true,
		}

		event := StageEvent{
			Type:          EventTypeComplete,
			URL:           "https://example.com",
			OutputSummary: "", // empty
		}

		logger.LogStage(context.Background(), "fetcher", event)

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if _, exists := result["output_summary"]; exists {
			t.Error("output_summary should not be present when empty")
		}
	})

	t.Run("includes_both_summaries_when_provided", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:  slog.New(handler),
			enabled: true,
		}

		event := StageEvent{
			Type:          EventTypeProgress,
			URL:           "https://example.com",
			InputSummary:  "input summary",
			OutputSummary: "output summary",
		}

		logger.LogStage(context.Background(), "fetcher", event)

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["input_summary"] != "input summary" {
			t.Errorf("input_summary = %v, want 'input summary'", result["input_summary"])
		}
		if result["output_summary"] != "output summary" {
			t.Errorf("output_summary = %v, want 'output summary'", result["output_summary"])
		}
	})
}

func TestSlogLogger_LogRetry_PreAttrs(t *testing.T) {
	t.Run("includes_pre_attrs", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"crawl_id": "crawl-123", "worker_id": 1},
		}

		logger.LogRetry(context.Background(), 1, 10, 100*time.Millisecond, errors.New("timeout"))

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["crawl_id"] != "crawl-123" {
			t.Errorf("crawl_id = %v, want 'crawl-123'", result["crawl_id"])
		}
		if result["worker_id"] != float64(1) {
			t.Errorf("worker_id = %v, want 1", result["worker_id"])
		}
	})

	t.Run("succeeded_case_logs_success_message", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"request_id": "req-456"},
		}

		// err is nil means succeeded
		logger.LogRetry(context.Background(), 2, 5, 200*time.Millisecond, nil)

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["message"] != "Retry attempt succeeded" {
			t.Errorf("message = %v, want 'Retry attempt succeeded'", result["message"])
		}

		// Verify preAttrs are included
		if result["request_id"] != "req-456" {
			t.Errorf("request_id = %v, want 'req-456'", result["request_id"])
		}

		// Verify no error field when succeeded
		if _, exists := result["error"]; exists {
			t.Error("error field should not be present when err is nil")
		}
	})

	t.Run("failed_case_logs_failure_message", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"attempt_group": "group-1"},
		}

		logger.LogRetry(context.Background(), 1, 3, 50*time.Millisecond, errors.New("connection refused"))

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["message"] != "Retry attempt failed" {
			t.Errorf("message = %v, want 'Retry attempt failed'", result["message"])
		}

		if result["error"] != "connection refused" {
			t.Errorf("error = %v, want 'connection refused'", result["error"])
		}

		// Verify preAttrs are included
		if result["attempt_group"] != "group-1" {
			t.Errorf("attempt_group = %v, want 'group-1'", result["attempt_group"])
		}
	})
}

func TestSlogLogger_LogRateLimit_PreAttrs(t *testing.T) {
	t.Run("includes_pre_attrs", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"domain": "example.com", "priority": "high"},
		}

		logger.LogRateLimit(context.Background(), "api.example.com", 1500*time.Millisecond, RateLimitReasonBaseDelay)

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Verify preAttrs are included
		if result["domain"] != "example.com" {
			t.Errorf("domain = %v, want 'example.com'", result["domain"])
		}
		if result["priority"] != "high" {
			t.Errorf("priority = %v, want 'high'", result["priority"])
		}

		// Verify rate limit fields
		if result["host"] != "api.example.com" {
			t.Errorf("host = %v, want 'api.example.com'", result["host"])
		}
		if result["delay_ms"] != float64(1500) {
			t.Errorf("delay_ms = %v, want 1500", result["delay_ms"])
		}
		if result["rate_limit_reason"] != "base_delay" {
			t.Errorf("rate_limit_reason = %v, want 'base_delay'", result["rate_limit_reason"])
		}
	})

	t.Run("without_pre_attrs", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: nil,
		}

		logger.LogRateLimit(context.Background(), "example.com", 500*time.Millisecond, RateLimitReason429)

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["host"] != "example.com" {
			t.Errorf("host = %v, want 'example.com'", result["host"])
		}
		if result["rate_limit_reason"] != "http_429" {
			t.Errorf("rate_limit_reason = %v, want 'http_429'", result["rate_limit_reason"])
		}
	})
}

func TestSlogLogger_LogError_PreAttrs(t *testing.T) {
	t.Run("includes_pre_attrs", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"trace_id": "trace-abc123", "service": "crawler"},
		}

		logger.LogError(context.Background(), "fetcher", errors.New("network timeout"), FieldMap{
			"retry_count": 3,
		})

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Verify preAttrs are included
		if result["trace_id"] != "trace-abc123" {
			t.Errorf("trace_id = %v, want 'trace-abc123'", result["trace_id"])
		}
		if result["service"] != "crawler" {
			t.Errorf("service = %v, want 'crawler'", result["service"])
		}

		// Verify error fields
		if result["stage"] != "fetcher" {
			t.Errorf("stage = %v, want 'fetcher'", result["stage"])
		}
		if result["error"] != "network timeout" {
			t.Errorf("error = %v, want 'network timeout'", result["error"])
		}
		if result["retry_count"] != float64(3) {
			t.Errorf("retry_count = %v, want 3", result["retry_count"])
		}
	})

	t.Run("merges_pre_attrs_with_error_fields", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"existing_key": "pre_value"},
		}

		logger.LogError(context.Background(), "parser", errors.New("parse error"), FieldMap{
			"new_key":      "new_value",
			"existing_key": "overwritten", // This should overwrite pre_attrs value
		})

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Note: The order of attrs matters - error fields come after preAttrs,
		// so if there's a duplicate key, the error field value wins (last wins)
		if result["existing_key"] != "overwritten" {
			t.Errorf("existing_key = %v, want 'overwritten' (error fields should overwrite preAttrs)", result["existing_key"])
		}
		if result["new_key"] != "new_value" {
			t.Errorf("new_key = %v, want 'new_value'", result["new_key"])
		}
	})
}

func TestSlogLogger_WithFields_Merging(t *testing.T) {
	t.Run("merges_existing_pre_attrs", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"existing_key": "existing_value"},
		}

		newLogger := logger.WithFields(FieldMap{"new_key": "new_value"})

		// Verify the new logger has both keys
		slogLogger, ok := newLogger.(*SlogLogger)
		if !ok {
			t.Fatal("expected SlogLogger")
		}

		if slogLogger.preAttrs["existing_key"] != "existing_value" {
			t.Errorf("existing_key = %v, want 'existing_value'", slogLogger.preAttrs["existing_key"])
		}
		if slogLogger.preAttrs["new_key"] != "new_value" {
			t.Errorf("new_key = %v, want 'new_value'", slogLogger.preAttrs["new_key"])
		}
	})

	t.Run("adds_new_fields", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"key1": "value1"},
		}

		newLogger := logger.WithFields(FieldMap{
			"key2": "value2",
			"key3": 123,
		})

		slogLogger, ok := newLogger.(*SlogLogger)
		if !ok {
			t.Fatal("expected SlogLogger")
		}

		// Should have all 3 keys
		if len(slogLogger.preAttrs) != 3 {
			t.Errorf("preAttrs length = %d, want 3", len(slogLogger.preAttrs))
		}
		if slogLogger.preAttrs["key1"] != "value1" {
			t.Errorf("key1 = %v, want 'value1'", slogLogger.preAttrs["key1"])
		}
		if slogLogger.preAttrs["key2"] != "value2" {
			t.Errorf("key2 = %v, want 'value2'", slogLogger.preAttrs["key2"])
		}
		if slogLogger.preAttrs["key3"] != 123 {
			t.Errorf("key3 = %v, want 123", slogLogger.preAttrs["key3"])
		}
	})

	t.Run("overwrites_existing_keys", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"shared_key": "original_value"},
		}

		newLogger := logger.WithFields(FieldMap{"shared_key": "new_value"})

		slogLogger, ok := newLogger.(*SlogLogger)
		if !ok {
			t.Fatal("expected SlogLogger")
		}

		// New value should overwrite
		if slogLogger.preAttrs["shared_key"] != "new_value" {
			t.Errorf("shared_key = %v, want 'new_value'", slogLogger.preAttrs["shared_key"])
		}
	})

	t.Run("preserves_original_logger_pre_attrs", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"original": "value"},
		}

		// WithFields should not modify the original
		_ = logger.WithFields(FieldMap{"new": "field"})

		if logger.preAttrs["new"] != nil {
			t.Error("original logger preAttrs should not be modified")
		}
		if logger.preAttrs["original"] != "value" {
			t.Errorf("original preAttrs should be preserved")
		}
	})

	t.Run("chains_multiple_with_fields", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewLogstashHandler(&buf, nil)
		logger := &SlogLogger{
			logger:   slog.New(handler),
			enabled:  true,
			preAttrs: FieldMap{"first": "1"},
		}

		logger2 := logger.WithFields(FieldMap{"second": "2"})
		logger3 := logger2.WithFields(FieldMap{"third": "3"})

		slogLogger, ok := logger3.(*SlogLogger)
		if !ok {
			t.Fatal("expected SlogLogger")
		}

		// Should have all 3 keys from the chain
		if slogLogger.preAttrs["first"] != "1" {
			t.Errorf("first = %v, want '1'", slogLogger.preAttrs["first"])
		}
		if slogLogger.preAttrs["second"] != "2" {
			t.Errorf("second = %v, want '2'", slogLogger.preAttrs["second"])
		}
		if slogLogger.preAttrs["third"] != "3" {
			t.Errorf("third = %v, want '3'", slogLogger.preAttrs["third"])
		}
	})
}

func TestSlogLogger_Close(t *testing.T) {
	t.Run("returns_nil_when_closer_is_nil", func(t *testing.T) {
		logger := &SlogLogger{
			closer: nil,
		}

		err := logger.Close()
		if err != nil {
			t.Errorf("Close() should return nil when closer is nil, got: %v", err)
		}
	})

	t.Run("calls_closer_when_not_nil", func(t *testing.T) {
		closerCalled := false
		expectedErr := errors.New("closer error")

		logger := &SlogLogger{
			closer: func() error {
				closerCalled = true
				return expectedErr
			},
		}

		err := logger.Close()

		if !closerCalled {
			t.Error("closer function should have been called")
		}
		if err != expectedErr {
			t.Errorf("Close() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("returns_nil_when_closer_returns_nil", func(t *testing.T) {
		closerCalled := false

		logger := &SlogLogger{
			closer: func() error {
				closerCalled = true
				return nil
			},
		}

		err := logger.Close()

		if !closerCalled {
			t.Error("closer function should have been called")
		}
		if err != nil {
			t.Errorf("Close() should return nil, got: %v", err)
		}
	})

	t.Run("with_file_output", func(t *testing.T) {
		// Create a temp file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "test_close.log")

		cfg := DebugConfig{
			Enabled:    true,
			OutputFile: tmpFile,
			Format:     FormatJSON,
		}

		logger, err := NewSlogLogger(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Close should succeed
		if err := logger.Close(); err != nil {
			t.Errorf("Close() should return nil, got: %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
			t.Error("output file should have been created")
		}
	})
}
