package metadata_test

import (
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

func TestNewAttr(t *testing.T) {
	tests := []struct {
		name    string
		key     metadata.AttributeKey
		value   string
		wantKey metadata.AttributeKey
		wantVal string
	}{
		{
			name:    "creates attribute with URL key",
			key:     metadata.AttrURL,
			value:   "https://example.com",
			wantKey: metadata.AttrURL,
			wantVal: "https://example.com",
		},
		{
			name:    "creates attribute with Host key",
			key:     metadata.AttrHost,
			value:   "example.com",
			wantKey: metadata.AttrHost,
			wantVal: "example.com",
		},
		{
			name:    "creates attribute with Path key",
			key:     metadata.AttrPath,
			value:   "/page",
			wantKey: metadata.AttrPath,
			wantVal: "/page",
		},
		{
			name:    "creates attribute with Depth key",
			key:     metadata.AttrDepth,
			value:   "0",
			wantKey: metadata.AttrDepth,
			wantVal: "0",
		},
		{
			name:    "creates attribute with empty value",
			key:     metadata.AttrMessage,
			value:   "",
			wantKey: metadata.AttrMessage,
			wantVal: "",
		},
		{
			name:    "creates attribute with Field key",
			key:     metadata.AttrField,
			value:   "title",
			wantKey: metadata.AttrField,
			wantVal: "title",
		},
		{
			name:    "creates attribute with HTTPStatus key",
			key:     metadata.AttrHTTPStatus,
			value:   "200",
			wantKey: metadata.AttrHTTPStatus,
			wantVal: "200",
		},
		{
			name:    "creates attribute with AssetURL key",
			key:     metadata.AttrAssetURL,
			value:   "https://example.com/image.png",
			wantKey: metadata.AttrAssetURL,
			wantVal: "https://example.com/image.png",
		},
		{
			name:    "creates attribute with WritePath key",
			key:     metadata.AttrWritePath,
			value:   "/output/page.md",
			wantKey: metadata.AttrWritePath,
			wantVal: "/output/page.md",
		},
		{
			name:    "creates attribute with Time key",
			key:     metadata.AttrTime,
			value:   "2024-01-01T00:00:00Z",
			wantKey: metadata.AttrTime,
			wantVal: "2024-01-01T00:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := metadata.NewAttr(tt.key, tt.value)

			if got.Key != tt.wantKey {
				t.Errorf("NewAttr().Key = %v, want %v", got.Key, tt.wantKey)
			}

			if got.Value != tt.wantVal {
				t.Errorf("NewAttr().Value = %v, want %v", got.Value, tt.wantVal)
			}
		})
	}
}

func TestArtifactKind(t *testing.T) {
	tests := []struct {
		name string
		kind metadata.ArtifactKind
		want string
	}{
		{
			name: "ArtifactMarkdown has correct value",
			kind: metadata.ArtifactMarkdown,
			want: "markdown",
		},
		{
			name: "ArtifactAsset has correct value",
			kind: metadata.ArtifactAsset,
			want: "asset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.kind) != tt.want {
				t.Errorf("ArtifactKind = %v, want %v", tt.kind, tt.want)
			}
		})
	}
}

func TestErrorCause(t *testing.T) {
	tests := []struct {
		name     string
		cause    metadata.ErrorCause
		wantInt  int
		wantDesc string
	}{
		{
			name:     "CauseUnknown has correct value",
			cause:    metadata.CauseUnknown,
			wantInt:  0,
			wantDesc: "CauseUnknown",
		},
		{
			name:     "CauseNetworkFailure has correct value",
			cause:    metadata.CauseNetworkFailure,
			wantInt:  1,
			wantDesc: "CauseNetworkFailure",
		},
		{
			name:     "CausePolicyDisallow has correct value",
			cause:    metadata.CausePolicyDisallow,
			wantInt:  2,
			wantDesc: "CausePolicyDisallow",
		},
		{
			name:     "CauseContentInvalid has correct value",
			cause:    metadata.CauseContentInvalid,
			wantInt:  3,
			wantDesc: "CauseContentInvalid",
		},
		{
			name:     "CauseStorageFailure has correct value",
			cause:    metadata.CauseStorageFailure,
			wantInt:  4,
			wantDesc: "CauseStorageFailure",
		},
		{
			name:     "CauseInvariantViolation has correct value",
			cause:    metadata.CauseInvariantViolation,
			wantInt:  5,
			wantDesc: "CauseInvariantViolation",
		},
		{
			name:     "CauseRetryFailure has correct value",
			cause:    metadata.CauseRetryFailure,
			wantInt:  6,
			wantDesc: "CauseRetryFailure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.cause) != tt.wantInt {
				t.Errorf("ErrorCause = %v, want %v", tt.cause, tt.wantInt)
			}
			// Verify cause is used in test context
			_ = tt.wantDesc
		})
	}
}

func TestAttributeKey(t *testing.T) {
	tests := []struct {
		name string
		key  metadata.AttributeKey
		want string
	}{
		{
			name: "AttrTime has correct value",
			key:  metadata.AttrTime,
			want: "time",
		},
		{
			name: "AttrURL has correct value",
			key:  metadata.AttrURL,
			want: "url",
		},
		{
			name: "AttrHost has correct value",
			key:  metadata.AttrHost,
			want: "host",
		},
		{
			name: "AttrPath has correct value",
			key:  metadata.AttrPath,
			want: "path",
		},
		{
			name: "AttrDepth has correct value",
			key:  metadata.AttrDepth,
			want: "depth",
		},
		{
			name: "AttrField has correct value",
			key:  metadata.AttrField,
			want: "field",
		},
		{
			name: "AttrHTTPStatus has correct value",
			key:  metadata.AttrHTTPStatus,
			want: "http_status",
		},
		{
			name: "AttrAssetURL has correct value",
			key:  metadata.AttrAssetURL,
			want: "asset_url",
		},
		{
			name: "AttrWritePath has correct value",
			key:  metadata.AttrWritePath,
			want: "write_path",
		},
		{
			name: "AttrMessage has correct value",
			key:  metadata.AttrMessage,
			want: "message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.key) != tt.want {
				t.Errorf("AttributeKey = %v, want %v", tt.key, tt.want)
			}
		})
	}
}

func TestAttributeFields(t *testing.T) {
	// Test that Attribute struct fields are accessible and can be set
	attr := metadata.Attribute{
		Key:   metadata.AttrURL,
		Value: "https://example.com/page",
	}

	// Verify field access
	if attr.Key != metadata.AttrURL {
		t.Errorf("Attribute.Key = %v, want %v", attr.Key, metadata.AttrURL)
	}

	if attr.Value != "https://example.com/page" {
		t.Errorf("Attribute.Value = %v, want %v", attr.Value, "https://example.com/page")
	}

	// Test modification
	attr.Value = "https://example.com/updated"
	if attr.Value != "https://example.com/updated" {
		t.Errorf("Attribute.Value after modification = %v, want %v", attr.Value, "https://example.com/updated")
	}
}

func TestArtifactKindComparison(t *testing.T) {
	// Test that ArtifactKind values can be compared
	markdown1 := metadata.ArtifactMarkdown
	markdown2 := metadata.ArtifactMarkdown
	asset := metadata.ArtifactAsset

	if markdown1 != markdown2 {
		t.Error("Same ArtifactKind values should be equal")
	}

	if markdown1 == asset {
		t.Error("Different ArtifactKind values should not be equal")
	}
}

func TestErrorCauseComparison(t *testing.T) {
	// Test that ErrorCause values can be compared
	cause1 := metadata.CauseUnknown
	cause2 := metadata.CauseUnknown
	cause3 := metadata.CauseNetworkFailure

	if cause1 != cause2 {
		t.Error("Same ErrorCause values should be equal")
	}

	if cause1 == cause3 {
		t.Error("Different ErrorCause values should not be equal")
	}
}

func TestErrorCauseOrder(t *testing.T) {
	// Test that ErrorCause values are ordered sequentially
	if metadata.CauseUnknown >= metadata.CauseRetryFailure {
		t.Error("CauseUnknown should be less than CauseRetryFailure")
	}

	if metadata.CauseNetworkFailure >= metadata.CausePolicyDisallow {
		t.Error("CauseNetworkFailure should be less than CausePolicyDisallow")
	}

	// Verify all causes are in valid range
	allCauses := []metadata.ErrorCause{
		metadata.CauseUnknown,
		metadata.CauseNetworkFailure,
		metadata.CausePolicyDisallow,
		metadata.CauseContentInvalid,
		metadata.CauseStorageFailure,
		metadata.CauseInvariantViolation,
		metadata.CauseRetryFailure,
	}

	for i, cause := range allCauses {
		if int(cause) != i {
			t.Errorf("Cause at index %d has value %d, want %d", i, cause, i)
		}
	}
}

func TestAttributeKeyString(t *testing.T) {
	// Test that AttributeKey can be converted to string
	key := metadata.AttrURL
	str := string(key)

	if str != "url" {
		t.Errorf("string(AttrURL) = %v, want %v", str, "url")
	}

	// Test string conversion for all attribute keys
	allKeys := []metadata.AttributeKey{
		metadata.AttrTime,
		metadata.AttrURL,
		metadata.AttrHost,
		metadata.AttrPath,
		metadata.AttrDepth,
		metadata.AttrField,
		metadata.AttrHTTPStatus,
		metadata.AttrAssetURL,
		metadata.AttrWritePath,
		metadata.AttrMessage,
	}

	expectedStrings := []string{
		"time",
		"url",
		"host",
		"path",
		"depth",
		"field",
		"http_status",
		"asset_url",
		"write_path",
		"message",
	}

	for i, key := range allKeys {
		if string(key) != expectedStrings[i] {
			t.Errorf("string(%v) = %v, want %v", key, string(key), expectedStrings[i])
		}
	}
}

func TestNewAttributeKeys(t *testing.T) {
	tests := []struct {
		name string
		key  metadata.AttributeKey
		want string
	}{
		{
			name: "AttrContentHash has correct value",
			key:  metadata.AttrContentHash,
			want: "content_hash",
		},
		{
			name: "AttrURLHash has correct value",
			key:  metadata.AttrURLHash,
			want: "url_hash",
		},
		{
			name: "AttrPageURL has correct value",
			key:  metadata.AttrPageURL,
			want: "page_url",
		},
		{
			name: "AttrStage has correct value",
			key:  metadata.AttrStage,
			want: "stage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.key) != tt.want {
				t.Errorf("AttributeKey = %v, want %v", string(tt.key), tt.want)
			}
		})
	}
}

func TestFetchKind(t *testing.T) {
	tests := []struct {
		name string
		kind metadata.FetchKind
		want string
	}{
		{name: "KindPage has correct value", kind: metadata.KindPage, want: "page"},
		{name: "KindAsset has correct value", kind: metadata.KindAsset, want: "asset"},
		{name: "KindRobots has correct value", kind: metadata.KindRobots, want: "robots"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.kind) != tt.want {
				t.Errorf("FetchKind = %v, want %v", tt.kind, tt.want)
			}
		})
	}
}

func TestFetchEventConstruction(t *testing.T) {
	now := time.Now()
	e := metadata.FetchEvent{
		FetchedAt:   now,
		FetchURL:    "https://example.com/page",
		HTTPStatus:  200,
		Duration:    time.Second,
		ContentType: "text/html",
		RetryCount:  1,
		CrawlDepth:  2,
		Kind:        metadata.KindPage,
	}

	if e.FetchURL != "https://example.com/page" {
		t.Errorf("FetchEvent.FetchURL = %v, want %v", e.FetchURL, "https://example.com/page")
	}
	if e.Kind != metadata.KindPage {
		t.Errorf("FetchEvent.Kind = %v, want %v", e.Kind, metadata.KindPage)
	}
	if e.FetchedAt != now {
		t.Errorf("FetchEvent.FetchedAt = %v, want %v", e.FetchedAt, now)
	}
}

func TestArtifactRecordConstruction(t *testing.T) {
	now := time.Now()
	r := metadata.ArtifactRecord{
		Kind:        metadata.ArtifactMarkdown,
		WritePath:   "/output/page.md",
		SourceURL:   "https://example.com/page",
		ContentHash: "abc123",
		Overwrite:   false,
		Bytes:       1024,
		RecordedAt:  now,
	}

	if r.Kind != metadata.ArtifactMarkdown {
		t.Errorf("ArtifactRecord.Kind = %v, want %v", r.Kind, metadata.ArtifactMarkdown)
	}
	if r.SourceURL != "https://example.com/page" {
		t.Errorf("ArtifactRecord.SourceURL = %v, want %v", r.SourceURL, "https://example.com/page")
	}
	if r.RecordedAt != now {
		t.Errorf("ArtifactRecord.RecordedAt = %v, want %v", r.RecordedAt, now)
	}
}

func TestPipelineStage(t *testing.T) {
	tests := []struct {
		name  string
		stage metadata.PipelineStage
		want  string
	}{
		{name: "StageExtract has correct value", stage: metadata.StageExtract, want: "extract"},
		{name: "StageSanitize has correct value", stage: metadata.StageSanitize, want: "sanitize"},
		{name: "StageConvert has correct value", stage: metadata.StageConvert, want: "convert"},
		{name: "StageNormalize has correct value", stage: metadata.StageNormalize, want: "normalize"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.stage) != tt.want {
				t.Errorf("PipelineStage = %v, want %v", tt.stage, tt.want)
			}
		})
	}
}

func TestPipelineEventConstruction(t *testing.T) {
	now := time.Now()
	e := metadata.PipelineEvent{
		Stage:      metadata.StageExtract,
		PageURL:    "https://example.com/page",
		Success:    true,
		RecordedAt: now,
		LinksFound: 5,
	}

	if e.Stage != metadata.StageExtract {
		t.Errorf("PipelineEvent.Stage = %v, want %v", e.Stage, metadata.StageExtract)
	}
	if !e.Success {
		t.Error("PipelineEvent.Success = false, want true")
	}
	if e.LinksFound != 5 {
		t.Errorf("PipelineEvent.LinksFound = %v, want 5", e.LinksFound)
	}
}

func TestSkipReason(t *testing.T) {
	tests := []struct {
		name   string
		reason metadata.SkipReason
		want   string
	}{
		{name: "SkipReasonRobotsDisallow has correct value", reason: metadata.SkipReasonRobotsDisallow, want: "robots_disallow"},
		{name: "SkipReasonOutOfScope has correct value", reason: metadata.SkipReasonOutOfScope, want: "out_of_scope"},
		{name: "SkipReasonAlreadyVisited has correct value", reason: metadata.SkipReasonAlreadyVisited, want: "already_visited"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.reason) != tt.want {
				t.Errorf("SkipReason = %v, want %v", tt.reason, tt.want)
			}
		})
	}
}

func TestSkipEventConstruction(t *testing.T) {
	now := time.Now()
	e := metadata.SkipEvent{
		SkippedURL: "https://example.com/disallowed",
		Reason:     metadata.SkipReasonRobotsDisallow,
		RecordedAt: now,
	}

	if e.SkippedURL != "https://example.com/disallowed" {
		t.Errorf("SkipEvent.SkippedURL = %v, want %v", e.SkippedURL, "https://example.com/disallowed")
	}
	if e.Reason != metadata.SkipReasonRobotsDisallow {
		t.Errorf("SkipEvent.Reason = %v, want %v", e.Reason, metadata.SkipReasonRobotsDisallow)
	}
}

func TestCrawlStatsConstruction(t *testing.T) {
	start := time.Now()
	end := start.Add(5 * time.Second)
	s := metadata.CrawlStats{
		StartedAt:             start,
		FinishedAt:            end,
		TotalPages:            10,
		TotalErrors:           2,
		TotalAssets:           5,
		ManualRetryQueueCount: 1,
	}

	if s.StartedAt != start {
		t.Errorf("CrawlStats.StartedAt = %v, want %v", s.StartedAt, start)
	}
	if !s.FinishedAt.After(s.StartedAt) {
		t.Error("CrawlStats.FinishedAt must be after StartedAt")
	}
	if s.TotalPages != 10 {
		t.Errorf("CrawlStats.TotalPages = %v, want 10", s.TotalPages)
	}
}

func TestErrorEventConstruction(t *testing.T) {
	now := time.Now()
	e := metadata.ErrorEvent{
		ObservedAt:  now,
		PackageName: "fetcher",
		Action:      "Fetch",
		Cause:       metadata.CauseNetworkFailure,
		Details:     "connection refused",
		Attrs:       []metadata.Attribute{metadata.NewAttr(metadata.AttrURL, "https://example.com")},
	}

	if e.PackageName != "fetcher" {
		t.Errorf("ErrorEvent.PackageName = %v, want fetcher", e.PackageName)
	}
	if e.Cause != metadata.CauseNetworkFailure {
		t.Errorf("ErrorEvent.Cause = %v, want %v", e.Cause, metadata.CauseNetworkFailure)
	}
	if len(e.Attrs) != 1 {
		t.Errorf("ErrorEvent.Attrs len = %v, want 1", len(e.Attrs))
	}
}

func TestEventKind(t *testing.T) {
	tests := []struct {
		name string
		kind metadata.EventKind
		want string
	}{
		{name: "EventKindFetch has correct value", kind: metadata.EventKindFetch, want: "fetch"},
		{name: "EventKindArtifact has correct value", kind: metadata.EventKindArtifact, want: "artifact"},
		{name: "EventKindPipeline has correct value", kind: metadata.EventKindPipeline, want: "pipeline"},
		{name: "EventKindSkip has correct value", kind: metadata.EventKindSkip, want: "skip"},
		{name: "EventKindError has correct value", kind: metadata.EventKindError, want: "error"},
		{name: "EventKindStats has correct value", kind: metadata.EventKindStats, want: "stats"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.kind) != tt.want {
				t.Errorf("EventKind = %v, want %v", tt.kind, tt.want)
			}
		})
	}
}

func TestEventConstruction(t *testing.T) {
	t.Run("fetch event", func(t *testing.T) {
		fe := &metadata.FetchEvent{Kind: metadata.KindPage, FetchURL: "https://example.com"}
		e := metadata.Event{Kind: metadata.EventKindFetch, Fetch: fe}
		if e.Kind != metadata.EventKindFetch {
			t.Errorf("Event.Kind = %v, want %v", e.Kind, metadata.EventKindFetch)
		}
		if e.Fetch == nil {
			t.Error("Event.Fetch must not be nil for fetch event")
		}
	})

	t.Run("artifact event", func(t *testing.T) {
		ar := &metadata.ArtifactRecord{Kind: metadata.ArtifactMarkdown}
		e := metadata.Event{Kind: metadata.EventKindArtifact, Artifact: ar}
		if e.Kind != metadata.EventKindArtifact {
			t.Errorf("Event.Kind = %v, want %v", e.Kind, metadata.EventKindArtifact)
		}
		if e.Artifact == nil {
			t.Error("Event.Artifact must not be nil for artifact event")
		}
	})

	t.Run("pipeline event", func(t *testing.T) {
		pe := &metadata.PipelineEvent{Stage: metadata.StageExtract, Success: true}
		e := metadata.Event{Kind: metadata.EventKindPipeline, Pipeline: pe}
		if e.Kind != metadata.EventKindPipeline {
			t.Errorf("Event.Kind = %v, want %v", e.Kind, metadata.EventKindPipeline)
		}
		if e.Pipeline == nil {
			t.Error("Event.Pipeline must not be nil for pipeline event")
		}
	})

	t.Run("skip event", func(t *testing.T) {
		se := &metadata.SkipEvent{Reason: metadata.SkipReasonRobotsDisallow}
		e := metadata.Event{Kind: metadata.EventKindSkip, Skip: se}
		if e.Kind != metadata.EventKindSkip {
			t.Errorf("Event.Kind = %v, want %v", e.Kind, metadata.EventKindSkip)
		}
		if e.Skip == nil {
			t.Error("Event.Skip must not be nil for skip event")
		}
	})

	t.Run("error event", func(t *testing.T) {
		ee := &metadata.ErrorEvent{Cause: metadata.CauseNetworkFailure}
		e := metadata.Event{Kind: metadata.EventKindError, Error: ee}
		if e.Kind != metadata.EventKindError {
			t.Errorf("Event.Kind = %v, want %v", e.Kind, metadata.EventKindError)
		}
		if e.Error == nil {
			t.Error("Event.Error must not be nil for error event")
		}
	})

	t.Run("stats event", func(t *testing.T) {
		cs := &metadata.CrawlStats{TotalPages: 3}
		e := metadata.Event{Kind: metadata.EventKindStats, Stats: cs}
		if e.Kind != metadata.EventKindStats {
			t.Errorf("Event.Kind = %v, want %v", e.Kind, metadata.EventKindStats)
		}
		if e.Stats == nil {
			t.Error("Event.Stats must not be nil for stats event")
		}
	})
}
