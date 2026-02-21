package debug

import "fmt"

// Format represents the output format for debug logging.
type Format string

const (
	// FormatJSON outputs logs in JSON format (Logstash/Elasticsearch compatible).
	FormatJSON Format = "json"
	// FormatText outputs logs in human-readable text format.
	FormatText Format = "text"
)

// DebugConfig holds configuration for debug logging.
type DebugConfig struct {
	// Enabled controls whether debug logging is active.
	Enabled bool

	// OutputFile is the path to write debug logs.
	// Empty means stdout only.
	OutputFile string

	// Format controls output format: "json" or "text".
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
		return FormatJSON, nil
	}

	switch Format(format) {
	case FormatJSON, FormatText:
		return Format(format), nil
	default:
		return "", fmt.Errorf("invalid debug format: %s (valid: json, text)", format)
	}
}

// IsFileOutput returns true if file output is configured.
func (c DebugConfig) IsFileOutput() bool {
	return c.OutputFile != ""
}
