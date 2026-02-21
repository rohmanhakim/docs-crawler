package debug

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
)

// MultiWriter writes to multiple outputs simultaneously.
// It supports writing to stdout and/or a file.
type MultiWriter struct {
	mu     sync.Mutex
	writer io.Writer
	file   *os.File
}

// NewMultiWriter creates a writer that outputs to stdout and/or file.
// If outputFile is empty, only stdout is used.
func NewMultiWriter(outputFile string) (*MultiWriter, error) {
	var writers []io.Writer
	var file *os.File

	// Always include stdout
	writers = append(writers, os.Stdout)

	// Optionally include file
	if outputFile != "" {
		f, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open debug log file: %w", err)
		}
		file = f
		// Use buffered writer for file output
		writers = append(writers, &fileOutput{
			file:   f,
			writer: bufio.NewWriter(f),
		})
	}

	return &MultiWriter{
		writer: io.MultiWriter(writers...),
		file:   file,
	}, nil
}

// Write implements io.Writer.
func (m *MultiWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writer.Write(p)
}

// Close closes the file handle if one was opened.
func (m *MultiWriter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.file != nil {
		return m.file.Close()
	}
	return nil
}

// fileOutput wraps a file with buffered writing.
type fileOutput struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
}

// Write writes to the buffered file writer and flushes.
func (f *fileOutput) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	n, err = f.writer.Write(p)
	if err != nil {
		return n, err
	}

	// Flush after each log entry to ensure durability
	if err := f.writer.Flush(); err != nil {
		return n, err
	}

	return n, nil
}
