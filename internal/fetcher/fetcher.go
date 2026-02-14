package fetcher

import (
	"context"
	"net/http"

	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
)

type Fetcher interface {
	Init(httpClient *http.Client)
	Fetch(
		ctx context.Context,
		crawlDepth int,
		fetchParam FetchParam,
		retryParam retry.RetryParam,
	) (FetchResult, failure.ClassifiedError)
}
