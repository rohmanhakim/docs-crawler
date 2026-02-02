package scheduler

import (
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/storage"
)

// timing-related data used to track when to fetch host during crawling
type hostTiming struct {
	lastFetchAt time.Time
	crawlDelay  time.Duration
}

type CrawlingExecution struct {
	WriteResults []storage.WriteResult
}

type PipelineOutcome struct {
	Continue bool
	Retry    bool
	Abort    bool
}
