package normalize_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
	recordErrorCalled      bool
	recordErrorAttrs       []metadata.Attribute
	recordFetchCalled      bool
	recordAssetFetchCalled bool
	recordArtifactCalled   bool
}

func (m *metadataSinkMock) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	details string,
	attrs []metadata.Attribute,
) {
	m.recordErrorCalled = true
	m.recordErrorAttrs = attrs
}

func (m *metadataSinkMock) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
	m.recordFetchCalled = true
}

func (m *metadataSinkMock) RecordAssetFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	retryCount int,
) {
	m.recordAssetFetchCalled = true
}

func (m *metadataSinkMock) RecordArtifact(kind metadata.ArtifactKind, path string, attrs []metadata.Attribute) {
	m.recordArtifactCalled = true
}

// Reset clears all recorded state
func (m *metadataSinkMock) Reset() {
	m.recordErrorCalled = false
	m.recordErrorAttrs = nil
	m.recordFetchCalled = false
	m.recordAssetFetchCalled = false
	m.recordArtifactCalled = false
}
