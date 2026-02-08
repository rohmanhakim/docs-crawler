package scheduler_test

import (
	"net/url"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// sanitizerMock is a testify mock for the sanitizer.HtmlSanitizer
type sanitizerMock struct {
	mock.Mock
}

// Sanitize mocks the Sanitize method
func (s *sanitizerMock) Sanitize(inputContentNode *html.Node) (sanitizer.SanitizedHTMLDoc, failure.ClassifiedError) {
	args := s.Called(inputContentNode)
	result := args.Get(0).(sanitizer.SanitizedHTMLDoc)
	var err failure.ClassifiedError
	if args.Get(1) != nil {
		err = args.Get(1).(failure.ClassifiedError)
	}
	return result, err
}

// newSanitizerMockForTest creates a properly configured sanitizer mock for tests
func newSanitizerMockForTest(t *testing.T) *sanitizerMock {
	t.Helper()
	m := new(sanitizerMock)
	return m
}

// setupSanitizerMockWithSuccess sets up the sanitizer mock to return a successful sanitization result
func setupSanitizerMockWithSuccess(m *sanitizerMock, discoveredURLs []url.URL) {
	// The actual implementation will use the mock's Return value
	m.On("Sanitize", mock.Anything).Return(createSanitizedHTMLDocForTest(discoveredURLs), nil)
}

// setupSanitizerMockWithFatalError sets up the sanitizer mock to return a fatal error
func setupSanitizerMockWithFatalError(m *sanitizerMock) {
	sanitizerErr := &sanitizer.SanitizationError{
		Message:   "fatal sanitization error",
		Retryable: false,
		Cause:     sanitizer.ErrCauseCompetingRoots,
	}
	m.On("Sanitize", mock.Anything).Return(sanitizer.SanitizedHTMLDoc{}, sanitizerErr)
}

// setupSanitizerMockWithRecoverableError sets up the sanitizer mock to return a recoverable error
// Note: Sanitizer errors are typically fatal per the design, but this allows testing error handling
func setupSanitizerMockWithRecoverableError(m *sanitizerMock) {
	sanitizerErr := &mockClassifiedError{
		msg:      "recoverable sanitization error",
		severity: failure.SeverityRecoverable,
	}
	m.On("Sanitize", mock.Anything).Return(sanitizer.SanitizedHTMLDoc{}, sanitizerErr)
}

// createSanitizedHTMLDocForTest creates a SanitizedHTMLDoc for testing
// Since the fields are private, we use the real sanitizer's internal structure
// In tests, we'll use the mock's On().Return() with a properly constructed result
func createSanitizedHTMLDocForTest(discoveredURLs []url.URL) sanitizer.SanitizedHTMLDoc {
	// We need to use the NewSanitizedHTMLDocForTest function if available
	// or construct it using a test helper from the sanitizer package
	// For now, return empty - the test will use mock.On().Return() directly
	return sanitizer.SanitizedHTMLDoc{}
}
