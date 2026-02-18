package metadata_test

import (
	"sync"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

// compile-time check: Recorder satisfies both interfaces.
var _ metadata.MetadataSink = (*metadata.Recorder)(nil)
var _ metadata.CrawlFinalizer = (*metadata.Recorder)(nil)

// newTestRecorder returns a pointer to a fresh Recorder for use in tests.
func newTestRecorder(t *testing.T) *metadata.Recorder {
	t.Helper()
	r := metadata.NewRecorder("test-worker")
	return &r
}

// TestRecorder_EachMethodAppendsOneEvent verifies that each Record* method
// appends exactly one event with the correct EventKind to the log.
func TestRecorder_EachMethodAppendsOneEvent(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		record   func(r *metadata.Recorder)
		wantKind metadata.EventKind
		// verify checks the typed payload pointer on the event.
		verify func(t *testing.T, e metadata.Event)
	}{
		{
			name: "RecordFetch appends EventKindFetch",
			record: func(r *metadata.Recorder) {
				r.RecordFetch(metadata.NewFetchEvent(
					now, "https://example.com", 200, time.Second,
					"text/html", 0, 1, metadata.KindPage,
				))
			},
			wantKind: metadata.EventKindFetch,
			verify: func(t *testing.T, e metadata.Event) {
				t.Helper()
				if e.Fetch() == nil {
					t.Fatal("Event.Fetch() is nil, want non-nil")
				}
				if e.Fetch().Kind() != metadata.KindPage {
					t.Errorf("Fetch().Kind() = %v, want %v", e.Fetch().Kind(), metadata.KindPage)
				}
				if e.Fetch().FetchURL() != "https://example.com" {
					t.Errorf("Fetch().FetchURL() = %v, want https://example.com", e.Fetch().FetchURL())
				}
			},
		},
		{
			name: "RecordArtifact appends EventKindArtifact",
			record: func(r *metadata.Recorder) {
				r.RecordArtifact(metadata.NewArtifactRecord(
					metadata.ArtifactMarkdown, "/out/page.md",
					"https://example.com/page", "hash123", false, 512, now,
				))
			},
			wantKind: metadata.EventKindArtifact,
			verify: func(t *testing.T, e metadata.Event) {
				t.Helper()
				if e.Artifact() == nil {
					t.Fatal("Event.Artifact() is nil, want non-nil")
				}
				if e.Artifact().Kind() != metadata.ArtifactMarkdown {
					t.Errorf("Artifact().Kind() = %v, want %v", e.Artifact().Kind(), metadata.ArtifactMarkdown)
				}
				if e.Artifact().WritePath() != "/out/page.md" {
					t.Errorf("Artifact().WritePath() = %v, want /out/page.md", e.Artifact().WritePath())
				}
			},
		},
		{
			name: "RecordPipelineStage appends EventKindPipeline",
			record: func(r *metadata.Recorder) {
				r.RecordPipelineStage(metadata.NewPipelineEvent(
					metadata.StageExtract, "https://example.com/page", true, now, 7,
				))
			},
			wantKind: metadata.EventKindPipeline,
			verify: func(t *testing.T, e metadata.Event) {
				t.Helper()
				if e.Pipeline() == nil {
					t.Fatal("Event.Pipeline() is nil, want non-nil")
				}
				if e.Pipeline().Stage() != metadata.StageExtract {
					t.Errorf("Pipeline().Stage() = %v, want %v", e.Pipeline().Stage(), metadata.StageExtract)
				}
				if !e.Pipeline().Success() {
					t.Error("Pipeline().Success() = false, want true")
				}
				if e.Pipeline().LinksFound() != 7 {
					t.Errorf("Pipeline().LinksFound() = %v, want 7", e.Pipeline().LinksFound())
				}
			},
		},
		{
			name: "RecordSkip appends EventKindSkip",
			record: func(r *metadata.Recorder) {
				r.RecordSkip(metadata.NewSkipEvent(
					"https://example.com/disallowed",
					metadata.SkipReasonRobotsDisallow,
					now,
				))
			},
			wantKind: metadata.EventKindSkip,
			verify: func(t *testing.T, e metadata.Event) {
				t.Helper()
				if e.Skip() == nil {
					t.Fatal("Event.Skip() is nil, want non-nil")
				}
				if e.Skip().Reason() != metadata.SkipReasonRobotsDisallow {
					t.Errorf("Skip().Reason() = %v, want %v", e.Skip().Reason(), metadata.SkipReasonRobotsDisallow)
				}
				if e.Skip().SkippedURL() != "https://example.com/disallowed" {
					t.Errorf("Skip().SkippedURL() = %v, want https://example.com/disallowed", e.Skip().SkippedURL())
				}
			},
		},
		{
			name: "RecordError appends EventKindError",
			record: func(r *metadata.Recorder) {
				r.RecordError(
					now, "fetcher", "Fetch",
					metadata.CauseNetworkFailure,
					"connection refused",
					[]metadata.Attribute{metadata.NewAttr(metadata.AttrURL, "https://example.com")},
				)
			},
			wantKind: metadata.EventKindError,
			verify: func(t *testing.T, e metadata.Event) {
				t.Helper()
				if e.Error() == nil {
					t.Fatal("Event.Error() is nil, want non-nil")
				}
				if e.Error().PackageName() != "fetcher" {
					t.Errorf("Error().PackageName() = %v, want fetcher", e.Error().PackageName())
				}
				if e.Error().Cause() != metadata.CauseNetworkFailure {
					t.Errorf("Error().Cause() = %v, want %v", e.Error().Cause(), metadata.CauseNetworkFailure)
				}
				if len(e.Error().Attrs()) != 1 {
					t.Errorf("Error().Attrs() len = %v, want 1", len(e.Error().Attrs()))
				}
			},
		},
		{
			name: "RecordFinalCrawlStats appends EventKindStats",
			record: func(r *metadata.Recorder) {
				r.RecordFinalCrawlStats(metadata.NewCrawlStats(
					now, now.Add(5*time.Second), 10, 2, 5, 1,
				))
			},
			wantKind: metadata.EventKindStats,
			verify: func(t *testing.T, e metadata.Event) {
				t.Helper()
				if e.Stats() == nil {
					t.Fatal("Event.Stats() is nil, want non-nil")
				}
				if e.Stats().TotalPages() != 10 {
					t.Errorf("Stats().TotalPages() = %v, want 10", e.Stats().TotalPages())
				}
				if !e.Stats().FinishedAt().After(e.Stats().StartedAt()) {
					t.Error("Stats().FinishedAt() must be after StartedAt()")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRecorder(t)

			tt.record(r)

			events := r.Events()
			if len(events) != 1 {
				t.Fatalf("Events() len = %v, want 1", len(events))
			}
			if events[0].Kind() != tt.wantKind {
				t.Errorf("Event.Kind() = %v, want %v", events[0].Kind(), tt.wantKind)
			}
			tt.verify(t, events[0])
		})
	}
}

// TestRecorder_Accumulation verifies that calling multiple Record* methods
// results in the correct number of events in the correct order.
func TestRecorder_Accumulation(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com", 200, time.Second, "text/html", 0, 0, metadata.KindPage))
	r.RecordPipelineStage(metadata.NewPipelineEvent(metadata.StageExtract, "https://example.com", true, now, 3))
	r.RecordArtifact(metadata.NewArtifactRecord(metadata.ArtifactMarkdown, "/out/page.md", "https://example.com", "hash", false, 512, now))

	events := r.Events()
	if len(events) != 3 {
		t.Fatalf("Events() len = %v, want 3", len(events))
	}

	wantKinds := []metadata.EventKind{
		metadata.EventKindFetch,
		metadata.EventKindPipeline,
		metadata.EventKindArtifact,
	}
	for i, want := range wantKinds {
		if events[i].Kind() != want {
			t.Errorf("events[%d].Kind() = %v, want %v", i, events[i].Kind(), want)
		}
	}
}

// TestRecorder_SnapshotIsolation verifies that the slice returned by Events()
// is a copy — mutating it does not affect the recorder's internal log.
func TestRecorder_SnapshotIsolation(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com", 200, time.Second, "text/html", 0, 0, metadata.KindPage))
	r.RecordSkip(metadata.NewSkipEvent("https://example.com/skip", metadata.SkipReasonRobotsDisallow, now))

	snapshot := r.Events()
	if len(snapshot) != 2 {
		t.Fatalf("Events() len = %v, want 2", len(snapshot))
	}

	// Truncate the returned slice — internal log must be unaffected.
	snapshot = snapshot[:0]

	if len(r.Events()) != 2 {
		t.Errorf("Events() len after snapshot truncation = %v, want 2 (snapshot must be a copy)", len(r.Events()))
	}
}

// TestRecorder_ConcurrentSafety exercises concurrent Record* calls to detect
// data races under the -race flag.
func TestRecorder_ConcurrentSafety(t *testing.T) {
	const goroutines = 50
	now := time.Now()
	r := newTestRecorder(t)

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com", 200, time.Millisecond, "text/html", 0, 0, metadata.KindPage))
		}()
		go func() {
			defer wg.Done()
			r.RecordSkip(metadata.NewSkipEvent("https://example.com/skip", metadata.SkipReasonRobotsDisallow, now))
		}()
		go func() {
			defer wg.Done()
			r.RecordError(now, "pkg", "action", metadata.CauseUnknown, "detail", nil)
		}()
	}

	wg.Wait()

	events := r.Events()
	wantCount := goroutines * 3
	if len(events) != wantCount {
		t.Errorf("Events() len = %v, want %v", len(events), wantCount)
	}
}

// TestRecorder_EmptyLog verifies that a new Recorder returns an empty,
// non-nil slice from Events().
func TestRecorder_EmptyLog(t *testing.T) {
	r := newTestRecorder(t)

	events := r.Events()
	if events == nil {
		t.Error("Events() returned nil, want empty non-nil slice")
	}
	if len(events) != 0 {
		t.Errorf("Events() len = %v, want 0 for a new recorder", len(events))
	}
}
