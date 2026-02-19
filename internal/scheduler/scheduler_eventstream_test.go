package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Event Stream Integration Tests
// These tests verify the complete event stream produced by a full crawl
// pipeline execution, ensuring all pipeline stages emit the expected events.
// ============================================================================

// TestEventStream_FullPipelineExecution verifies that a complete crawl pipeline
// execution produces the expected event stream with all required event types.
//
// This test uses REAL pipeline stage implementations (not mocks) to ensure
// that events are actually emitted by each stage. Only infrastructure
// (rate limiter, sleeper, failure journal) is mocked.
func TestEventStream_FullPipelineExecution(t *testing.T) {
	// Setup test server
	server := setupEventStreamServer(t)
	defer server.Close()

	// Create output directory
	outputDir := t.TempDir()

	// Write config file
	configPath := writeEventStreamConfig(t, server.URL, outputDir)

	// Create a real Recorder - this is both MetadataSink and CrawlFinalizer
	rec := metadata.NewRecorder("integration-test-worker")

	// Create real pipeline stage implementations
	cachedRobot := robots.NewCachedRobot(&rec)
	crawlFrontier := frontier.NewCrawlFrontier()
	htmlFetcher := fetcher.NewHtmlFetcher(&rec)
	domExtractor := extractor.NewDomExtractor(&rec)
	htmlSanitizer := sanitizer.NewHTMLSanitizer(&rec)
	conversionRule := mdconvert.NewRule(&rec)
	assetResolver := assets.NewLocalResolver(&rec)
	markdownConstraint := normalize.NewMarkdownConstraint(&rec)
	storageSink := storage.NewLocalSink(&rec)

	// Create infrastructure mocks (these don't emit metadata events)
	rateLimiter := newRateLimiterMockForTest(t)
	rateLimiter.On("ResolveDelay", mock.AnythingOfType("string")).Return(time.Duration(0)).Maybe()

	sleeper := newSleeperMock(t)
	sleeper.On("Sleep", mock.AnythingOfType("time.Duration")).Return()

	failureJournal := newFailureJournalMockForTest(t)

	// Build scheduler with real pipeline stages
	ctx := context.Background()
	s := scheduler.NewSchedulerWithDeps(
		ctx,
		&rec, // CrawlFinalizer
		&rec, // MetadataSink
		rateLimiter,
		&crawlFrontier,
		&htmlFetcher,
		&cachedRobot,
		&domExtractor,
		&htmlSanitizer,
		conversionRule,
		&assetResolver,
		&markdownConstraint,
		&storageSink,
		sleeper,
		failureJournal,
	)

	// Execute crawl
	init, err := s.InitializeCrawling(configPath)
	require.NoError(t, err, "InitializeCrawling should succeed")

	_, err = s.ExecuteCrawlingWithState(init)
	require.NoError(t, err, "ExecuteCrawlingWithState should succeed")

	// Collect events
	events := rec.Events()
	t.Logf("Total events recorded: %d", len(events))

	// Log all events for debugging
	for i, e := range events {
		switch e.Kind() {
		case metadata.EventKindError:
			if e.Error() != nil {
				t.Logf("  [%d] Kind=%s, Package=%s, Action=%s, Cause=%v, Error=%s",
					i, e.Kind(), e.Error().PackageName(), e.Error().Action(), e.Error().Cause(), e.Error().ErrorString())
			}
		case metadata.EventKindPipeline:
			if e.Pipeline() != nil {
				t.Logf("  [%d] Kind=%s, Stage=%s, Success=%v", i, e.Kind(), e.Pipeline().Stage(), e.Pipeline().Success())
			}
		case metadata.EventKindFetch:
			if e.Fetch() != nil {
				t.Logf("  [%d] Kind=%s, FetchKind=%s, URL=%s, Status=%d", i, e.Kind(), e.Fetch().Kind(), e.Fetch().FetchURL(), e.Fetch().HTTPStatus())
			}
		default:
			t.Logf("  [%d] Kind=%s", i, e.Kind())
		}
	}

	// Group events for assertions
	byKind := collectEventKinds(events)
	byStage := collectPipelineStages(events)
	byFetchKind := collectFetchKinds(events)

	// ========================================
	// Assertions: Fetch events
	// ========================================
	assert.NotEmpty(t, byFetchKind[metadata.KindRobots],
		"should have at least one robots.txt fetch event")
	assert.NotEmpty(t, byFetchKind[metadata.KindPage],
		"should have at least one page fetch event")

	// Verify robots fetch has valid HTTP status
	if len(byFetchKind[metadata.KindRobots]) > 0 {
		robotsFetch := byFetchKind[metadata.KindRobots][0]
		assert.Equal(t, 200, robotsFetch.HTTPStatus(),
			"robots.txt fetch should return HTTP 200")
		assert.NotZero(t, robotsFetch.FetchedAt(),
			"robots.txt fetch should have non-zero FetchedAt timestamp")
	}

	// Verify page fetch has valid fields
	if len(byFetchKind[metadata.KindPage]) > 0 {
		pageFetch := byFetchKind[metadata.KindPage][0]
		assert.Equal(t, 200, pageFetch.HTTPStatus(),
			"page fetch should return HTTP 200")
		assert.NotZero(t, pageFetch.FetchedAt(),
			"page fetch should have non-zero FetchedAt timestamp")
		assert.Equal(t, "text/html", pageFetch.ContentType(),
			"page fetch should have text/html content type")
	}

	// ========================================
	// Assertions: Pipeline stage events
	// ========================================
	assert.NotEmpty(t, byStage[metadata.StageExtract],
		"should have at least one extract pipeline event")
	assert.NotEmpty(t, byStage[metadata.StageSanitize],
		"should have at least one sanitize pipeline event")
	assert.NotEmpty(t, byStage[metadata.StageConvert],
		"should have at least one convert pipeline event")
	assert.NotEmpty(t, byStage[metadata.StageNormalize],
		"should have at least one normalize pipeline event")

	// Verify all pipeline events indicate success
	for stage, events := range byStage {
		for i, pe := range events {
			assert.True(t, pe.Success(),
				"pipeline event %d for stage %s should indicate success", i, stage)
		}
	}

	// ========================================
	// Assertions: Artifact event
	// ========================================
	artifactEvents := byKind[metadata.EventKindArtifact]
	assert.NotEmpty(t, artifactEvents,
		"should have at least one artifact event")

	if len(artifactEvents) > 0 {
		artifact := artifactEvents[0].Artifact()
		require.NotNil(t, artifact, "artifact should not be nil")
		assert.Equal(t, metadata.ArtifactMarkdown, artifact.Kind(),
			"artifact should be of kind markdown")
		assert.NotEmpty(t, artifact.WritePath(),
			"artifact should have a write path")
		assert.NotEmpty(t, artifact.SourceURL(),
			"artifact should have a source URL")
		assert.NotZero(t, artifact.RecordedAt(),
			"artifact should have non-zero RecordedAt timestamp")
	}

	// ========================================
	// Assertions: Stats event
	// ========================================
	statsEvents := byKind[metadata.EventKindStats]
	require.Len(t, statsEvents, 1,
		"should have exactly one stats event")

	if len(statsEvents) > 0 {
		stats := statsEvents[0].Stats()
		require.NotNil(t, stats, "stats should not be nil")

		assert.False(t, stats.StartedAt().IsZero(),
			"stats should have non-zero StartedAt timestamp")
		assert.False(t, stats.FinishedAt().IsZero(),
			"stats should have non-zero FinishedAt timestamp")
		assert.True(t, stats.FinishedAt().After(stats.StartedAt()) || stats.FinishedAt().Equal(stats.StartedAt()),
			"FinishedAt should be at or after StartedAt")
		assert.GreaterOrEqual(t, stats.TotalPages(), 0,
			"TotalPages should be non-negative")
	}

	// ========================================
	// Assertions: Event ordering (sensible order check)
	// ========================================
	// Find first occurrence indices of each event kind
	var robotsFetchIdx, pageFetchIdx, extractIdx, sanitizeIdx, convertIdx, normalizeIdx, artifactIdx, statsIdx int
	robotsFetchIdx, pageFetchIdx, extractIdx, sanitizeIdx, convertIdx, normalizeIdx, artifactIdx, statsIdx = -1, -1, -1, -1, -1, -1, -1, -1

	for i, e := range events {
		switch e.Kind() {
		case metadata.EventKindFetch:
			if e.Fetch() != nil {
				if e.Fetch().Kind() == metadata.KindRobots && robotsFetchIdx == -1 {
					robotsFetchIdx = i
				}
				if e.Fetch().Kind() == metadata.KindPage && pageFetchIdx == -1 {
					pageFetchIdx = i
				}
			}
		case metadata.EventKindPipeline:
			if e.Pipeline() != nil {
				switch e.Pipeline().Stage() {
				case metadata.StageExtract:
					if extractIdx == -1 {
						extractIdx = i
					}
				case metadata.StageSanitize:
					if sanitizeIdx == -1 {
						sanitizeIdx = i
					}
				case metadata.StageConvert:
					if convertIdx == -1 {
						convertIdx = i
					}
				case metadata.StageNormalize:
					if normalizeIdx == -1 {
						normalizeIdx = i
					}
				}
			}
		case metadata.EventKindArtifact:
			if artifactIdx == -1 {
				artifactIdx = i
			}
		case metadata.EventKindStats:
			if statsIdx == -1 {
				statsIdx = i
			}
		}
	}

	// Verify sensible ordering (if indices are found)
	t.Logf("Event order indices: robots=%d, page=%d, extract=%d, sanitize=%d, convert=%d, normalize=%d, artifact=%d, stats=%d",
		robotsFetchIdx, pageFetchIdx, extractIdx, sanitizeIdx, convertIdx, normalizeIdx, artifactIdx, statsIdx)

	assert.Less(t, robotsFetchIdx, pageFetchIdx,
		"robots fetch should occur before page fetch")
	assert.Less(t, pageFetchIdx, extractIdx,
		"page fetch should occur before extract")
	assert.Less(t, extractIdx, sanitizeIdx,
		"extract should occur before sanitize")
	assert.Less(t, sanitizeIdx, convertIdx,
		"sanitize should occur before convert")
	assert.Less(t, convertIdx, normalizeIdx,
		"convert should occur before normalize")
	assert.Less(t, normalizeIdx, artifactIdx,
		"normalize should occur before artifact")
	assert.Less(t, artifactIdx, statsIdx,
		"artifact should occur before stats")
}
