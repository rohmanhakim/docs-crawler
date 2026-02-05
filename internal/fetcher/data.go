package fetcher

import (
	"net/url"
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
	url  url.URL
	body []byte
	meta ResponseMeta
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
	return f.meta.transferredSizeByte
}

func (f *FetchResult) Headers() map[string]string {
	return f.meta.responseHeaders
}

type ResponseMeta struct {
	statusCode          int
	transferredSizeByte uint64
	responseHeaders     map[string]string
}

// NewFetchResultForTest creates a FetchResult for testing purposes.
// This allows test packages to construct FetchResult values without
// accessing unexported fields directly.
func NewFetchResultForTest(
	url url.URL,
	body []byte,
	statusCode int,
	contentType string,
	transferredSizeByte uint64,
	responseHeaders map[string]string,
) FetchResult {
	return FetchResult{
		url:  url,
		body: body,
		meta: ResponseMeta{
			statusCode:          statusCode,
			transferredSizeByte: transferredSizeByte,
			responseHeaders:     responseHeaders,
		},
	}
}
