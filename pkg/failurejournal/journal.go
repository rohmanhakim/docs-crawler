package failurejournal

import "sync"

// Sink defines the interface for recording failures.
type Sink interface {
	// Record records a recoverable failure.
	Record(record FailureRecord)

	// Flush writes any buffered records to persistent storage.
	Flush() error

	// Path returns the file path where failures are stored.
	Path() string
}

// Journal defines the interface for managing the failure journal.
// It extends Sink with read and management capabilities.
type Journal interface {
	Sink

	// Read reads all failure records from storage.
	Read() ([]FailureRecord, error)

	// Clear removes all recorded failures.
	Clear() error

	// Count returns the number of recorded failures.
	Count() int
}

// InMemoryJournal provides an in-memory implementation of Journal.
// This is useful for testing and for cases where persistence is not needed.
type InMemoryJournal struct {
	mu      sync.RWMutex
	records []FailureRecord
	path    string
}

// NewInMemoryJournal creates a new in-memory journal.
func NewInMemoryJournal() *InMemoryJournal {
	return &InMemoryJournal{
		records: make([]FailureRecord, 0),
	}
}

// Record records a recoverable failure.
func (j *InMemoryJournal) Record(record FailureRecord) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.records = append(j.records, record)
}

// Flush is a no-op for in-memory journal.
func (j *InMemoryJournal) Flush() error {
	// No-op for in-memory journal
	return nil
}

// Path returns an empty string for in-memory journal.
func (j *InMemoryJournal) Path() string {
	return ""
}

// Read reads all failure records.
func (j *InMemoryJournal) Read() ([]FailureRecord, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	// Return a copy to prevent external mutation
	records := make([]FailureRecord, len(j.records))
	copy(records, j.records)
	return records, nil
}

// Clear removes all recorded failures.
func (j *InMemoryJournal) Clear() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.records = j.records[:0]
	return nil
}

// Count returns the number of recorded failures.
func (j *InMemoryJournal) Count() int {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return len(j.records)
}
