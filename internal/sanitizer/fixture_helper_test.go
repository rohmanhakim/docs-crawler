package sanitizer_test

import (
	"os"
	"path/filepath"
	"testing"
)

// fixtureDir returns the path to the fixture directory
func fixtureDir() string {
	return filepath.Join(".", "fixture")
}

// loadFixture reads a fixture file and returns its contents as bytes.
// This is used for black box testing via the Extract() method.
func loadFixture(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join(fixtureDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read fixture %s: %v", filename, err)
	}
	return data
}
