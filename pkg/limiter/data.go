package limiter

import "time"

// timing-related data used to track when to fetch host during crawling
type hostTiming struct {
	lastFetchAt  time.Time
	backoffDelay time.Duration
	crawlDelay   time.Duration
	backoffCount int
}

func (h *hostTiming) CrawlDelay() time.Duration {
	return h.crawlDelay
}

func (h *hostTiming) BackOffDelay() time.Duration {
	return h.backoffDelay
}

func (h *hostTiming) LastFetchAt() time.Time {
	return h.lastFetchAt
}

func (h *hostTiming) BackoffCount() int {
	return h.backoffCount
}
