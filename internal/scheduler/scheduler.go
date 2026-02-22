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
	"github.com/rohmanhakim/docs-crawler/internal/build"
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
	"github.com/rohmanhakim/docs-crawler/pkg/limiter"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
	"github.com/rohmanhakim/docs-crawler/pkg/urlutil"
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
	rateLimiter            limiter.RateLimiter
	sleeper                timeutil.Sleeper
	stageDumper            stagedump.Dumper
	debugLogger            debug.DebugLogger
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
	rateLimiter := limiter.NewConcurrentRateLimiter()
	sleeper := timeutil.NewRealSleeper()
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
		sleeper:                &sleeper,
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
	rateLimiter limiter.RateLimiter,
	frontier frontier.Frontier,
	fetcher fetcher.Fetcher,
	robot robots.Robot,
	domExtractor extractor.Extractor,
	sanitizer sanitizer.Sanitizer,
	rule mdconvert.ConvertRule,
	resolver assets.Resolver,
	constraint normalize.Constraint,
	storageSink storage.Sink,
	sleeper timeutil.Sleeper,
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
		sleeper:                sleeper,
		stageDumper:            stageDumper,
		debugLogger:            debugLogger,
	}
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
		s.rateLimiter.SetCrawlDelay(s.currentHost, robotsDecision.CrawlDelay)
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
	s.rateLimiter.SetRandomSeed(cfg.RandomSeed())

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

	// Apply rate limiting delay after successful robots check
	delay := s.rateLimiter.ResolveDelay(s.currentHost)
	s.sleeper.Sleep(delay)

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

	// If frontier still has URL to be crawl...
	for {
		nextCrawlToken, ok := s.frontier.Dequeue()
		if !ok {
			break
		}

		urlStr := getURLString(nextCrawlToken.URL())

		// Log pipeline start for this URL
		s.debugLogger.LogStage(s.ctx, "pipeline", debug.StageEvent{
			Type: debug.EventTypeStart,
			URL:  urlStr,
		})

		// 3. Fetch Page URL
		fetchStartTime := time.Now()
		s.debugLogger.LogStage(s.ctx, "fetcher", debug.StageEvent{
			Type: debug.EventTypeStart,
			URL:  urlStr,
		})

		fetchResult, err := s.htmlFetcher.Fetch(s.ctx, nextCrawlToken.Depth(), nextCrawlToken.URL(), RetryParam(cfg))
		if err != nil {
			if err.Impact() == failure.ImpactLevelAbort {
				return CrawlingExecution{}, err
			}
			// Log fetcher error
			s.debugLogger.LogStage(s.ctx, "fetcher", debug.StageEvent{
				Type:     debug.EventTypeError,
				URL:      urlStr,
				Duration: time.Since(fetchStartTime),
			})
			// Track for manual retry if eligible
			if err.RetryPolicy() == failure.RetryPolicyManual {
				s.failureJournal.Record(failurejournal.FailureRecord{
					URL:        getURLString(nextCrawlToken.URL()),
					Stage:      failurejournal.StageFetch,
					Error:      err.Error(),
					RetryCount: 0,
					Timestamp:  time.Now(),
				})
			}
			// recoverable → log already done → count error
			totalErrors++
			continue
		}

		// Log fetcher completion
		s.debugLogger.LogStage(s.ctx, "fetcher", debug.StageEvent{
			Type:     debug.EventTypeComplete,
			URL:      getURLString(fetchResult.URL()),
			Duration: time.Since(fetchStartTime),
			Fields: debug.FieldMap{
				"status_code": fetchResult.Code(),
			},
		})

		// Dump fetched HTML
		s.stageDumper.DumpFetcherOutput(urlStr, fetchResult.Body())

		// 4. Extract HTML DOM
		extractionResult, err := s.domExtractor.Extract(fetchResult.URL(), fetchResult.Body())
		if err != nil {
			if err.Impact() == failure.ImpactLevelAbort {
				return CrawlingExecution{}, err
			}
			// Note: Extraction errors are deterministic (content invalid).
			// Do NOT record to failure journal - retrying the same content yields the same error.
			totalErrors++
			continue
		}

		// Dump extraction result
		s.stageDumper.DumpExtractorOutput(urlStr, extractionResult.ContentNode)

		// 5. Sanitize extracted HTML
		sanitizedHtml, err := s.htmlSanitizer.Sanitize(extractionResult.ContentNode)
		if err != nil {
			if err.Impact() == failure.ImpactLevelAbort {
				return CrawlingExecution{}, err
			}
			// Note: Sanitization errors are deterministic (invariant violations).
			// Do NOT record to failure journal - retrying the same content yields the same error.
			totalErrors++
			continue
		}

		// Dump sanitization result
		s.stageDumper.DumpSanitizerOutput(urlStr, sanitizedHtml.GetContentNode())

		// 5.2 Resolve relative URLs to absolute URLs and filter by host
		discoveredURLs := sanitizedHtml.GetDiscoveredURLs()

		// 5.3 Resolve all URLs to absolute form using the seed scheme and current host
		resolvedURLs := make([]url.URL, 0, len(discoveredURLs))
		for _, u := range discoveredURLs {
			resolved := urlutil.Resolve(u, seedScheme, s.currentHost)
			resolvedURLs = append(resolvedURLs, resolved)
		}

		// 5.4 Filter to only keep URLs from the current host
		filteredURLs := urlutil.FilterByHost(s.currentHost, resolvedURLs)

		// 5.5 submit all discovered links through robots checking to frontier
		for _, discoveredurl := range filteredURLs {
			submissionErr := s.SubmitUrlForAdmission(discoveredurl, frontier.SourceCrawl, nextCrawlToken.Depth()+1)
			if submissionErr != nil {
				// Check if this is a robots error that requires backoff
				if robotsErr, ok := submissionErr.(*robots.RobotsError); ok {
					s.recordRobotsErrorAndBackoff(robotsErr, discoveredurl)
				}
				// Submission errors are scheduler-level errors, count them
				totalErrors++
				// Continue processing other URLs, don't abort the crawl
			}
		}

		// 6. HTML → Markdown Conversion
		markdownDoc, err := s.markdownConversionRule.Convert(sanitizedHtml, getURLString(fetchResult.URL()))
		if err != nil {
			if err.Impact() == failure.ImpactLevelAbort {
				return CrawlingExecution{}, err
			}
			// Note: Conversion errors are deterministic (conversion failures).
			// Do NOT record to failure journal - retrying the same content yields the same error.
			totalErrors++
			continue
		}

		// Dump markdown conversion result
		s.stageDumper.DumpMDConvertOutput(urlStr, markdownDoc.GetMarkdownContent())

		// 7. Assets Resolution
		resolveParam := assets.NewResolveParam(cfg.OutputDir(), cfg.MaxAssetSize(), cfg.HashAlgo())
		assetfulMarkdown, err := s.assetResolver.Resolve(
			s.ctx,
			fetchResult.URL(),
			markdownDoc,
			resolveParam,
			RetryParam(cfg),
		)
		if err != nil {
			if err.Impact() == failure.ImpactLevelAbort {
				return CrawlingExecution{}, err
			}
			// Track for manual retry if eligible
			if err.RetryPolicy() == failure.RetryPolicyManual {
				s.failureJournal.Record(failurejournal.FailureRecord{
					URL:        getURLString(nextCrawlToken.URL()),
					Stage:      failurejournal.StageAsset,
					Error:      err.Error(),
					RetryCount: 0,
					Timestamp:  time.Now(),
				})
			}
			totalErrors++
			// Continue to process the markdown even if asset resolution had errors
		}
		// Count assets processed - use the actual count of successfully resolved local assets
		totalAssets += len(assetfulMarkdown.LocalAssets())

		// Dump asset resolving result
		s.stageDumper.DumpAssetResolverOutput(urlStr, assetfulMarkdown.Content())

		// 8. Markdown Normalization
		normalizeParam := normalize.NewNormalizeParam(
			build.FullVersion(),
			fetchResult.FetchedAt(),
			cfg.HashAlgo(),
			nextCrawlToken.Depth(),
			cfg.AllowedPathPrefix(),
		)
		normalizedMarkdown, err := s.markdownConstraint.Normalize(
			fetchResult.URL(),
			assetfulMarkdown,
			normalizeParam,
		)
		if err != nil {
			if err.Impact() == failure.ImpactLevelAbort {
				return CrawlingExecution{}, err
			}
			// Note: Normalization errors are deterministic (invariant violations).
			// Do NOT record to failure journal - retrying the same content yields the same error.
			totalErrors++
			continue
		}

		// 9. Write Artifact
		writeResult, err := s.storageSink.Write(
			cfg.OutputDir(),
			normalizedMarkdown,
			cfg.HashAlgo(),
		)
		if err != nil {
			if err.Impact() == failure.ImpactLevelAbort {
				return CrawlingExecution{}, err
			}
			// Track for manual retry if eligible
			if err.RetryPolicy() == failure.RetryPolicyManual {
				s.failureJournal.Record(failurejournal.FailureRecord{
					URL:        getURLString(nextCrawlToken.URL()),
					Stage:      failurejournal.StageStorage,
					Error:      err.Error(),
					RetryCount: 0,
					Timestamp:  time.Now(),
				})
			}
			// recoverable → log already done → count error
			totalErrors++
			continue
		}
		s.writeResults = append(s.writeResults, writeResult)

		// Apply rate limiting delay at the end of the crawl loop
		delay := s.rateLimiter.ResolveDelay(s.currentHost)
		s.sleeper.Sleep(delay)
	}

	// Stats are recorded by defer - return successful execution result
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
			s.rateLimiter.Backoff(targetURL.Host)
		}
	}
}

func RetryParam(cfg config.Config) retry.RetryParam {
	return retry.NewRetryParam(
		cfg.BaseDelay(),
		cfg.Jitter(),
		cfg.RandomSeed(),
		cfg.MaxAttempt(),
		timeutil.NewBackoffParam(
			cfg.BackoffInitialDuration(),
			cfg.BackoffMultiplier(),
			cfg.BackoffMaxDuration(),
		),
	)
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
	// s.rateLimiter.RegisterHost(host)
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

	rateLimiter := limiter.NewConcurrentRateLimiter()
	sleeper := timeutil.NewRealSleeper()

	// Initialize stage dumper based on config
	var stageDumper stagedump.Dumper = stagedump.NewNoOpDumper()
	if cfg.DumpStageOutput() != "" {
		stageDumper = stagedump.NewFileDumper(cfg.DumpStageOutput(), cfg.DryRun())
	}

	// Initialize debug logger based on config
	debugConfig, err := debug.NewDebugConfig(cfg.Debug(), cfg.DebugFile(), cfg.DebugFormat())
	if err != nil {
		log.Printf("failed to create debug config: %v, using NoOpLogger", err)
	}
	debugLogger, err := debug.NewSlogLogger(debugConfig)
	if err != nil {
		log.Printf("failed to create debug logger: %v, using NoOpLogger", err)
		debugLogger = debug.NewNoOpLogger()
	}

	// Propagate debug logger to all components
	fetcher.SetDebugLogger(debugLogger)
	ext.SetDebugLogger(debugLogger)
	sanitizer.SetDebugLogger(debugLogger)
	cachedRobot.SetDebugLogger(debugLogger)
	frontier.SetDebugLogger(debugLogger)
	rateLimiter.SetDebugLogger(debugLogger)
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
		sleeper:                &sleeper,
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
	s.rateLimiter.SetRandomSeed(cfg.RandomSeed())

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

	// Apply rate limiting delay after successful robots check
	delay := s.rateLimiter.ResolveDelay(s.currentHost)
	s.sleeper.Sleep(delay)

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
