package failurejournal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// FileSink persists failures to a JSON Lines file.
type FileSink struct {
	path    string
	mu      sync.Mutex
	records []FailureRecord
}

// NewFileSink creates a new FileSink that writes to the specified path.
func NewFileSink(path string) *FileSink {
	return &FileSink{
		path:    path,
		records: make([]FailureRecord, 0),
	}
}

// Record records a recoverable failure.
func (s *FileSink) Record(record FailureRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
}

// Flush writes any buffered records to persistent storage.
func (s *FileSink) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.records) == 0 {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Open file for append
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write each record as JSON line
	enc := json.NewEncoder(f)
	for _, r := range s.records {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}

	s.records = s.records[:0] // Clear buffer
	return nil
}

// Path returns the file path where failures are stored.
func (s *FileSink) Path() string {
	return s.path
}

// FileJournal provides a file-based implementation of Journal.
type FileJournal struct {
	*FileSink
}

// NewFileJournal creates a new FileJournal that persists to the specified path.
func NewFileJournal(path string) *FileJournal {
	return &FileJournal{
		FileSink: NewFileSink(path),
	}
}

// Read reads all failure records from storage.
func (s *FileSink) Read() ([]FailureRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []FailureRecord{}, nil
	}
	if err != nil {
		return nil, err
	}

	var records []FailureRecord
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var r FailureRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue // Skip malformed lines
		}
		records = append(records, r)
	}

	return records, nil
}

// Clear removes all recorded failures.
func (s *FileSink) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = s.records[:0]
	return os.Remove(s.path)
}

// Count returns the number of recorded failures.
func (s *FileSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

func splitLines(s string) []string {
	// Simple line split - could use bufio.Scanner for large files
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
