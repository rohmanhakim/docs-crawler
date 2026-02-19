package metadatatest

import (
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

// SinkMock is a comprehensive test double for metadata.MetadataSink.
// It tracks all calls with boolean flags and stores all recorded events
// in slices for inspection in tests.
//
// Usage:
//
//	mock := &metadatatest.SinkMock{}
//	component := NewComponent(mock)
//	// ... exercise component ...
//	if mock.RecordFetchCalled {
//	    // assert on mock.FetchEvents
//	}
type SinkMock struct {
	// Call tracking (boolean flags)
	RecordFetchCalled    bool
	RecordArtifactCalled bool
	RecordPipelineCalled bool
	RecordSkipCalled     bool
	RecordErrorCalled    bool

	// Recorded events (slices for inspection)
	FetchEvents    []metadata.FetchEvent
	Artifacts      []metadata.ArtifactRecord
	PipelineEvents []metadata.PipelineEvent
	SkipEvents     []metadata.SkipEvent
	ErrorRecords   []metadata.ErrorRecord
}

// Compile-time interface check
var _ metadata.MetadataSink = (*SinkMock)(nil)

func (m *SinkMock) RecordFetch(event metadata.FetchEvent) {
	m.RecordFetchCalled = true
	m.FetchEvents = append(m.FetchEvents, event)
}

func (m *SinkMock) RecordArtifact(record metadata.ArtifactRecord) {
	m.RecordArtifactCalled = true
	m.Artifacts = append(m.Artifacts, record)
}

func (m *SinkMock) RecordPipelineStage(event metadata.PipelineEvent) {
	m.RecordPipelineCalled = true
	m.PipelineEvents = append(m.PipelineEvents, event)
}

func (m *SinkMock) RecordSkip(event metadata.SkipEvent) {
	m.RecordSkipCalled = true
	m.SkipEvents = append(m.SkipEvents, event)
}

func (m *SinkMock) RecordError(record metadata.ErrorRecord) {
	m.RecordErrorCalled = true
	m.ErrorRecords = append(m.ErrorRecords, record)
}

// Reset clears all recorded state, returning the mock to its zero state.
// This is useful for reusing the same mock across multiple test cases.
func (m *SinkMock) Reset() {
	m.RecordFetchCalled = false
	m.RecordArtifactCalled = false
	m.RecordPipelineCalled = false
	m.RecordSkipCalled = false
	m.RecordErrorCalled = false
	m.FetchEvents = nil
	m.Artifacts = nil
	m.PipelineEvents = nil
	m.SkipEvents = nil
	m.ErrorRecords = nil
}

// LastFetch returns the most recent FetchEvent, or nil if none recorded.
func (m *SinkMock) LastFetch() *metadata.FetchEvent {
	if len(m.FetchEvents) == 0 {
		return nil
	}
	return &m.FetchEvents[len(m.FetchEvents)-1]
}

// LastArtifact returns the most recent ArtifactRecord, or nil if none recorded.
func (m *SinkMock) LastArtifact() *metadata.ArtifactRecord {
	if len(m.Artifacts) == 0 {
		return nil
	}
	return &m.Artifacts[len(m.Artifacts)-1]
}

// LastPipelineEvent returns the most recent PipelineEvent, or nil if none recorded.
func (m *SinkMock) LastPipelineEvent() *metadata.PipelineEvent {
	if len(m.PipelineEvents) == 0 {
		return nil
	}
	return &m.PipelineEvents[len(m.PipelineEvents)-1]
}

// LastSkip returns the most recent SkipEvent, or nil if none recorded.
func (m *SinkMock) LastSkip() *metadata.SkipEvent {
	if len(m.SkipEvents) == 0 {
		return nil
	}
	return &m.SkipEvents[len(m.SkipEvents)-1]
}

// LastError returns the most recent ErrorRecord, or nil if none recorded.
func (m *SinkMock) LastError() *metadata.ErrorRecord {
	if len(m.ErrorRecords) == 0 {
		return nil
	}
	return &m.ErrorRecords[len(m.ErrorRecords)-1]
}

// GetFetchRecords returns all recorded FetchEvent calls.
// This is a convenience accessor for tests that need to inspect all fetch events.
func (m *SinkMock) GetFetchRecords() []metadata.FetchEvent {
	return m.FetchEvents
}

// GetArtifactRecords returns all recorded ArtifactRecord calls.
// This is a convenience accessor for tests that need to inspect all artifact records.
func (m *SinkMock) GetArtifactRecords() []metadata.ArtifactRecord {
	return m.Artifacts
}

// GetErrorRecords returns all recorded ErrorRecord calls.
// This is a convenience accessor for tests that need to inspect all error records.
func (m *SinkMock) GetErrorRecords() []metadata.ErrorRecord {
	return m.ErrorRecords
}
