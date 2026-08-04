package scheduler

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/build"
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/failurejournal"
	"github.com/rohmanhakim/docs-crawler/pkg/urlutil"
	gopipeline "github.com/rohmanhakim/gopipeline"
)

// CrawlResult holds the outcome of processing a single URL through the crawl pipeline.
// It is returned as the success value in gopipeline.StageResult[CrawlResult].
type CrawlResult struct {
	// WriteResult is the storage write result for the processed page.
	// Zero value if the page was not written (e.g., asset resolution failed but page was still processed).
	WriteResult storage.WriteResult

	// AssetCount is the number of local assets resolved for this page.
	AssetCount int

	// URL is the string representation of the processed URL (for logging/stats).
	URL string
}

func (s *Scheduler) BuildCrawlTask(cfg config.Config, seedScheme string) CrawlTaskFunc {
	return func(ctx context.Context, runner *gopipeline.StageRunner, idx int, token frontier.CrawlToken, stageName string) gopipeline.StageResult[CrawlResult] {
		urlStr := getURLString(token.URL())

		// Log pipeline start for this URL, including pool item index for concurrent debugging
		s.debugLogger.LogStage(ctx, "pipeline", debug.StageEvent{
			Type: debug.EventTypeStart,
			URL:  fmt.Sprintf("idx:%d url:%s", idx, urlStr),
		})

		// 1. Fetch Page URL
		fetchStartTime := time.Now()
		s.debugLogger.LogStage(ctx, "fetcher", debug.StageEvent{
			Type: debug.EventTypeStart,
			URL:  urlStr,
		})

		fetchResult, err := s.htmlFetcher.Fetch(ctx, token.Depth(), token.URL(), RetryOptions(cfg))
		if err != nil {
			s.handleFetchError(ctx, token, urlStr, fetchStartTime, err)
			return gopipeline.NewStageResult(CrawlResult{}, err)
		}

		// Log fetcher completion
		s.debugLogger.LogStage(ctx, "fetcher", debug.StageEvent{
			Type:     debug.EventTypeComplete,
			URL:      getURLString(fetchResult.URL()),
			Duration: time.Since(fetchStartTime),
			Fields: debug.FieldMap{
				"status_code": fetchResult.Code(),
			},
		})

		// Dump fetched HTML
		s.stageDumper.DumpFetcherOutput(urlStr, fetchResult.Body())

		// 2. Extract HTML DOM
		extractionResult, err := s.domExtractor.Extract(fetchResult.URL(), fetchResult.Body())
		if err != nil {
			s.debugLogger.LogStage(ctx, "extractor", debug.StageEvent{
				Type: debug.EventTypeError,
				URL:  urlStr,
			})
			return gopipeline.NewStageResult(CrawlResult{}, err)
		}

		// Dump extraction result
		s.stageDumper.DumpExtractorOutput(urlStr, extractionResult.ContentNode)

		// 3. Sanitize extracted HTML
		sanitizedHtml, err := s.htmlSanitizer.Sanitize(extractionResult.ContentNode)
		if err != nil {
			s.debugLogger.LogStage(ctx, "sanitizer", debug.StageEvent{
				Type: debug.EventTypeError,
				URL:  urlStr,
			})
			return gopipeline.NewStageResult(CrawlResult{}, err)
		}

		// Dump sanitization result
		s.stageDumper.DumpSanitizerOutput(urlStr, sanitizedHtml.GetContentNode())

		// 4. Link discovery — resolve, filter, and submit discovered URLs to frontier
		s.discoverAndSubmitLinks(ctx, sanitizedHtml.GetDiscoveredURLs(), token, seedScheme)

		// 5. HTML → Markdown Conversion
		markdownDoc, err := s.markdownConversionRule.Convert(sanitizedHtml, getURLString(fetchResult.URL()))
		if err != nil {
			return gopipeline.NewStageResult(CrawlResult{}, err)
		}

		// Dump markdown conversion result
		s.stageDumper.DumpMDConvertOutput(urlStr, markdownDoc.GetMarkdownContent())

		// 6. Asset Resolution
		resolveParam := assets.NewResolveParam(cfg.OutputDir(), cfg.MaxAssetSize(), cfg.HashAlgo())
		assetfulMarkdown, err := s.assetResolver.Resolve(
			ctx,
			fetchResult.URL(),
			markdownDoc,
			resolveParam,
			RetryOptions(cfg),
		)
		if err != nil {
			// Track for manual retry if eligible
			if err.RetryPolicy() == failure.RetryPolicyManual {
				s.failureJournal.Record(failurejournal.FailureRecord{
					URL:        getURLString(token.URL()),
					Stage:      failurejournal.StageAsset,
					Error:      err.Error(),
					RetryCount: 0,
					Timestamp:  time.Now(),
				})
			}
			// Continue to process the markdown even if asset resolution had errors
		}

		assetCount := len(assetfulMarkdown.LocalAssets())

		// Dump asset resolving result
		s.stageDumper.DumpAssetResolverOutput(urlStr, assetfulMarkdown.Content())

		// 7. Markdown Normalization
		normalizeParam := normalize.NewNormalizeParam(
			build.FullVersion(),
			fetchResult.FetchedAt(),
			cfg.HashAlgo(),
			token.Depth(),
			cfg.AllowedPathPrefix(),
		)
		normalizedMarkdown, err := s.markdownConstraint.Normalize(
			fetchResult.URL(),
			assetfulMarkdown,
			normalizeParam,
		)
		if err != nil {
			return gopipeline.NewStageResult(CrawlResult{}, err)
		}

		// 8. Write Artifact
		writeResult, err := s.storageSink.Write(
			cfg.OutputDir(),
			normalizedMarkdown,
			cfg.HashAlgo(),
		)
		if err != nil {
			// Track for manual retry if eligible
			if err.RetryPolicy() == failure.RetryPolicyManual {
				s.failureJournal.Record(failurejournal.FailureRecord{
					URL:        getURLString(token.URL()),
					Stage:      failurejournal.StageStorage,
					Error:      err.Error(),
					RetryCount: 0,
					Timestamp:  time.Now(),
				})
			}
			return gopipeline.NewStageResult(CrawlResult{}, err)
		}

		// Apply rate limiting delay at the end of successful processing
		if err := s.rateLimiter.Wait(ctx, s.currentHost); err != nil {
			return gopipeline.NewStageResult(CrawlResult{}, err)
		}

		return gopipeline.NewStageResult(CrawlResult{
			WriteResult: writeResult,
			AssetCount:  assetCount,
			URL:         urlStr,
		}, nil)
	}
}

// handleFetchError handles fetch-specific error logic: debug logging and failure journal recording.
func (s *Scheduler) handleFetchError(ctx context.Context, token frontier.CrawlToken, urlStr string, fetchStartTime time.Time, err failure.ClassifiedError) {
	s.debugLogger.LogStage(ctx, "fetcher", debug.StageEvent{
		Type:     debug.EventTypeError,
		URL:      urlStr,
		Duration: time.Since(fetchStartTime),
	})
	// Track for manual retry if eligible
	if err.RetryPolicy() == failure.RetryPolicyManual {
		s.failureJournal.Record(failurejournal.FailureRecord{
			URL:        getURLString(token.URL()),
			Stage:      failurejournal.StageFetch,
			Error:      err.Error(),
			RetryCount: 0,
			Timestamp:  time.Now(),
		})
	}
}

// discoverAndSubmitLinks resolves discovered URLs, filters by host, and submits them
// to the frontier via the admission pipeline (robots check → frontier).
func (s *Scheduler) discoverAndSubmitLinks(ctx context.Context, discoveredURLs []url.URL, token frontier.CrawlToken, seedScheme string) {
	// Resolve all URLs to absolute form using the seed scheme and current host
	resolvedURLs := make([]url.URL, 0, len(discoveredURLs))
	for _, u := range discoveredURLs {
		resolved := urlutil.Resolve(u, seedScheme, s.currentHost)
		resolvedURLs = append(resolvedURLs, resolved)
	}

	// Filter to only keep URLs from the current host
	filteredURLs := urlutil.FilterByHost(s.currentHost, resolvedURLs)

	// Submit all discovered links through robots checking to frontier
	for _, discoveredURL := range filteredURLs {
		submissionErr := s.SubmitUrlForAdmission(discoveredURL, frontier.SourceCrawl, token.Depth()+1)
		if submissionErr != nil {
			// Check if this is a robots error that requires backoff
			if robotsErr, ok := submissionErr.(*robots.RobotsError); ok {
				s.recordRobotsErrorAndBackoff(robotsErr, discoveredURL)
			}
		}
	}
}

// CrawlTaskFunc is the function signature for processing a single URL through the crawl pipeline.
// It matches gopipeline.Pool's task function signature:
//
//	func(ctx context.Context, runner *gopipeline.StageRunner, idx int, item T, stageName string) gopipeline.StageResult[R]
//
// Where:
//   - T = frontier.CrawlToken (the URL + depth to process)
//   - R = CrawlResult (the outcome data)
//
// The function encapsulates the full processing pipeline for a single URL:
// Fetch → Extract → Sanitize → [link discovery] → Convert → Resolve → Normalize → Write
//
// Error handling:
//   - Recoverable errors (ImpactLevelContinue) are returned as StageResult errors.
//     gopipeline's CollectAll strategy collects them without stopping other workers.
//   - Abort errors (ImpactLevelAbort) are returned as StageResult errors.
//     The scheduler checks for these after the pool batch and stops the crawl.
//
// Link discovery:
//   - Discovered URLs are submitted to the frontier via SubmitUrlForAdmission.
//   - This is a scheduler concern (admission control), not a pipeline stage.
type CrawlTaskFunc func(
	ctx context.Context,
	runner *gopipeline.StageRunner,
	idx int,
	token frontier.CrawlToken,
	stageName string,
) gopipeline.StageResult[CrawlResult]
