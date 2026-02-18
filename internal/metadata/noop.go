package metadata

import "time"

// NoopSink implements MetadataSink with empty bodies.
// The scheduler (or a test) can inject NoopSink when metadata recording is
// not required, keeping metadata orthogonal to crawl control flow.
type NoopSink struct{}

func (n *NoopSink) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause ErrorCause,
	details string,
	attrs []Attribute,
) {
}

func (n *NoopSink) RecordFetch(event FetchEvent) {}

func (n *NoopSink) RecordArtifact(record ArtifactRecord) {}

func (n *NoopSink) RecordPipelineStage(event PipelineEvent) {}

func (n *NoopSink) RecordSkip(event SkipEvent) {}

var _ MetadataSink = (*NoopSink)(nil)
