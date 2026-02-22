package debug_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/debug"
)

// TestNewMultiWriter tests the NewMultiWriter constructor
func TestNewMultiWriter(t *testing.T) {
	tests := []struct {
		name       string
		outputFile string
		wantErr    bool
		errContain string
	}{
		{
			name:       "stdout only with empty string",
			outputFile: "",
			wantErr:    false,
		},
		{
			name:       "stdout and file",
			outputFile: "", // Will be set to temp file in test
			wantErr:    false,
		},
		{
			name:       "invalid file path",
			outputFile: "/nonexistent/directory/that/does/not/exist/log.txt",
			wantErr:    true,
			errContain: "failed to open debug log file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var outputFile string
			var cleanup func()

			// Handle temp file for valid file tests
			if tt.name == "stdout and file" {
				tmpDir := t.TempDir()
				outputFile = filepath.Join(tmpDir, "debug.log")
				cleanup = func() {} // TempDir is auto-cleaned
			} else {
				outputFile = tt.outputFile
			}

			mw, err := debug.NewMultiWriter(outputFile)

			// Cleanup
			if cleanup != nil {
				cleanup()
			}
			if mw != nil {
				defer mw.Close()
			}

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("Error should contain %q, got %q", tt.errContain, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if mw == nil {
				t.Fatal("Expected MultiWriter to be non-nil")
			}
		})
	}
}

// TestNewMultiWriterStdoutOnly tests creating a MultiWriter with stdout only
func TestNewMultiWriterStdoutOnly(t *testing.T) {
	mw, err := debug.NewMultiWriter("")
	if err != nil {
		t.Fatalf("NewMultiWriter() error = %v", err)
	}
	defer mw.Close()

	if mw == nil {
		t.Fatal("Expected MultiWriter to be non-nil")
	}
}

// TestNewMultiWriterWithFile tests creating a MultiWriter with stdout and file
func TestNewMultiWriterWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "debug.log")

	mw, err := debug.NewMultiWriter(outputFile)
	if err != nil {
		t.Fatalf("NewMultiWriter() error = %v", err)
	}
	defer mw.Close()

	// Verify file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("Expected debug log file to be created")
	}
}

// TestNewMultiWriterFileAppend tests that NewMultiWriter appends to existing file
func TestNewMultiWriterFileAppend(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "debug.log")

	// Write initial content
	initialContent := []byte("initial content\n")
	if err := os.WriteFile(outputFile, initialContent, 0600); err != nil {
		t.Fatalf("Failed to write initial file: %v", err)
	}

	// Create MultiWriter (should append, not overwrite)
	mw, err := debug.NewMultiWriter(outputFile)
	if err != nil {
		t.Fatalf("NewMultiWriter() error = %v", err)
	}
	defer mw.Close()

	// Verify initial content is preserved
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if !bytes.Contains(content, initialContent) {
		t.Error("Expected initial content to be preserved (append mode)")
	}
}

// TestMultiWriterWrite tests the Write method
func TestMultiWriterWrite(t *testing.T) {
	t.Run("Write returns correct byte count", func(t *testing.T) {
		mw, err := debug.NewMultiWriter("")
		if err != nil {
			t.Fatalf("NewMultiWriter() error = %v", err)
		}
		defer mw.Close()

		data := []byte("test message\n")
		n, err := mw.Write(data)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if n != len(data) {
			t.Errorf("Write() returned %d bytes, expected %d", n, len(data))
		}
	})

	t.Run("Write to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputFile := filepath.Join(tmpDir, "debug.log")

		mw, err := debug.NewMultiWriter(outputFile)
		if err != nil {
			t.Fatalf("NewMultiWriter() error = %v", err)
		}
		defer mw.Close()

		data := []byte("test log entry\n")
		n, err := mw.Write(data)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if n != len(data) {
			t.Errorf("Write() returned %d bytes, expected %d", n, len(data))
		}

		// Verify content was written to file
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if !bytes.Contains(content, data) {
			t.Errorf("Expected file to contain %q, got %q", string(data), string(content))
		}
	})

	t.Run("Write empty slice", func(t *testing.T) {
		mw, err := debug.NewMultiWriter("")
		if err != nil {
			t.Fatalf("NewMultiWriter() error = %v", err)
		}
		defer mw.Close()

		n, err := mw.Write([]byte{})
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if n != 0 {
			t.Errorf("Write() returned %d bytes, expected 0", n)
		}
	})
}

// TestMultiWriterConcurrentWrite tests concurrent writes for thread safety
func TestMultiWriterConcurrentWrite(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "debug.log")

	mw, err := debug.NewMultiWriter(outputFile)
	if err != nil {
		t.Fatalf("NewMultiWriter() error = %v", err)
	}
	defer mw.Close()

	const goroutines = 10
	const writesPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				data := []byte("goroutine message\n")
				n, err := mw.Write(data)
				if err != nil {
					t.Errorf("Write() error = %v", err)
					return
				}
				if n != len(data) {
					t.Errorf("Write() returned %d bytes, expected %d", n, len(data))
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify file content
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expectedLines := goroutines * writesPerGoroutine
	actualLines := bytes.Count(content, []byte("goroutine message\n"))
	if actualLines != expectedLines {
		t.Errorf("Expected %d lines in file, got %d", expectedLines, actualLines)
	}
}

// TestMultiWriterClose tests the Close method
func TestMultiWriterClose(t *testing.T) {
	t.Run("Close with no file", func(t *testing.T) {
		mw, err := debug.NewMultiWriter("")
		if err != nil {
			t.Fatalf("NewMultiWriter() error = %v", err)
		}

		err = mw.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("Close with file", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputFile := filepath.Join(tmpDir, "debug.log")

		mw, err := debug.NewMultiWriter(outputFile)
		if err != nil {
			t.Fatalf("NewMultiWriter() error = %v", err)
		}

		// Write some data before closing
		_, err = mw.Write([]byte("test\n"))
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}

		err = mw.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("Multiple close calls returns error on second call", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputFile := filepath.Join(tmpDir, "debug.log")

		mw, err := debug.NewMultiWriter(outputFile)
		if err != nil {
			t.Fatalf("NewMultiWriter() error = %v", err)
		}

		// First close
		err = mw.Close()
		if err != nil {
			t.Errorf("First Close() error = %v", err)
		}

		// Second close returns error (file already closed)
		// This is expected Go behavior - closing an already-closed file returns an error
		err = mw.Close()
		if err == nil {
			t.Error("Expected error on second Close(), got nil")
		}
	})
}

// TestMultiWriterImplementsIOWriter verifies MultiWriter implements io.Writer
func TestMultiWriterImplementsIOWriter(t *testing.T) {
	// This test will fail at compile time if MultiWriter doesn't implement io.Writer
	var _ io.Writer = (*debug.MultiWriter)(nil)

	mw, err := debug.NewMultiWriter("")
	if err != nil {
		t.Fatalf("NewMultiWriter() error = %v", err)
	}
	defer mw.Close()

	// Test that it can be used where io.Writer is expected
	var w io.Writer = mw
	data := []byte("test\n")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() returned %d bytes, expected %d", n, len(data))
	}
}

// TestMultiWriterWriteAfterClose tests behavior when writing after close
func TestMultiWriterWriteAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "debug.log")

	mw, err := debug.NewMultiWriter(outputFile)
	if err != nil {
		t.Fatalf("NewMultiWriter() error = %v", err)
	}

	// Close the writer
	err = mw.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Try to write after close - this should fail because file is closed
	_, err = mw.Write([]byte("after close\n"))
	if err == nil {
		t.Error("Expected error when writing to closed file, got nil")
	}
}

// TestMultiWriterFilePermissions tests that file is created with correct permissions
func TestMultiWriterFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "debug.log")

	mw, err := debug.NewMultiWriter(outputFile)
	if err != nil {
		t.Fatalf("NewMultiWriter() error = %v", err)
	}
	defer mw.Close()

	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	expectedPerms := os.FileMode(0600)
	// Check permission bits only (mask off file type bits)
	if info.Mode().Perm() != expectedPerms {
		t.Errorf("Expected file permissions %o, got %o", expectedPerms, info.Mode().Perm())
	}
}
