package scheduler

import (
	"fmt"
	"net/url"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal"
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
	- Apply robots crawl-delay inside scheduler timing logic
*/

type Scheduler struct {
	metadataSink           metadata.MetadataSink
	crawlFinalizer         metadata.CrawlFinalizer
	robot                  robots.Robot
	robotsCrawlDelay       *time.Duration
	frontier               *frontier.Frontier
	htmlFetcher            fetcher.HtmlFetcher
	domExtractor           extractor.DomExtractor
	htmlSanitizer          sanitizer.HtmlSanitizer
	markdownConversionRule mdconvert.Rule
	assetResolver          assets.Resolver
	markdownConstraint     normalize.MarkdownConstraint
	storageSink            storage.Sink
	writeResults           []storage.WriteResult
	currentHost            string
	hostTimings            map[string]hostTiming
}

func NewScheduler() Scheduler {
	recorder := metadata.NewRecorder("sample-single-sync-worker")
	robot := robots.NewRobot(&recorder)
	frontier := frontier.NewFrontier()
	fetcher := fetcher.NewHtmlFetcher(&recorder)
	extractor := extractor.NewDomExtractor(&recorder)
	sanitizer := sanitizer.NewHTMLSanitizer(&recorder)
	conversionRule := mdconvert.NewRule()
	resolver := assets.NewResolver(&recorder)
	markdownConstraint := normalize.NewMarkdownConstraint(&recorder)
	storageSink := storage.NewSink(&recorder)
	return Scheduler{
		metadataSink:           &recorder,
		crawlFinalizer:         &recorder,
		robot:                  robot,
		frontier:               &frontier,
		htmlFetcher:            fetcher,
		domExtractor:           extractor,
		htmlSanitizer:          sanitizer,
		markdownConversionRule: conversionRule,
		assetResolver:          resolver,
		markdownConstraint:     markdownConstraint,
		storageSink:            storageSink,
	}
}

// NewSchedulerWithDeps creates a Scheduler with injected dependencies for testing.
// This constructor allows tests to provide mock implementations of metadata interfaces
// to verify behavior without relying on real infrastructure.
func NewSchedulerWithDeps(
	crawlFinalizer metadata.CrawlFinalizer,
	metadataSink metadata.MetadataSink,
) Scheduler {
	robot := robots.NewRobot(metadataSink)
	frontier := frontier.NewFrontier()
	fetcher := fetcher.NewHtmlFetcher(metadataSink)
	extractor := extractor.NewDomExtractor(metadataSink)
	sanitizer := sanitizer.NewHTMLSanitizer(metadataSink)
	conversionRule := mdconvert.NewRule()
	resolver := assets.NewResolver(metadataSink)
	markdownConstraint := normalize.NewMarkdownConstraint(metadataSink)
	storageSink := storage.NewSink(metadataSink)
	return Scheduler{
		metadataSink:           metadataSink,
		crawlFinalizer:         crawlFinalizer,
		robot:                  robot,
		frontier:               &frontier,
		htmlFetcher:            fetcher,
		domExtractor:           extractor,
		htmlSanitizer:          sanitizer,
		markdownConversionRule: conversionRule,
		assetResolver:          resolver,
		markdownConstraint:     markdownConstraint,
		storageSink:            storageSink,
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
) internal.ClassifiedError {
	// Fetch robots.txt
	robotsDecision, robotsError := s.robot.Decide(url)
	// Robots infrastructure failure → scheduler-level error
	if robotsError != nil {
		return robotsError
	}

	if robotsDecision.CrawlDelay != nil {
		crawlDelay := *robotsDecision.CrawlDelay
		if crawlDelay > time.Duration(0) {
			currentHostTiming, exists := s.hostTimings[s.currentHost]
			if exists {
				currentHostTiming.crawlDelay = crawlDelay
			} else {
				s.hostTimings[s.currentHost] = hostTiming{
					crawlDelay: crawlDelay,
				}
			}
		}
	}

	// Robots explicitly disallowed → normal, terminal outcome
	if !robotsDecision.Allowed {
		// Important:
		// - metadata already emitted by robots
		// - NO retry
		// - NO abort
		// - NO frontier submission

		// TODO: if had CrawlDelay, postpone the downstream activity (real implementation)
		// for now (for illustrative purposes) just continue
		s.robotsCrawlDelay = robotsDecision.CrawlDelay
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

	// 1.1 Initialize Robots and Frontier
	s.robot.Init(cfg.UserAgent())
	s.frontier.Init(cfg)

	// 2. Fetch robots.txt & decide the crawling policy for this hostname based on that
	s.currentHost = cfg.SeedURLs()[0].Host
	err = s.SubmitUrlForAdmission(cfg.SeedURLs()[0], frontier.SourceSeed, 0)
	if err != nil {
		return CrawlingExecution{}, err
	}

	// If frontier still has URL to be crawl...
	for {
		nextCrawlToken, ok := s.frontier.Dequeue()
		if !ok {
			break
		}

		// 3. Fetch Page URL
		fetchResult, err := s.htmlFetcher.Fetch(nextCrawlToken.URL())
		if err != nil {
			if err.Severity() == internal.SeverityFatal {
				return CrawlingExecution{}, err
			}
			// recoverable → log already done → count error
			totalErrors++
			continue
		}

		// 4. Extract HTML DOM
		extractionResult, err := s.domExtractor.Extract(fetchResult)
		if err != nil {
			if err.Severity() == internal.SeverityFatal {
				return CrawlingExecution{}, err
			}
			totalErrors++
			continue
		}

		// 5. Sanitize extracted HTML
		sanitizedHtml, err := s.htmlSanitizer.Sanitize(extractionResult)
		if err != nil {
			if err.Severity() == internal.SeverityFatal {
				return CrawlingExecution{}, err
			}
			totalErrors++
			continue
		}

		// 5.1 submit all discovered links through robots checking to frontier
		for _, discoveredurl := range sanitizedHtml.GetDiscoveredURLs() {
			submissionErr := s.SubmitUrlForAdmission(discoveredurl, frontier.SourceCrawl, nextCrawlToken.Depth()+1)
			if submissionErr != nil {
				// Submission errors are scheduler-level errors, count them
				totalErrors++
				// Continue processing other URLs, don't abort the crawl
			}
		}

		// 6. HTML → Markdown Conversion
		markdownDoc := s.markdownConversionRule.Convert(sanitizedHtml)

		// 7. Assets Resolution
		assetfulMarkdown, err := s.assetResolver.Resolve(markdownDoc)
		if err != nil {
			if err.Severity() == internal.SeverityFatal {
				return CrawlingExecution{}, err
			}
			totalErrors++
			// Continue to process the markdown even if asset resolution had errors
		}
		// Count assets processed (for now, this is a placeholder until asset resolver exposes count)
		// TODO: Extract actual asset count from assetfulMarkdown when available
		totalAssets += 0

		// 8. Markdown Normalization
		normalizedMarkdown, err := s.markdownConstraint.Normalize(assetfulMarkdown)
		if err != nil {
			if err.Severity() == internal.SeverityFatal {
				return CrawlingExecution{}, err
			}
			totalErrors++
			continue
		}

		// 9. Write Artifact
		writeResult, err := s.storageSink.Write(normalizedMarkdown)
		if err != nil {
			if err.Severity() == internal.SeverityFatal {
				return CrawlingExecution{}, err
			}
			// recoverable → log already done → count error
			totalErrors++
			continue
		}
		s.writeResults = append(s.writeResults, writeResult)
	}

	// Stats are recorded by defer - return successful execution result
	return CrawlingExecution{
		WriteResults: s.writeResults,
	}, nil
}

// ---------------------------------------------------------------------------
// Test Helper Methods
// These methods are exported to enable testing of SubmitUrlForAdmission()
// and other scheduler internals. They are not part of the public API.
// ---------------------------------------------------------------------------

// InitRobot initializes the robot with the given user agent.
// This is a test helper method.
func (s *Scheduler) InitRobot(userAgent string) {
	s.robot.Init(userAgent)
}

// SetCurrentHost sets the current host for hostTimings tracking.
// This is a test helper method to simulate the host context.
func (s *Scheduler) SetCurrentHost(host string) {
	s.currentHost = host
	// Initialize hostTimings map if not already done
	if s.hostTimings == nil {
		s.hostTimings = make(map[string]hostTiming)
	}
}

// FrontierVisitedCount returns the number of URLs in the frontier's visited set.
// This is a test helper method to verify frontier state.
func (s *Scheduler) FrontierVisitedCount() int {
	if s.frontier == nil {
		return 0
	}
	return s.frontier.VisitedCount()
}

// HasHostTiming reports whether the given host exists in hostTimings.
// This is a test helper method.
func (s *Scheduler) HasHostTiming(host string) bool {
	if s.hostTimings == nil {
		return false
	}
	_, exists := s.hostTimings[host]
	return exists
}

// GetHostCrawlDelay returns the crawl delay for the given host.
// Returns 0 if the host does not exist in hostTimings.
// This is a test helper method.
func (s *Scheduler) GetHostCrawlDelay(host string) time.Duration {
	if s.hostTimings == nil {
		return 0
	}
	if timing, exists := s.hostTimings[host]; exists {
		return timing.crawlDelay
	}
	return 0
}

// DequeueFromFrontier dequeues a token from the frontier.
// This is a test helper method to verify frontier contents.
func (s *Scheduler) DequeueFromFrontier() (frontier.CrawlToken, bool) {
	if s.frontier == nil {
		return frontier.CrawlToken{}, false
	}
	return s.frontier.Dequeue()
}
