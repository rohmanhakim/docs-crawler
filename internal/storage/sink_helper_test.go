package storage_test

import (
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
)

// metadataSinkMock is a mock for metadata.MetadataSink
type metadataSinkMock struct {
	recordErrorCalled      bool
	recordErrorObservedAt  time.Time
	recordErrorPackageName string
	recordErrorAction      string
	recordErrorCause       metadata.ErrorCause
	recordErrorDetails     string
	recordErrorAttrs       []metadata.Attribute
	recordFetchCalled      bool
	recordAssetFetchCalled bool
	recordArtifactCalled   bool
	recordArtifactKind     metadata.ArtifactKind
	recordArtifactPath     string
	recordArtifactAttrs    []metadata.Attribute
}

func (m *metadataSinkMock) RecordError(
	observedAt time.Time,
	packageName string,
	action string,
	cause metadata.ErrorCause,
	details string,
	attrs []metadata.Attribute,
) {
	m.recordErrorCalled = true
	m.recordErrorObservedAt = observedAt
	m.recordErrorPackageName = packageName
	m.recordErrorAction = action
	m.recordErrorCause = cause
	m.recordErrorDetails = details
	m.recordErrorAttrs = attrs
}

func (m *metadataSinkMock) RecordFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	contentType string,
	retryCount int,
	crawlDepth int,
) {
	m.recordFetchCalled = true
}

func (m *metadataSinkMock) RecordAssetFetch(
	fetchUrl string,
	httpStatus int,
	duration time.Duration,
	retryCount int,
) {
	m.recordAssetFetchCalled = true
}

func (m *metadataSinkMock) RecordArtifact(kind metadata.ArtifactKind, path string, attrs []metadata.Attribute) {
	m.recordArtifactCalled = true
	m.recordArtifactKind = kind
	m.recordArtifactPath = path
	m.recordArtifactAttrs = attrs
}

// Reset clears all recorded state
func (m *metadataSinkMock) Reset() {
	m.recordErrorCalled = false
	m.recordErrorObservedAt = time.Time{}
	m.recordErrorPackageName = ""
	m.recordErrorAction = ""
	m.recordErrorCause = 0
	m.recordErrorDetails = ""
	m.recordErrorAttrs = nil
	m.recordFetchCalled = false
	m.recordAssetFetchCalled = false
	m.recordArtifactCalled = false
	m.recordArtifactKind = ""
	m.recordArtifactPath = ""
	m.recordArtifactAttrs = nil
}

// createTestNormalizedDoc creates a normalized document for testing
func createTestNormalizedDoc(sourceURL, canonicalURL, contentHash string, content []byte) normalize.NormalizedMarkdownDoc {
	frontmatter := normalize.NewFrontmatter(
		"Test Title", // title
		sourceURL,    // sourceURL
		canonicalURL, // canonicalURL
		1,            // crawlDepth
		"docs",       // section
		"doc123",     // docID
		contentHash,  // contentHash
		time.Now(),   // fetchedAt
		"1.0.0",      // crawlerVersion
	)
	return normalize.NewNormalizedMarkdownDoc(frontmatter, content)
}

// computeExpectedURLHash computes the expected URL hash for a given canonical URL
func computeExpectedURLHash(canonicalURL string, hashAlgo hashutil.HashAlgo) string {
	hash, _ := hashutil.HashBytes([]byte(canonicalURL), hashAlgo)
	return hash[:12] // First 12 hex characters
}

// findAttrValue finds an attribute value by key in a slice of attributes
func findAttrValue(attrs []metadata.Attribute, key metadata.AttributeKey) string {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Value
		}
	}
	return ""
}
