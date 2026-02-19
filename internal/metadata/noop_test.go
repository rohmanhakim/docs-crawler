package metadata_test

import (
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

// compile-time check: NoopSink satisfies MetadataSink interface.
var _ metadata.MetadataSink = (*metadata.NoopSink)(nil)

// TestNoopSink_InterfaceCompliance verifies compile-time interface compliance.
func TestNoopSink_InterfaceCompliance(t *testing.T) {
	// This test exists to make the compile-time check visible in test output.
	// The actual check is at package level: var _ metadata.MetadataSink = (*NoopSink)(nil)
	var sink metadata.MetadataSink = &metadata.NoopSink{}
	_ = sink
}

// TestNoopSink_RecordFetch verifies that RecordFetch executes without panic
// and produces no side effects. Since NoopSink is an empty sink, the method
// should silently succeed regardless of input.
func TestNoopSink_RecordFetch(t *testing.T) {
	sink := &metadata.NoopSink{}
	now := time.Now()

	// Should not panic with valid event
	event := metadata.NewFetchEvent(
		now, "https://example.com/page", 200, time.Second,
		"text/html", 0, 1, metadata.KindPage,
	)
	sink.RecordFetch(event)

	// Should not panic with different fetch kinds
	assetEvent := metadata.NewFetchEvent(
		now, "https://example.com/asset.png", 200, time.Millisecond,
		"image/png", 1, 2, metadata.KindAsset,
	)
	sink.RecordFetch(assetEvent)

	robotsEvent := metadata.NewFetchEvent(
		now, "https://example.com/robots.txt", 200, time.Millisecond,
		"text/plain", 0, 0, metadata.KindRobots,
	)
	sink.RecordFetch(robotsEvent)

	// Should not panic with zero values
	zeroEvent := metadata.NewFetchEvent(
		time.Time{}, "", 0, 0, "", 0, 0, "",
	)
	sink.RecordFetch(zeroEvent)
}

// TestNoopSink_RecordArtifact verifies that RecordArtifact executes without panic
// and produces no side effects. Since NoopSink is an empty sink, the method
// should silently succeed regardless of input.
func TestNoopSink_RecordArtifact(t *testing.T) {
	sink := &metadata.NoopSink{}
	now := time.Now()

	// Should not panic with markdown artifact
	markdown := metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown, "/out/page.md",
		"https://example.com/page", "hash123", false, 512, now,
	)
	sink.RecordArtifact(markdown)

	// Should not panic with asset artifact
	asset := metadata.NewArtifactRecord(
		metadata.ArtifactAsset, "/out/assets/image.png",
		"https://example.com/image.png", "hash456", true, 1024, now,
	)
	sink.RecordArtifact(asset)

	// Should not panic with zero values
	zeroArtifact := metadata.NewArtifactRecord(
		"", "", "", "", false, 0, time.Time{},
	)
	sink.RecordArtifact(zeroArtifact)
}

// TestNoopSink_RecordPipelineStage verifies that RecordPipelineStage executes without panic
// and produces no side effects. Since NoopSink is an empty sink, the method
// should silently succeed regardless of input.
func TestNoopSink_RecordPipelineStage(t *testing.T) {
	sink := &metadata.NoopSink{}
	now := time.Now()

	// Should not panic with all pipeline stages
	stages := []metadata.PipelineStage{
		metadata.StageExtract,
		metadata.StageSanitize,
		metadata.StageConvert,
		metadata.StageNormalize,
	}

	for _, stage := range stages {
		event := metadata.NewPipelineEvent(
			stage, "https://example.com/page", true, now, 5,
		)
		sink.RecordPipelineStage(event)
	}

	// Should not panic with failed stage
	failedEvent := metadata.NewPipelineEvent(
		metadata.StageExtract, "https://example.com/failed", false, now, 0,
	)
	sink.RecordPipelineStage(failedEvent)

	// Should not panic with zero values
	zeroEvent := metadata.NewPipelineEvent(
		"", "", false, time.Time{}, 0,
	)
	sink.RecordPipelineStage(zeroEvent)
}

// TestNoopSink_RecordSkip verifies that RecordSkip executes without panic
// and produces no side effects. Since NoopSink is an empty sink, the method
// should silently succeed regardless of input.
func TestNoopSink_RecordSkip(t *testing.T) {
	sink := &metadata.NoopSink{}
	now := time.Now()

	// Should not panic with all skip reasons
	reasons := []metadata.SkipReason{
		metadata.SkipReasonRobotsDisallow,
		metadata.SkipReasonOutOfScope,
		metadata.SkipReasonAlreadyVisited,
	}

	for _, reason := range reasons {
		event := metadata.NewSkipEvent(
			"https://example.com/skip", reason, now,
		)
		sink.RecordSkip(event)
	}

	// Should not panic with zero values
	zeroEvent := metadata.NewSkipEvent("", "", time.Time{})
	sink.RecordSkip(zeroEvent)
}

// TestNoopSink_RecordError verifies that RecordError executes without panic
// and produces no side effects. Since NoopSink is an empty sink, the method
// should silently succeed regardless of input.
func TestNoopSink_RecordError(t *testing.T) {
	sink := &metadata.NoopSink{}
	now := time.Now()

	// Should not panic with various error causes
	causes := []metadata.ErrorCause{
		metadata.CauseUnknown,
		metadata.CauseNetworkFailure,
		metadata.CausePolicyDisallow,
		metadata.CauseContentInvalid,
		metadata.CauseStorageFailure,
		metadata.CauseInvariantViolation,
		metadata.CauseRetryFailure,
	}

	for _, cause := range causes {
		record := metadata.NewErrorRecord(
			now, "package", "action", cause, "error message",
			[]metadata.Attribute{metadata.NewAttr(metadata.AttrURL, "https://example.com")},
		)
		sink.RecordError(record)
	}

	// Should not panic with attributes
	withAttrs := metadata.NewErrorRecord(
		now, "fetcher", "Fetch", metadata.CauseNetworkFailure,
		"connection refused",
		[]metadata.Attribute{
			metadata.NewAttr(metadata.AttrURL, "https://example.com"),
			metadata.NewAttr(metadata.AttrHTTPStatus, "503"),
			metadata.NewAttr(metadata.AttrDepth, "2"),
		},
	)
	sink.RecordError(withAttrs)

	// Should not panic with nil attributes
	nilAttrs := metadata.NewErrorRecord(
		now, "package", "action", metadata.CauseUnknown, "error", nil,
	)
	sink.RecordError(nilAttrs)

	// Should not panic with zero values
	zeroRecord := metadata.NewErrorRecord(
		time.Time{}, "", "", 0, "", nil,
	)
	sink.RecordError(zeroRecord)
}

// TestNoopSink_AllMethods verifies that all MetadataSink methods can be called
// in sequence without panic, confirming the sink is a true no-op.
func TestNoopSink_AllMethods(t *testing.T) {
	sink := &metadata.NoopSink{}
	now := time.Now()

	// Call all methods - none should panic
	sink.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com", 200, time.Second,
		"text/html", 0, 1, metadata.KindPage,
	))

	sink.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown, "/out/page.md",
		"https://example.com", "hash", false, 512, now,
	))

	sink.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageExtract, "https://example.com", true, now, 5,
	))

	sink.RecordSkip(metadata.NewSkipEvent(
		"https://example.com/skip", metadata.SkipReasonRobotsDisallow, now,
	))

	sink.RecordError(metadata.NewErrorRecord(
		now, "package", "action", metadata.CauseUnknown, "error", nil,
	))
}

// TestNoopSink_NilReceiver verifies that NoopSink methods can be called
// with a nil receiver without panic. This is valid for empty structs
// with no fields.
func TestNoopSink_NilReceiver(t *testing.T) {
	var sink *metadata.NoopSink
	now := time.Now()

	// All methods should work even with nil receiver
	// (valid for structs with no fields)
	sink.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com", 200, time.Second,
		"text/html", 0, 1, metadata.KindPage,
	))

	sink.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown, "/out/page.md",
		"https://example.com", "hash", false, 512, now,
	))

	sink.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageExtract, "https://example.com", true, now, 5,
	))

	sink.RecordSkip(metadata.NewSkipEvent(
		"https://example.com/skip", metadata.SkipReasonRobotsDisallow, now,
	))

	sink.RecordError(metadata.NewErrorRecord(
		now, "package", "action", metadata.CauseUnknown, "error", nil,
	))
}

// TestNoopSink_EmptySink confirms that NoopSink is truly an empty sink
// with no observable state or side effects. Multiple calls should have
// no cumulative effect.
func TestNoopSink_EmptySink(t *testing.T) {
	sink := &metadata.NoopSink{}
	now := time.Now()

	// Make multiple calls to each method
	for i := 0; i < 10; i++ {
		sink.RecordFetch(metadata.NewFetchEvent(
			now, "https://example.com", 200, time.Second,
			"text/html", 0, 1, metadata.KindPage,
		))
		sink.RecordArtifact(metadata.NewArtifactRecord(
			metadata.ArtifactMarkdown, "/out/page.md",
			"https://example.com", "hash", false, 512, now,
		))
		sink.RecordPipelineStage(metadata.NewPipelineEvent(
			metadata.StageExtract, "https://example.com", true, now, 5,
		))
		sink.RecordSkip(metadata.NewSkipEvent(
			"https://example.com/skip", metadata.SkipReasonRobotsDisallow, now,
		))
		sink.RecordError(metadata.NewErrorRecord(
			now, "package", "action", metadata.CauseUnknown, "error", nil,
		))
	}

	// No assertions needed - if we reach here, NoopSink is truly empty
	// and does not accumulate state or panic on repeated calls.
}
