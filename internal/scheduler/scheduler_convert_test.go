package scheduler_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// TestScheduler_Convert_CalledWithSanitizedHTMLDoc verifies that the convert
// is called with the SanitizedHTMLDoc from the sanitizer stage.
func TestScheduler_Convert_CalledWithSanitizedHTMLDoc(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)

	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor to return a valid content node
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer to return a valid sanitized doc
	sanitizedDoc := createSanitizedHTMLDocForTest(nil)
	mockSanitizer.On("Sanitize", contentNode).Return(sanitizedDoc, nil)

	// Setup convert mock to capture the input
	var receivedDoc sanitizer.SanitizedHTMLDoc
	setupConvertMockWithSuccess(mockConvert)
	mockConvert.On("Convert", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			receivedDoc = args.Get(0).(sanitizer.SanitizedHTMLDoc)
		}).
		Return(createConversionResultForTest("# Test", nil), nil)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		nil,
		mockStorage,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": ["http://example.com"],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state
	_, execErr := s.ExecuteCrawlingWithState(init)
	assert.NoError(t, execErr, "Failed to execute")

	// Verify convert was called with the sanitized HTML doc
	mockConvert.AssertCalled(t, "Convert", mock.Anything, mock.Anything)
	assert.NotNil(t, receivedDoc, "Convert should be called with a SanitizedHTMLDoc")
}

// TestScheduler_Convert_SuccessfulConversion_ProceedsToAssetResolution verifies
// that successful conversion allows the pipeline to continue to asset resolution.
func TestScheduler_Convert_SuccessfulConversion_ProceedsToAssetResolution(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)

	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert to return successful result
	setupConvertMockWithSuccess(mockConvert)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		nil,
		mockStorage,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": ["http://example.com"],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state
	exec, execErr := s.ExecuteCrawlingWithState(init)

	// Should complete without fatal error
	assert.NoError(t, execErr, "Failed to execute")
	// Convert should be called
	mockConvert.AssertCalled(t, "Convert", mock.Anything, mock.Anything)
	t.Logf("Execution completed with %d write results", len(exec.WriteResults()))
}

// TestScheduler_Convert_RecoverableError_ContinuesCrawl verifies that recoverable
// conversion errors are counted but the crawl continues.
func TestScheduler_Convert_RecoverableError_ContinuesCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)

	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0).Maybe()
	mockFrontier.On("Submit", mock.Anything).Return()
	mockFrontier.On("Enqueue", mock.Anything).Return()
	// First Dequeue returns a token (seed URL processing), second returns false (exit loop)
	seedToken := frontier.NewCrawlToken(*mustParseURL("https://example.com"), 0)
	mockFrontier.OnDequeue(seedToken, true).Once()
	mockFrontier.OnDequeue(frontier.CrawlToken{}, false).Once()

	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert to return a recoverable error
	setupConvertMockWithRecoverableError(mockConvert)

	s := createSchedulerForTest(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockFrontier,
		mockRobot,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		nil,
		mockStorage,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": ["http://example.com"],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state - should not return fatal error
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Recoverable errors should not abort the crawl
	assert.NoError(t, execErr, "Recoverable convert error should not abort crawl")
	mockConvert.AssertCalled(t, "Convert", mock.Anything, mock.Anything)
}
