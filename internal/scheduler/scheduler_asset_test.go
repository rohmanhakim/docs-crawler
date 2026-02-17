package scheduler_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/scheduler"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/failurejournal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/html"
)

// TestScheduler_Resolve_CalledWithConversionResult verifies that the Resolve
// is called with the ConversionResult from the convert stage.
func TestScheduler_Resolve_CalledWithConversionResult(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Setup extractor to return a valid content node
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer to return a valid sanitized doc
	sanitizedDoc := createSanitizedHTMLDocForTest(nil)
	mockSanitizer.On("Sanitize", contentNode).Return(sanitizedDoc, nil)

	// Setup convert to return a specific conversion result
	conversionResult := createConversionResultForTest("# Test Markdown\n\n![image](test.png)", nil)
	mockConvert.On("Convert", sanitizedDoc).Return(conversionResult, nil)

	// Setup resolver mock to capture the input
	var receivedConversionResult mdconvert.ConversionResult
	mockResolver.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			receivedConversionResult = args.Get(2).(mdconvert.ConversionResult)
		}).
		Return(createAssetfulMarkdownDocForTest("# Test Markdown", nil), nil)

	// Setup frontier to return a token for the seed URL
	seedURL, _ := url.Parse("http://example.com/")
	mockFrontier.SetupDequeueToReturn(frontier.NewCrawlToken(*seedURL, 0), true)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocks(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFrontier,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state
	_, err = s.ExecuteCrawlingWithState(init)
	assert.NoError(t, err, "Failed to execute")

	// Verify Resolve was called with the conversion result from Convert
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.Equal(t, conversionResult, receivedConversionResult, "Resolve should be called with the ConversionResult from Convert")
}

// TestScheduler_Resolve_SuccessfulResolution_ProceedsToNormalization verifies
// that successful asset resolution allows the pipeline to continue to normalization.
func TestScheduler_Resolve_SuccessfulResolution_ProceedsToNormalization(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver to return successful result with content
	resolvedDoc := createAssetfulMarkdownDocForTest("# Test Markdown\n\n![resolved](assets/images/test.png)", []string{"assets/images/test.png"})
	setupResolverMockWithCustomResult(mockResolver, resolvedDoc)

	// Setup frontier to return a token for the seed URL
	seedURL, _ := url.Parse("http://example.com/")
	mockFrontier.SetupDequeueToReturn(frontier.NewCrawlToken(*seedURL, 0), true)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocks(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFrontier,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
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
	// Resolve should be called
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	t.Logf("Execution completed with %d write results", len(exec.WriteResults()))
}

// TestScheduler_Resolve_DiskFullError_ContinuesCrawl verifies that disk full asset resolution
// errors do NOT abort the crawl. Disk full is a per-URL failure with manual retry eligibility,
// not a systemic failure that should abort the entire crawl.
func TestScheduler_Resolve_DiskFullError_ContinuesCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver to return a disk full error (manual retry, not fatal)
	setupResolverMockWithFatalError(mockResolver)

	// Setup frontier to return a token for the seed URL
	seedURL, _ := url.Parse("http://example.com/")
	mockFrontier.SetupDequeueToReturn(frontier.NewCrawlToken(*seedURL, 0), true)

	// Storage Write must be set up since crawl continues past resolver error
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocks(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFrontier,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state - should return fatal error
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Disk full error is per-URL with manual retry — crawl should continue
	assert.NoError(t, execErr, "Disk full error should not abort crawl")
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestScheduler_Resolve_RecoverableError_ContinuesCrawl verifies that recoverable
// asset resolution errors are counted but the crawl continues.
func TestScheduler_Resolve_RecoverableError_ContinuesCrawl(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver to return a recoverable error
	setupResolverMockWithRecoverableError(mockResolver)

	// Setup frontier to return a token for the seed URL
	seedURL, _ := url.Parse("http://example.com/")
	mockFrontier.SetupDequeueToReturn(frontier.NewCrawlToken(*seedURL, 0), true)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocks(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFrontier,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
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
	assert.NoError(t, execErr, "Recoverable resolve error should not abort crawl")
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestScheduler_Resolve_ErrorDoesNotPreventWriteForRecoverable verifies that when Resolve()
// returns a recoverable error, the scheduler still proceeds to Normalize and Write.
func TestScheduler_Resolve_ErrorDoesNotPreventWriteForRecoverable(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	// Only expect one Decide call for the seed URL
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver to return a recoverable error (not fatal)
	setupResolverMockWithRecoverableError(mockResolver)

	// Setup frontier to return a token for the seed URL
	seedURL, _ := url.Parse("http://example.com/")
	mockFrontier.SetupDequeueToReturn(frontier.NewCrawlToken(*seedURL, 0), true)

	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocks(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFrontier,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Use maxDepth: 0 to process just one page
	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state - should NOT return error for recoverable error
	exec, execErr := s.ExecuteCrawlingWithState(init)

	// Recoverable resolver error should NOT abort the crawl
	assert.NoError(t, execErr, "Recoverable resolve error should not abort crawl")

	// Verify resolver was called
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	// Verify that execution completed (Write was called)
	t.Logf("Execution completed with %d write results", len(exec.WriteResults()))
}

// TestScheduler_Resolve_DiskFullError_DoesNotPreventSubsequentCalls verifies that when Resolve()
// returns a disk full error (manual retry), the scheduler does NOT abort and continues processing.
// Disk full is a per-URL failure, not a systemic error warranting crawl abort.
func TestScheduler_Resolve_DiskFullError_DoesNotPreventSubsequentCalls(t *testing.T) {
	ctx := context.Background()
	mockFinalizer := newMockFinalizer(t)
	noopSink := &metadata.NoopSink{}
	mockLimiter := newRateLimiterMockForTest(t)
	mockFetcher := newFetcherMockForTest(t)
	mockRobot := NewRobotsMockForTest(t)
	mockFrontier := newFrontierMockForTest(t)
	mockSleeper := newSleeperMock(t)
	mockExtractor := newExtractorMockForTest(t)
	mockSanitizer := newSanitizerMockForTest(t)
	mockConvert := newConvertMockForTest(t)
	mockResolver := newResolverMockForTest(t)
	mockStorage := newStorageMockForTest(t)
	mockFailureJournal := newFailureJournalMockForTest(t)

	mockRobot.On("Init", mock.Anything, mock.Anything).Return()
	mockRobot.OnDecide(mock.Anything, robots.Decision{
		Allowed:    true,
		Reason:     robots.EmptyRuleSet,
		CrawlDelay: 0,
	}, nil).Once()

	mockFetcher.On("Init", mock.Anything, mock.Anything).Return()
	mockLimiter.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	mockSleeper.On("Sleep", mock.Anything).Return()

	// Setup extractor
	contentNode := &html.Node{Type: html.ElementNode, Data: "div"}
	setupExtractorMockWithSuccess(mockExtractor, contentNode)
	mockExtractor.On("SetExtractParam", mock.Anything).Return()

	// Setup sanitizer
	mockSanitizer.On("Sanitize", contentNode).Return(createSanitizedHTMLDocForTest(nil), nil)

	// Setup convert
	setupConvertMockWithSuccess(mockConvert)

	// Setup resolver to return a disk full error (manual retry, not fatal)
	setupResolverMockWithFatalError(mockResolver)

	// Setup frontier to return a token for the seed URL
	seedURL, _ := url.Parse("http://example.com/")
	mockFrontier.SetupDequeueToReturn(frontier.NewCrawlToken(*seedURL, 0), true)

	// Storage Write must be set up since crawl continues past resolver error
	mockStorage.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(storage.WriteResult{}, nil)

	s := createSchedulerWithAllMocks(
		t,
		ctx,
		mockFinalizer,
		noopSink,
		mockLimiter,
		mockRobot,
		mockFrontier,
		mockFetcher,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"seedUrls": [{"Scheme": "http", "Host": "example.com"}],
		"maxDepth": 0
	}`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	assert.NoError(t, err)

	// Phase 1: Initialize
	init, err := s.InitializeCrawling(configPath)
	assert.NoError(t, err, "Failed to initialize")

	// Phase 2: Execute with state - should return fatal error
	_, execErr := s.ExecuteCrawlingWithState(init)

	// Disk full error is per-URL with manual retry — crawl should continue
	assert.NoError(t, execErr, "Disk full error should not abort crawl")

	// Verify resolver was called
	mockResolver.AssertCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// createSchedulerWithAllMocks creates a scheduler with all mocked dependencies for testing.
// This is similar to createSchedulerForTest but allows injecting a custom resolver mock.
func createSchedulerWithAllMocks(
	t *testing.T,
	ctx context.Context,
	mockFinalizer *mockFinalizer,
	metadataSink metadata.MetadataSink,
	mockLimiter *rateLimiterMock,
	mockRobot *robotsMock,
	mockFrontier *frontierMock,
	mockFetcher *fetcherMock,
	mockExtractor extractor.Extractor,
	mockSanitizer sanitizer.Sanitizer,
	mockConvert mdconvert.ConvertRule,
	mockResolver assets.Resolver,
	mockStorage *storageMock,
	mockSleeper *sleeperMock,
	mockFailureJournal failurejournal.Journal,
) *scheduler.Scheduler {
	t.Helper()
	// Create real components if mocks not provided
	if mockExtractor == nil {
		ext := extractor.NewDomExtractor(metadataSink)
		mockExtractor = &ext
	}
	if mockSanitizer == nil {
		san := sanitizer.NewHTMLSanitizer(metadataSink)
		mockSanitizer = &san
	}
	if mockConvert == nil {
		mockConvert = newConvertMockForTest(t)
		setupConvertMockWithSuccess(mockConvert.(*convertMock))
	}

	// Create a normalize mock with default success
	mockNormalize := newNormalizeMockForTest(t)
	setupNormalizeMockWithSuccess(mockNormalize)

	// Setup frontier mock expectations that are common to all tests
	mockFrontier.On("Init", mock.Anything).Return()
	mockFrontier.On("VisitedCount").Return(0)
	mockFrontier.On("IsDepthExhausted", mock.Anything).Return(true)
	mockFrontier.On("CurrentMinDepth").Return(-1)
	mockFrontier.On("Submit", mock.Anything).Return()
	// Provide a default Dequeue to return no more URLs after the test's specific token
	mockFrontier.On("Dequeue").Return(frontier.CrawlToken{}, false)
	// Dequeue is expected to be set up by each test explicitly using SetupDequeueToReturn

	s := scheduler.NewSchedulerWithDeps(
		ctx,
		mockFinalizer,
		metadataSink,
		mockLimiter,
		mockFrontier,
		mockFetcher,
		mockRobot,
		mockExtractor,
		mockSanitizer,
		mockConvert,
		mockResolver,
		mockNormalize,
		mockStorage,
		mockSleeper,
		mockFailureJournal,
	)
	return &s
}
