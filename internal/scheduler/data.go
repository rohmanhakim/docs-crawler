package scheduler

import (
	"github.com/rohmanhakim/docs-crawler/internal/storage"
)

type CrawlingExecution struct {
	WriteResults []storage.WriteResult
}

type PipelineOutcome struct {
	Continue bool
	Retry    bool
	Abort    bool
}
