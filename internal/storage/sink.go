package storage

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/fileutil"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
)

/*
Responsibilities
- Persist Markdown files
- Write assets
- Ensure deterministic filenames

Output Characteristics
- Stable directory layout
- Idempotent writes
- Overwrite-safe reruns
*/

type Sink interface {
	Write(
		outputDir string,
		normalizedDoc normalize.NormalizedMarkdownDoc,
		hashAlgo hashutil.HashAlgo,
	) (WriteResult, failure.ClassifiedError)
}

type LocalSink struct {
	metadataSink metadata.MetadataSink
}

func NewLocalSink(
	metadataSink metadata.MetadataSink,
) LocalSink {
	return LocalSink{
		metadataSink: metadataSink,
	}
}

func (s *LocalSink) Write(
	outputDir string,
	normalizedDoc normalize.NormalizedMarkdownDoc,
	hashAlgo hashutil.HashAlgo,
) (WriteResult, failure.ClassifiedError) {
	writeResult, err := write(outputDir, normalizedDoc, hashAlgo)
	if err != nil {
		var storageError *StorageError
		errors.As(err, &storageError)
		s.metadataSink.RecordError(
			time.Now(),
			"storage",
			"LocalSink.Write",
			mapStorageErrorToMetadataCause(storageError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, normalizedDoc.Frontmatter().SourceURL()),
				metadata.NewAttr(metadata.AttrWritePath, storageError.Path),
			},
		)
		return WriteResult{}, storageError
	}
	s.metadataSink.RecordArtifact(
		metadata.ArtifactMarkdown,
		writeResult.Path(),
		[]metadata.Attribute{
			metadata.NewAttr(metadata.AttrWritePath, writeResult.Path()),
			metadata.NewAttr(metadata.AttrURL, normalizedDoc.Frontmatter().SourceURL()),
			metadata.NewAttr(metadata.AttrField, writeResult.URLHash()),
			metadata.NewAttr(metadata.AttrField, writeResult.ContentHash()),
		},
	)
	return writeResult, nil
}

func write(
	outputDir string,
	normalizedDoc normalize.NormalizedMarkdownDoc,
	hashAlgo hashutil.HashAlgo,
) (WriteResult, failure.ClassifiedError) {
	// Get canonical URL for filename hashing (per filename-invariants.md)
	canonicalURL := normalizedDoc.Frontmatter().CanonicalURL()

	// Hash the canonical URL using specified algorithm
	urlHashFull, err := hashutil.HashBytes([]byte(canonicalURL), hashAlgo)
	if err != nil {
		return WriteResult{}, &StorageError{
			Message:   err.Error(),
			Retryable: false,
			Cause:     ErrCauseHashComputationFailed,
			Path:      "",
		}
	}

	// Use first 12 hex characters for filename (per user's requirement)
	urlHash := urlHashFull[:12]

	// Prepare output directory
	if err := fileutil.EnsureDir(outputDir); err != nil {
		var fileErr *fileutil.FileError
		if errors.As(err, &fileErr) {
			cause := ErrCauseWriteFailure
			retryable := false
			if fileErr.Cause == fileutil.ErrCausePathError {
				// Could be disk full or permission issue
				cause = ErrCausePathError
				retryable = true // disk full is retryable
			}
			return WriteResult{}, &StorageError{
				Message:   err.Error(),
				Retryable: retryable,
				Cause:     cause,
				Path:      outputDir,
			}
		}
		return WriteResult{}, &StorageError{
			Message:   err.Error(),
			Retryable: false,
			Cause:     ErrCauseWriteFailure,
			Path:      outputDir,
		}
	}

	// Construct full file path: outputDir/<url_hash>.md
	filename := urlHash + ".md"
	fullPath := filepath.Join(outputDir, filename)

	// Write content to file
	content := normalizedDoc.Content()
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		cause := ErrCauseWriteFailure
		retryable := false
		// Check if it's a disk full error (ENOSPC)
		if errors.Is(err, syscall.ENOSPC) {
			cause = ErrCauseDiskFull
			retryable = true // disk full is retryable
		}
		return WriteResult{}, &StorageError{
			Message:   err.Error(),
			Retryable: retryable,
			Cause:     cause,
			Path:      fullPath,
		}
	}

	// Get content hash from frontmatter
	contentHash := normalizedDoc.Frontmatter().ContentHash()

	// Construct WriteResult
	writeResult := NewWriteResult(urlHash, fullPath, contentHash)
	return writeResult, nil
}
