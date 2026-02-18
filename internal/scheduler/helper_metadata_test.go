package scheduler_test

import (
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

// errorRecordingSink is a test double that counts errors
type errorRecordingSink struct {
	errorCount int
}

var _ metadata.MetadataSink = (*errorRecordingSink)(nil)

func (e *errorRecordingSink) RecordError(record metadata.ErrorRecord) {
	e.errorCount++
}

func (e *errorRecordingSink) RecordFetch(event metadata.FetchEvent) {}

func (e *errorRecordingSink) RecordArtifact(record metadata.ArtifactRecord) {}

func (e *errorRecordingSink) RecordPipelineStage(event metadata.PipelineEvent) {}

func (e *errorRecordingSink) RecordSkip(event metadata.SkipEvent) {}
