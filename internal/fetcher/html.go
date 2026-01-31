package fetcher

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

/*
Responsibilities

- Perform HTTP requests
- Apply headers and timeouts
- Handle redirects safely
- Classify responses

Fetch Semantics

- Only successful HTML responses are processed
- Non-HTML content is discarded
- Redirect chains are bounded
- All responses are logged with metadata

The fetcher never parses content; it only returns bytes and metadata.
*/

type HtmlFetcher struct {
	metadataSink metadata.MetadataSink
}

func NewHtmlFetcher(
	metadataSink metadata.MetadataSink,
) HtmlFetcher {
	return HtmlFetcher{
		metadataSink: metadataSink,
	}
}

func (h *HtmlFetcher) Fetch(
	url url.URL,
) (FetchResult, internal.ClassifiedError) {
	h.metadataSink.RecordFetch(
		"https://my-fetch-url-exaple",
		200,
		0*time.Second,
		"text/html",
		0,
		0,
	)
	result, err := fetch()
	if err != nil {
		var fetchError *FetchError
		errors.As(err, &fetchError)
		h.metadataSink.RecordError(
			time.Now(),
			"fetcher",
			"HtmlFetcher.Fetch",
			mapFetchErrorToMetadataCause(fetchError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, fmt.Sprintf("%v", url)),
			},
		)
		return FetchResult{}, fetchError
	}
	return result, nil
}
func fetch() (FetchResult, *FetchError) {
	return FetchResult{}, nil
}
