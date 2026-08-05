package scheduler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/stagedump"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/failurejournal"
	"github.com/rohmanhakim/docs-crawler/pkg/urlutil"
	gopipeline "github.com/rohmanhakim/gopipeline"
	ratelimiter "github.com/rohmanhakim/rate-limiter"
	"github.com/rohmanhakim/retrier"
)

/*
 Scheduler is the sole control-plane authority of the crawl.

 Determinism and admission guarantees:
 - Scheduler is the ONLY component allowed to decide whether a URL
   may enter the crawl frontier.
 - All semantic admission checks (robots.txt, scope, depth, limits)
   MUST be completed before submitting a URL to the frontier.
 - No other component may enqueue, reject, or reorder URLs.
 - The frontier should only accept already-admitted URLs.
 - Pipeline stages may detect and classify failure, but must never decide retry, continuation, or abortion.

 The scheduler coordinates pipeline execution but does not delegate
 control-flow decisions to downstream stages.

 Metadata emission is observational only and MUST NOT influence
 scheduling, retries, or crawl termination.

 Scheduler Responsibilities:
 - Coordinate crawl lifecycle
 - Enforce global limits (pages, depth)
 - Manage graceful shutdown
 - Aggregate crawl statistics
 - Decide whether a robots outcome proceeds to the frontier.
 - The sole authority on:
	- retry
	- continue
	- abort
 TODO:
	- Introduce worker-scoped recorders when concurrency exists
*/

type Scheduler struct {
	ctx                    context.Context
	httpClient             *http.Client
	metadataSink           metadata.MetadataSink
	crawlFinalizer         metadata.CrawlFinalizer
	robot                  robots.Robot
	frontier               frontier.Frontier
	failureJournal         failurejournal.Journal
	htmlFetcher            fetcher.Fetcher
	domExtractor           extractor.Extractor
	htmlSanitizer          sanitizer.Sanitizer
	markdownConversionRule mdconvert.ConvertRule
	assetResolver          assets.Resolver
	markdownConstraint     normalize.Constraint
	storageSink            storage.Sink
	writeResults           []storage.WriteResult
	currentHost            string
	rateLimiter            ratelimiter.RateLimiter
	stageDumper            stagedump.Dumper
	debugLogger            debug.DebugLogger
	stageRunner            *gopipeline.StageRunner
}

func NewScheduler() Scheduler {
	recorder := metadata.NewRecorder("sample-single-sync-worker")
	cachedRobot := robots.NewCachedRobot(&recorder)
	frontier := frontier.NewCrawlFrontier()
	fetcher := fetcher.NewHtmlFetcher(&recorder)
	ext := extractor.NewDomExtractor(&recorder)
	sanitizer := sanitizer.NewHTMLSanitizer(&recorder)
	conversionRule := mdconvert.NewRule(&recorder)
	resolver := assets.NewLocalResolver(&recorder)
	markdownConstraint := normalize.NewMarkdownConstraint(&recorder)
	storageSink := storage.NewLocalSink(&recorder)
	rateLimiter := ratelimiter.NewConcurrentRateLimiter()
	return Scheduler{
		metadataSink:           &recorder,
		crawlFinalizer:         &recorder,
		robot:                  &cachedRobot,
		frontier:               &frontier,
		htmlFetcher:            &fetcher,
		domExtractor:           &ext,
		htmlSanitizer:          &sanitizer,
		markdownConversionRule: conversionRule,
		assetResolver:          &resolver,
		markdownConstraint:     &markdownConstraint,
		storageSink:            storageSink,
		rateLimiter:            rateLimiter,
	}
}

// NewSchedulerWithDeps creates a Scheduler with injected dependencies for testing.
// This constructor allows tests to provide mock implementations of metadata interfaces
// to verify behavior without relying on real infrastructure.
// The failureJournal parameter is optional - if not provided, an in-memory journal will be created.
func NewSchedulerWithDeps(
	ctx context.Context,
	crawlFinalizer metadata.CrawlFinalizer,
	metadataSink metadata.MetadataSink,
	rateLimiter ratelimiter.RateLimiter,
	frontier frontier.Frontier,
	fetcher fetcher.Fetcher,
	robot robots.Robot,
	domExtractor extractor.Extractor,
	sanitizer sanitizer.Sanitizer,
	rule mdconvert.ConvertRule,
	resolver assets.Resolver,
	constraint normalize.Constraint,
	storageSink storage.Sink,
	failureJournal failurejournal.Journal,
	stageDumper stagedump.Dumper,
	debugLogger debug.DebugLogger,
) Scheduler {
	return Scheduler{
		ctx:                    ctx,
		metadataSink:           metadataSink,
		crawlFinalizer:         crawlFinalizer,
		robot:                  robot,
		frontier:               frontier,
		failureJournal:         failureJournal,
		htmlFetcher:            fetcher,
		domExtractor:           domExtractor,
		htmlSanitizer:          sanitizer,
		markdownConversionRule: rule,
		assetResolver:          resolver,
		markdownConstraint:     constraint,
		storageSink:            storageSink,
		rateLimiter:            rateLimiter,
		stageDumper:            stageDumper,
		debugLogger:            debugLogger,
		stageRunner:            newStageRunner(),
	}
}

// newStageRunner creates a default StageRunner with an in-memory journal.
func newStageRunner() *gopipeline.StageRunner {
	runner, err := gopipeline.NewStageRunner(gopipeline.NewInMemoryJournal())
	if err != nil {
		panic(fmt.Sprintf("failed to create default StageRunner: %v", err))
	}
	return runner
}

// newPoolStageRunner creates a StageRunner configured for concurrent pool processing.
// Uses CollectAll error strategy to gather all errors, and PoolFailureIndividual
// failure mode to track each pool item separately for fine-grained retry control.
func newPoolStageRunner() *gopipeline.StageRunner {
	runner, err := gopipeline.NewStageRunner(
		gopipeline.NewInMemoryJournal(),
		gopipeline.WithErrorStrategy(gopipeline.CollectAll),
		gopipeline.WithPoolFailureMode(gopipeline.PoolFailureIndividual),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create pool StageRunner: %v", err))
	}
	return runner
}

// SubmitUrlForAdmission performs all semantic checks required for a URL
// to enter the crawl frontier.
//
// This function is the single admission choke point for the system.
// If this function returns nil, the URL is guaranteed to be admissible
// and safe to submit to the frontier.
//
// No other code path may call Frontier.Submit.
// - Only the scheduler imports frontier
// - Only the scheduler constructs CrawlAdmissionCandidate
// - Pipeline stages never see frontier types
func (s *Scheduler) SubmitUrlForAdmission(
	url url.URL,
	sourceContext frontier.SourceContext,
	depth int,
) failure.ClassifiedError {
	// Canonicalize the URL before any checks to ensure:
	// - Consistent robots.txt enforcement (e.g., /docs/ and /docs are the same)
	// - Proper deduplication (query params and fragments are normalized)
	// - Deterministic crawl behavior
	canonicalURL := urlutil.Canonicalize(url)

	// Fetch robots.txt using the canonicalized URL
	robotsDecision, robotsError := s.robot.Decide(canonicalURL)
	// Robots infrastructure failure → scheduler-level error
	if robotsError != nil {
		return robotsError
	}

	// Reset backoff after successful robots request
	if s.rateLimiter != nil {
		s.rateLimiter.ResetBackoff(canonicalURL.Host)
	}

	if robotsDecision.CrawlDelay > 0 && s.rateLimiter != nil {
		s.rateLimiter.SetResourceDelay(s.currentHost, robotsDecision.CrawlDelay)
	}

	// Robots explicitly disallowed -> normal, terminal outcome
	if !robotsDecision.Allowed {
		// Important:
		// - metadata already emitted by robots
		// - NO retry
		// - NO abort
		// - NO frontier submission
		s.metadataSink.RecordSkip(metadata.NewSkipEvent(
			canonicalURL.String(),
			metadata.SkipReasonRobotsDisallow,
			time.Now(),
		))
		return nil
	}

	// Only submit to frontier if robots allowed
	// Use the canonical URL from the robots decision to ensure consistency
	candidate := frontier.NewCrawlAdmissionCandidate(
		robotsDecision.Url,
		sourceContext,
		frontier.NewDiscoveryMetadata(
			depth,
			nil,
		),
	)

	// Submit Allowed URL for Admission by Frontier
	s.frontier.Submit(candidate)
	return nil
}

// InitializeCrawling performs all initialization steps up to just before the crawl loop.
// This includes:
// - Loading and validating configuration
// - Initializing HTTP client, rate limiter, robots, frontier
// - Configuring extractor, fetcher, asset resolver
// - Submitting seed URL to frontier
// - Applying initial rate limiting delay
//
// This method can be tested independently without waiting for the execution phase.
func (s *Scheduler) InitializeCrawling(configPath string) (init *CrawlInitialization, err error) {
	// Track initialization start time for stats recording
	initStartTime := time.Now()

	// Ensure stats are recorded only if initialization fails.
	// On success, ExecuteCrawlingWithState will handle final stats recording.
	defer func() {
		if err != nil && s.crawlFinalizer != nil {
			// Only record stats on failure - this captures init duration
			// when initialization fails before execution begins
			s.crawlFinalizer.RecordFinalCrawlStats(metadata.NewCrawlStats(
				initStartTime,
				time.Now(),
				0, // totalWebPages - no pages during init
				0, // totalProcessedPages - no pages during init
				0, // totalErrors - no errors during init
				0, // totalAssets - no assets during init
				0, // manualRetryQueueCount - no failures during init
			))
		}
	}()

	// 1. Prepare config File
	cfg, err := config.WithConfigFile(configPath)
	if err != nil {
		s.metadataSink.RecordError(metadata.NewErrorRecord(
			time.Now(),
			"config",
			"config.WithConfigFile",
			metadata.CauseContentInvalid,
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrField, fmt.Sprintf("field: %v", "theFieldError")),
			},
		))
		return nil, err
	}

	// Initialize file-based failure journal in output directory.
	// Only set if not already injected externally (e.g., via NewSchedulerWithDeps).
	if s.failureJournal == nil {
		journalPath := filepath.Join(cfg.OutputDir(), "failures.jsonl")
		s.failureJournal = failurejournal.NewFileJournal(journalPath)
	}

	// Note: We intentionally don't store the cancel function here.
	// The context should remain valid throughout the crawl operation.
	// Cancellation is handled by the HTTP client's timeout or explicit cancellation.
	ctx, _ := context.WithTimeout(context.Background(), cfg.Timeout())
	if s.ctx == nil {
		s.ctx = ctx
	}

	// Validate that at least one seed URL exists
	if len(cfg.SeedURLs()) == 0 {
		err = fmt.Errorf("no seed URLs configured")
		s.metadataSink.RecordError(metadata.NewErrorRecord(
			time.Now(),
			"config",
			"config validation",
			metadata.CauseContentInvalid,
			err.Error(),
			[]metadata.Attribute{},
		))
		return nil, err
	}

	// 1.1 Initialize HTTP Client
	s.httpClient = createHttpClient(
		cfg.MaxIdleConns(),
		cfg.MaxIdleConnsPerHost(),
		cfg.IdleConnTimeout(),
		cfg.Timeout(),
	)

	// 1.2 Initialize rate limiter
	s.rateLimiter.SetBaseDelay(cfg.BaseDelay())
	s.rateLimiter.SetJitter(cfg.Jitter())

	// 1.3 Initialize Robots and Frontier
	s.robot.Init(cfg.UserAgent(), s.httpClient)
	s.frontier.Init(cfg)

	// 1.4 Configure DOM Extractor with extraction parameters from config
	extractParam := extractor.ExtractParam{
		BodySpecificityBias:  cfg.BodySpecificityBias(),
		LinkDensityThreshold: cfg.LinkDensityThreshold(),
		ScoreMultiplier: extractor.ContentScoreMultiplier{
			NonWhitespaceDivisor: cfg.ScoreMultiplierNonWhitespaceDivisor(),
			Paragraphs:           cfg.ScoreMultiplierParagraphs(),
			Headings:             cfg.ScoreMultiplierHeadings(),
			CodeBlocks:           cfg.ScoreMultiplierCodeBlocks(),
			ListItems:            cfg.ScoreMultiplierListItems(),
		},
		Threshold: extractor.MeaningfulThreshold{
			MinNonWhitespace:    cfg.ThresholdMinNonWhitespace(),
			MinHeadings:         cfg.ThresholdMinHeadings(),
			MinParagraphsOrCode: cfg.ThresholdMinParagraphsOrCode(),
			MaxLinkDensity:      cfg.ThresholdMaxLinkDensity(),
		},
		SelectorBlacklist: cfg.SelectorBlacklist(),
	}
	s.domExtractor.SetExtractParam(extractParam)

	// 1.5 Initialize Fetcher
	s.htmlFetcher.Init(s.httpClient, cfg.UserAgent())

	// 1.6 Initialize Asset Resolver
	s.assetResolver.Init(s.httpClient, cfg.UserAgent())

	// 2. Fetch robots.txt & decide the crawling policy for this hostname based on that
	s.currentHost = cfg.SeedURLs()[0].Host
	seedScheme := cfg.SeedURLs()[0].Scheme
	err = s.SubmitUrlForAdmission(cfg.SeedURLs()[0], frontier.SourceSeed, 0)
	if err != nil {
		// Check if this is a robots error that requires backoff
		if robotsErr, ok := err.(*robots.RobotsError); ok {
			s.recordRobotsErrorAndBackoff(robotsErr, cfg.SeedURLs()[0])
		}
		return nil, err
	}

	// Apply rate limiting delay after successful robots check using Wait
	if err := s.rateLimiter.Wait(s.ctx, s.currentHost); err != nil {
		return nil, err
	}

	// Return the initialization state
	return &CrawlInitialization{
		config:              cfg,
		httpClient:          s.httpClient,
		currentHost:         s.currentHost,
		seedScheme:          seedScheme,
		initialDelayApplied: true,
	}, nil
}

// ExecuteCrawlingWithState runs the crawl execution loop using the provided initialization state.
// This method handles the actual page fetching, extraction, and processing.
// It manages its own deferred stat recording to ensure accurate execution timing.
func (s *Scheduler) ExecuteCrawlingWithState(init *CrawlInitialization) (CrawlingExecution, error) {
	// Track execution start time for duration calculation
	execStartTime := time.Now()

	// Statistics tracking
	var totalErrors int
	var totalAssets int

	// Ensure the failure journal is flushed to disk on crawl completion,
	// regardless of whether execution succeeds or fails.
	if s.failureJournal != nil {
		defer func() {
			if flushErr := s.failureJournal.Flush(); flushErr != nil {
				log.Printf("failed to flush failure journal: %v", flushErr)
			}
		}()
	}

	// Ensure final stats are recorded even if errors occur
	// This defer captures the execution phase duration only
	defer func() {
		s.crawlFinalizer.RecordFinalCrawlStats(metadata.NewCrawlStats(
			execStartTime,
			time.Now(),
			s.frontier.VisitedCount(),
			len(s.writeResults),
			totalErrors,
			totalAssets,
			s.failureJournal.Count(),
		))
	}()

	cfg := init.config
	seedScheme := init.seedScheme

	// Build the crawl task closure that processes a single URL through the full pipeline.
	// This closure captures the scheduler's dependencies and initialization config.
	crawlTask := s.BuildCrawlTask(cfg, seedScheme)

	// Create a pool-stage runner configured for concurrent processing.
	// Uses CollectAll error strategy to gather all errors, and PoolFailureIndividual
	// failure mode to track each pool item separately for fine-grained retry control.
	poolRunner := newPoolStageRunner()

	// Determine worker count from config. The pool handles gracefully when
	// Workers > len(tokens) — it simply spawns len(tokens) goroutines.
	workers := cfg.Concurrency()

	// Batched concurrent BFS loop:
	// 1. Drain all available tokens from the frontier
	// 2. Process the batch concurrently via gopipeline.Pool
	// 3. Workers discover new URLs during processing, which are submitted to the frontier
	// 4. Loop again to drain the frontier for the next batch
	// 5. Repeat until the frontier is empty or an abort error occurs
	for {
		tokens := s.collectTokensFromFrontier()
		if len(tokens) == 0 {
			break // frontier exhausted — crawl complete
		}

		// Log pipeline start for the batch, including worker count for debugging.
		s.debugLogger.LogStage(s.ctx, "pipeline", debug.StageEvent{
			Type: debug.EventTypeStart,
			URL:  fmt.Sprintf("batch-size:%d workers:%d", len(tokens), workers),
		})

		// Execute the pipeline using gopipeline.Pool for concurrent processing.
		// Pool spawns N workers that process tokens from a shared input channel.
		// Each worker calls crawlTask for a single token.
		result := gopipeline.Pool(s.ctx, poolRunner, "crawl", tokens,
			gopipeline.PoolConfig{Workers: workers},
			func(ctx context.Context, runner *gopipeline.StageRunner, idx int, token frontier.CrawlToken, stageName string) gopipeline.StageResult[CrawlResult] {
				return crawlTask(ctx, runner, idx, token, stageName)
			},
		)

		// Handle the result — with CollectAll, errors are collected in PartialResultsError
		outputs, err := result.Decompose()
		if err != nil {
			if partialErr, ok := err.(*gopipeline.PartialResultsError); ok {
				// Count all errors
				totalErrors += len(partialErr.Errors())

				// Check for abort errors — scheduler retains control authority
				for _, e := range partialErr.Errors() {
					if classifiedErr, ok := e.(failure.ClassifiedError); ok {
						if classifiedErr.Impact() == failure.ImpactLevelAbort {
							return CrawlingExecution{}, e
						}
					}
				}

				// Extract successful results from partial results
				for _, raw := range partialErr.PartialResults() {
					if output, ok := raw.(CrawlResult); ok {
						totalAssets += output.AssetCount
						s.writeResults = append(s.writeResults, output.WriteResult)
					}
				}
			} else {
				// Non-partial error — check for abort
				if classifiedErr, ok := err.(failure.ClassifiedError); ok {
					if classifiedErr.Impact() == failure.ImpactLevelAbort {
						return CrawlingExecution{}, err
					}
				}
				totalErrors++
			}
		} else {
			// All succeeded — collect all results
			for _, output := range outputs {
				totalAssets += output.AssetCount
				s.writeResults = append(s.writeResults, output.WriteResult)
			}
		}
	}

	// Stats are recorded by defer -> return successful execution result
	return NewCrawlingExecution(s.writeResults, s.frontier.VisitedCount(), totalAssets, totalErrors), nil
}

func createHttpClient(
	maxIdleConns int,
	maxIdleConnsPerHost int,
	idleConnTimeout time.Duration,
	baseTimeout time.Duration,
) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
	}

	client := &http.Client{
		Timeout:   baseTimeout,
		Transport: transport,
	}

	return client
}

// recordRobotsErrorAndBackoff records a robots error using metadataSink and
// triggers exponential backoff on the rate limiter if the error cause warrants it.
// This method handles ErrCauseHttpTooManyRequests (429) and ErrCauseHttpServerError (5xx)
// by recording the error and applying backoff to the current host.
func (s *Scheduler) recordRobotsErrorAndBackoff(robotsErr *robots.RobotsError, targetURL url.URL) {
	// Only record and backoff for specific HTTP error causes
	if robotsErr.Cause == robots.ErrCauseHttpTooManyRequests ||
		robotsErr.Cause == robots.ErrCauseHttpServerError {
		s.metadataSink.RecordError(metadata.NewErrorRecord(
			time.Now(),
			"scheduler",
			"SubmitUrlForAdmission",
			metadata.CauseNetworkFailure,
			robotsErr.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, targetURL.String()),
				metadata.NewAttr(metadata.AttrHost, targetURL.Host),
				metadata.NewAttr(metadata.AttrPath, targetURL.Path),
			},
		))
		if s.rateLimiter != nil {
			s.rateLimiter.Backoff(s.ctx, targetURL.Host)
		}
	}
}

func RetryOptions(cfg config.Config) []retrier.RetryOption {
	return []retrier.RetryOption{
		retrier.WithMaxAttempts(cfg.MaxAttempt()),
		retrier.WithInitialDuration(cfg.BackoffInitialDuration()),
		retrier.WithJitter(cfg.Jitter()),
		retrier.WithMultiplier(cfg.BackoffMultiplier()),
		retrier.WithMaxDuration(cfg.BackoffMaxDuration()),
	}
}

// ---------------------------------------------------------------------------
// Token Collection
// ---------------------------------------------------------------------------

// collectTokensFromFrontier dequeues all available tokens from the frontier
// into a slice. Returns an empty slice (not nil) if the frontier is empty.
// This is used to collect tokens for batch processing with gopipeline.Pool.
func (s *Scheduler) collectTokensFromFrontier() []frontier.CrawlToken {
	tokens := make([]frontier.CrawlToken, 0)
	for {
		token, ok := s.frontier.Dequeue()
		if !ok {
			break
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// ---------------------------------------------------------------------------
// Test Helper Methods
// These methods are exported to enable testing of SubmitUrlForAdmission()
// and other scheduler internals. They are not part of the public API.
// ---------------------------------------------------------------------------

// SetCurrentHost sets the current host.
// This is a test helper method to simulate the host context.
func (s *Scheduler) SetCurrentHost(host string) {
	s.currentHost = host
}

// FrontierVisitedCount returns the number of URLs in the frontier's visited set.
// This is a test helper method to verify frontier state.
func (s *Scheduler) FrontierVisitedCount() int {
	if s.frontier == nil {
		return 0
	}
	return s.frontier.VisitedCount()
}

// DequeueFromFrontier dequeues a token from the frontier.
// This is a test helper method to verify frontier contents.
func (s *Scheduler) DequeueFromFrontier() (frontier.CrawlToken, bool) {
	if s.frontier == nil {
		return frontier.CrawlToken{}, false
	}
	return s.frontier.Dequeue()
}

// FailureJournalPath returns the file path of the failure journal.
// This is a test helper method to verify journal initialization.
func (s *Scheduler) FailureJournalPath() string {
	if s.failureJournal == nil {
		return ""
	}
	return s.failureJournal.Path()
}

// getURLString safely extracts a string from a url.URL.
// This works around potential pointer receiver issues.
func getURLString(u url.URL) string {
	return u.String()
}

// NewSchedulerWithConfig creates a new Scheduler with config-based dependency injection.
// This constructor determines whether to use DryRunSink or LocalSink based on cfg.DryRun().
func NewSchedulerWithConfig(cfg config.Config) Scheduler {
	recorder := metadata.NewRecorder("sample-single-sync-worker")
	cachedRobot := robots.NewCachedRobot(&recorder)
	frontier := frontier.NewCrawlFrontier()
	fetcher := fetcher.NewHtmlFetcher(&recorder)
	ext := extractor.NewDomExtractor(&recorder)
	sanitizer := sanitizer.NewHTMLSanitizer(&recorder)
	conversionRule := mdconvert.NewRule(&recorder)
	markdownConstraint := normalize.NewMarkdownConstraint(&recorder)

	var resolver assets.Resolver
	var storageSink storage.Sink
	if cfg.DryRun() {
		resolver = assets.NewDryRunResolver(&recorder)
		storageSink = storage.NewDryRunSink(&recorder)
	} else {
		r := assets.NewLocalResolver(&recorder)
		resolver = &r
		storageSink = storage.NewLocalSink(&recorder)
	}

	// Create rate limiter with config values
	rateLimiter := ratelimiter.NewConcurrentRateLimiter(
		ratelimiter.WithJitter(cfg.Jitter()),
		ratelimiter.WithInitialDuration(cfg.BackoffInitialDuration()),
		ratelimiter.WithMultiplier(cfg.BackoffMultiplier()),
		ratelimiter.WithMaxDuration(cfg.BackoffMaxDuration()),
	)

	// Initialize stage dumper based on config
	var stageDumper stagedump.Dumper = stagedump.NewNoOpDumper()
	if cfg.DumpStageOutput() != "" {
		stageDumper = stagedump.NewFileDumper(cfg.DumpStageOutput(), cfg.DryRun())
	}

	// Initialize debug logger based on config
	var debugLogger debug.DebugLogger = debug.NewNoOpLogger()
	debugConfig, err := debug.NewDebugConfig(cfg.Debug(), cfg.DebugFile(), cfg.DebugFormat())
	if err != nil {
		log.Printf("failed to create debug config: %v, using NoOpLogger", err)
	} else {
		debugLogger, err = debug.NewSlogLogger(debugConfig)
		if err != nil {
			log.Printf("failed to create debug logger: %v, using NoOpLogger", err)
			debugLogger = debug.NewNoOpLogger()
		}
	}

	// Propagate debug logger to all components
	fetcher.SetDebugLogger(debugLogger)
	ext.SetDebugLogger(debugLogger)
	sanitizer.SetDebugLogger(debugLogger)
	cachedRobot.SetDebugLogger(debugLogger)
	frontier.SetDebugLogger(debugLogger)
	conversionRule.SetDebugLogger(debugLogger)
	markdownConstraint.SetDebugLogger(debugLogger)

	// Set debug logger for resolver and storage sink
	// Note: These may be pointer or interface types, handle accordingly
	if r, ok := resolver.(*assets.LocalResolver); ok {
		r.SetDebugLogger(debugLogger)
	}
	if s, ok := storageSink.(*storage.LocalSink); ok {
		s.SetDebugLogger(debugLogger)
	}
	if s, ok := storageSink.(*storage.DryRunSink); ok {
		s.SetDebugLogger(debugLogger)
	}

	// Set base delay on rate limiter
	rateLimiter.SetBaseDelay(cfg.BaseDelay())

	return Scheduler{
		metadataSink:           &recorder,
		crawlFinalizer:         &recorder,
		robot:                  &cachedRobot,
		frontier:               &frontier,
		htmlFetcher:            &fetcher,
		domExtractor:           &ext,
		htmlSanitizer:          &sanitizer,
		markdownConversionRule: conversionRule,
		assetResolver:          resolver,
		markdownConstraint:     &markdownConstraint,
		storageSink:            storageSink,
		rateLimiter:            rateLimiter,
		stageDumper:            stageDumper,
		debugLogger:            debugLogger,
	}
}

// InitializeWithConfig initializes the scheduler with a pre-built Config object.
// This is used by CLI when config is built from CLI flags rather than a config file.
func (s *Scheduler) InitializeWithConfig(cfg config.Config) (init *CrawlInitialization, err error) {
	initStartTime := time.Now()

	defer func() {
		if err != nil && s.crawlFinalizer != nil {
			s.crawlFinalizer.RecordFinalCrawlStats(metadata.NewCrawlStats(
				initStartTime,
				time.Now(),
				0, // totalWebPages
				0, // totalProcessedPages
				0, // totalErrors
				0, // totalAssets
				0, // manualRetryQueueCount
			))
		}
	}()

	// Validate that at least one seed URL exists
	if len(cfg.SeedURLs()) == 0 {
		err = fmt.Errorf("no seed URLs configured")
		s.metadataSink.RecordError(metadata.NewErrorRecord(
			time.Now(),
			"config",
			"config validation",
			metadata.CauseContentInvalid,
			err.Error(),
			[]metadata.Attribute{},
		))
		return nil, err
	}

	// Initialize file-based failure journal in output directory
	if s.failureJournal == nil {
		journalPath := filepath.Join(cfg.OutputDir(), "failures.jsonl")
		s.failureJournal = failurejournal.NewFileJournal(journalPath)
	}

	// Note: We intentionally don't store the cancel function here.
	// The context should remain valid throughout the crawl operation.
	// Cancellation is handled by the HTTP client's timeout or explicit cancellation.
	ctx, _ := context.WithTimeout(context.Background(), cfg.Timeout())
	if s.ctx == nil {
		s.ctx = ctx
	}

	// Initialize HTTP Client
	s.httpClient = createHttpClient(
		cfg.MaxIdleConns(),
		cfg.MaxIdleConnsPerHost(),
		cfg.IdleConnTimeout(),
		cfg.Timeout(),
	)

	// Initialize rate limiter
	s.rateLimiter.SetBaseDelay(cfg.BaseDelay())
	s.rateLimiter.SetJitter(cfg.Jitter())

	// Initialize Robots and Frontier
	s.robot.Init(cfg.UserAgent(), s.httpClient)
	s.frontier.Init(cfg)

	// Configure DOM Extractor
	extractParam := extractor.ExtractParam{
		BodySpecificityBias:  cfg.BodySpecificityBias(),
		LinkDensityThreshold: cfg.LinkDensityThreshold(),
		ScoreMultiplier: extractor.ContentScoreMultiplier{
			NonWhitespaceDivisor: cfg.ScoreMultiplierNonWhitespaceDivisor(),
			Paragraphs:           cfg.ScoreMultiplierParagraphs(),
			Headings:             cfg.ScoreMultiplierHeadings(),
			CodeBlocks:           cfg.ScoreMultiplierCodeBlocks(),
			ListItems:            cfg.ScoreMultiplierListItems(),
		},
		Threshold: extractor.MeaningfulThreshold{
			MinNonWhitespace:    cfg.ThresholdMinNonWhitespace(),
			MinHeadings:         cfg.ThresholdMinHeadings(),
			MinParagraphsOrCode: cfg.ThresholdMinParagraphsOrCode(),
			MaxLinkDensity:      cfg.ThresholdMaxLinkDensity(),
		},
		SelectorBlacklist: cfg.SelectorBlacklist(),
	}
	s.domExtractor.SetExtractParam(extractParam)

	// Initialize Fetcher
	s.htmlFetcher.Init(s.httpClient, cfg.UserAgent())

	// Initialize Asset Resolver
	s.assetResolver.Init(s.httpClient, cfg.UserAgent())

	// Submit seed URL to frontier
	s.currentHost = cfg.SeedURLs()[0].Host
	seedScheme := cfg.SeedURLs()[0].Scheme
	err = s.SubmitUrlForAdmission(cfg.SeedURLs()[0], frontier.SourceSeed, 0)
	if err != nil {
		if robotsErr, ok := err.(*robots.RobotsError); ok {
			s.recordRobotsErrorAndBackoff(robotsErr, cfg.SeedURLs()[0])
		}
		return nil, err
	}

	// Apply rate limiting delay after successful robots check using Wait
	if err := s.rateLimiter.Wait(s.ctx, s.currentHost); err != nil {
		return nil, err
	}

	return &CrawlInitialization{
		config:              cfg,
		httpClient:          s.httpClient,
		currentHost:         s.currentHost,
		seedScheme:          seedScheme,
		initialDelayApplied: true,
	}, nil
}

// GetMetadataRecorder returns the metadata sink for reading recorded events.
// This is useful for printing events after a dry-run crawl.
func (s *Scheduler) GetMetadataRecorder() metadata.MetadataSink {
	return s.metadataSink
}
