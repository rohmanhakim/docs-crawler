package scheduler_test

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/stretchr/testify/mock"
)

// convertMock is a testify mock for the mdconvert.ConvertRule
type convertMock struct {
	mock.Mock
}

// Convert mocks the Convert method
func (c *convertMock) Convert(
	sanitizedHTMLDoc sanitizer.SanitizedHTMLDoc,
) (mdconvert.ConversionResult, failure.ClassifiedError) {
	args := c.Called(sanitizedHTMLDoc)
	result := args.Get(0).(mdconvert.ConversionResult)
	var err failure.ClassifiedError
	if args.Get(1) != nil {
		err = args.Get(1).(failure.ClassifiedError)
	}
	return result, err
}

// newConvertMockForTest creates a properly configured convert mock for tests
func newConvertMockForTest(t *testing.T) *convertMock {
	t.Helper()
	m := new(convertMock)
	return m
}

// setupConvertMockWithSuccess sets up the convert mock to return a successful conversion result
func setupConvertMockWithSuccess(m *convertMock) {
	result := mdconvert.NewConversionResult(
		[]byte("# Test Markdown\n\nThis is test content."),
		[]mdconvert.LinkRef{},
	)
	m.On("Convert", mock.Anything).Return(result, nil)
}

// setupConvertMockWithFatalError sets up the convert mock to return a fatal error
func setupConvertMockWithFatalError(m *convertMock) {
	convertErr := &mdconvert.ConversionError{
		Message:   "fatal conversion error",
		Retryable: false,
		Cause:     mdconvert.ErrCauseConversionFailure,
	}
	m.On("Convert", mock.Anything).Return(mdconvert.ConversionResult{}, convertErr)
}

// setupConvertMockWithRecoverableError sets up the convert mock to return a recoverable error
func setupConvertMockWithRecoverableError(m *convertMock) {
	convertErr := &mdconvert.ConversionError{
		Message:   "recoverable conversion error",
		Retryable: true,
		Cause:     mdconvert.ErrCauseConversionFailure,
	}
	m.On("Convert", mock.Anything).Return(mdconvert.ConversionResult{}, convertErr)
}

// setupConvertMockWithCustomResult sets up the convert mock to return a custom result
func setupConvertMockWithCustomResult(m *convertMock, result mdconvert.ConversionResult) {
	m.On("Convert", mock.Anything).Return(result, nil)
}

// createConversionResultForTest creates a ConversionResult for testing
func createConversionResultForTest(content string, linkRefs []mdconvert.LinkRef) mdconvert.ConversionResult {
	if linkRefs == nil {
		linkRefs = []mdconvert.LinkRef{}
	}
	return mdconvert.NewConversionResult([]byte(content), linkRefs)
}
