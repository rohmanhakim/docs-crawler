package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/treeprinter"
)

// TestEventPrinter_PageFetchWithPipeline verifies tree output for a page with pipeline stages.
func TestEventPrinter_PageFetchWithPipeline(t *testing.T) {
	var buf bytes.Buffer
	tp := treeprinter.NewTreePrinterWithWriter(&buf)
	ep := NewEventPrinter(tp)
	rec := metadata.NewRecorder("test")

	// Record events
	rec.RecordFetch(metadata.NewFetchEvent(
		time.Now(),
		"https://example.com/docs/index.html",
		200,
		500*time.Millisecond,
		"text/html",
		0,
		0,
		metadata.KindPage,
	))
	rec.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageExtract,
		"https://example.com/docs/index.html",
		true,
		time.Now(),
		12,
	))
	rec.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageSanitize,
		"https://example.com/docs/index.html",
		true,
		time.Now(),
		0,
	))
	rec.RecordArtifact(metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown,
		"output/docs/index.md",
		"https://example.com/docs/index.html",
		"abc123",
		false,
		1024,
		time.Now(),
	))

	// Print all events
	for _, e := range rec.Events() {
		ep.PrintEvent(e)
	}
	ep.Flush()

	output := buf.String()
	// Verify tree structure
	if !bytes.Contains([]byte(output), []byte("[FETCH] https://example.com/docs/index.html")) {
		t.Error("expected FETCH line in output")
	}
	if !bytes.Contains([]byte(output), []byte("├── [extract] success (12 links)")) {
		t.Errorf("expected extract child with tree prefix, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("└── [ARTIFACT]")) {
		t.Errorf("expected last child with └── prefix, got:\n%s", output)
	}
}

// TestEventPrinter_MultiplePages verifies tree output for multiple page fetches.
func TestEventPrinter_MultiplePages(t *testing.T) {
	var buf bytes.Buffer
	tp := treeprinter.NewTreePrinterWithWriter(&buf)
	ep := NewEventPrinter(tp)
	rec := metadata.NewRecorder("test")

	// First page
	rec.RecordFetch(metadata.NewFetchEvent(
		time.Now(),
		"https://example.com/page1.html",
		200,
		100*time.Millisecond,
		"text/html",
		0, 0,
		metadata.KindPage,
	))
	rec.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageExtract,
		"https://example.com/page1.html",
		true,
		time.Now(),
		0,
	))

	// Second page
	rec.RecordFetch(metadata.NewFetchEvent(
		time.Now(),
		"https://example.com/page2.html",
		200,
		100*time.Millisecond,
		"text/html",
		0, 0,
		metadata.KindPage,
	))
	rec.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageExtract,
		"https://example.com/page2.html",
		false,
		time.Now(),
		0,
	))

	// Print all events
	for _, e := range rec.Events() {
		ep.PrintEvent(e)
	}
	ep.Flush()

	output := buf.String()
	// Should have two separate FETCH lines
	if count := bytes.Count([]byte(output), []byte("[FETCH]")); count != 2 {
		t.Errorf("expected 2 FETCH lines, got %d", count)
	}
	// First page should have └── (only one child)
	if !bytes.Contains([]byte(output), []byte("└── [extract] success")) {
		t.Errorf("expected first page extract with └──, got:\n%s", output)
	}
}

// TestEventPrinter_SkipEvent verifies SKIP events are printed standalone.
func TestEventPrinter_SkipEvent(t *testing.T) {
	var buf bytes.Buffer
	tp := treeprinter.NewTreePrinterWithWriter(&buf)
	ep := NewEventPrinter(tp)
	rec := metadata.NewRecorder("test")

	rec.RecordSkip(metadata.NewSkipEvent(
		"https://example.com/admin",
		metadata.SkipReasonRobotsDisallow,
		time.Now(),
	))

	// Print all events
	for _, e := range rec.Events() {
		ep.PrintEvent(e)
	}
	ep.Flush()

	output := buf.String()
	// SKIP should not have tree prefix
	if bytes.Contains([]byte(output), []byte("├──")) || bytes.Contains([]byte(output), []byte("└──")) {
		t.Errorf("SKIP should not have tree prefix, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("[SKIP] https://example.com/admin - robots_disallow")) {
		t.Errorf("expected SKIP line, got:\n%s", output)
	}
}

// TestEventPrinter_AssetFetch verifies asset fetches are children of the current page.
func TestEventPrinter_AssetFetch(t *testing.T) {
	var buf bytes.Buffer
	tp := treeprinter.NewTreePrinterWithWriter(&buf)
	ep := NewEventPrinter(tp)
	rec := metadata.NewRecorder("test")

	// Page fetch first
	rec.RecordFetch(metadata.NewFetchEvent(
		time.Now(),
		"https://example.com/page.html",
		200,
		100*time.Millisecond,
		"text/html",
		0, 0,
		metadata.KindPage,
	))

	// Asset fetch should be child
	rec.RecordFetch(metadata.NewFetchEvent(
		time.Now(),
		"https://example.com/image.png",
		200,
		100*time.Millisecond,
		"image/png",
		0, 0,
		metadata.KindAsset,
	))

	// Print all events
	for _, e := range rec.Events() {
		ep.PrintEvent(e)
	}
	ep.Flush()

	output := buf.String()
	// Asset fetch should be indented as child
	if !bytes.Contains([]byte(output), []byte("└── [FETCH] https://example.com/image.png")) {
		t.Errorf("expected asset fetch as child with └──, got:\n%s", output)
	}
}

// TestEventPrinter_StatsFinalizesParent verifies STATS events finalize the current tree.
func TestEventPrinter_StatsFinalizesParent(t *testing.T) {
	var buf bytes.Buffer
	tp := treeprinter.NewTreePrinterWithWriter(&buf)
	ep := NewEventPrinter(tp)
	rec := metadata.NewRecorder("test")

	// Page fetch
	rec.RecordFetch(metadata.NewFetchEvent(
		time.Now(),
		"https://example.com/page.html",
		200,
		100*time.Millisecond,
		"text/html",
		0, 0,
		metadata.KindPage,
	))
	rec.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageExtract,
		"https://example.com/page.html",
		true,
		time.Now(),
		0,
	))

	// STATS should finalize the parent
	rec.RecordFinalCrawlStats(metadata.NewCrawlStats(
		time.Now().Add(-1*time.Minute),
		time.Now(),
		1,
		1,
		0,
		0,
		0,
	))

	// Print all events
	for _, e := range rec.Events() {
		ep.PrintEvent(e)
	}
	ep.Flush()

	output := buf.String()
	// Should have tree structure for page
	if !bytes.Contains([]byte(output), []byte("└── [extract] success")) {
		t.Errorf("expected extract child before STATS, got:\n%s", output)
	}
	// Should have STATS section
	if !bytes.Contains([]byte(output), []byte("--- CRAWL STATS ---")) {
		t.Errorf("expected CRAWL STATS section, got:\n%s", output)
	}
}

// TestEventPrinter_PipelineFailure verifies failed pipeline stages are shown correctly.
func TestEventPrinter_PipelineFailure(t *testing.T) {
	var buf bytes.Buffer
	tp := treeprinter.NewTreePrinterWithWriter(&buf)
	ep := NewEventPrinter(tp)
	rec := metadata.NewRecorder("test")

	rec.RecordFetch(metadata.NewFetchEvent(
		time.Now(),
		"https://example.com/page.html",
		200,
		100*time.Millisecond,
		"text/html",
		0, 0,
		metadata.KindPage,
	))
	rec.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageConvert,
		"https://example.com/page.html",
		false,
		time.Now(),
		0,
	))

	// Print all events
	for _, e := range rec.Events() {
		ep.PrintEvent(e)
	}
	ep.Flush()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("[convert] failed")) {
		t.Errorf("expected convert failed, got:\n%s", output)
	}
}

// TestEventPrinter_ErrorEvent verifies ERROR events are children of current page.
func TestEventPrinter_ErrorEvent(t *testing.T) {
	var buf bytes.Buffer
	tp := treeprinter.NewTreePrinterWithWriter(&buf)
	ep := NewEventPrinter(tp)
	rec := metadata.NewRecorder("test")

	// Page fetch
	rec.RecordFetch(metadata.NewFetchEvent(
		time.Now(),
		"https://example.com/page.html",
		200,
		100*time.Millisecond,
		"text/html",
		0, 0,
		metadata.KindPage,
	))

	// Error should be child of page
	rec.RecordError(metadata.NewErrorRecord(
		time.Now(),
		"mdconvert",
		"Convert",
		metadata.CauseContentInvalid,
		"multiple H1 headings",
		nil,
	))

	// Print all events
	for _, e := range rec.Events() {
		ep.PrintEvent(e)
	}
	ep.Flush()

	output := buf.String()
	// Error should be child with tree prefix
	if !bytes.Contains([]byte(output), []byte("└── [ERROR]")) {
		t.Errorf("expected ERROR as child with └──, got:\n%s", output)
	}
}
