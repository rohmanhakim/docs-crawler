package storage_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/metadata/metadatatest"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
)

func TestDryRunSink_Write_Success(t *testing.T) {
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
			// Setup mock metadata sink
			mockSink := &metadatatest.SinkMock{}
			sink := storage.NewDryRunSink(mockSink)

			// Create test document
			doc := createTestNormalizedDoc(
				tt.sourceURL,
				tt.canonicalURL,
				tt.contentHash,
				[]byte(tt.content),
			)

			// Execute write (note: outputDir is ignored in dry-run)
			result, writeErr := sink.Write("", doc, tt.hashAlgo)

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

			// In dry-run mode, path is just the filename (relative)
			expectedPath := expectedHash + ".md"
			if result.Path() != expectedPath {
				t.Errorf("expected Path %s, got %s", expectedPath, result.Path())
			}

			// Verify no actual file was written (dry-run)
			if _, err := os.Stat(result.Path()); err == nil {
				t.Errorf("expected file %s to NOT exist in dry-run mode", result.Path())
			}

			// Verify metadata recording
			if mockSink.RecordArtifactCalled {
				// Should have been called exactly once
				if len(mockSink.Artifacts) != 1 {
					t.Errorf("expected 1 artifact, got %d", len(mockSink.Artifacts))
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

				// DryRunSink passes false for overwrite since no actual file exists
				if ar.Overwrite() {
					t.Error("expected artifact Overwrite to be false (dry-run doesn't write)")
				}

				if ar.RecordedAt().IsZero() {
					t.Error("expected artifact RecordedAt to be non-zero")
				}
			} else {
				t.Error("expected RecordArtifact to be called")
			}

			// No errors should be recorded on success
			if mockSink.RecordErrorCalled {
				t.Error("expected RecordError not to be called for successful write")
			}
		})
	}
}

func TestDryRunSink_Write_Idempotent(t *testing.T) {
	mockSink := &metadatatest.SinkMock{}
	sink := storage.NewDryRunSink(mockSink)

	canonicalURL := "https://example.com/docs/page"
	sourceURL := "https://example.com/docs/page"
	content := "# Test Content"
	contentHash := "hash123"

	doc := createTestNormalizedDoc(sourceURL, canonicalURL, contentHash, []byte(content))

	// First write
	result1, err1 := sink.Write("", doc, hashutil.HashAlgoSHA256)
	if err1 != nil {
		t.Fatalf("first write failed: %v", err1)
	}

	mockSink.Reset()

	// Second write (should be identical)
	result2, err2 := sink.Write("", doc, hashutil.HashAlgoSHA256)
	if err2 != nil {
		t.Fatalf("second write failed: %v", err2)
	}

	// Verify deterministic results
	if result1.URLHash() != result2.URLHash() {
		t.Error("expected same URLHash for idempotent writes")
	}

	if result1.Path() != result2.Path() {
		t.Errorf("expected same Path for idempotent writes: %s vs %s", result1.Path(), result2.Path())
	}

	if result1.ContentHash() != result2.ContentHash() {
		t.Error("expected same ContentHash for idempotent writes")
	}
}

func TestDryRunSink_Write_HashFailure(t *testing.T) {
	tests := []struct {
		name           string
		hashAlgo       hashutil.HashAlgo
		expectedError  bool
		expectedCause  string
		expectArtifact bool
		expectErrorRec bool
	}{
		{
			name:           "invalid hash algorithm",
			hashAlgo:       "invalid_algo",
			expectedError:  true,
			expectedCause:  "hash computation failed",
			expectArtifact: false,
			expectErrorRec: false, // DryRunSink does not call RecordError on hash failure
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSink := &metadatatest.SinkMock{}
			sink := storage.NewDryRunSink(mockSink)

			doc := createTestNormalizedDoc(
				"https://example.com/page",
				"https://example.com/page",
				"hash123",
				[]byte("content"),
			)

			// Use underscore to ignore result on error path
			_, err := sink.Write("", doc, tt.hashAlgo)

			// Should return an error
			if tt.expectedError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			// Verify error is StorageError with correct cause
			if err != nil {
				storageErr, ok := err.(*storage.StorageError)
				if !ok {
					t.Fatalf("expected *storage.StorageError, got %T", err)
				}
				if !strings.Contains(storageErr.Error(), tt.expectedCause) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedCause, storageErr.Error())
				}
			}

			// Verify artifact recording behavior
			if tt.expectArtifact && !mockSink.RecordArtifactCalled {
				t.Error("expected RecordArtifact to be called")
			}
			if !tt.expectArtifact && mockSink.RecordArtifactCalled {
				t.Error("expected RecordArtifact NOT to be called")
			}

			// Verify error recording behavior
			if tt.expectErrorRec && !mockSink.RecordErrorCalled {
				t.Error("expected RecordError to be called on failure")
			}
			if !tt.expectErrorRec && mockSink.RecordErrorCalled {
				t.Error("expected RecordError NOT to be called")
			}
		})
	}
}

func TestDryRunSink_Write_FilenameDeterminism(t *testing.T) {
	tests := []struct {
		name         string
		canonicalURL string
		hashAlgo     hashutil.HashAlgo
	}{
		{
			name:         "deterministic filename with SHA256",
			canonicalURL: "https://docs.example.com/getting-started",
			hashAlgo:     hashutil.HashAlgoSHA256,
		},
		{
			name:         "deterministic filename with BLAKE3",
			canonicalURL: "https://docs.example.com/getting-started",
			hashAlgo:     hashutil.HashAlgoBLAKE3,
		},
		{
			name:         "deterministic filename with special characters",
			canonicalURL: "https://example.com/docs/page?query=value#fragment",
			hashAlgo:     hashutil.HashAlgoSHA256,
		},
		{
			name:         "deterministic filename with unicode",
			canonicalURL: "https://example.com/docs/café",
			hashAlgo:     hashutil.HashAlgoSHA256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSink := &metadatatest.SinkMock{}
			sink := storage.NewDryRunSink(mockSink)

			doc := createTestNormalizedDoc(
				tt.canonicalURL,
				tt.canonicalURL,
				"contentHash",
				[]byte("content"),
			)

			// First call
			result1, err := sink.Write("", doc, tt.hashAlgo)
			if err != nil {
				t.Fatalf("write failed: %v", err)
			}

			// Verify URL hash length
			if len(result1.URLHash()) != 12 {
				t.Errorf("expected URLHash length 12, got %d (%s)", len(result1.URLHash()), result1.URLHash())
			}

			// Verify filename format
			expectedFilename := result1.URLHash() + ".md"
			if result1.Path() != expectedFilename {
				t.Errorf("expected filename %s, got %s", expectedFilename, result1.Path())
			}

			// Second call - verify determinism
			mockSink.Reset()
			result2, err := sink.Write("", doc, tt.hashAlgo)
			if err != nil {
				t.Fatalf("second write failed: %v", err)
			}

			if result1.URLHash() != result2.URLHash() {
				t.Error("URL hash should be deterministic across runs")
			}
			if result1.Path() != result2.Path() {
				t.Errorf("path should be deterministic: %s vs %s", result1.Path(), result2.Path())
			}
		})
	}
}

func TestDryRunSink_Write_MultipleDocuments(t *testing.T) {
	mockSink := &metadatatest.SinkMock{}
	sink := storage.NewDryRunSink(mockSink)

	docs := []struct {
		canonicalURL string
		content      string
	}{
		{"https://example.com/docs/page1", "# Page 1"},
		{"https://example.com/docs/page2", "# Page 2"},
		{"https://example.com/docs/page3", "# Page 3"},
	}

	writtenPaths := make(map[string]bool)

	for i, docData := range docs {
		doc := createTestNormalizedDoc(
			docData.canonicalURL,
			docData.canonicalURL,
			"hash",
			[]byte(docData.content),
		)

		result, err := sink.Write("", doc, hashutil.HashAlgoSHA256)
		if err != nil {
			t.Fatalf("write failed for %s: %v", docData.canonicalURL, err)
		}

		// Verify no duplicate paths
		if writtenPaths[result.Path()] {
			t.Errorf("duplicate path generated for doc %d: %s", i, result.Path())
		}
		writtenPaths[result.Path()] = true

		mockSink.Reset()
	}

	// Verify all 3 files exist
	if len(writtenPaths) != 3 {
		t.Errorf("expected 3 unique paths, got %d", len(writtenPaths))
	}
}

func TestDryRunSink_Write_MetadataRecording(t *testing.T) {
	mockSink := &metadatatest.SinkMock{}
	sink := storage.NewDryRunSink(mockSink)

	canonicalURL := "https://example.com/docs/test"
	sourceURL := "https://example.com/docs/test"
	content := "# Test Content\n\nBody text."
	contentHash := "testcontenthash123"

	doc := createTestNormalizedDoc(sourceURL, canonicalURL, contentHash, []byte(content))

	result, err := sink.Write("", doc, hashutil.HashAlgoSHA256)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Verify RecordArtifact was called exactly once
	if !mockSink.RecordArtifactCalled {
		t.Error("expected RecordArtifact to be called")
	}
	if len(mockSink.Artifacts) != 1 {
		t.Errorf("expected 1 artifact record, got %d", len(mockSink.Artifacts))
	}

	ar := mockSink.LastArtifact()

	// Verify all artifact fields
	if ar.Kind() != metadata.ArtifactMarkdown {
		t.Errorf("expected artifact kind %s, got %s", metadata.ArtifactMarkdown, ar.Kind())
	}
	if ar.WritePath() != result.Path() {
		t.Errorf("expected artifact WritePath %s, got %s", result.Path(), ar.WritePath())
	}
	if ar.SourceURL() != sourceURL {
		t.Errorf("expected artifact SourceURL %s, got %s", sourceURL, ar.SourceURL())
	}
	if ar.ContentHash() != contentHash {
		t.Errorf("expected artifact ContentHash %s, got %s", contentHash, ar.ContentHash())
	}
	if ar.Bytes() != int64(len(content)) {
		t.Errorf("expected artifact Bytes %d, got %d", int64(len(content)), ar.Bytes())
	}
	// DryRunSink passes false for overwrite since no actual file is written
	if ar.Overwrite() {
		t.Error("expected artifact Overwrite to be false")
	}
	if ar.RecordedAt().IsZero() {
		t.Error("expected artifact RecordedAt to be non-zero")
	}

	// Verify timestamp is recent
	if time.Since(ar.RecordedAt()) > time.Second {
		t.Errorf("expected artifact RecordedAt to be recent, got %v", ar.RecordedAt())
	}

	// Verify RecordError was NOT called
	if mockSink.RecordErrorCalled {
		t.Error("expected RecordError not to be called on success")
	}
}

func TestDryRunSink_Constructor(t *testing.T) {
	mockSink := &metadatatest.SinkMock{}
	sink := storage.NewDryRunSink(mockSink)

	if sink == nil {
		t.Fatal("expected NewDryRunSink to return non-nil")
	}

	// Verify metadataSink field is set
	// Since metadataSink is not exported, we can't directly check it,
	// but we can verify behavior by calling Write and checking that
	// metadata is recorded through the mock
	doc := createTestNormalizedDoc(
		"https://example.com/page",
		"https://example.com/page",
		"hash",
		[]byte("content"),
	)

	_, err := sink.Write("", doc, hashutil.HashAlgoSHA256)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if !mockSink.RecordArtifactCalled {
		t.Error("expected RecordArtifact to be called, indicating metadataSink was properly set")
	}
}
