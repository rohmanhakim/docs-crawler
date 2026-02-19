package metadatatest_test

import (
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/metadata/metadatatest"
)

// compile-time check: SinkMock satisfies MetadataSink interface.
var _ metadata.MetadataSink = (*metadatatest.SinkMock)(nil)

// TestSinkMock_RecordFetch verifies that RecordFetch sets the called flag
// and appends the event to the FetchEvents slice.
func TestSinkMock_RecordFetch(t *testing.T) {
	mock := &metadatatest.SinkMock{}
	now := time.Now()

	// Initial state
	if mock.RecordFetchCalled {
		t.Error("RecordFetchCalled should be false initially")
	}
	if len(mock.FetchEvents) != 0 {
		t.Errorf("FetchEvents len = %d, want 0", len(mock.FetchEvents))
	}

	// Record first event
	event1 := metadata.NewFetchEvent(
		now, "https://example.com/page1", 200, time.Second,
		"text/html", 0, 1, metadata.KindPage,
	)
	mock.RecordFetch(event1)

	if !mock.RecordFetchCalled {
		t.Error("RecordFetchCalled should be true after RecordFetch")
	}
	if len(mock.FetchEvents) != 1 {
		t.Errorf("FetchEvents len = %d, want 1", len(mock.FetchEvents))
	}
	if mock.FetchEvents[0].FetchURL() != "https://example.com/page1" {
		t.Errorf("FetchEvents[0].FetchURL() = %v, want https://example.com/page1",
			mock.FetchEvents[0].FetchURL())
	}

	// Record second event
	event2 := metadata.NewFetchEvent(
		now, "https://example.com/asset1", 200, time.Millisecond,
		"image/png", 1, 2, metadata.KindAsset,
	)
	mock.RecordFetch(event2)

	if len(mock.FetchEvents) != 2 {
		t.Errorf("FetchEvents len = %d, want 2", len(mock.FetchEvents))
	}
	if mock.FetchEvents[1].Kind() != metadata.KindAsset {
		t.Errorf("FetchEvents[1].Kind() = %v, want %v",
			mock.FetchEvents[1].Kind(), metadata.KindAsset)
	}
}

// TestSinkMock_RecordArtifact verifies that RecordArtifact sets the called flag
// and appends the record to the Artifacts slice.
func TestSinkMock_RecordArtifact(t *testing.T) {
	mock := &metadatatest.SinkMock{}
	now := time.Now()

	// Initial state
	if mock.RecordArtifactCalled {
		t.Error("RecordArtifactCalled should be false initially")
	}
	if len(mock.Artifacts) != 0 {
		t.Errorf("Artifacts len = %d, want 0", len(mock.Artifacts))
	}

	// Record first artifact
	artifact1 := metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown, "/out/page.md",
		"https://example.com/page", "hash123", false, 512, now,
	)
	mock.RecordArtifact(artifact1)

	if !mock.RecordArtifactCalled {
		t.Error("RecordArtifactCalled should be true after RecordArtifact")
	}
	if len(mock.Artifacts) != 1 {
		t.Errorf("Artifacts len = %d, want 1", len(mock.Artifacts))
	}
	if mock.Artifacts[0].WritePath() != "/out/page.md" {
		t.Errorf("Artifacts[0].WritePath() = %v, want /out/page.md",
			mock.Artifacts[0].WritePath())
	}

	// Record second artifact
	artifact2 := metadata.NewArtifactRecord(
		metadata.ArtifactAsset, "/out/assets/image.png",
		"https://example.com/image.png", "hash456", true, 1024, now,
	)
	mock.RecordArtifact(artifact2)

	if len(mock.Artifacts) != 2 {
		t.Errorf("Artifacts len = %d, want 2", len(mock.Artifacts))
	}
	if mock.Artifacts[1].Kind() != metadata.ArtifactAsset {
		t.Errorf("Artifacts[1].Kind() = %v, want %v",
			mock.Artifacts[1].Kind(), metadata.ArtifactAsset)
	}
}

// TestSinkMock_RecordPipelineStage verifies that RecordPipelineStage sets the called flag
// and appends the event to the PipelineEvents slice.
func TestSinkMock_RecordPipelineStage(t *testing.T) {
	mock := &metadatatest.SinkMock{}
	now := time.Now()

	// Initial state
	if mock.RecordPipelineCalled {
		t.Error("RecordPipelineCalled should be false initially")
	}
	if len(mock.PipelineEvents) != 0 {
		t.Errorf("PipelineEvents len = %d, want 0", len(mock.PipelineEvents))
	}

	// Record first pipeline event
	event1 := metadata.NewPipelineEvent(
		metadata.StageExtract, "https://example.com/page", true, now, 7,
	)
	mock.RecordPipelineStage(event1)

	if !mock.RecordPipelineCalled {
		t.Error("RecordPipelineCalled should be true after RecordPipelineStage")
	}
	if len(mock.PipelineEvents) != 1 {
		t.Errorf("PipelineEvents len = %d, want 1", len(mock.PipelineEvents))
	}
	if mock.PipelineEvents[0].Stage() != metadata.StageExtract {
		t.Errorf("PipelineEvents[0].Stage() = %v, want %v",
			mock.PipelineEvents[0].Stage(), metadata.StageExtract)
	}

	// Record second pipeline event
	event2 := metadata.NewPipelineEvent(
		metadata.StageSanitize, "https://example.com/page", true, now, 0,
	)
	mock.RecordPipelineStage(event2)

	if len(mock.PipelineEvents) != 2 {
		t.Errorf("PipelineEvents len = %d, want 2", len(mock.PipelineEvents))
	}
	if !mock.PipelineEvents[0].Success() {
		t.Error("PipelineEvents[0].Success() should be true")
	}
}

// TestSinkMock_RecordSkip verifies that RecordSkip sets the called flag
// and appends the event to the SkipEvents slice.
func TestSinkMock_RecordSkip(t *testing.T) {
	mock := &metadatatest.SinkMock{}
	now := time.Now()

	// Initial state
	if mock.RecordSkipCalled {
		t.Error("RecordSkipCalled should be false initially")
	}
	if len(mock.SkipEvents) != 0 {
		t.Errorf("SkipEvents len = %d, want 0", len(mock.SkipEvents))
	}

	// Record first skip event
	skip1 := metadata.NewSkipEvent(
		"https://example.com/disallowed",
		metadata.SkipReasonRobotsDisallow,
		now,
	)
	mock.RecordSkip(skip1)

	if !mock.RecordSkipCalled {
		t.Error("RecordSkipCalled should be true after RecordSkip")
	}
	if len(mock.SkipEvents) != 1 {
		t.Errorf("SkipEvents len = %d, want 1", len(mock.SkipEvents))
	}
	if mock.SkipEvents[0].Reason() != metadata.SkipReasonRobotsDisallow {
		t.Errorf("SkipEvents[0].Reason() = %v, want %v",
			mock.SkipEvents[0].Reason(), metadata.SkipReasonRobotsDisallow)
	}

	// Record second skip event
	skip2 := metadata.NewSkipEvent(
		"https://example.com/out-of-scope",
		metadata.SkipReasonOutOfScope,
		now,
	)
	mock.RecordSkip(skip2)

	if len(mock.SkipEvents) != 2 {
		t.Errorf("SkipEvents len = %d, want 2", len(mock.SkipEvents))
	}
	if mock.SkipEvents[1].SkippedURL() != "https://example.com/out-of-scope" {
		t.Errorf("SkipEvents[1].SkippedURL() = %v, want https://example.com/out-of-scope",
			mock.SkipEvents[1].SkippedURL())
	}
}

// TestSinkMock_RecordError verifies that RecordError sets the called flag
// and appends the record to the ErrorRecords slice.
func TestSinkMock_RecordError(t *testing.T) {
	mock := &metadatatest.SinkMock{}
	now := time.Now()

	// Initial state
	if mock.RecordErrorCalled {
		t.Error("RecordErrorCalled should be false initially")
	}
	if len(mock.ErrorRecords) != 0 {
		t.Errorf("ErrorRecords len = %d, want 0", len(mock.ErrorRecords))
	}

	// Record first error
	err1 := metadata.NewErrorRecord(
		now, "fetcher", "Fetch",
		metadata.CauseNetworkFailure,
		"connection refused",
		[]metadata.Attribute{metadata.NewAttr(metadata.AttrURL, "https://example.com")},
	)
	mock.RecordError(err1)

	if !mock.RecordErrorCalled {
		t.Error("RecordErrorCalled should be true after RecordError")
	}
	if len(mock.ErrorRecords) != 1 {
		t.Errorf("ErrorRecords len = %d, want 1", len(mock.ErrorRecords))
	}
	if mock.ErrorRecords[0].PackageName() != "fetcher" {
		t.Errorf("ErrorRecords[0].PackageName() = %v, want fetcher",
			mock.ErrorRecords[0].PackageName())
	}

	// Record second error
	err2 := metadata.NewErrorRecord(
		now, "storage", "Write",
		metadata.CauseStorageFailure,
		"disk full",
		nil,
	)
	mock.RecordError(err2)

	if len(mock.ErrorRecords) != 2 {
		t.Errorf("ErrorRecords len = %d, want 2", len(mock.ErrorRecords))
	}
	if mock.ErrorRecords[1].Cause() != metadata.CauseStorageFailure {
		t.Errorf("ErrorRecords[1].Cause() = %v, want %v",
			mock.ErrorRecords[1].Cause(), metadata.CauseStorageFailure)
	}
}

// TestSinkMock_Reset verifies that Reset clears all recorded state.
func TestSinkMock_Reset(t *testing.T) {
	mock := &metadatatest.SinkMock{}
	now := time.Now()

	// Record some events
	mock.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com", 200, time.Second,
		"text/html", 0, 1, metadata.KindPage,
	))
	mock.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown, "/out/page.md",
		"https://example.com", "hash", false, 512, now,
	))
	mock.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageExtract, "https://example.com", true, now, 3,
	))
	mock.RecordSkip(metadata.NewSkipEvent(
		"https://example.com/skip", metadata.SkipReasonRobotsDisallow, now,
	))
	mock.RecordError(metadata.NewErrorRecord(
		now, "pkg", "action", metadata.CauseUnknown, "error", nil,
	))

	// Verify state before reset
	if !mock.RecordFetchCalled || !mock.RecordArtifactCalled ||
		!mock.RecordPipelineCalled || !mock.RecordSkipCalled ||
		!mock.RecordErrorCalled {
		t.Fatal("all called flags should be true before reset")
	}

	// Reset
	mock.Reset()

	// Verify state after reset
	if mock.RecordFetchCalled {
		t.Error("RecordFetchCalled should be false after Reset")
	}
	if mock.RecordArtifactCalled {
		t.Error("RecordArtifactCalled should be false after Reset")
	}
	if mock.RecordPipelineCalled {
		t.Error("RecordPipelineCalled should be false after Reset")
	}
	if mock.RecordSkipCalled {
		t.Error("RecordSkipCalled should be false after Reset")
	}
	if mock.RecordErrorCalled {
		t.Error("RecordErrorCalled should be false after Reset")
	}
	if len(mock.FetchEvents) != 0 {
		t.Errorf("FetchEvents len = %d, want 0 after Reset", len(mock.FetchEvents))
	}
	if len(mock.Artifacts) != 0 {
		t.Errorf("Artifacts len = %d, want 0 after Reset", len(mock.Artifacts))
	}
	if len(mock.PipelineEvents) != 0 {
		t.Errorf("PipelineEvents len = %d, want 0 after Reset", len(mock.PipelineEvents))
	}
	if len(mock.SkipEvents) != 0 {
		t.Errorf("SkipEvents len = %d, want 0 after Reset", len(mock.SkipEvents))
	}
	if len(mock.ErrorRecords) != 0 {
		t.Errorf("ErrorRecords len = %d, want 0 after Reset", len(mock.ErrorRecords))
	}
}

// TestSinkMock_Reset_AllowsReuse verifies that a mock can be reused after Reset.
func TestSinkMock_Reset_AllowsReuse(t *testing.T) {
	mock := &metadatatest.SinkMock{}
	now := time.Now()

	// First use
	mock.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com/first", 200, time.Second,
		"text/html", 0, 1, metadata.KindPage,
	))

	// Reset
	mock.Reset()

	// Second use - should work like a fresh mock
	mock.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com/second", 404, time.Millisecond,
		"text/html", 0, 2, metadata.KindPage,
	))

	if len(mock.FetchEvents) != 1 {
		t.Errorf("FetchEvents len = %d, want 1 after reuse", len(mock.FetchEvents))
	}
	if mock.FetchEvents[0].FetchURL() != "https://example.com/second" {
		t.Errorf("FetchEvents[0].FetchURL() = %v, want https://example.com/second",
			mock.FetchEvents[0].FetchURL())
	}
}

// TestSinkMock_LastFetch verifies LastFetch returns the most recent event or nil.
func TestSinkMock_LastFetch(t *testing.T) {
	mock := &metadatatest.SinkMock{}

	// Empty mock returns nil
	if last := mock.LastFetch(); last != nil {
		t.Errorf("LastFetch() = %v, want nil for empty mock", last)
	}

	now := time.Now()

	// One event
	mock.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com/first", 200, time.Second,
		"text/html", 0, 1, metadata.KindPage,
	))
	last := mock.LastFetch()
	if last == nil {
		t.Fatal("LastFetch() = nil, want non-nil")
	}
	if last.FetchURL() != "https://example.com/first" {
		t.Errorf("LastFetch().FetchURL() = %v, want https://example.com/first",
			last.FetchURL())
	}

	// Multiple events - returns most recent
	mock.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com/second", 200, time.Second,
		"text/html", 0, 2, metadata.KindPage,
	))
	last = mock.LastFetch()
	if last == nil {
		t.Fatal("LastFetch() = nil, want non-nil")
	}
	if last.FetchURL() != "https://example.com/second" {
		t.Errorf("LastFetch().FetchURL() = %v, want https://example.com/second",
			last.FetchURL())
	}
}

// TestSinkMock_LastArtifact verifies LastArtifact returns the most recent record or nil.
func TestSinkMock_LastArtifact(t *testing.T) {
	mock := &metadatatest.SinkMock{}

	// Empty mock returns nil
	if last := mock.LastArtifact(); last != nil {
		t.Errorf("LastArtifact() = %v, want nil for empty mock", last)
	}

	now := time.Now()

	// One artifact
	mock.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown, "/out/first.md",
		"https://example.com/first", "hash1", false, 100, now,
	))
	last := mock.LastArtifact()
	if last == nil {
		t.Fatal("LastArtifact() = nil, want non-nil")
	}
	if last.WritePath() != "/out/first.md" {
		t.Errorf("LastArtifact().WritePath() = %v, want /out/first.md",
			last.WritePath())
	}

	// Multiple artifacts - returns most recent
	mock.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown, "/out/second.md",
		"https://example.com/second", "hash2", false, 200, now,
	))
	last = mock.LastArtifact()
	if last == nil {
		t.Fatal("LastArtifact() = nil, want non-nil")
	}
	if last.WritePath() != "/out/second.md" {
		t.Errorf("LastArtifact().WritePath() = %v, want /out/second.md",
			last.WritePath())
	}
}

// TestSinkMock_LastPipelineEvent verifies LastPipelineEvent returns the most recent event or nil.
func TestSinkMock_LastPipelineEvent(t *testing.T) {
	mock := &metadatatest.SinkMock{}

	// Empty mock returns nil
	if last := mock.LastPipelineEvent(); last != nil {
		t.Errorf("LastPipelineEvent() = %v, want nil for empty mock", last)
	}

	now := time.Now()

	// One event
	mock.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageExtract, "https://example.com/page", true, now, 5,
	))
	last := mock.LastPipelineEvent()
	if last == nil {
		t.Fatal("LastPipelineEvent() = nil, want non-nil")
	}
	if last.Stage() != metadata.StageExtract {
		t.Errorf("LastPipelineEvent().Stage() = %v, want %v",
			last.Stage(), metadata.StageExtract)
	}

	// Multiple events - returns most recent
	mock.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageSanitize, "https://example.com/page", true, now, 0,
	))
	last = mock.LastPipelineEvent()
	if last == nil {
		t.Fatal("LastPipelineEvent() = nil, want non-nil")
	}
	if last.Stage() != metadata.StageSanitize {
		t.Errorf("LastPipelineEvent().Stage() = %v, want %v",
			last.Stage(), metadata.StageSanitize)
	}
}

// TestSinkMock_LastSkip verifies LastSkip returns the most recent event or nil.
func TestSinkMock_LastSkip(t *testing.T) {
	mock := &metadatatest.SinkMock{}

	// Empty mock returns nil
	if last := mock.LastSkip(); last != nil {
		t.Errorf("LastSkip() = %v, want nil for empty mock", last)
	}

	now := time.Now()

	// One event
	mock.RecordSkip(metadata.NewSkipEvent(
		"https://example.com/first", metadata.SkipReasonRobotsDisallow, now,
	))
	last := mock.LastSkip()
	if last == nil {
		t.Fatal("LastSkip() = nil, want non-nil")
	}
	if last.SkippedURL() != "https://example.com/first" {
		t.Errorf("LastSkip().SkippedURL() = %v, want https://example.com/first",
			last.SkippedURL())
	}

	// Multiple events - returns most recent
	mock.RecordSkip(metadata.NewSkipEvent(
		"https://example.com/second", metadata.SkipReasonOutOfScope, now,
	))
	last = mock.LastSkip()
	if last == nil {
		t.Fatal("LastSkip() = nil, want non-nil")
	}
	if last.SkippedURL() != "https://example.com/second" {
		t.Errorf("LastSkip().SkippedURL() = %v, want https://example.com/second",
			last.SkippedURL())
	}
}

// TestSinkMock_LastError verifies LastError returns the most recent record or nil.
func TestSinkMock_LastError(t *testing.T) {
	mock := &metadatatest.SinkMock{}

	// Empty mock returns nil
	if last := mock.LastError(); last != nil {
		t.Errorf("LastError() = %v, want nil for empty mock", last)
	}

	now := time.Now()

	// One error
	mock.RecordError(metadata.NewErrorRecord(
		now, "fetcher", "Fetch", metadata.CauseNetworkFailure, "first error", nil,
	))
	last := mock.LastError()
	if last == nil {
		t.Fatal("LastError() = nil, want non-nil")
	}
	if last.ErrorString() != "first error" {
		t.Errorf("LastError().ErrorString() = %v, want 'first error'",
			last.ErrorString())
	}

	// Multiple errors - returns most recent
	mock.RecordError(metadata.NewErrorRecord(
		now, "storage", "Write", metadata.CauseStorageFailure, "second error", nil,
	))
	last = mock.LastError()
	if last == nil {
		t.Fatal("LastError() = nil, want non-nil")
	}
	if last.ErrorString() != "second error" {
		t.Errorf("LastError().ErrorString() = %v, want 'second error'",
			last.ErrorString())
	}
}

// TestSinkMock_GetFetchRecords verifies GetFetchRecords returns all recorded events.
func TestSinkMock_GetFetchRecords(t *testing.T) {
	mock := &metadatatest.SinkMock{}

	// Empty mock returns nil slice (not empty non-nil)
	records := mock.GetFetchRecords()
	if records != nil {
		t.Errorf("GetFetchRecords() = %v, want nil for empty mock", records)
	}

	now := time.Now()

	// Add events
	mock.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com/first", 200, time.Second,
		"text/html", 0, 1, metadata.KindPage,
	))
	mock.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com/second", 200, time.Second,
		"text/html", 0, 2, metadata.KindPage,
	))

	records = mock.GetFetchRecords()
	if len(records) != 2 {
		t.Errorf("GetFetchRecords() len = %d, want 2", len(records))
	}
	if records[0].FetchURL() != "https://example.com/first" {
		t.Errorf("records[0].FetchURL() = %v, want https://example.com/first",
			records[0].FetchURL())
	}
	if records[1].FetchURL() != "https://example.com/second" {
		t.Errorf("records[1].FetchURL() = %v, want https://example.com/second",
			records[1].FetchURL())
	}
}

// TestSinkMock_GetArtifactRecords verifies GetArtifactRecords returns all recorded artifacts.
func TestSinkMock_GetArtifactRecords(t *testing.T) {
	mock := &metadatatest.SinkMock{}

	// Empty mock returns nil slice
	records := mock.GetArtifactRecords()
	if records != nil {
		t.Errorf("GetArtifactRecords() = %v, want nil for empty mock", records)
	}

	now := time.Now()

	// Add artifacts
	mock.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown, "/out/first.md",
		"https://example.com/first", "hash1", false, 100, now,
	))
	mock.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactAsset, "/out/asset.png",
		"https://example.com/asset", "hash2", true, 200, now,
	))

	records = mock.GetArtifactRecords()
	if len(records) != 2 {
		t.Errorf("GetArtifactRecords() len = %d, want 2", len(records))
	}
	if records[0].Kind() != metadata.ArtifactMarkdown {
		t.Errorf("records[0].Kind() = %v, want %v",
			records[0].Kind(), metadata.ArtifactMarkdown)
	}
	if records[1].Kind() != metadata.ArtifactAsset {
		t.Errorf("records[1].Kind() = %v, want %v",
			records[1].Kind(), metadata.ArtifactAsset)
	}
}

// TestSinkMock_GetErrorRecords verifies GetErrorRecords returns all recorded errors.
func TestSinkMock_GetErrorRecords(t *testing.T) {
	mock := &metadatatest.SinkMock{}

	// Empty mock returns nil slice
	records := mock.GetErrorRecords()
	if records != nil {
		t.Errorf("GetErrorRecords() = %v, want nil for empty mock", records)
	}

	now := time.Now()

	// Add errors
	mock.RecordError(metadata.NewErrorRecord(
		now, "fetcher", "Fetch", metadata.CauseNetworkFailure, "network error", nil,
	))
	mock.RecordError(metadata.NewErrorRecord(
		now, "storage", "Write", metadata.CauseStorageFailure, "disk error", nil,
	))

	records = mock.GetErrorRecords()
	if len(records) != 2 {
		t.Errorf("GetErrorRecords() len = %d, want 2", len(records))
	}
	if records[0].PackageName() != "fetcher" {
		t.Errorf("records[0].PackageName() = %v, want fetcher",
			records[0].PackageName())
	}
	if records[1].PackageName() != "storage" {
		t.Errorf("records[1].PackageName() = %v, want storage",
			records[1].PackageName())
	}
}

// TestSinkMock_InterfaceCompliance verifies compile-time interface compliance.
func TestSinkMock_InterfaceCompliance(t *testing.T) {
	// This test exists to make the compile-time check visible in test output.
	// The actual check is at package level: var _ metadata.MetadataSink = (*SinkMock)(nil)
	mock := &metadatatest.SinkMock{}
	var _ metadata.MetadataSink = mock
}

// TestSinkMock_MultipleRecordTypes verifies that different record types
// can be recorded independently without interference.
func TestSinkMock_MultipleRecordTypes(t *testing.T) {
	mock := &metadatatest.SinkMock{}
	now := time.Now()

	// Record different types
	mock.RecordFetch(metadata.NewFetchEvent(
		now, "https://example.com", 200, time.Second,
		"text/html", 0, 1, metadata.KindPage,
	))
	mock.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown, "/out/page.md",
		"https://example.com", "hash", false, 512, now,
	))
	mock.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageExtract, "https://example.com", true, now, 5,
	))
	mock.RecordSkip(metadata.NewSkipEvent(
		"https://example.com/skip", metadata.SkipReasonRobotsDisallow, now,
	))
	mock.RecordError(metadata.NewErrorRecord(
		now, "pkg", "action", metadata.CauseUnknown, "error", nil,
	))

	// Verify all called flags are set
	if !mock.RecordFetchCalled {
		t.Error("RecordFetchCalled should be true")
	}
	if !mock.RecordArtifactCalled {
		t.Error("RecordArtifactCalled should be true")
	}
	if !mock.RecordPipelineCalled {
		t.Error("RecordPipelineCalled should be true")
	}
	if !mock.RecordSkipCalled {
		t.Error("RecordSkipCalled should be true")
	}
	if !mock.RecordErrorCalled {
		t.Error("RecordErrorCalled should be true")
	}

	// Verify each slice has exactly one entry
	if len(mock.FetchEvents) != 1 {
		t.Errorf("FetchEvents len = %d, want 1", len(mock.FetchEvents))
	}
	if len(mock.Artifacts) != 1 {
		t.Errorf("Artifacts len = %d, want 1", len(mock.Artifacts))
	}
	if len(mock.PipelineEvents) != 1 {
		t.Errorf("PipelineEvents len = %d, want 1", len(mock.PipelineEvents))
	}
	if len(mock.SkipEvents) != 1 {
		t.Errorf("SkipEvents len = %d, want 1", len(mock.SkipEvents))
	}
	if len(mock.ErrorRecords) != 1 {
		t.Errorf("ErrorRecords len = %d, want 1", len(mock.ErrorRecords))
	}
}
