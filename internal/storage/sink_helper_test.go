package storage_test

import (
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata/metadatatest"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
)

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

// metadataSinkMock is an alias to the shared mock for backward compatibility
// with existing test code. New tests should use metadatatest.SinkMock directly.
type metadataSinkMock = metadatatest.SinkMock
