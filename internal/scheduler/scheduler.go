package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
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
	metadataSink           metadata.MetadataSink
	crawlFinalizer         metadata.CrawlFinalizer
	robot                  robots.Robot
	frontier               *frontier.Frontier
	htmlFetcher            fetcher.Fetcher
	domExtractor           extractor.Extractor
	htmlSanitizer          sanitizer.Sanitizer
	markdownConversionRule mdconvert.ConvertRule
	assetResolver          assets.Resolver
	markdownConstraint     normalize.MarkdownConstraint
	storageSink            storage.Sink
	writeResults           []storage.WriteResult
	currentHost            string
	rateLimiter            limiter.RateLimiter
	sleeper                timeutil.Sleeper
}

func NewScheduler() Scheduler {
	recorder := metadata.NewRecorder("sample-single-sync-worker")
	cachedRobot := robots.NewCachedRobot(&recorder)
	frontier := frontier.NewFrontier()
	fetcher := fetcher.NewHtmlFetcher(&recorder)
	ext := extractor.NewDomExtractor(&recorder)
	sanitizer := sanitizer.NewHTMLSanitizer(&recorder)
	conversionRule := mdconvert.NewRule(&recorder)
	resolver := assets.NewLocalResolver(&recorder, &http.Client{}, "docs-crawler/1.0")
	markdownConstraint := normalize.NewMarkdownConstraint(&recorder)
	storageSink := storage.NewSink(&recorder)
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
		markdownConstraint:     markdownConstraint,
		storageSink:            storageSink,
		rateLimiter:            rateLimiter,
		sleeper:                &sleeper,
	}
}

// NewSchedulerWithDeps creates a Scheduler with injected dependencies for testing.
// This constructor allows tests to provide mock implementations of metadata interfaces
// to verify behavior without relying on real infrastructure.
func NewSchedulerWithDeps(
	ctx context.Context,
	crawlFinalizer metadata.CrawlFinalizer,
	metadataSink metadata.MetadataSink,
	rateLimiter limiter.RateLimiter,
	fetcher fetcher.Fetcher,
	robot robots.Robot,
	domExtractor extractor.Extractor,
	sanitizer sanitizer.Sanitizer,
	rule mdconvert.ConvertRule,
	resolver assets.Resolver,
	sleeper timeutil.Sleeper,
) Scheduler {
	markdownConstraint := normalize.NewMarkdownConstraint(metadataSink)
	storageSink := storage.NewSink(metadataSink)
	frontier := frontier.NewFrontier()
	return Scheduler{
		ctx:                    ctx,
		metadataSink:           metadataSink,
		crawlFinalizer:         crawlFinalizer,
		robot:                  robot,
		frontier:               &frontier,
		htmlFetcher:            fetcher,
		domExtractor:           domExtractor,
		htmlSanitizer:          sanitizer,
		markdownConversionRule: rule,
		assetResolver:          resolver,
		markdownConstraint:     markdownConstraint,
		storageSink:            storageSink,
		rateLimiter:            rateLimiter,
		sleeper:                sleeper,
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
	// Fetch robots.txt
	robotsDecision, robotsError := s.robot.Decide(url)
	// Robots infrastructure failure → scheduler-level error
	if robotsError != nil {
		return robotsError
	}

	// Reset backoff after successful robots request
	if s.rateLimiter != nil {
		s.rateLimiter.ResetBackoff(url.Host)
	}

	if robotsDecision.CrawlDelay > 0 && s.rateLimiter != nil {
		s.rateLimiter.SetCrawlDelay(s.currentHost, robotsDecision.CrawlDelay)
	}

	// Robots explicitly disallowed → normal, terminal outcome
	if !robotsDecision.Allowed {
		// Important:
		// - metadata already emitted by robots
		// - NO retry
		// - NO abort
		// - NO frontier submission
		// TODO: record to metadataSink that robots explcitly disallowed the URL
		return nil
	}

	// Only submit to frontier if robots allowed
	candidate := frontier.NewCrawlAdmissionCandidate(
		robotsDecision.Url,
		sourceContext,
		frontier.DiscoveryMetadata{
			Depth: depth,
		},
	)

	// Submit Allowed URL for Admission by Frontier
	s.frontier.Submit(candidate)
	return nil
}

// Current implementation uses a single recorder and single execution path.
// This does not imply a global ordering guarantee.
// TODO: In the future consider implementing global ordering guarantee
func (s *Scheduler) ExecuteCrawling(configPath string) (CrawlingExecution, error) {
	// Track crawl start time for duration calculation
	crawlStartTime := time.Now()

	// Statistics tracking
	var totalErrors int
	var totalAssets int

	// Ensure final stats are recorded even if errors occur
	defer func() {
		crawlDuration := time.Since(crawlStartTime)
		totalPages := s.frontier.VisitedCount()
		s.crawlFinalizer.RecordFinalCrawlStats(
			totalPages,
			totalErrors,
			totalAssets,
			crawlDuration,
		)
	}()

	// 1. Prepare config File
	cfg, err := config.WithConfigFile(configPath)
	if err != nil {
		s.metadataSink.RecordError(
			time.Now(),
			"config",
			"config.WithConfigFile",
			metadata.CauseContentInvalid,
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrField, fmt.Sprintf("field: %v", "theFieldError")),
			},
		)
		return CrawlingExecution{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout())
	defer cancel()
	if s.ctx == nil {
		s.ctx = ctx
	}

	// Validate that at least one seed URL exists
	if len(cfg.SeedURLs()) == 0 {
		err := fmt.Errorf("no seed URLs configured")
		s.metadataSink.RecordError(
			time.Now(),
			"config",
			"config validation",
			metadata.CauseContentInvalid,
			err.Error(),
			[]metadata.Attribute{},
		)
		return CrawlingExecution{}, err
	}

	// 1.1 Initialize rate limiter
	s.rateLimiter.SetBaseDelay(cfg.BaseDelay())
	s.rateLimiter.SetJitter(cfg.Jitter())
	s.rateLimiter.SetRandomSeed(cfg.RandomSeed())

	// 1.2 Initialize Robots and Frontier
	s.robot.Init(cfg.UserAgent())
	s.frontier.Init(cfg)

	// 1.3 Configure DOM Extractor with extraction parameters from config
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
	}
	s.domExtractor.SetExtractParam(extractParam)

	// 2. Fetch robots.txt & decide the crawling policy for this hostname based on that
	s.currentHost = cfg.SeedURLs()[0].Host
	seedScheme := cfg.SeedURLs()[0].Scheme
	err = s.SubmitUrlForAdmission(cfg.SeedURLs()[0], frontier.SourceSeed, 0)
	if err != nil {
		// Check if this is a robots error that requires backoff
		if robotsErr, ok := err.(*robots.RobotsError); ok {
			s.recordRobotsErrorAndBackoff(robotsErr, cfg.SeedURLs()[0])
		}
		return CrawlingExecution{}, err
	}

	// Apply rate limiting delay after successful robots check
	delay := s.rateLimiter.ResolveDelay(s.currentHost)
	s.sleeper.Sleep(delay)

	// If frontier still has URL to be crawl...
	for {
		nextCrawlToken, ok := s.frontier.Dequeue()
		if !ok {
			break
		}

		// 3. Fetch Page URL
		fetchParam := fetcher.NewFetchParam(
			nextCrawlToken.URL(),
			cfg.UserAgent(),
		)
		fetchResult, err := s.htmlFetcher.Fetch(s.ctx, nextCrawlToken.Depth(), fetchParam, RetryParam(cfg))
		if err != nil {
			if err.Severity() == failure.SeverityFatal {
				return CrawlingExecution{}, err
			}
			// recoverable → log already done → count error
			totalErrors++
			continue
		}

		// 4. Extract HTML DOM
		extractionResult, err := s.domExtractor.Extract(fetchResult.URL(), fetchResult.Body())
		if err != nil {
			if err.Severity() == failure.SeverityFatal {
				return CrawlingExecution{}, err
			}
			totalErrors++
			continue
		}

		// 5. Sanitize extracted HTML
		sanitizedHtml, err := s.htmlSanitizer.Sanitize(extractionResult.ContentNode)
		if err != nil {
			if err.Severity() == failure.SeverityFatal {
				return CrawlingExecution{}, err
			}
			totalErrors++
			continue
		}

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
		markdownDoc, err := s.markdownConversionRule.Convert(sanitizedHtml)
		if err != nil {
			if err.Severity() == failure.SeverityFatal {
				return CrawlingExecution{}, err
			}
			totalErrors++
			continue
		}

		// 7. Assets Resolution
		resolveParam := assets.NewResolveParam(cfg.OutputDir(), cfg.MaxAssetSize())
		assetfulMarkdown, err := s.assetResolver.Resolve(
			s.ctx,
			fetchResult.URL(),
			markdownDoc,
			resolveParam,
			RetryParam(cfg),
		)
		if err != nil {
			if err.Severity() == failure.SeverityFatal {
				return CrawlingExecution{}, err
			}
			totalErrors++
			// Continue to process the markdown even if asset resolution had errors
		}
		// Count assets processed - use the actual count of successfully resolved local assets
		totalAssets += len(assetfulMarkdown.LocalAssets())

		// 8. Markdown Normalization
		normalizedMarkdown, err := s.markdownConstraint.Normalize(assetfulMarkdown)
		if err != nil {
			if err.Severity() == failure.SeverityFatal {
				return CrawlingExecution{}, err
			}
			totalErrors++
			continue
		}

		// 9. Write Artifact
		writeResult, err := s.storageSink.Write(normalizedMarkdown)
		if err != nil {
			if err.Severity() == failure.SeverityFatal {
				return CrawlingExecution{}, err
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
	return CrawlingExecution{
		WriteResults: s.writeResults,
	}, nil
}

// recordRobotsErrorAndBackoff records a robots error using metadataSink and
// triggers exponential backoff on the rate limiter if the error cause warrants it.
// This method handles ErrCauseHttpTooManyRequests (429) and ErrCauseHttpServerError (5xx)
// by recording the error and applying backoff to the current host.
func (s *Scheduler) recordRobotsErrorAndBackoff(robotsErr *robots.RobotsError, targetURL url.URL) {
	// Only record and backoff for specific HTTP error causes
	if robotsErr.Cause == robots.ErrCauseHttpTooManyRequests ||
		robotsErr.Cause == robots.ErrCauseHttpServerError {
		s.metadataSink.RecordError(
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
		)
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

// InitWith initializes the dependencies with the given data.
// This is a test helper method.
func (s *Scheduler) InitWith(userAgent string, baseDelay time.Duration, jitter time.Duration, randomSeed int64) {
	s.robot.Init(userAgent)
	s.rateLimiter.SetBaseDelay(baseDelay)
	s.rateLimiter.SetJitter(jitter)
	s.rateLimiter.SetRandomSeed(randomSeed)
}

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

// SetConvertRule sets the markdown conversion rule for testing.
// This is a test helper method to inject mock conversion rules.
func (s *Scheduler) SetConvertRule(rule mdconvert.ConvertRule) {
	s.markdownConversionRule = rule
}
