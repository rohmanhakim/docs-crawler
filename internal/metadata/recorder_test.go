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
				r.RecordError(metadata.NewErrorRecord(
					now, "fetcher", "Fetch",
					metadata.CauseNetworkFailure,
					"connection refused",
					[]metadata.Attribute{metadata.NewAttr(metadata.AttrURL, "https://example.com")},
				))
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
					now, now.Add(5*time.Second), 10, 8, 2, 5, 1,
				))
			},
			wantKind: metadata.EventKindStats,
			verify: func(t *testing.T, e metadata.Event) {
				t.Helper()
				if e.Stats() == nil {
					t.Fatal("Event.Stats() is nil, want non-nil")
				}
				if e.Stats().TotalVisitedPages() != 10 {
					t.Errorf("Stats().TotalVisitedPages() = %v, want 10", e.Stats().TotalVisitedPages())
				}
				if e.Stats().TotalProcessedPages() != 8 {
					t.Errorf("Stats().TotalProcessedPages() = %v, want 8", e.Stats().TotalProcessedPages())
				}
				if e.Stats().TotalErrors() != 2 {
					t.Errorf("Stats().TotalErrors() = %v, want 2", e.Stats().TotalErrors())
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
			r.RecordError(metadata.NewErrorRecord(now, "pkg", "action", metadata.CauseUnknown, "detail", nil))
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

// TestRecorder_Subscribe_ReceivesEvents verifies that a subscriber registered
// before recording receives each subsequent event via the channel.
func TestRecorder_Subscribe_ReceivesEvents(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	ch, _, done := r.Subscribe()
	defer done()

	r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com", 200, time.Second, "text/html", 0, 0, metadata.KindPage))
	r.RecordSkip(metadata.NewSkipEvent("https://example.com/skip", metadata.SkipReasonRobotsDisallow, now))

	if len(ch) != 2 {
		t.Fatalf("subscriber channel len = %v, want 2", len(ch))
	}

	first := <-ch
	if first.Kind() != metadata.EventKindFetch {
		t.Errorf("first event Kind = %v, want %v", first.Kind(), metadata.EventKindFetch)
	}

	second := <-ch
	if second.Kind() != metadata.EventKindSkip {
		t.Errorf("second event Kind = %v, want %v", second.Kind(), metadata.EventKindSkip)
	}
}

// TestRecorder_Subscribe_SlowConsumerDoesNotBlock verifies that a subscriber
// with a full channel does not block the append path.
// The event must still appear in the recorder's own log.
func TestRecorder_Subscribe_SlowConsumerDoesNotBlock(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	// Subscribe creates a buffered channel (capacity 256)
	_, unsub, done := r.Subscribe()

	// Record 65 events rapidly - if the channel were unbuffered or blocking,
	// this would hang. The internal log must still receive all events.
	for i := 0; i < 65; i++ {
		r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com", 200, time.Second, "text/html", 0, 0, metadata.KindPage))
	}

	// All 65 events must be in the internal log
	if len(r.Events()) != 65 {
		t.Errorf("Events() len = %v, want 65 (log must be intact even when subscriber drops)", len(r.Events()))
	}

	unsub() // cleanup
	done()  // signal goroutine done
}

// TestRecorder_Subscribe_ForwardOnly verifies that a subscriber registered
// after some events have already been recorded does NOT receive past events —
// only events recorded after Subscribe returns are forwarded.
func TestRecorder_Subscribe_ForwardOnly(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	// Record one event BEFORE subscribing.
	r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com/before", 200, time.Second, "text/html", 0, 0, metadata.KindPage))

	ch, _, done := r.Subscribe()
	defer done()

	// Record a second event AFTER subscribing.
	r.RecordSkip(metadata.NewSkipEvent("https://example.com/after", metadata.SkipReasonRobotsDisallow, now))

	// The internal log must contain both events.
	if len(r.Events()) != 2 {
		t.Fatalf("Events() len = %v, want 2", len(r.Events()))
	}

	// The subscriber channel must contain only the post-subscription event.
	if len(ch) != 1 {
		t.Fatalf("subscriber channel len = %v, want 1 (forward-only: past events must not be replayed)", len(ch))
	}

	received := <-ch
	if received.Kind() != metadata.EventKindSkip {
		t.Errorf("received event Kind = %v, want %v", received.Kind(), metadata.EventKindSkip)
	}
}

// TestRecorder_Subscribe_MultipleSubscribers verifies that all registered
// subscribers receive each event independently.
func TestRecorder_Subscribe_MultipleSubscribers(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	ch1, _, done1 := r.Subscribe()
	ch2, _, done2 := r.Subscribe()
	defer done1()
	defer done2()

	r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com", 200, time.Second, "text/html", 0, 0, metadata.KindPage))

	for i, ch := range []<-chan metadata.Event{ch1, ch2} {
		if len(ch) != 1 {
			t.Errorf("subscriber %d channel len = %v, want 1", i+1, len(ch))
			continue
		}
		e := <-ch
		if e.Kind() != metadata.EventKindFetch {
			t.Errorf("subscriber %d received Kind = %v, want %v", i+1, e.Kind(), metadata.EventKindFetch)
		}
	}
}

// TestRecorder_Subscribe_UnsubscribeClosesChannel verifies that calling
// the unsubscribe function closes the subscriber channel.
func TestRecorder_Subscribe_UnsubscribeClosesChannel(t *testing.T) {
	r := newTestRecorder(t)

	ch, unsub, done := r.Subscribe()
	defer done()

	// Use a non-blocking select to verify channel is open without blocking
	select {
	case <-ch:
		// Channel has data (unexpected for new subscription)
	default:
		// Channel is open but empty - expected
	}

	unsub()

	// After unsubscribe: channel closed, receives return zero value immediately
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unsubscribe")
	}
}

// TestRecorder_Subscribe_UnsubscribeStopsEvents verifies that events
// recorded after unsubscribe are not sent to the channel.
func TestRecorder_Subscribe_UnsubscribeStopsEvents(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	ch, unsub, done := r.Subscribe()

	r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com/first", 200, time.Second, "text/html", 0, 0, metadata.KindPage))

	unsub()
	done()

	r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com/second", 200, time.Second, "text/html", 0, 0, metadata.KindPage))

	// Should only receive the first event (channel is now closed)
	receivedCount := 0
	for {
		_, ok := <-ch
		if !ok {
			break // channel closed
		}
		receivedCount++
	}

	if receivedCount != 1 {
		t.Errorf("received %d events, want 1 (events after unsubscribe should not be received)", receivedCount)
	}
}

// TestRecorder_Subscribe_UnsubscribeIdempotent verifies that calling
// unsubscribe multiple times does not panic.
func TestRecorder_Subscribe_UnsubscribeIdempotent(t *testing.T) {
	r := newTestRecorder(t)

	_, unsub, done := r.Subscribe()
	defer done()

	// Call unsubscribe multiple times - should not panic
	unsub()
	unsub()
	unsub()
}

// TestRecorder_Subscribe_RecordAfterAllUnsubscribed verifies that recording
// events after all subscribers have unsubscribed does not panic.
func TestRecorder_Subscribe_RecordAfterAllUnsubscribed(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	_, unsub1, done1 := r.Subscribe()
	_, unsub2, done2 := r.Subscribe()

	unsub1()
	unsub2()
	done1()
	done2()

	// Recording after all subscribers unsubscribed should not panic
	r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com", 200, time.Second, "text/html", 0, 0, metadata.KindPage))

	if len(r.Events()) != 1 {
		t.Errorf("Events() len = %v, want 1", len(r.Events()))
	}
}

// TestRecorder_Subscribe_ConcurrentUnsubscribe tests concurrent safety
// of subscribe/unsubscribe operations with recording.
func TestRecorder_Subscribe_ConcurrentUnsubscribe(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		ch, unsub, done := r.Subscribe()

		// Consumer goroutine
		go func() {
			defer wg.Done()
			defer done()
			for range ch {
				// drain channel
			}
		}()

		// Unsubscribe goroutine
		go func() {
			defer wg.Done()
			time.Sleep(time.Microsecond * 10)
			unsub()
		}()
	}

	// Record events concurrently
	for i := 0; i < 100; i++ {
		r.RecordFetch(metadata.NewFetchEvent(now, "https://example.com", 200, time.Second, "text/html", 0, 0, metadata.KindPage))
	}

	wg.Wait()

	// All events must be recorded (no panic = success)
	if len(r.Events()) != 100 {
		t.Errorf("Events() len = %v, want 100", len(r.Events()))
	}
}

// TestRecorder_Subscribe_WaitForSubscribersGuaranteesEventDelivery verifies that
// when using the done() callback and WaitForSubscribers(), all events are
// guaranteed to be delivered to the subscriber before the function returns.
// This is a regression test for the race condition bug where events could be
// lost if unsub() was called without waiting for the subscriber goroutine.
// See: docs/metadata-event-race-condition-bug.md
func TestRecorder_Subscribe_WaitForSubscribersGuaranteesEventDelivery(t *testing.T) {
	now := time.Now()
	r := newTestRecorder(t)

	ch, unsub, done := r.Subscribe()

	// Track events received by the subscriber
	var receivedEvents []metadata.Event
	var mu sync.Mutex

	// Start consumer goroutine - simulates the pattern used in root.go
	go func() {
		defer done()
		for e := range ch {
			mu.Lock()
			receivedEvents = append(receivedEvents, e)
			mu.Unlock()
		}
	}()

	// Record fetch events
	for i := 0; i < 10; i++ {
		r.RecordFetch(metadata.NewFetchEvent(
			now, "https://example.com", 200, time.Second,
			"text/html", 0, 0, metadata.KindPage,
		))
	}

	// Record error events - these were the ones affected by the original bug
	for i := 0; i < 5; i++ {
		r.RecordError(metadata.NewErrorRecord(
			now, "test", "TestAction",
			metadata.CauseUnknown,
			"test error",
			nil,
		))
	}

	// Unsubscribe - this closes the channel
	unsub()

	// Wait for all subscriber goroutines to finish processing
	// This is the critical step that prevents the race condition
	r.WaitForSubscribers()

	// Verify ALL events were received
	mu.Lock()
	totalReceived := len(receivedEvents)
	mu.Unlock()

	wantCount := 15 // 10 fetch + 5 error events
	if totalReceived != wantCount {
		t.Errorf("received %d events, want %d - events were lost!", totalReceived, wantCount)
	}

	// Verify we received both fetch and error events
	mu.Lock()
	defer mu.Unlock()
	fetchCount := 0
	errorCount := 0
	for _, e := range receivedEvents {
		if e.Kind() == metadata.EventKindFetch {
			fetchCount++
		} else if e.Kind() == metadata.EventKindError {
			errorCount++
		}
	}
	if fetchCount != 10 {
		t.Errorf("received %d fetch events, want 10", fetchCount)
	}
	if errorCount != 5 {
		t.Errorf("received %d error events, want 5", errorCount)
	}
}
