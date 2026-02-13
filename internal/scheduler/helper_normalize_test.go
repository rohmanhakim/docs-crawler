package scheduler_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/stretchr/testify/mock"
)

// normalizeMock mocks the normalize.MarkdownConstraint type
// It embeds the real MarkdownConstraint to satisfy the type requirement
// but overrides the Normalize method for mocking
type normalizeMock struct {
	normalize.MarkdownConstraint
	mock.Mock
}

// Normalize mocks the Normalize method of MarkdownConstraint
func (n *normalizeMock) Normalize(
	fetchUrl url.URL,
	assetfulMarkdownDoc assets.AssetfulMarkdownDoc,
	normalizeParam normalize.NormalizeParam,
) (normalize.NormalizedMarkdownDoc, failure.ClassifiedError) {
	args := n.Called(fetchUrl, assetfulMarkdownDoc, normalizeParam)
	doc := args.Get(0).(normalize.NormalizedMarkdownDoc)
	var err failure.ClassifiedError
	if args.Get(1) != nil {
		err = args.Get(1).(failure.ClassifiedError)
	}
	return doc, err
}

func newNormalizeMockForTest(t *testing.T) *normalizeMock {
	t.Helper()
	m := new(normalizeMock)
	// Initialize the embedded MarkdownConstraint with a noop sink to avoid nil pointer dereference
	// if the real Normalize method is called instead of the mock's override
	m.MarkdownConstraint = normalize.NewMarkdownConstraint(&metadata.NoopSink{})
	return m
}

// setupNormalizeMockWithSuccess sets up the normalize mock to return a successful result
func setupNormalizeMockWithSuccess(m *normalizeMock) {
	// Return a properly initialized NormalizedMarkdownDoc with non-empty content
	doc := normalize.NewNormalizedMarkdownDoc(
		normalize.NewFrontmatter(
			"Test Title",
			"http://example.com/test",
			"http://example.com/test",
			1,
			"test",
			"doc123",
			"sha256:abc123",
			time.Time{},
			"v0.1.0",
		),
		[]byte("# Test Title\n\nTest content for normalization."),
	)
	m.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(doc, nil)
}

// setupNormalizeMockWithFatalError sets up the normalize mock to return a fatal error
func setupNormalizeMockWithFatalError(m *normalizeMock) {
	normalizeErr := &normalize.NormalizationError{
		Message:   "fatal normalization error: broken H1 invariant",
		Retryable: false,
		Cause:     normalize.ErrCauseBrokenH1Invariant,
	}
	m.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(normalize.NormalizedMarkdownDoc{}, normalizeErr)
}

// setupNormalizeMockWithRecoverableError sets up the normalize mock to return a recoverable error
func setupNormalizeMockWithRecoverableError(m *normalizeMock) {
	normalizeErr := &normalize.NormalizationError{
		Message:   "recoverable normalization error",
		Retryable: true,
		Cause:     normalize.ErrCauseHashComputationFailed,
	}
	m.On("Normalize", mock.Anything, mock.Anything, mock.Anything).
		Return(normalize.NormalizedMarkdownDoc{}, normalizeErr)
}

// createNormalizedMarkdownDocForTest creates a NormalizedMarkdownDoc for testing
func createNormalizedMarkdownDocForTest(content string) normalize.NormalizedMarkdownDoc {
	return normalize.NewNormalizedMarkdownDoc(
		normalize.NewFrontmatter(
			"Test Title",
			"http://example.com/test",
			"http://example.com/test",
			1,
			"test",
			"doc123",
			"sha256:abc123",
			time.Time{}, // fetchedAt - use zero time
			"v0.1.0",
		),
		[]byte(content),
	)
}
