package fetcher

import (
	"net/url"
)

// HTTP boundary

type FetchResult struct {
	url          url.URL
	responseMeta ResponseMeta
}

func (f *FetchResult) GetFetchURL() url.URL {
	return f.url
}

type ResponseMeta struct{}
