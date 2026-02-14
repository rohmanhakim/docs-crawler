package scheduler

import (
	"github.com/rohmanhakim/docs-crawler/internal/storage"
)

type CrawlingExecution struct {
	writeResults []storage.WriteResult
	totalAssets  int
}

func NewCrawlingExecution(
	writeResults []storage.WriteResult,
	totalAssets int,
) CrawlingExecution {
	return CrawlingExecution{
		writeResults: writeResults,
		totalAssets:  totalAssets,
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
