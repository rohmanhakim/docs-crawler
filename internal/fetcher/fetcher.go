package fetcher

import (
	"context"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
)

type Fetcher interface {
	Fetch(
		ctx context.Context,
		crawlDepth int,
		fetchParam FetchParam,
		retryParam retry.RetryParam,
	) (FetchResult, failure.ClassifiedError)
}
