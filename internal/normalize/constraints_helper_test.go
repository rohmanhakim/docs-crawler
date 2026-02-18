package normalize_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

// fixtureDir returns the path to the fixture directory
func fixtureDir() string {
	return filepath.Join(".", "fixture")
}

// loadFixture reads a fixture file and returns its contents as bytes.
// This is used for black box testing via the Normalize() method.
func loadFixture(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join(fixtureDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read fixture %s: %v", filename, err)
	}
	return data
}

// metadataSinkMock is a mock for metadata.MetadataSink
type metadataSinkMock struct {
	recordErrorCalled         bool
	recordErrorAttrs          []metadata.Attribute
	recordFetchCalled         bool
	recordArtifactCalled      bool
	recordPipelineStageCalled bool
	recordPipelineEvent       *metadata.PipelineEvent
	recordSkipCalled          bool
}

func (m *metadataSinkMock) RecordError(record metadata.ErrorRecord) {
	m.recordErrorCalled = true
	m.recordErrorAttrs = record.Attrs()
}

func (m *metadataSinkMock) RecordFetch(event metadata.FetchEvent) {
	m.recordFetchCalled = true
}

func (m *metadataSinkMock) RecordArtifact(record metadata.ArtifactRecord) {
	m.recordArtifactCalled = true
}

func (m *metadataSinkMock) RecordPipelineStage(event metadata.PipelineEvent) {
	m.recordPipelineStageCalled = true
	m.recordPipelineEvent = &event
}

func (m *metadataSinkMock) RecordSkip(event metadata.SkipEvent) {
	m.recordSkipCalled = true
}

// Reset clears all recorded state
func (m *metadataSinkMock) Reset() {
	m.recordErrorCalled = false
	m.recordErrorAttrs = nil
	m.recordFetchCalled = false
	m.recordArtifactCalled = false
	m.recordPipelineStageCalled = false
	m.recordPipelineEvent = nil
	m.recordSkipCalled = false
}

var _ metadata.MetadataSink = (*metadataSinkMock)(nil)
