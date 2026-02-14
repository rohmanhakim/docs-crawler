package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

// GetFileExtension extracts the file extension from a path, or empty string if none
func GetFileExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	// Remove the leading dot
	return strings.TrimPrefix(ext, ".")
}

// EnsureDir check if a given directory plus the following path exist, then create one if not
func EnsureDir(dir string, path ...string) failure.ClassifiedError {
	targetPath := []string{dir}
	targetPath = append(targetPath, path...)

	assetsDir := filepath.Join(targetPath...)
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return &FileError{
			Message:   fmt.Sprintf("%v", err),
			Retryable: false,
			Cause:     ErrCausePathError,
		}
	}
	return nil
}
