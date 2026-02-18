package scheduler_test

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/failurejournal"
	"github.com/stretchr/testify/mock"
)

// newFileJournalForTest creates a real FileJournal at the given path for use in tests
// that need a concrete journal implementation (not a mock) with a deterministic Path().
func newFileJournalForTest(path string) *failurejournal.FileJournal {
	return failurejournal.NewFileJournal(path)
}

// failureJournalMock is a testify mock for the failurejournal.Journal interface
type failureJournalMock struct {
	mock.Mock
}

// Record mocks the Record method
func (f *failureJournalMock) Record(record failurejournal.FailureRecord) {
	f.Called(record)
}

// Flush mocks the Flush method
func (f *failureJournalMock) Flush() error {
	args := f.Called()
	return args.Error(0)
}

// Path mocks the Path method
func (f *failureJournalMock) Path() string {
	args := f.Called()
	return args.String(0)
}

// Read mocks the Read method
func (f *failureJournalMock) Read() ([]failurejournal.FailureRecord, error) {
	args := f.Called()
	return args.Get(0).([]failurejournal.FailureRecord), args.Error(1)
}

// Clear mocks the Clear method
func (f *failureJournalMock) Clear() error {
	args := f.Called()
	return args.Error(0)
}

// Count mocks the Count method
func (f *failureJournalMock) Count() int {
	args := f.Called()
	return args.Int(0)
}

// newFailureJournalMockForTest creates a properly configured failure journal mock for tests
func newFailureJournalMockForTest(t *testing.T) *failureJournalMock {
	t.Helper()
	m := new(failureJournalMock)
	// Set up default expectations
	// Record can be called multiple times - use Maybe() to allow any call
	m.On("Record", mock.Anything).Return().Maybe()
	m.On("Flush").Return(nil)
	m.On("Path").Return("")
	m.On("Read").Return([]failurejournal.FailureRecord{}, nil)
	m.On("Clear").Return(nil)
	m.On("Count").Return(0)
	return m
}
