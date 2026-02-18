package storage_test

import (
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
)

// Compile-time interface check.
var _ metadata.MetadataSink = (*metadataSinkMock)(nil)

// metadataSinkMock is a mock for metadata.MetadataSink
type metadataSinkMock struct {
	recordErrorCalled    bool
	recordErrorRecord    metadata.ErrorRecord
	recordFetchCalled    bool
	recordArtifactCalled bool
	recordArtifactRecord metadata.ArtifactRecord
	recordPipelineCalled bool
	recordSkipCalled     bool
}

func (m *metadataSinkMock) RecordError(record metadata.ErrorRecord) {
	m.recordErrorCalled = true
	m.recordErrorRecord = record
}

func (m *metadataSinkMock) RecordFetch(event metadata.FetchEvent) {
	m.recordFetchCalled = true
}

func (m *metadataSinkMock) RecordArtifact(record metadata.ArtifactRecord) {
	m.recordArtifactCalled = true
	m.recordArtifactRecord = record
}

func (m *metadataSinkMock) RecordPipelineStage(event metadata.PipelineEvent) {
	m.recordPipelineCalled = true
}

func (m *metadataSinkMock) RecordSkip(event metadata.SkipEvent) {
	m.recordSkipCalled = true
}

// Reset clears all recorded state
func (m *metadataSinkMock) Reset() {
	m.recordErrorCalled = false
	m.recordErrorRecord = metadata.ErrorRecord{}
	m.recordFetchCalled = false
	m.recordArtifactCalled = false
	m.recordArtifactRecord = metadata.ArtifactRecord{}
	m.recordPipelineCalled = false
	m.recordSkipCalled = false
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
