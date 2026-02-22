package storage

import (
	"context"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
)

/*
DryRunSink is a storage implementation that simulates writes without
touching the filesystem.

It follows the same contract as Sink:
- Computes deterministic filename (same as LocalSink)
- Records artifact metadata
- Returns WriteResult

This allows the scheduler to run unchanged while suppressing
the irreversible disk write side effect.
*/

type DryRunSink struct {
	metadataSink metadata.MetadataSink
	debugLogger  debug.DebugLogger
}

func NewDryRunSink(
	metadataSink metadata.MetadataSink,
) *DryRunSink {
	return &DryRunSink{
		metadataSink: metadataSink,
		debugLogger:  debug.NewNoOpLogger(),
	}
}

// SetDebugLogger sets the debug logger for the sink.
// This is optional and defaults to NoOpLogger.
func (d *DryRunSink) SetDebugLogger(logger debug.DebugLogger) {
	d.debugLogger = logger
}

// Write simulates a storage write without touching the filesystem.
// It computes the deterministic filename and records artifact metadata.
func (d *DryRunSink) Write(
	outputDir string,
	normalizedDoc normalize.NormalizedMarkdownDoc,
	hashAlgo hashutil.HashAlgo,
) (WriteResult, failure.ClassifiedError) {
	// Get canonical URL for filename hashing
	canonicalURL := normalizedDoc.Frontmatter().CanonicalURL()

	// Log hash computation step
	if d.debugLogger.Enabled() {
		d.debugLogger.LogStep(context.TODO(), "storage", "compute_hash", debug.FieldMap{
			"canonical_url": canonicalURL,
			"hash_algo":     string(hashAlgo),
		})
	}

	// Compute the same URL hash as LocalSink would
	urlHashFull, err := hashutil.HashBytes([]byte(canonicalURL), hashAlgo)
	if err != nil {
		// Log hash computation failure
		if d.debugLogger.Enabled() {
			d.debugLogger.LogStep(context.TODO(), "storage", "write_failed", debug.FieldMap{
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

	// Use first 12 hex characters for filename (same as LocalSink)
	urlHash := urlHashFull[:12]

	// Construct the full path (without actually writing)
	filename := urlHash + ".md"
	fullPath := filename // Relative path in dry-run mode

	// Get content hash from frontmatter
	contentHash := normalizedDoc.Frontmatter().ContentHash()

	// Create the WriteResult (same as LocalSink)
	writeResult := NewWriteResult(urlHash, fullPath, contentHash)

	// Log dry-run write
	if d.debugLogger.Enabled() {
		d.debugLogger.LogStep(context.TODO(), "storage", "dryrun_write", debug.FieldMap{
			"file_path":  fullPath,
			"size_bytes": len(normalizedDoc.Content()),
			"url_hash":   urlHash,
			"dry_run":    true,
		})
	}

	// Record artifact metadata (same as LocalSink would)
	// overwrite = false because no actual file is written in dry-run mode
	d.metadataSink.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown,
		writeResult.Path(),
		normalizedDoc.Frontmatter().SourceURL(),
		writeResult.ContentHash(),
		false, // overwrite = false (no actual file written in dry-run)
		int64(len(normalizedDoc.Content())),
		time.Now(),
	))

	return writeResult, nil
}
