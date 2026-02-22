package debug_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/debug"
)

// TestNewLogstashHandler tests the NewLogstashHandler constructor
func TestNewLogstashHandler(t *testing.T) {
	tests := []struct {
		name      string
		opts      *debug.LogstashHandlerOptions
		wantLevel slog.Level
	}{
		{
			name:      "Nil options uses defaults",
			opts:      nil,
			wantLevel: slog.LevelDebug,
		},
		{
			name:      "Empty options uses defaults",
			opts:      &debug.LogstashHandlerOptions{},
			wantLevel: slog.LevelDebug,
		},
		{
			name: "Custom level warn",
			opts: &debug.LogstashHandlerOptions{
				Level: slog.LevelWarn,
			},
			wantLevel: slog.LevelWarn,
		},
		{
			name: "With include fields",
			opts: &debug.LogstashHandlerOptions{
				IncludeFields: []string{"@timestamp", "level", "message"},
			},
			wantLevel: slog.LevelDebug,
		},
		{
			name: "With exclude fields",
			opts: &debug.LogstashHandlerOptions{
				ExcludeFields: []string{"thread_name"},
			},
			wantLevel: slog.LevelDebug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := debug.NewLogstashHandler(&buf, tt.opts)

			if handler == nil {
				t.Fatal("Expected handler to be non-nil")
			}

			// Verify level by checking Enabled
			ctx := context.Background()
			if !handler.Enabled(ctx, tt.wantLevel) {
				t.Errorf("Expected handler to be enabled for level %v", tt.wantLevel)
			}
		})
	}
}

// TestLogstashHandlerEnabled tests the Enabled method
func TestLogstashHandlerEnabled(t *testing.T) {
	tests := []struct {
		name       string
		level      slog.Level
		checkLevel slog.Level
		want       bool
	}{
		{
			name:       "Debug level enabled for debug",
			level:      slog.LevelDebug,
			checkLevel: slog.LevelDebug,
			want:       true,
		},
		{
			name:       "Warn level disabled for debug",
			level:      slog.LevelWarn,
			checkLevel: slog.LevelDebug,
			want:       false,
		},
		{
			name:       "Warn level enabled for warn",
			level:      slog.LevelWarn,
			checkLevel: slog.LevelWarn,
			want:       true,
		},
		{
			name:       "Warn level enabled for error",
			level:      slog.LevelWarn,
			checkLevel: slog.LevelError,
			want:       true,
		},
		{
			name:       "Error level disabled for warn",
			level:      slog.LevelError,
			checkLevel: slog.LevelWarn,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := debug.NewLogstashHandler(&buf, &debug.LogstashHandlerOptions{
				Level: tt.level,
			})

			ctx := context.Background()
			got := handler.Enabled(ctx, tt.checkLevel)
			if got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLogstashHandlerHandle tests the Handle method
func TestLogstashHandlerHandle(t *testing.T) {
	t.Run("Basic log entry", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, nil)

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test message",
		}

		err := handler.Handle(context.Background(), record)
		if err != nil {
			t.Fatalf("Handle() error = %v", err)
		}

		// Verify JSON output
		var entry map[string]any
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Verify required fields
		if entry["@timestamp"] != "2026-02-22T13:00:00Z" {
			t.Errorf("Expected @timestamp '2026-02-22T13:00:00Z', got %v", entry["@timestamp"])
		}
		if entry["@version"] != "1" {
			t.Errorf("Expected @version '1', got %v", entry["@version"])
		}
		if entry["level"] != "INFO" {
			t.Errorf("Expected level 'INFO', got %v", entry["level"])
		}
		if entry["logger_name"] != "docs-crawler" {
			t.Errorf("Expected logger_name 'docs-crawler', got %v", entry["logger_name"])
		}
		if entry["message"] != "test message" {
			t.Errorf("Expected message 'test message', got %v", entry["message"])
		}
		if entry["thread_name"] != "main" {
			t.Errorf("Expected thread_name 'main', got %v", entry["thread_name"])
		}
	})

	t.Run("Log entry with attributes", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, nil)

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelDebug,
			Message: "test with attrs",
		}
		record.AddAttrs(
			slog.String("key1", "value1"),
			slog.Int("count", 42),
		)

		err := handler.Handle(context.Background(), record)
		if err != nil {
			t.Fatalf("Handle() error = %v", err)
		}

		var entry map[string]any
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		if entry["key1"] != "value1" {
			t.Errorf("Expected key1 'value1', got %v", entry["key1"])
		}
		if entry["count"] != float64(42) {
			t.Errorf("Expected count 42, got %v", entry["count"])
		}
	})
}

// TestLogstashHandlerWithAttrs tests the WithAttrs method
func TestLogstashHandlerWithAttrs(t *testing.T) {
	t.Run("With attrs returns new handler", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, nil)

		newHandler := handler.WithAttrs([]slog.Attr{
			slog.String("context", "test"),
		})

		if newHandler == nil {
			t.Fatal("WithAttrs() returned nil")
		}

		// Verify original handler is unchanged
		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "original handler",
		}
		handler.Handle(context.Background(), record)

		var entry map[string]any
		json.Unmarshal(buf.Bytes(), &entry)
		if _, exists := entry["context"]; exists {
			t.Error("Original handler should not have context attribute")
		}

		// Verify new handler has the attribute
		buf.Reset()
		newHandler.Handle(context.Background(), record)
		json.Unmarshal(buf.Bytes(), &entry)
		if entry["context"] != "test" {
			t.Errorf("Expected context 'test', got %v", entry["context"])
		}
	})

	t.Run("Empty attrs returns same handler", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, nil)

		newHandler := handler.WithAttrs([]slog.Attr{})
		if newHandler != handler {
			t.Error("WithAttrs with empty slice should return same handler")
		}
	})

	t.Run("Chained WithAttrs accumulates attributes", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, nil)

		handler = handler.WithAttrs([]slog.Attr{slog.String("a", "1")}).(*debug.LogstashHandler)
		handler = handler.WithAttrs([]slog.Attr{slog.String("b", "2")}).(*debug.LogstashHandler)

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test",
		}
		handler.Handle(context.Background(), record)

		var entry map[string]any
		json.Unmarshal(buf.Bytes(), &entry)
		if entry["a"] != "1" || entry["b"] != "2" {
			t.Errorf("Expected both a and b attributes, got %v", entry)
		}
	})
}

// TestLogstashHandlerWithGroup tests the WithGroup method
func TestLogstashHandlerWithGroup(t *testing.T) {
	t.Run("With group prefixes keys", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, nil)

		groupedHandler := handler.WithGroup("request").WithAttrs([]slog.Attr{
			slog.String("url", "https://example.com"),
		})

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test",
		}
		groupedHandler.Handle(context.Background(), record)

		var entry map[string]any
		json.Unmarshal(buf.Bytes(), &entry)
		if entry["request.url"] != "https://example.com" {
			t.Errorf("Expected request.url, got %v", entry)
		}
	})

	t.Run("Empty group name returns same handler", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, nil)

		newHandler := handler.WithGroup("")
		if newHandler != handler {
			t.Error("WithGroup with empty name should return same handler")
		}
	})

	t.Run("Nested groups", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, nil)

		handler = handler.WithGroup("http").(*debug.LogstashHandler)
		handler = handler.WithGroup("request").(*debug.LogstashHandler)
		handler = handler.WithAttrs([]slog.Attr{slog.String("method", "GET")}).(*debug.LogstashHandler)

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test",
		}
		handler.Handle(context.Background(), record)

		var entry map[string]any
		json.Unmarshal(buf.Bytes(), &entry)
		if entry["http.request.method"] != "GET" {
			t.Errorf("Expected http.request.method, got %v", entry)
		}
	})
}

// TestLogstashHandlerFieldFiltering tests include/exclude field filtering
func TestLogstashHandlerFieldFiltering(t *testing.T) {
	t.Run("Include fields filters to specified fields", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, &debug.LogstashHandlerOptions{
			IncludeFields: []string{"@timestamp", "level", "message"},
		})

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test",
		}
		record.AddAttrs(slog.String("extra", "value"))

		handler.Handle(context.Background(), record)

		var entry map[string]any
		json.Unmarshal(buf.Bytes(), &entry)

		// Included fields should be present
		if _, exists := entry["@timestamp"]; !exists {
			t.Error("@timestamp should be included")
		}
		if _, exists := entry["level"]; !exists {
			t.Error("level should be included")
		}
		if _, exists := entry["message"]; !exists {
			t.Error("message should be included")
		}

		// Non-included fields should be excluded
		if _, exists := entry["@version"]; exists {
			t.Error("@version should be excluded")
		}
		if _, exists := entry["extra"]; exists {
			t.Error("extra should be excluded")
		}
	})

	t.Run("Exclude fields removes specified fields", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, &debug.LogstashHandlerOptions{
			ExcludeFields: []string{"thread_name", "logger_name"},
		})

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test",
		}

		handler.Handle(context.Background(), record)

		var entry map[string]any
		json.Unmarshal(buf.Bytes(), &entry)

		// Excluded fields should not be present
		if _, exists := entry["thread_name"]; exists {
			t.Error("thread_name should be excluded")
		}
		if _, exists := entry["logger_name"]; exists {
			t.Error("logger_name should be excluded")
		}

		// Other fields should be present
		if _, exists := entry["@timestamp"]; !exists {
			t.Error("@timestamp should be present")
		}
	})

	t.Run("Exclude takes precedence over include", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewLogstashHandler(&buf, &debug.LogstashHandlerOptions{
			IncludeFields: []string{"@timestamp", "level", "sensitive"},
			ExcludeFields: []string{"sensitive"},
		})

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test",
		}
		record.AddAttrs(slog.String("sensitive", "secret"))

		handler.Handle(context.Background(), record)

		var entry map[string]any
		json.Unmarshal(buf.Bytes(), &entry)

		// Sensitive should be excluded even if in include list
		if _, exists := entry["sensitive"]; exists {
			t.Error("sensitive should be excluded")
		}
	})
}

// TestLogstashHandlerGroupValues tests handling of group values
func TestLogstashHandlerGroupValues(t *testing.T) {
	var buf bytes.Buffer
	handler := debug.NewLogstashHandler(&buf, nil)

	record := slog.Record{
		Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
		Level:   slog.LevelInfo,
		Message: "test",
	}
	record.AddAttrs(slog.Group("http",
		slog.String("method", "GET"),
		slog.Int("status", 200),
	))

	handler.Handle(context.Background(), record)

	var entry map[string]any
	json.Unmarshal(buf.Bytes(), &entry)

	if entry["http.method"] != "GET" {
		t.Errorf("Expected http.method 'GET', got %v", entry["http.method"])
	}
	if entry["http.status"] != float64(200) {
		t.Errorf("Expected http.status 200, got %v", entry["http.status"])
	}
}

// TestNewTextHandler tests the NewTextHandler constructor
func TestNewTextHandler(t *testing.T) {
	tests := []struct {
		name      string
		opts      *debug.TextHandlerOptions
		wantLevel slog.Level
	}{
		{
			name:      "Nil options uses defaults",
			opts:      nil,
			wantLevel: slog.LevelDebug,
		},
		{
			name:      "Empty options uses defaults",
			opts:      &debug.TextHandlerOptions{},
			wantLevel: slog.LevelDebug,
		},
		{
			name: "Custom level",
			opts: &debug.TextHandlerOptions{
				Level: slog.LevelWarn,
			},
			wantLevel: slog.LevelWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := debug.NewTextHandler(&buf, tt.opts)

			if handler == nil {
				t.Fatal("Expected handler to be non-nil")
			}

			ctx := context.Background()
			if !handler.Enabled(ctx, tt.wantLevel) {
				t.Errorf("Expected handler to be enabled for level %v", tt.wantLevel)
			}
		})
	}
}

// TestTextHandlerEnabled tests the Enabled method
func TestTextHandlerEnabled(t *testing.T) {
	tests := []struct {
		name       string
		level      slog.Level
		checkLevel slog.Level
		want       bool
	}{
		{
			name:       "Debug level enabled for debug",
			level:      slog.LevelDebug,
			checkLevel: slog.LevelDebug,
			want:       true,
		},
		{
			name:       "Warn level disabled for debug",
			level:      slog.LevelWarn,
			checkLevel: slog.LevelDebug,
			want:       false,
		},
		{
			name:       "Warn level enabled for error",
			level:      slog.LevelWarn,
			checkLevel: slog.LevelError,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := debug.NewTextHandler(&buf, &debug.TextHandlerOptions{
				Level: tt.level,
			})

			ctx := context.Background()
			got := handler.Enabled(ctx, tt.checkLevel)
			if got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTextHandlerHandle tests the Handle method
func TestTextHandlerHandle(t *testing.T) {
	t.Run("Basic text output", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewTextHandler(&buf, nil)

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test message",
		}

		err := handler.Handle(context.Background(), record)
		if err != nil {
			t.Fatalf("Handle() error = %v", err)
		}

		output := buf.String()
		// Verify format: 2026-02-22T13:00:00.000Z [INFO]
		if !strings.Contains(output, "2026-02-22T13:00:00.000Z") {
			t.Errorf("Expected timestamp in output, got %s", output)
		}
		if !strings.Contains(output, "[INFO]") {
			t.Errorf("Expected [INFO] in output, got %s", output)
		}
	})

	t.Run("Text output with attributes", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewTextHandler(&buf, nil)

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelDebug,
			Message: "test",
		}
		record.AddAttrs(
			slog.String("url", "https://example.com"),
			slog.Int("count", 5),
		)

		err := handler.Handle(context.Background(), record)
		if err != nil {
			t.Fatalf("Handle() error = %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "url=https://example.com") {
			t.Errorf("Expected url attribute in output, got %s", output)
		}
		if !strings.Contains(output, "count=5") {
			t.Errorf("Expected count attribute in output, got %s", output)
		}
	})

	t.Run("Text output with group attributes", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewTextHandler(&buf, nil)

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test",
		}
		record.AddAttrs(slog.Group("http",
			slog.String("method", "GET"),
			slog.Int("status", 200),
		))

		err := handler.Handle(context.Background(), record)
		if err != nil {
			t.Fatalf("Handle() error = %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "http={method=GET status=200}") {
			t.Errorf("Expected grouped attributes in output, got %s", output)
		}
	})
}

// TestTextHandlerWithAttrs tests the WithAttrs method
func TestTextHandlerWithAttrs(t *testing.T) {
	t.Run("With attrs returns new handler", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewTextHandler(&buf, nil)

		newHandler := handler.WithAttrs([]slog.Attr{
			slog.String("service", "crawler"),
		})

		if newHandler == nil {
			t.Fatal("WithAttrs() returned nil")
		}

		// Verify new handler has the attribute
		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test",
		}
		newHandler.Handle(context.Background(), record)

		output := buf.String()
		if !strings.Contains(output, "service=crawler") {
			t.Errorf("Expected service attribute in output, got %s", output)
		}
	})

	t.Run("Empty attrs returns same handler", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewTextHandler(&buf, nil)

		newHandler := handler.WithAttrs([]slog.Attr{})
		if newHandler != handler {
			t.Error("WithAttrs with empty slice should return same handler")
		}
	})

	t.Run("Chained WithAttrs accumulates attributes", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewTextHandler(&buf, nil)

		handler = handler.WithAttrs([]slog.Attr{slog.String("a", "1")}).(*debug.TextHandler)
		handler = handler.WithAttrs([]slog.Attr{slog.String("b", "2")}).(*debug.TextHandler)

		record := slog.Record{
			Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
			Level:   slog.LevelInfo,
			Message: "test",
		}
		handler.Handle(context.Background(), record)

		output := buf.String()
		if !strings.Contains(output, "a=1") || !strings.Contains(output, "b=2") {
			t.Errorf("Expected both a and b attributes in output, got %s", output)
		}
	})
}

// TestTextHandlerWithGroup tests the WithGroup method
func TestTextHandlerWithGroup(t *testing.T) {
	t.Run("WithGroup returns same handler (no-op)", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewTextHandler(&buf, nil)

		newHandler := handler.WithGroup("ignored")
		if newHandler != handler {
			t.Error("TextHandler.WithGroup should return same handler (no-op)")
		}
	})

	t.Run("WithGroup with empty name returns same handler", func(t *testing.T) {
		var buf bytes.Buffer
		handler := debug.NewTextHandler(&buf, nil)

		newHandler := handler.WithGroup("")
		if newHandler != handler {
			t.Error("WithGroup with empty name should return same handler")
		}
	})
}

// TestTextHandlerLevel tests different log levels
func TestTextHandlerLevel(t *testing.T) {
	tests := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelDebug, "[DEBUG]"},
		{slog.LevelInfo, "[INFO]"},
		{slog.LevelWarn, "[WARN]"},
		{slog.LevelError, "[ERROR]"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			var buf bytes.Buffer
			handler := debug.NewTextHandler(&buf, nil)

			record := slog.Record{
				Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
				Level:   tt.level,
				Message: "test",
			}
			handler.Handle(context.Background(), record)

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected %s in output, got %s", tt.expected, output)
			}
		})
	}
}

// TestLogstashHandlerSlogIntegration tests that LogstashHandler implements slog.Handler
func TestLogstashHandlerSlogIntegration(t *testing.T) {
	var buf bytes.Buffer
	handler := debug.NewLogstashHandler(&buf, &debug.LogstashHandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(handler)
	logger.Info("test message", "key", "value", "count", 123)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if entry["message"] != "test message" {
		t.Errorf("Expected message 'test message', got %v", entry["message"])
	}
	if entry["key"] != "value" {
		t.Errorf("Expected key 'value', got %v", entry["key"])
	}
	if entry["count"] != float64(123) {
		t.Errorf("Expected count 123, got %v", entry["count"])
	}
}

// TestTextHandlerSlogIntegration tests that TextHandler implements slog.Handler
func TestTextHandlerSlogIntegration(t *testing.T) {
	var buf bytes.Buffer
	handler := debug.NewTextHandler(&buf, &debug.TextHandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(handler)
	logger.Info("test message", "url", "https://example.com", "status", 200)

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("Expected [INFO] in output, got %s", output)
	}
	if !strings.Contains(output, "url=https://example.com") {
		t.Errorf("Expected url attribute in output, got %s", output)
	}
	if !strings.Contains(output, "status=200") {
		t.Errorf("Expected status attribute in output, got %s", output)
	}
}

// TestLogstashHandlerWithAttrsAndRecordAttrs tests that handler attrs and record attrs are both included
func TestLogstashHandlerWithAttrsAndRecordAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := debug.NewLogstashHandler(&buf, nil)
	handler = handler.WithAttrs([]slog.Attr{slog.String("handler_attr", "from_handler")}).(*debug.LogstashHandler)

	record := slog.Record{
		Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
		Level:   slog.LevelInfo,
		Message: "test",
	}
	record.AddAttrs(slog.String("record_attr", "from_record"))

	handler.Handle(context.Background(), record)

	var entry map[string]any
	json.Unmarshal(buf.Bytes(), &entry)

	if entry["handler_attr"] != "from_handler" {
		t.Errorf("Expected handler_attr, got %v", entry)
	}
	if entry["record_attr"] != "from_record" {
		t.Errorf("Expected record_attr, got %v", entry)
	}
}

// TestTextHandlerWithAttrsAndRecordAttrs tests that handler attrs and record attrs are both included
func TestTextHandlerWithAttrsAndRecordAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := debug.NewTextHandler(&buf, nil)
	handler = handler.WithAttrs([]slog.Attr{slog.String("handler_attr", "from_handler")}).(*debug.TextHandler)

	record := slog.Record{
		Time:    time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC),
		Level:   slog.LevelInfo,
		Message: "test",
	}
	record.AddAttrs(slog.String("record_attr", "from_record"))

	handler.Handle(context.Background(), record)

	output := buf.String()
	if !strings.Contains(output, "handler_attr=from_handler") {
		t.Errorf("Expected handler_attr in output, got %s", output)
	}
	if !strings.Contains(output, "record_attr=from_record") {
		t.Errorf("Expected record_attr in output, got %s", output)
	}
}
