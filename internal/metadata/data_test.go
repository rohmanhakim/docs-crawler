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

			if got.Key() != tt.wantKey {
				t.Errorf("NewAttr().Key() = %v, want %v", got.Key(), tt.wantKey)
			}

			if got.Value() != tt.wantVal {
				t.Errorf("NewAttr().Value() = %v, want %v", got.Value(), tt.wantVal)
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
	attr := metadata.NewAttr(metadata.AttrURL, "https://example.com/page")

	if attr.Key() != metadata.AttrURL {
		t.Errorf("Attribute.Key() = %v, want %v", attr.Key(), metadata.AttrURL)
	}

	if attr.Value() != "https://example.com/page" {
		t.Errorf("Attribute.Value() = %v, want %v", attr.Value(), "https://example.com/page")
	}

	// Attributes are immutable — verify a new one reflects the updated value
	updated := metadata.NewAttr(metadata.AttrURL, "https://example.com/updated")
	if updated.Value() != "https://example.com/updated" {
		t.Errorf("Attribute.Value() after new construction = %v, want %v", updated.Value(), "https://example.com/updated")
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
	e := metadata.NewFetchEvent(
		now,
		"https://example.com/page",
		200,
		time.Second,
		"text/html",
		1,
		2,
		metadata.KindPage,
	)

	if e.FetchURL() != "https://example.com/page" {
		t.Errorf("FetchEvent.FetchURL() = %v, want %v", e.FetchURL(), "https://example.com/page")
	}
	if e.Kind() != metadata.KindPage {
		t.Errorf("FetchEvent.Kind() = %v, want %v", e.Kind(), metadata.KindPage)
	}
	if e.FetchedAt() != now {
		t.Errorf("FetchEvent.FetchedAt() = %v, want %v", e.FetchedAt(), now)
	}
	if e.HTTPStatus() != 200 {
		t.Errorf("FetchEvent.HTTPStatus() = %v, want 200", e.HTTPStatus())
	}
	if e.Duration() != time.Second {
		t.Errorf("FetchEvent.Duration() = %v, want %v", e.Duration(), time.Second)
	}
	if e.ContentType() != "text/html" {
		t.Errorf("FetchEvent.ContentType() = %v, want text/html", e.ContentType())
	}
	if e.RetryCount() != 1 {
		t.Errorf("FetchEvent.RetryCount() = %v, want 1", e.RetryCount())
	}
	if e.CrawlDepth() != 2 {
		t.Errorf("FetchEvent.CrawlDepth() = %v, want 2", e.CrawlDepth())
	}
}

func TestArtifactRecordConstruction(t *testing.T) {
	now := time.Now()
	r := metadata.NewArtifactRecord(
		metadata.ArtifactMarkdown,
		"/output/page.md",
		"https://example.com/page",
		"abc123",
		false,
		1024,
		now,
	)

	if r.Kind() != metadata.ArtifactMarkdown {
		t.Errorf("ArtifactRecord.Kind() = %v, want %v", r.Kind(), metadata.ArtifactMarkdown)
	}
	if r.WritePath() != "/output/page.md" {
		t.Errorf("ArtifactRecord.WritePath() = %v, want /output/page.md", r.WritePath())
	}
	if r.SourceURL() != "https://example.com/page" {
		t.Errorf("ArtifactRecord.SourceURL() = %v, want %v", r.SourceURL(), "https://example.com/page")
	}
	if r.ContentHash() != "abc123" {
		t.Errorf("ArtifactRecord.ContentHash() = %v, want abc123", r.ContentHash())
	}
	if r.Overwrite() != false {
		t.Errorf("ArtifactRecord.Overwrite() = %v, want false", r.Overwrite())
	}
	if r.Bytes() != 1024 {
		t.Errorf("ArtifactRecord.Bytes() = %v, want 1024", r.Bytes())
	}
	if r.RecordedAt() != now {
		t.Errorf("ArtifactRecord.RecordedAt() = %v, want %v", r.RecordedAt(), now)
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
	e := metadata.NewPipelineEvent(
		metadata.StageExtract,
		"https://example.com/page",
		true,
		now,
		5,
	)

	if e.Stage() != metadata.StageExtract {
		t.Errorf("PipelineEvent.Stage() = %v, want %v", e.Stage(), metadata.StageExtract)
	}
	if e.PageURL() != "https://example.com/page" {
		t.Errorf("PipelineEvent.PageURL() = %v, want https://example.com/page", e.PageURL())
	}
	if !e.Success() {
		t.Error("PipelineEvent.Success() = false, want true")
	}
	if e.RecordedAt() != now {
		t.Errorf("PipelineEvent.RecordedAt() = %v, want %v", e.RecordedAt(), now)
	}
	if e.LinksFound() != 5 {
		t.Errorf("PipelineEvent.LinksFound() = %v, want 5", e.LinksFound())
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
	e := metadata.NewSkipEvent(
		"https://example.com/disallowed",
		metadata.SkipReasonRobotsDisallow,
		now,
	)

	if e.SkippedURL() != "https://example.com/disallowed" {
		t.Errorf("SkipEvent.SkippedURL() = %v, want %v", e.SkippedURL(), "https://example.com/disallowed")
	}
	if e.Reason() != metadata.SkipReasonRobotsDisallow {
		t.Errorf("SkipEvent.Reason() = %v, want %v", e.Reason(), metadata.SkipReasonRobotsDisallow)
	}
	if e.RecordedAt() != now {
		t.Errorf("SkipEvent.RecordedAt() = %v, want %v", e.RecordedAt(), now)
	}
}

func TestCrawlStatsConstruction(t *testing.T) {
	start := time.Now()
	end := start.Add(5 * time.Second)
	s := metadata.NewCrawlStats(start, end, 10, 2, 5, 1)

	if s.StartedAt() != start {
		t.Errorf("CrawlStats.StartedAt() = %v, want %v", s.StartedAt(), start)
	}
	if !s.FinishedAt().After(s.StartedAt()) {
		t.Error("CrawlStats.FinishedAt() must be after StartedAt()")
	}
	if s.TotalPages() != 10 {
		t.Errorf("CrawlStats.TotalPages() = %v, want 10", s.TotalPages())
	}
	if s.TotalErrors() != 2 {
		t.Errorf("CrawlStats.TotalErrors() = %v, want 2", s.TotalErrors())
	}
	if s.TotalAssets() != 5 {
		t.Errorf("CrawlStats.TotalAssets() = %v, want 5", s.TotalAssets())
	}
	if s.ManualRetryQueueCount() != 1 {
		t.Errorf("CrawlStats.ManualRetryQueueCount() = %v, want 1", s.ManualRetryQueueCount())
	}
}

func TestErrorEventConstruction(t *testing.T) {
	now := time.Now()
	attrs := []metadata.Attribute{metadata.NewAttr(metadata.AttrURL, "https://example.com")}
	e := metadata.NewErrorEvent(
		now,
		"fetcher",
		"Fetch",
		metadata.CauseNetworkFailure,
		"connection refused",
		attrs,
	)

	if e.PackageName() != "fetcher" {
		t.Errorf("ErrorEvent.PackageName() = %v, want fetcher", e.PackageName())
	}
	if e.Action() != "Fetch" {
		t.Errorf("ErrorEvent.Action() = %v, want Fetch", e.Action())
	}
	if e.Cause() != metadata.CauseNetworkFailure {
		t.Errorf("ErrorEvent.Cause() = %v, want %v", e.Cause(), metadata.CauseNetworkFailure)
	}
	if e.Details() != "connection refused" {
		t.Errorf("ErrorEvent.Details() = %v, want connection refused", e.Details())
	}
	if e.ObservedAt() != now {
		t.Errorf("ErrorEvent.ObservedAt() = %v, want %v", e.ObservedAt(), now)
	}
	if len(e.Attrs()) != 1 {
		t.Errorf("ErrorEvent.Attrs() len = %v, want 1", len(e.Attrs()))
	}
}

func TestErrorEventAttrsImmutability(t *testing.T) {
	// Verify that Attrs() returns a copy — mutating the returned slice must not
	// affect the event's internal state.
	attrs := []metadata.Attribute{metadata.NewAttr(metadata.AttrURL, "https://example.com")}
	e := metadata.NewErrorEvent(time.Now(), "pkg", "action", metadata.CauseUnknown, "details", attrs)

	got := e.Attrs()
	got[0] = metadata.NewAttr(metadata.AttrHost, "mutated")

	// The event must still return the original attribute.
	if e.Attrs()[0].Key() != metadata.AttrURL {
		t.Error("ErrorEvent.Attrs() is not a copy — external mutation affected internal state")
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
	t.Run("fetch event getters", func(t *testing.T) {
		fe := metadata.NewFetchEvent(
			time.Now(), "https://example.com", 200, time.Second, "text/html", 0, 0, metadata.KindPage,
		)
		if fe.Kind() != metadata.KindPage {
			t.Errorf("FetchEvent.Kind() = %v, want %v", fe.Kind(), metadata.KindPage)
		}
		if fe.FetchURL() != "https://example.com" {
			t.Errorf("FetchEvent.FetchURL() = %v, want https://example.com", fe.FetchURL())
		}
	})

	t.Run("artifact record getters", func(t *testing.T) {
		ar := metadata.NewArtifactRecord(
			metadata.ArtifactMarkdown, "/out/page.md", "https://example.com", "hash", false, 512, time.Now(),
		)
		if ar.Kind() != metadata.ArtifactMarkdown {
			t.Errorf("ArtifactRecord.Kind() = %v, want %v", ar.Kind(), metadata.ArtifactMarkdown)
		}
	})

	t.Run("pipeline event getters", func(t *testing.T) {
		pe := metadata.NewPipelineEvent(metadata.StageExtract, "https://example.com", true, time.Now(), 3)
		if pe.Stage() != metadata.StageExtract {
			t.Errorf("PipelineEvent.Stage() = %v, want %v", pe.Stage(), metadata.StageExtract)
		}
		if !pe.Success() {
			t.Error("PipelineEvent.Success() = false, want true")
		}
	})

	t.Run("skip event getters", func(t *testing.T) {
		se := metadata.NewSkipEvent("https://example.com/disallowed", metadata.SkipReasonRobotsDisallow, time.Now())
		if se.Reason() != metadata.SkipReasonRobotsDisallow {
			t.Errorf("SkipEvent.Reason() = %v, want %v", se.Reason(), metadata.SkipReasonRobotsDisallow)
		}
	})

	t.Run("error event getters", func(t *testing.T) {
		ee := metadata.NewErrorEvent(time.Now(), "pkg", "action", metadata.CauseNetworkFailure, "details", nil)
		if ee.Cause() != metadata.CauseNetworkFailure {
			t.Errorf("ErrorEvent.Cause() = %v, want %v", ee.Cause(), metadata.CauseNetworkFailure)
		}
	})

	t.Run("crawl stats getters", func(t *testing.T) {
		start := time.Now()
		cs := metadata.NewCrawlStats(start, start.Add(time.Second), 3, 0, 0, 0)
		if cs.TotalPages() != 3 {
			t.Errorf("CrawlStats.TotalPages() = %v, want 3", cs.TotalPages())
		}
	})
}
