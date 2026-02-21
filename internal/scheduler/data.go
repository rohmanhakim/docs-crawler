package scheduler

import (
	"net/http"

	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
)

type CrawlingExecution struct {
	writeResults  []storage.WriteResult
	totalWebPages int
	totalAssets   int
	totalErrors   int
}

func NewCrawlingExecution(
	writeResults []storage.WriteResult,
	totalWebPages int,
	totalAssets int,
	totalErrors int,
) CrawlingExecution {
	return CrawlingExecution{
		writeResults:  writeResults,
		totalWebPages: totalWebPages,
		totalAssets:   totalAssets,
		totalErrors:   totalErrors,
	}
}

func (c *CrawlingExecution) WriteResults() []storage.WriteResult {
	return c.writeResults
}

func (c *CrawlingExecution) TotalAssets() int {
	return c.totalAssets
}

type PipelineOutcome struct {
	Continue bool
	Retry    bool
	Abort    bool
}

// CrawlInitialization holds all state needed to execute a crawl.
// This allows splitting the crawl lifecycle into two phases:
// 1. Initialize - sets up all components and submits seed URL
// 2. Execute - runs the actual crawling loop
type CrawlInitialization struct {
	config              config.Config
	httpClient          *http.Client
	currentHost         string
	seedScheme          string
	initialDelayApplied bool
}

// Config returns the loaded configuration.
func (i *CrawlInitialization) Config() config.Config {
	return i.config
}

// HttpClient returns the initialized HTTP client.
func (i *CrawlInitialization) HttpClient() *http.Client {
	return i.httpClient
}

// CurrentHost returns the hostname being crawled.
func (i *CrawlInitialization) CurrentHost() string {
	return i.currentHost
}

// SeedScheme returns the scheme (http/https) of the seed URL.
func (i *CrawlInitialization) SeedScheme() string {
	return i.seedScheme
}

// InitialDelayApplied indicates whether the initial rate limiting delay was applied.
func (i *CrawlInitialization) InitialDelayApplied() bool {
	return i.initialDelayApplied
}

// TotalPages returns the number of pages processed (write results count).
func (c *CrawlingExecution) TotalPages() int {
	return len(c.writeResults)
}

// TotalVisitedPages returns the number of web pages fetched from the frontier.
func (c *CrawlingExecution) TotalVisitedPages() int {
	return c.totalWebPages
}

// TotalErrors returns the total number of errors encountered during crawling.
func (c *CrawlingExecution) TotalErrors() int {
	return c.totalErrors
}
