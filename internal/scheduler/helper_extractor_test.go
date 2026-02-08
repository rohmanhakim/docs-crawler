package scheduler_test

import (
	"net/url"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// extractorMock is a testify mock for the extractor.Extractor
type extractorMock struct {
	mock.Mock
}

// Extract mocks the Extract method
func (e *extractorMock) Extract(sourceUrl url.URL, htmlByte []byte) (extractor.ExtractionResult, failure.ClassifiedError) {
	args := e.Called(sourceUrl, htmlByte)
	result := args.Get(0).(extractor.ExtractionResult)
	var err failure.ClassifiedError
	if args.Get(1) != nil {
		err = args.Get(1).(failure.ClassifiedError)
	}
	return result, err
}

// SetExtractParam mocks the SetExtractParam method
func (e *extractorMock) SetExtractParam(params extractor.ExtractParam) {
	e.Called(params)
}

// newExtractorMockForTest creates a properly configured extractor mock for tests
func newExtractorMockForTest(t *testing.T) *extractorMock {
	t.Helper()
	m := new(extractorMock)
	return m
}

// setupExtractorMockWithSuccess sets up the extractor mock to return a successful extraction result
func setupExtractorMockWithSuccess(m *extractorMock, contentNode *html.Node) {
	result := extractor.ExtractionResult{
		DocumentRoot: contentNode, // Using contentNode as DocumentRoot for simplicity
		ContentNode:  contentNode,
	}
	m.On("Extract", mock.Anything, mock.Anything).Return(result, nil)
}

// setupExtractorMockWithError sets up the extractor mock to return an error
func setupExtractorMockWithError(m *extractorMock, err failure.ClassifiedError) {
	m.On("Extract", mock.Anything, mock.Anything).Return(extractor.ExtractionResult{}, err)
}

// setupExtractorMockWithSetExtractParamExpectation sets up the extractor mock to expect SetExtractParam call
func setupExtractorMockWithSetExtractParamExpectation(m *extractorMock, params extractor.ExtractParam) {
	m.On("SetExtractParam", params).Return()
}

func verifyExtractParam(t *testing.T, actual extractor.ExtractParam, expected extractor.ExtractParam) {
	t.Helper()
	assert.InDelta(t, expected.BodySpecificityBias, actual.BodySpecificityBias, 0.0001, "BodySpecificityBias mismatch")
	assert.InDelta(t, expected.LinkDensityThreshold, actual.LinkDensityThreshold, 0.0001, "LinkDensityThreshold mismatch")
	assert.InDelta(t, expected.ScoreMultiplier.NonWhitespaceDivisor, actual.ScoreMultiplier.NonWhitespaceDivisor, 0.0001, "NonWhitespaceDivisor mismatch")
	assert.InDelta(t, expected.ScoreMultiplier.Paragraphs, actual.ScoreMultiplier.Paragraphs, 0.0001, "Paragraphs multiplier mismatch")
	assert.InDelta(t, expected.ScoreMultiplier.Headings, actual.ScoreMultiplier.Headings, 0.0001, "Headings multiplier mismatch")
	assert.InDelta(t, expected.ScoreMultiplier.CodeBlocks, actual.ScoreMultiplier.CodeBlocks, 0.0001, "CodeBlocks multiplier mismatch")
	assert.InDelta(t, expected.ScoreMultiplier.ListItems, actual.ScoreMultiplier.ListItems, 0.0001, "ListItems multiplier mismatch")
	assert.Equal(t, expected.Threshold.MinNonWhitespace, actual.Threshold.MinNonWhitespace, "MinNonWhitespace mismatch")
	assert.Equal(t, expected.Threshold.MinHeadings, actual.Threshold.MinHeadings, "MinHeadings mismatch")
	assert.Equal(t, expected.Threshold.MinParagraphsOrCode, actual.Threshold.MinParagraphsOrCode, "MinParagraphsOrCode mismatch")
	assert.InDelta(t, expected.Threshold.MaxLinkDensity, actual.Threshold.MaxLinkDensity, 0.0001, "MaxLinkDensity mismatch")
}
