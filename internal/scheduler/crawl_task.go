package scheduler

import (
	"context"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
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
