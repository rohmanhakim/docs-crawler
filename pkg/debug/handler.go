package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
)

// encodeJSON marshals a map to JSON bytes.
func encodeJSON(m map[string]any) ([]byte, error) {
	return json.Marshal(m)
}

// LogstashHandler is a custom slog.Handler that outputs logs in Logstash/Elasticsearch
// compatible JSON format as specified in the design document.
type LogstashHandler struct {
	mu            sync.Mutex
	w             io.Writer
	level         slog.Level
	attrs         []slog.Attr
	groups        []string
	includeFields []string
	excludeFields []string
}

// NewLogstashHandler creates a new LogstashHandler writing to the specified writer.
func NewLogstashHandler(w io.Writer, opts *LogstashHandlerOptions) *LogstashHandler {
	if opts == nil {
		opts = &LogstashHandlerOptions{}
	}

	level := opts.Level
	if level == 0 {
		level = slog.LevelDebug
	}

	return &LogstashHandler{
		w:             w,
		level:         level,
		includeFields: opts.IncludeFields,
		excludeFields: opts.ExcludeFields,
	}
}

// LogstashHandlerOptions configures the LogstashHandler.
type LogstashHandlerOptions struct {
	// Level is the minimum log level to output.
	Level slog.Level

	// IncludeFields filters fields to include (empty = all).
	IncludeFields []string

	// ExcludeFields filters fields to exclude.
	ExcludeFields []string
}

// Enabled returns true if the handler should log at the given level.
func (h *LogstashHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle processes the log record and writes it in Logstash JSON format.
func (h *LogstashHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Build the log entry following the design document structure
	entry := map[string]any{
		"@timestamp":  r.Time.Format(time.RFC3339Nano),
		"@version":    "1",
		"level":       r.Level.String(),
		"logger_name": "docs-crawler",
		"message":     r.Message,
		"thread_name": "main",
	}

	// Add attrs from handler context
	for _, attr := range h.attrs {
		h.addField(entry, attr.Key, attr.Value)
	}

	// Add attrs from the record
	r.Attrs(func(attr slog.Attr) bool {
		h.addField(entry, attr.Key, attr.Value)
		return true
	})

	// Apply field filtering
	entry = h.filterFields(entry)

	// Write JSON line
	jsonData, err := encodeJSON(entry)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(h.w, string(jsonData))
	return err
}

// WithAttrs returns a new handler with the given attributes added.
func (h *LogstashHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	newHandler := h.clone()
	newHandler.attrs = append(newHandler.attrs, attrs...)
	return newHandler
}

// WithGroup returns a new handler with the given group name prepended to attribute keys.
func (h *LogstashHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	newHandler := h.clone()
	newHandler.groups = append(newHandler.groups, name)
	return newHandler
}

// clone creates a copy of the handler.
func (h *LogstashHandler) clone() *LogstashHandler {
	return &LogstashHandler{
		w:             h.w,
		level:         h.level,
		attrs:         slices.Clone(h.attrs),
		groups:        slices.Clone(h.groups),
		includeFields: h.includeFields,
		excludeFields: h.excludeFields,
	}
}

// addField adds a field to the entry map, respecting group prefixes.
func (h *LogstashHandler) addField(entry map[string]any, key string, value slog.Value) {
	// Build the full key with group prefix
	fullKey := key
	if len(h.groups) > 0 {
		fullKey = strings.Join(h.groups, ".") + "." + key
	}

	// Handle different value kinds
	switch value.Kind() {
	case slog.KindGroup:
		// For group values, recursively add fields
		groupAttrs := value.Group()
		for _, attr := range groupAttrs {
			h.addField(entry, fullKey+"."+attr.Key, attr.Value)
		}
	case slog.KindLogValuer:
		entry[fullKey] = value.Resolve()
	default:
		entry[fullKey] = value.Any()
	}
}

// filterFields applies include/exclude field filtering.
func (h *LogstashHandler) filterFields(entry map[string]any) map[string]any {
	if len(h.includeFields) == 0 && len(h.excludeFields) == 0 {
		return entry
	}

	result := make(map[string]any)
	for key, value := range entry {
		// Check exclude list first
		if slices.Contains(h.excludeFields, key) {
			continue
		}

		// Check include list (if specified)
		if len(h.includeFields) > 0 && !slices.Contains(h.includeFields, key) {
			continue
		}

		result[key] = value
	}

	return result
}

// TextHandler creates a human-readable text output.
type TextHandler struct {
	mu    sync.Mutex
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
}

// NewTextHandler creates a new TextHandler writing to the specified writer.
func NewTextHandler(w io.Writer, opts *TextHandlerOptions) *TextHandler {
	if opts == nil {
		opts = &TextHandlerOptions{}
	}

	level := opts.Level
	if level == 0 {
		level = slog.LevelDebug
	}

	return &TextHandler{
		w:     w,
		level: level,
	}
}

// TextHandlerOptions configures the TextHandler.
type TextHandlerOptions struct {
	// Level is the minimum log level to output.
	Level slog.Level
}

// Enabled returns true if the handler should log at the given level.
func (h *TextHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle processes the log record and writes it in human-readable text format.
func (h *TextHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Format: 2026-02-21T13:50:00.123Z [DEBUG] [stage] [event_type] key=value key2=value2
	var sb strings.Builder

	// Timestamp
	sb.WriteString(r.Time.Format("2006-01-02T15:04:05.000Z"))

	// Level
	sb.WriteString(" [")
	sb.WriteString(r.Level.String())
	sb.WriteString("]")

	// Collect fields for output
	fields := make([]string, 0)

	// Add attrs from handler context
	for _, attr := range h.attrs {
		fields = append(fields, formatField(attr.Key, attr.Value))
	}

	// Add attrs from the record
	r.Attrs(func(attr slog.Attr) bool {
		fields = append(fields, formatField(attr.Key, attr.Value))
		return true
	})

	// Write fields
	if len(fields) > 0 {
		sb.WriteString(" ")
		sb.WriteString(strings.Join(fields, " "))
	}

	sb.WriteString("\n")

	_, err := h.w.Write([]byte(sb.String()))
	return err
}

// WithAttrs returns a new handler with the given attributes added.
func (h *TextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	newHandler := &TextHandler{
		w:     h.w,
		level: h.level,
		attrs: slices.Clone(h.attrs),
	}
	newHandler.attrs = append(newHandler.attrs, attrs...)
	return newHandler
}

// WithGroup returns a new handler with the given group name.
// Text handler doesn't support grouping, so it's a no-op.
func (h *TextHandler) WithGroup(name string) slog.Handler {
	return h
}

// formatField formats a key-value pair for text output.
func formatField(key string, value slog.Value) string {
	switch value.Kind() {
	case slog.KindGroup:
		// For groups, format as nested key=value
		parts := make([]string, 0)
		for _, attr := range value.Group() {
			parts = append(parts, formatField(attr.Key, attr.Value))
		}
		return fmt.Sprintf("%s={%s}", key, strings.Join(parts, " "))
	default:
		return fmt.Sprintf("%s=%v", key, value.Any())
	}
}
