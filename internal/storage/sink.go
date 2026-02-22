package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
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
	debugLogger  debug.DebugLogger
}

func NewLocalSink(
	metadataSink metadata.MetadataSink,
) *LocalSink {
	return &LocalSink{
		metadataSink: metadataSink,
		debugLogger:  debug.NewNoOpLogger(),
	}
}

// SetDebugLogger sets the debug logger for the sink.
// This is optional and defaults to NoOpLogger.
// If logger is nil, NoOpLogger is used as a safe default.
func (s *LocalSink) SetDebugLogger(logger debug.DebugLogger) {
	if logger == nil {
		s.debugLogger = debug.NewNoOpLogger()
		return
	}
	s.debugLogger = logger
}

func (s *LocalSink) Write(
	outputDir string,
	normalizedDoc normalize.NormalizedMarkdownDoc,
	hashAlgo hashutil.HashAlgo,
) (WriteResult, failure.ClassifiedError) {
	writeResult, err := write(outputDir, normalizedDoc, hashAlgo, s.debugLogger)
	if err != nil {
		var storageError *StorageError
		errors.As(err, &storageError)
		s.metadataSink.RecordError(metadata.NewErrorRecord(
			time.Now(),
			"storage",
			"LocalSink.Write",
			mapStorageErrorToMetadataCause(storageError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, normalizedDoc.Frontmatter().SourceURL()),
				metadata.NewAttr(metadata.AttrWritePath, storageError.Path),
			},
		))
		return WriteResult{}, storageError
	}
	s.metadataSink.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown,
		writeResult.Path(),
		normalizedDoc.Frontmatter().SourceURL(),
		writeResult.ContentHash(),
		false,
		int64(len(normalizedDoc.Content())),
		time.Now(),
	))
	return writeResult, nil
}

func write(
	outputDir string,
	normalizedDoc normalize.NormalizedMarkdownDoc,
	hashAlgo hashutil.HashAlgo,
	logger debug.DebugLogger,
) (WriteResult, failure.ClassifiedError) {
	// Get canonical URL for filename hashing (per filename-invariants.md)
	canonicalURL := normalizedDoc.Frontmatter().CanonicalURL()

	// Log hash computation step
	if logger.Enabled() {
		logger.LogStep(context.TODO(), "storage", "compute_hash", debug.FieldMap{
			"canonical_url": canonicalURL,
			"hash_algo":     string(hashAlgo),
		})
	}

	// Hash the canonical URL using specified algorithm
	urlHashFull, err := hashutil.HashBytes([]byte(canonicalURL), hashAlgo)
	if err != nil {
		// Log hash computation failure
		if logger.Enabled() {
			logger.LogStep(context.TODO(), "storage", "write_failed", debug.FieldMap{
				"error_cause": string(ErrCauseHashComputationFailed),
				"error_msg":   err.Error(),
			})
		}
		return WriteResult{}, NewStorageError(
			ErrCauseHashComputationFailed,
			err.Error(),
			"",
		)
	}

	// Use first 12 hex characters for filename (per user's requirement)
	urlHash := urlHashFull[:12]

	// Log ensure directory step
	if logger.Enabled() {
		logger.LogStep(context.TODO(), "storage", "ensure_dir", debug.FieldMap{
			"output_dir": outputDir,
		})
	}

	// Prepare output directory
	if err := fileutil.EnsureDir(outputDir); err != nil {
		var fileErr *fileutil.FileError
		if errors.As(err, &fileErr) {
			var cause StorageErrorCause
			if fileErr.Cause == fileutil.ErrCausePathError {
				// Could be disk full or permission issue
				cause = ErrCausePathError
			} else {
				cause = ErrCauseWriteFailure
			}
			// Log directory creation failure
			if logger.Enabled() {
				logger.LogStep(context.TODO(), "storage", "write_failed", debug.FieldMap{
					"output_dir":  outputDir,
					"error_cause": string(cause),
					"error_msg":   err.Error(),
				})
			}
			return WriteResult{}, NewStorageError(
				cause,
				err.Error(),
				outputDir,
			)
		}
		// Log directory creation failure
		if logger.Enabled() {
			logger.LogStep(context.TODO(), "storage", "write_failed", debug.FieldMap{
				"output_dir":  outputDir,
				"error_cause": string(ErrCauseWriteFailure),
				"error_msg":   err.Error(),
			})
		}
		return WriteResult{}, NewStorageError(
			ErrCauseWriteFailure,
			err.Error(),
			outputDir,
		)
	}

	// Construct full file path: outputDir/<url_hash>.md
	filename := urlHash + ".md"
	fullPath := filepath.Join(outputDir, filename)

	// Write content to file
	content := normalizedDoc.Content()
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		var cause StorageErrorCause
		// Check if it's a disk full error (ENOSPC)
		if errors.Is(err, syscall.ENOSPC) {
			cause = ErrCauseDiskFull
		} else {
			cause = ErrCauseWriteFailure
		}
		// Log write failure
		if logger.Enabled() {
			logger.LogStep(context.TODO(), "storage", "write_failed", debug.FieldMap{
				"file_path":   fullPath,
				"error_cause": string(cause),
				"error_msg":   err.Error(),
			})
		}
		return WriteResult{}, NewStorageError(
			cause,
			err.Error(),
			fullPath,
		)
	}

	// Get content hash from frontmatter
	contentHash := normalizedDoc.Frontmatter().ContentHash()

	// Construct WriteResult
	writeResult := NewWriteResult(urlHash, fullPath, contentHash)

	// Log successful write
	if logger.Enabled() {
		logger.LogStep(context.TODO(), "storage", "write_file", debug.FieldMap{
			"file_path":  fullPath,
			"size_bytes": len(content),
			"url_hash":   urlHash,
		})
	}

	return writeResult, nil
}
