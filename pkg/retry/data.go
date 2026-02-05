package retry

import (
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/timeutil"
)

// RetryParam holds the parameters for retry logic.
// These parameters are passed from outside (e.g., config) and should not
// be known by the retry handler internally.
type RetryParam struct {
	BaseDelay    time.Duration
	Jitter       time.Duration
	RandomSeed   int64
	MaxAttempts  int
	BackoffParam timeutil.BackoffParam
}

// NewRetryParam creates a new RetryParam with the given settings.
func NewRetryParam(
	baseDelay time.Duration,
	jitter time.Duration,
	randomSeed int64,
	maxAttempts int,
	backoffParam timeutil.BackoffParam,
) RetryParam {
	return RetryParam{
		BaseDelay:    baseDelay,
		Jitter:       jitter,
		RandomSeed:   randomSeed,
		MaxAttempts:  maxAttempts,
		BackoffParam: backoffParam,
	}
}
