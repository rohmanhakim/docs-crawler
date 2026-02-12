package fetcher

import (
	"net/url"
	"time"
)

// HTTP boundary

type FetchParam struct {
	fetchUrl  url.URL
	userAgent string
}

func NewFetchParam(fetchUrl url.URL, userAgent string) FetchParam {
	return FetchParam{
		fetchUrl:  fetchUrl,
		userAgent: userAgent,
	}
}

type FetchResult struct {
	url       url.URL
	body      []byte
	meta      ResponseMeta
	fetchedAt time.Time
}

func (f *FetchResult) URL() url.URL {
	return f.url
}

func (f *FetchResult) Body() []byte {
	return f.body
}

func (f *FetchResult) Code() int {
	return f.meta.statusCode
}

func (f *FetchResult) SizeByte() uint64 {
	return uint64(len(f.body))
}

func (f *FetchResult) Headers() map[string]string {
	return f.meta.responseHeaders
}

func (f *FetchResult) FetchedAt() time.Time {
	return f.fetchedAt
}

type ResponseMeta struct {
	statusCode      int
	responseHeaders map[string]string
}

// NewFetchResultForTest creates a FetchResult for testing purposes.
// This allows test packages to construct FetchResult values without
// accessing unexported fields directly.
func NewFetchResultForTest(
	url url.URL,
	body []byte,
	statusCode int,
	contentType string,
	responseHeaders map[string]string,
	fetchedAt time.Time,
) FetchResult {
	return FetchResult{
		url:       url,
		body:      body,
		fetchedAt: fetchedAt,
		meta: ResponseMeta{
			statusCode:      statusCode,
			responseHeaders: responseHeaders,
		},
	}
}
