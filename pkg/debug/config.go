package debug

import (
	"fmt"

	"github.com/rohmanhakim/dlog"
)

// Format represents the output format for debug logging.
type Format string

const (
	// FormatJSON outputs logs in JSON format (Logstash/Elasticsearch compatible).
	FormatJSON Format = "json"
	// FormatText outputs logs in human-readable text format.
	FormatText Format = "text"
	// FormatLogfmt outputs logs in logfmt format (key=value pairs).
	FormatLogfmt Format = "logfmt"
	// FormatLogstash outputs logs in Logstash/Elasticsearch compatible JSON format.
	FormatLogstash Format = "logstash"
)

// DebugConfig holds configuration for debug logging.
type DebugConfig struct {
	// Enabled controls whether debug logging is active.
	Enabled bool

	// OutputFile is the path to write debug logs.
	// Empty means stdout only.
	OutputFile string

	// Format controls output format: "json", "text", "logfmt", or "logstash".
	Format Format

	// IncludeFields filters fields to include (empty = all).
	IncludeFields []string

	// ExcludeFields filters fields to exclude.
	ExcludeFields []string
}

// NewDebugConfig creates a DebugConfig from CLI flags.
func NewDebugConfig(enabled bool, outputFile string, format string) (DebugConfig, error) {
	f, err := parseFormat(format)
	if err != nil {
		return DebugConfig{}, err
	}

	return DebugConfig{
		Enabled:       enabled,
		OutputFile:    outputFile,
		Format:        f,
		IncludeFields: []string{},
		ExcludeFields: []string{},
	}, nil
}

// parseFormat parses a format string and returns the corresponding Format.
func parseFormat(format string) (Format, error) {
	if format == "" {
		return FormatLogstash, nil
	}

	switch Format(format) {
	case FormatJSON, FormatText, FormatLogfmt, FormatLogstash:
		return Format(format), nil
	default:
		return "", fmt.Errorf("invalid debug format: %s (valid: json, text, logfmt, logstash)", format)
	}
}

// IsFileOutput returns true if file output is configured.
func (c DebugConfig) IsFileOutput() bool {
	return c.OutputFile != ""
}

// NewLogger creates a DebugLogger from the configuration.
// It uses dlog internally and wraps it with DomainLogger for domain-specific methods.
func (c DebugConfig) NewLogger() (DebugLogger, error) {
	// Map our format to dlog format
	var dlogFormat dlog.Format
	switch c.Format {
	case FormatJSON:
		dlogFormat = dlog.FormatJSON
	case FormatText:
		dlogFormat = dlog.FormatText
	case FormatLogfmt:
		dlogFormat = dlog.FormatLogfmt
	case FormatLogstash:
		dlogFormat = dlog.FormatLogstash
	default:
		dlogFormat = dlog.FormatLogstash
	}

	// Build options
	var opts []dlog.Option
	if c.OutputFile != "" {
		opts = append(opts, dlog.WithOutputFile(c.OutputFile))
	}
	if len(c.IncludeFields) > 0 {
		opts = append(opts, dlog.WithIncludeFields(c.IncludeFields))
	}
	if len(c.ExcludeFields) > 0 {
		opts = append(opts, dlog.WithExcludeFields(c.ExcludeFields))
	}

	// Create dlog logger
	dlogLogger, err := dlog.NewSlogLogger(c.Enabled, dlogFormat, opts...)
	if err != nil {
		return nil, err
	}

	// Wrap with DomainLogger for domain-specific methods
	return NewDomainLogger(dlogLogger), nil
}

// NewSlogLogger creates a DebugLogger from a DebugConfig.
// This is a convenience function equivalent to config.NewLogger().
// It is provided for backward compatibility.
func NewSlogLogger(config DebugConfig) (DebugLogger, error) {
	return config.NewLogger()
}
