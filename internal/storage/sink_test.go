package storage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/debug/debugtest"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
)

func TestLocalSink_Write_Success(t *testing.T) {
	tests := []struct {
		name         string
		hashAlgo     hashutil.HashAlgo
		sourceURL    string
		canonicalURL string
		content      string
		contentHash  string
	}{
		{
			name:         "successful write with SHA256",
			hashAlgo:     hashutil.HashAlgoSHA256,
			sourceURL:    "https://example.com/docs/page1",
			canonicalURL: "https://example.com/docs/page1",
			content:      "# Page 1\n\nThis is the content of page 1.",
			contentHash:  "abc123def456",
		},
		{
			name:         "successful write with BLAKE3",
			hashAlgo:     hashutil.HashAlgoBLAKE3,
			sourceURL:    "https://example.com/docs/page2",
			canonicalURL: "https://example.com/docs/page2",
			content:      "# Page 2\n\nThis is the content of page 2.",
			contentHash:  "xyz789uvw012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tempDir, err := os.MkdirTemp("", "storage-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Setup mock
			mockSink := &metadataSinkMock{}
			mockLogger := debugtest.NewLoggerMock()
			sink := storage.NewLocalSink(mockSink)
			sink.SetDebugLogger(mockLogger)

			// Create test document
			doc := createTestNormalizedDoc(
				tt.sourceURL,
				tt.canonicalURL,
				tt.contentHash,
				[]byte(tt.content),
			)

			// Execute write
			result, writeErr := sink.Write(tempDir, doc, tt.hashAlgo)

			// Assertions
			if writeErr != nil {
				t.Errorf("expected no error, got: %v", writeErr)
			}

			// Verify WriteResult
			expectedHash := computeExpectedURLHash(tt.canonicalURL, tt.hashAlgo)
			if result.URLHash() != expectedHash {
				t.Errorf("expected URLHash %s, got %s", expectedHash, result.URLHash())
			}

			if result.ContentHash() != tt.contentHash {
				t.Errorf("expected ContentHash %s, got %s", tt.contentHash, result.ContentHash())
			}

			expectedPath := filepath.Join(tempDir, expectedHash+".md")
			if result.Path() != expectedPath {
				t.Errorf("expected Path %s, got %s", expectedPath, result.Path())
			}

			// Verify file was written
			writtenContent, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Errorf("failed to read written file: %v", err)
			}
			if string(writtenContent) != tt.content {
				t.Errorf("expected content %q, got %q", tt.content, string(writtenContent))
			}

			// Verify metadata recording
			if mockSink.RecordErrorCalled {
				t.Error("expected RecordError not to be called for successful write")
			}

			if !mockSink.RecordArtifactCalled {
				t.Error("expected RecordArtifact to be called")
			}

			ar := mockSink.LastArtifact()
			if ar.Kind() != metadata.ArtifactMarkdown {
				t.Errorf("expected artifact kind %s, got %s", metadata.ArtifactMarkdown, ar.Kind())
			}

			if ar.WritePath() != expectedPath {
				t.Errorf("expected artifact WritePath %s, got %s", expectedPath, ar.WritePath())
			}

			if ar.SourceURL() != tt.sourceURL {
				t.Errorf("expected artifact SourceURL %s, got %s", tt.sourceURL, ar.SourceURL())
			}

			if ar.ContentHash() != tt.contentHash {
				t.Errorf("expected artifact ContentHash %s, got %s", tt.contentHash, ar.ContentHash())
			}

			if ar.Bytes() != int64(len(tt.content)) {
				t.Errorf("expected artifact Bytes %d, got %d", int64(len(tt.content)), ar.Bytes())
			}

			if ar.RecordedAt().IsZero() {
				t.Error("expected artifact RecordedAt to be non-zero")
			}

			// Verify debug logging
			if !mockLogger.LogStepCalled {
				t.Error("expected LogStep to be called for debug logging")
			}

			// Verify compute_hash step was logged
			computeHashSteps := mockLogger.StepsByName("compute_hash")
			if len(computeHashSteps) == 0 {
				t.Error("expected compute_hash step to be logged")
			} else {
				if computeHashSteps[0].Fields["canonical_url"] != tt.canonicalURL {
					t.Errorf("expected canonical_url %s in compute_hash step, got %v", tt.canonicalURL, computeHashSteps[0].Fields["canonical_url"])
				}
				if computeHashSteps[0].Fields["hash_algo"] != string(tt.hashAlgo) {
					t.Errorf("expected hash_algo %s in compute_hash step, got %v", tt.hashAlgo, computeHashSteps[0].Fields["hash_algo"])
				}
			}

			// Verify ensure_dir step was logged
			ensureDirSteps := mockLogger.StepsByName("ensure_dir")
			if len(ensureDirSteps) == 0 {
				t.Error("expected ensure_dir step to be logged")
			}

			// Verify write_file step was logged
			writeFileSteps := mockLogger.StepsByName("write_file")
			if len(writeFileSteps) == 0 {
				t.Error("expected write_file step to be logged")
			} else {
				if writeFileSteps[0].Fields["file_path"] != expectedPath {
					t.Errorf("expected file_path %s in write_file step, got %v", expectedPath, writeFileSteps[0].Fields["file_path"])
				}
				if writeFileSteps[0].Fields["url_hash"] != expectedHash {
					t.Errorf("expected url_hash %s in write_file step, got %v", expectedHash, writeFileSteps[0].Fields["url_hash"])
				}
			}
		})
	}
}

func TestLocalSink_Write_Idempotent(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockSink := &metadataSinkMock{}
	sink := storage.NewLocalSink(mockSink)

	canonicalURL := "https://example.com/docs/page"
	sourceURL := "https://example.com/docs/page"
	content := "# Test Content"
	contentHash := "hash123"

	doc := createTestNormalizedDoc(sourceURL, canonicalURL, contentHash, []byte(content))

	// First write
	result1, err1 := sink.Write(tempDir, doc, hashutil.HashAlgoSHA256)
	if err1 != nil {
		t.Fatalf("first write failed: %v", err1)
	}

	mockSink.Reset()

	// Second write (should overwrite)
	result2, err2 := sink.Write(tempDir, doc, hashutil.HashAlgoSHA256)
	if err2 != nil {
		t.Fatalf("second write failed: %v", err2)
	}

	// Verify deterministic results
	if result1.URLHash() != result2.URLHash() {
		t.Error("expected same URLHash for idempotent writes")
	}

	if result1.Path() != result2.Path() {
		t.Error("expected same Path for idempotent writes")
	}

	if result1.ContentHash() != result2.ContentHash() {
		t.Error("expected same ContentHash for idempotent writes")
	}

	// Verify file still contains content
	writtenContent, err := os.ReadFile(result1.Path())
	if err != nil {
		t.Errorf("failed to read file after second write: %v", err)
	}
	if string(writtenContent) != content {
		t.Errorf("content mismatch after second write: expected %q, got %q", content, string(writtenContent))
	}
}

func TestLocalSink_Write_ErrorHandling(t *testing.T) {
	tests := []struct {
		name                 string
		setupFunc            func() (string, func())
		expectedError        bool
		expectMetadata       bool
		expectedErrorDetails string
	}{
		{
			name: "write to read-only directory",
			setupFunc: func() (string, func()) {
				tempDir, _ := os.MkdirTemp("", "storage-test-ro-*")
				os.Chmod(tempDir, 0555) // Read-only
				return tempDir, func() {
					os.Chmod(tempDir, 0755) // Restore permissions for cleanup
					os.RemoveAll(tempDir)
				}
			},
			expectedError:        true,
			expectMetadata:       true,
			expectedErrorDetails: "storage error: write failed",
		},
		{
			name: "write to non-existent path with parent read-only",
			setupFunc: func() (string, func()) {
				tempDir, _ := os.MkdirTemp("", "storage-test-*")
				os.Chmod(tempDir, 0555) // Read-only
				return filepath.Join(tempDir, "subdir"), func() {
					os.Chmod(tempDir, 0755) // Restore permissions for cleanup
					os.RemoveAll(tempDir)
				}
			},
			expectedError:        true,
			expectMetadata:       true,
			expectedErrorDetails: "storage error: path error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir, cleanup := tt.setupFunc()
			defer cleanup()

			mockSink := &metadataSinkMock{}
			mockLogger := debugtest.NewLoggerMock()
			sink := storage.NewLocalSink(mockSink)
			sink.SetDebugLogger(mockLogger)

			doc := createTestNormalizedDoc(
				"https://example.com/page",
				"https://example.com/page",
				"hash123",
				[]byte("content"),
			)

			_, writeErr := sink.Write(outputDir, doc, hashutil.HashAlgoSHA256)

			if tt.expectedError && writeErr == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectedError && writeErr != nil {
				t.Errorf("expected no error but got: %v", writeErr)
			}

			if tt.expectMetadata {
				if !mockSink.RecordErrorCalled {
					t.Error("expected RecordError to be called on failure")
				}

				er := mockSink.LastError()

				// Verify RecordError parameters
				if er.PackageName() != "storage" {
					t.Errorf("expected packageName 'storage', got: %s", er.PackageName())
				}

				if er.Action() != "LocalSink.Write" {
					t.Errorf("expected action 'LocalSink.Write', got: %s", er.Action())
				}

				// Cause should be StorageFailure for write failures
				if er.Cause() != metadata.CauseStorageFailure {
					t.Errorf("expected cause CauseStorageFailure (%d), got: %d", metadata.CauseStorageFailure, er.Cause())
				}

				// Assert against actual error details value
				if !strings.Contains(er.ErrorString(), tt.expectedErrorDetails) {
					t.Errorf("expected error details to contain %q, got: %s", tt.expectedErrorDetails, er.ErrorString())
				}

				// ObservedAt should be a recent timestamp (within last minute)
				timeDiff := time.Since(er.ObservedAt())
				if timeDiff > time.Minute {
					t.Errorf("expected observedAt to be recent, but was %v ago", timeDiff)
				}

				// Verify error metadata attributes
				var urlValue, writePathValue string
				for _, attr := range er.Attrs() {
					switch attr.Key() {
					case metadata.AttrURL:
						urlValue = attr.Value()
					case metadata.AttrWritePath:
						writePathValue = attr.Value()
					}
				}
				if urlValue != "https://example.com/page" {
					t.Errorf("expected AttrURL in error metadata, got: %s", urlValue)
				}

				if writePathValue == "" {
					t.Error("expected AttrWritePath in error metadata")
				}
			}

			if mockSink.RecordArtifactCalled {
				t.Error("expected RecordArtifact not to be called on failure")
			}

			// Verify debug logging for error case
			if !mockLogger.LogStepCalled {
				t.Error("expected LogStep to be called for debug logging")
			}

			// Verify write_failed step was logged
			writeFailedSteps := mockLogger.StepsByName("write_failed")
			if len(writeFailedSteps) == 0 {
				t.Error("expected write_failed step to be logged on error")
			}
		})
	}
}

func TestLocalSink_Write_FilenameDeterminism(t *testing.T) {
	tests := []struct {
		name         string
		canonicalURL string
		hashAlgo     hashutil.HashAlgo
		expectedLen  int // Expected length of URL hash (should be 12)
	}{
		{
			name:         "deterministic filename with SHA256",
			canonicalURL: "https://docs.example.com/getting-started",
			hashAlgo:     hashutil.HashAlgoSHA256,
			expectedLen:  12,
		},
		{
			name:         "deterministic filename with BLAKE3",
			canonicalURL: "https://docs.example.com/getting-started",
			hashAlgo:     hashutil.HashAlgoBLAKE3,
			expectedLen:  12,
		},
		{
			name:         "deterministic filename with special characters",
			canonicalURL: "https://example.com/docs/page?query=value#fragment",
			hashAlgo:     hashutil.HashAlgoSHA256,
			expectedLen:  12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, _ := os.MkdirTemp("", "storage-test-*")
			defer os.RemoveAll(tempDir)

			mockSink := &metadataSinkMock{}
			sink := storage.NewLocalSink(mockSink)

			doc := createTestNormalizedDoc(
				tt.canonicalURL,
				tt.canonicalURL,
				"contentHash",
				[]byte("content"),
			)

			result, err := sink.Write(tempDir, doc, tt.hashAlgo)
			if err != nil {
				t.Fatalf("write failed: %v", err)
			}

			// Verify URL hash length
			if len(result.URLHash()) != tt.expectedLen {
				t.Errorf("expected URLHash length %d, got %d (%s)", tt.expectedLen, len(result.URLHash()), result.URLHash())
			}

			// Verify filename format
			expectedFilename := result.URLHash() + ".md"
			if filepath.Base(result.Path()) != expectedFilename {
				t.Errorf("expected filename %s, got %s", expectedFilename, filepath.Base(result.Path()))
			}

			// Run twice and verify determinism
			mockSink.Reset()
			result2, err := sink.Write(tempDir, doc, tt.hashAlgo)
			if err != nil {
				t.Fatalf("second write failed: %v", err)
			}

			if result.URLHash() != result2.URLHash() {
				t.Error("filename hash should be deterministic across runs")
			}
		})
	}
}

func TestLocalSink_Write_MultipleDocuments(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "storage-test-*")
	defer os.RemoveAll(tempDir)

	mockSink := &metadataSinkMock{}
	sink := storage.NewLocalSink(mockSink)

	docs := []struct {
		canonicalURL string
		content      string
	}{
		{"https://example.com/docs/page1", "# Page 1"},
		{"https://example.com/docs/page2", "# Page 2"},
		{"https://example.com/docs/page3", "# Page 3"},
	}

	writtenPaths := make(map[string]bool)

	for _, docData := range docs {
		doc := createTestNormalizedDoc(
			docData.canonicalURL,
			docData.canonicalURL,
			"hash",
			[]byte(docData.content),
		)

		result, err := sink.Write(tempDir, doc, hashutil.HashAlgoSHA256)
		if err != nil {
			t.Fatalf("write failed for %s: %v", docData.canonicalURL, err)
		}

		// Verify no duplicate paths
		if writtenPaths[result.Path()] {
			t.Errorf("duplicate path generated: %s", result.Path())
		}
		writtenPaths[result.Path()] = true

		// Verify file exists
		if _, err := os.Stat(result.Path()); os.IsNotExist(err) {
			t.Errorf("file not found: %s", result.Path())
		}

		mockSink.Reset()
	}

	// Verify all 3 files exist
	if len(writtenPaths) != 3 {
		t.Errorf("expected 3 unique paths, got %d", len(writtenPaths))
	}
}

func TestWriteResult_Methods(t *testing.T) {
	result := storage.NewWriteResult("urlhash123", "/path/to/file.md", "contenthash456")

	if result.URLHash() != "urlhash123" {
		t.Errorf("expected URLHash urlhash123, got %s", result.URLHash())
	}

	if result.Path() != "/path/to/file.md" {
		t.Errorf("expected Path /path/to/file.md, got %s", result.Path())
	}

	if result.ContentHash() != "contenthash456" {
		t.Errorf("expected ContentHash contenthash456, got %s", result.ContentHash())
	}
}

func TestLocalSink_Write_DebugLoggerDisabled(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup mock with disabled logger
	mockSink := &metadataSinkMock{}
	mockLogger := debugtest.NewLoggerMock()
	mockLogger.SetEnabled(false) // Disable debug logging
	sink := storage.NewLocalSink(mockSink)
	sink.SetDebugLogger(mockLogger)

	doc := createTestNormalizedDoc(
		"https://example.com/page",
		"https://example.com/page",
		"hash123",
		[]byte("content"),
	)

	// Execute write
	_, writeErr := sink.Write(tempDir, doc, hashutil.HashAlgoSHA256)

	// Verify write succeeded
	if writeErr != nil {
		t.Errorf("expected no error, got: %v", writeErr)
	}

	// Verify no debug logging occurred (logger is disabled)
	if mockLogger.LogStepCalled {
		t.Error("expected LogStep not to be called when debug logger is disabled")
	}
}
