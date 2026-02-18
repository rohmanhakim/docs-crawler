package metadata_test

import (
	"testing"

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
