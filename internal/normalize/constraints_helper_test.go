package normalize_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/metadata/metadatatest"
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

// metadataSinkMock is an alias to the shared mock for backward compatibility
// with existing test code. New tests should use metadatatest.SinkMock directly.
type metadataSinkMock = metadatatest.SinkMock
