package scheduler_test

import (
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

// errorRecordingSink is a test double that counts errors
type errorRecordingSink struct {
	errorCount int
}

func (e *errorRecordingSink) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	details string,
	attrs []metadata.Attribute,
) {
	e.errorCount++
}

func (e *errorRecordingSink) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
}

func (e *errorRecordingSink) RecordArtifact(kind metadata.ArtifactKind, path string, attrs []metadata.Attribute) {
}
func (e *errorRecordingSink) RecordAssetFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	retryCount int,
) {
}
