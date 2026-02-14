package fileutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/fileutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "file with extension",
			path:     "document.pdf",
			expected: "pdf",
		},
		{
			name:     "file with multiple dots",
			path:     "archive.tar.gz",
			expected: "gz",
		},
		{
			name:     "file without extension",
			path:     "README",
			expected: "",
		},
		{
			name:     "dotfile without extension",
			path:     ".gitignore",
			expected: "gitignore",
		},
		{
			name:     "file with leading dot and extension",
			path:     ".env.local",
			expected: "local",
		},
		{
			name:     "path with directories",
			path:     "/home/user/documents/file.txt",
			expected: "txt",
		},
		{
			name:     "windows path with extension",
			path:     "C:\\Users\\user\\file.docx",
			expected: "docx",
		},
		{
			name:     "empty string",
			path:     "",
			expected: "",
		},
		{
			name:     "file with dot at end",
			path:     "file.",
			expected: "",
		},
		{
			name:     "hidden file with extension",
			path:     ".gitignore.backup",
			expected: "backup",
		},
		{
			name:     "path ending with slash",
			path:     "/some/directory/",
			expected: "",
		},
		{
			name:     "just a dot",
			path:     ".",
			expected: "",
		},
		{
			name:     "double dot",
			path:     "..",
			expected: "",
		},
		{
			name:     "unicode filename",
			path:     "文档.pdf",
			expected: "pdf",
		},
		{
			name:     "uppercase extension",
			path:     "file.PDF",
			expected: "PDF",
		},
		{
			name:     "mixed case extension",
			path:     "file.TxT",
			expected: "TxT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileutil.GetFileExtension(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnsureDir_SinglePathComponent(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "testdir")

	err := fileutil.EnsureDir(targetDir)
	require.NoError(t, err)

	info, statErr := os.Stat(targetDir)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestEnsureDir_MultiplePathComponents(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "parent", "child", "grandchild")

	err := fileutil.EnsureDir(tmpDir, "parent", "child", "grandchild")
	require.NoError(t, err)

	info, statErr := os.Stat(targetDir)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestEnsureDir_DirectoryAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "existing")

	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	err = fileutil.EnsureDir(targetDir)
	require.NoError(t, err)
}

func TestEnsureDir_EmptyPathVariadic(t *testing.T) {
	tmpDir := t.TempDir()

	err := fileutil.EnsureDir(tmpDir)
	require.NoError(t, err)

	info, statErr := os.Stat(tmpDir)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestEnsureDir_PermissionError(t *testing.T) {
	if filepath.Separator == '\\' {
		t.Skip("Skipping permission test on Windows")
	}

	tmpDir := t.TempDir()
	readonlyDir := filepath.Join(tmpDir, "readonly")
	err := os.MkdirAll(readonlyDir, 0555)
	require.NoError(t, err)

	targetDir := filepath.Join(readonlyDir, "subdir")
	err = fileutil.EnsureDir(targetDir)
	assert.Error(t, err)

	var fileErr *fileutil.FileError
	if assert.ErrorAs(t, err, &fileErr) {
		assert.False(t, fileErr.Retryable)
		assert.Equal(t, fileutil.ErrCausePathError, fileErr.Cause)
	}
}

func TestEnsureDir_InvalidPath(t *testing.T) {
	tmpDir := t.TempDir()

	targetDir := filepath.Join(tmpDir, "", "subdir")
	err := fileutil.EnsureDir(targetDir)
	require.NoError(t, err)

	info, statErr := os.Stat(targetDir)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestEnsureDir_ReturnsNilOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	err := fileutil.EnsureDir(tmpDir, "newdir")
	assert.NoError(t, err)
	assert.Nil(t, err)
}
