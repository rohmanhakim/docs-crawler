package debugtest

import (
	"context"
	"sync"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/debug"
)

// LoggerMock is a comprehensive test double for debug.DebugLogger.
// It tracks all calls with boolean flags and stores all recorded events
// in slices for inspection in tests.
//
// Usage:
//
//	mock := &debugtest.LoggerMock{}
//	component := NewComponent(mock)
//	// ... exercise component ...
//	if mock.LogStageCalled {
//	    // assert on mock.StageEntries
//	}
type LoggerMock struct {
	mu sync.Mutex

	// Call tracking (boolean flags)
	LogStageCalled     bool
	LogRetryCalled     bool
	LogRateLimitCalled bool
	LogStepCalled      bool
	LogErrorCalled     bool
	WithFieldsCalled   bool
	CloseCalled        bool

	// Recorded entries (slices for inspection)
	StageEntries     []StageEntry
	RetryEntries     []RetryEntry
	RateLimitEntries []RateLimitEntry
	StepEntries      []StepEntry
	ErrorEntries     []ErrorEntry

	// WithFields tracking
	WithFieldsRecords []debug.FieldMap

	// Behavior configuration
	enabled bool
}

// StageEntry represents a recorded LogStage call.
type StageEntry struct {
	Stage string
	Event debug.StageEvent
}

// RetryEntry represents a recorded LogRetry call.
type RetryEntry struct {
	Attempt     int
	MaxAttempts int
	Backoff     time.Duration
	Err         error
}

// RateLimitEntry represents a recorded LogRateLimit call.
type RateLimitEntry struct {
	Host   string
	Delay  time.Duration
	Reason debug.RateLimitReason
}

// StepEntry represents a recorded LogStep call.
type StepEntry struct {
	Stage  string
	Step   string
	Fields debug.FieldMap
}

// ErrorEntry represents a recorded LogError call.
type ErrorEntry struct {
	Stage  string
	Err    error
	Fields debug.FieldMap
}

// Compile-time interface check
var _ debug.DebugLogger = (*LoggerMock)(nil)

// NewLoggerMock creates a new LoggerMock with enabled=true by default.
func NewLoggerMock() *LoggerMock {
	return &LoggerMock{
		enabled: true,
	}
}

// Enabled returns true by default. Use SetEnabled() to change behavior.
func (m *LoggerMock) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// SetEnabled configures the enabled state for testing different scenarios.
func (m *LoggerMock) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

// LogStage records a stage event.
func (m *LoggerMock) LogStage(_ context.Context, stage string, event debug.StageEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LogStageCalled = true
	m.StageEntries = append(m.StageEntries, StageEntry{
		Stage: stage,
		Event: event,
	})
}

// LogRetry records a retry event.
func (m *LoggerMock) LogRetry(_ context.Context, attempt int, maxAttempts int, backoff time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LogRetryCalled = true
	m.RetryEntries = append(m.RetryEntries, RetryEntry{
		Attempt:     attempt,
		MaxAttempts: maxAttempts,
		Backoff:     backoff,
		Err:         err,
	})
}

// LogRateLimit records a rate limit event.
func (m *LoggerMock) LogRateLimit(_ context.Context, host string, delay time.Duration, reason debug.RateLimitReason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LogRateLimitCalled = true
	m.RateLimitEntries = append(m.RateLimitEntries, RateLimitEntry{
		Host:   host,
		Delay:  delay,
		Reason: reason,
	})
}

// LogStep records a step event.
func (m *LoggerMock) LogStep(_ context.Context, stage string, step string, fields debug.FieldMap) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LogStepCalled = true
	m.StepEntries = append(m.StepEntries, StepEntry{
		Stage:  stage,
		Step:   step,
		Fields: fields,
	})
}

// LogError records an error event.
func (m *LoggerMock) LogError(_ context.Context, stage string, err error, fields debug.FieldMap) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LogErrorCalled = true
	m.ErrorEntries = append(m.ErrorEntries, ErrorEntry{
		Stage:  stage,
		Err:    err,
		Fields: fields,
	})
}

// WithFields records the fields and returns a new LoggerMock with those fields.
// The returned logger shares the same underlying data structures for tracking.
func (m *LoggerMock) WithFields(fields debug.FieldMap) debug.DebugLogger {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.WithFieldsCalled = true
	m.WithFieldsRecords = append(m.WithFieldsRecords, fields)

	// Return a new mock that shares the same tracking slices
	// but starts with the pre-populated fields context
	return &LoggerMock{
		enabled:           m.enabled,
		StageEntries:      m.StageEntries,
		RetryEntries:      m.RetryEntries,
		RateLimitEntries:  m.RateLimitEntries,
		StepEntries:       m.StepEntries,
		ErrorEntries:      m.ErrorEntries,
		WithFieldsRecords: m.WithFieldsRecords,
	}
}

// Close records that Close was called and returns nil.
func (m *LoggerMock) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CloseCalled = true
	return nil
}

// Reset clears all recorded state, returning the mock to its zero state.
// This is useful for reusing the same mock across multiple test cases.
func (m *LoggerMock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LogStageCalled = false
	m.LogRetryCalled = false
	m.LogRateLimitCalled = false
	m.LogStepCalled = false
	m.LogErrorCalled = false
	m.WithFieldsCalled = false
	m.CloseCalled = false
	m.StageEntries = nil
	m.RetryEntries = nil
	m.RateLimitEntries = nil
	m.StepEntries = nil
	m.ErrorEntries = nil
	m.WithFieldsRecords = nil
	m.enabled = true
}

// LastStage returns the most recent StageEntry, or nil if none recorded.
func (m *LoggerMock) LastStage() *StageEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.StageEntries) == 0 {
		return nil
	}
	return &m.StageEntries[len(m.StageEntries)-1]
}

// LastRetry returns the most recent RetryEntry, or nil if none recorded.
func (m *LoggerMock) LastRetry() *RetryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.RetryEntries) == 0 {
		return nil
	}
	return &m.RetryEntries[len(m.RetryEntries)-1]
}

// LastRateLimit returns the most recent RateLimitEntry, or nil if none recorded.
func (m *LoggerMock) LastRateLimit() *RateLimitEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.RateLimitEntries) == 0 {
		return nil
	}
	return &m.RateLimitEntries[len(m.RateLimitEntries)-1]
}

// LastStep returns the most recent StepEntry, or nil if none recorded.
func (m *LoggerMock) LastStep() *StepEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.StepEntries) == 0 {
		return nil
	}
	return &m.StepEntries[len(m.StepEntries)-1]
}

// LastError returns the most recent ErrorEntry, or nil if none recorded.
func (m *LoggerMock) LastError() *ErrorEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.ErrorEntries) == 0 {
		return nil
	}
	return &m.ErrorEntries[len(m.ErrorEntries)-1]
}

// GetStageEntries returns all recorded StageEntry calls.
// This is a convenience accessor for tests that need to inspect all stage entries.
func (m *LoggerMock) GetStageEntries() []StageEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]StageEntry{}, m.StageEntries...)
}

// GetRetryEntries returns all recorded RetryEntry calls.
// This is a convenience accessor for tests that need to inspect all retry entries.
func (m *LoggerMock) GetRetryEntries() []RetryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]RetryEntry{}, m.RetryEntries...)
}

// GetRateLimitEntries returns all recorded RateLimitEntry calls.
// This is a convenience accessor for tests that need to inspect all rate limit entries.
func (m *LoggerMock) GetRateLimitEntries() []RateLimitEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]RateLimitEntry{}, m.RateLimitEntries...)
}

// GetStepEntries returns all recorded StepEntry calls.
// This is a convenience accessor for tests that need to inspect all step entries.
func (m *LoggerMock) GetStepEntries() []StepEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]StepEntry{}, m.StepEntries...)
}

// GetErrorEntries returns all recorded ErrorEntry calls.
// This is a convenience accessor for tests that need to inspect all error entries.
func (m *LoggerMock) GetErrorEntries() []ErrorEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ErrorEntry{}, m.ErrorEntries...)
}

// StagesByType returns all stage entries matching the given event type.
func (m *LoggerMock) StagesByType(eventType debug.EventType) []StageEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []StageEntry
	for _, entry := range m.StageEntries {
		if entry.Event.Type == eventType {
			result = append(result, entry)
		}
	}
	return result
}

// StepsByStage returns all step entries for the given stage.
func (m *LoggerMock) StepsByStage(stage string) []StepEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []StepEntry
	for _, entry := range m.StepEntries {
		if entry.Stage == stage {
			result = append(result, entry)
		}
	}
	return result
}

// StepsByName returns all step entries matching the given step name.
func (m *LoggerMock) StepsByName(step string) []StepEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []StepEntry
	for _, entry := range m.StepEntries {
		if entry.Step == step {
			result = append(result, entry)
		}
	}
	return result
}

// TotalCalls returns the total number of logging calls made.
func (m *LoggerMock) TotalCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.StageEntries) +
		len(m.RetryEntries) +
		len(m.RateLimitEntries) +
		len(m.StepEntries) +
		len(m.ErrorEntries)
}
