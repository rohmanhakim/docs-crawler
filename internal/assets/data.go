package assets

import (
	"net/url"
	"time"
)

type AssetFetchResult struct {
	fetchUrl   url.URL
	httpStatus int
	duration   time.Duration
	data       []byte
}

func NewAssetFetchResult(
	fetchUrl url.URL,
	httpStatus int,
	duration time.Duration,
	data []byte,
) AssetFetchResult {
	return AssetFetchResult{
		fetchUrl:   fetchUrl,
		httpStatus: httpStatus,
		duration:   duration,
		data:       data,
	}
}

func (a *AssetFetchResult) URL() url.URL {
	return a.fetchUrl
}

func (a *AssetFetchResult) Data() []byte {
	return a.data
}

func (a *AssetFetchResult) Status() int {
	return a.httpStatus
}

func (a *AssetFetchResult) Duration() time.Duration {
	return a.duration
}

type ResolveParam struct {
	outputDir    string
	maxAssetSize int64
}

func NewResolveParam(outputDir string, maxAssetSize int64) ResolveParam {
	return ResolveParam{
		outputDir:    outputDir,
		maxAssetSize: maxAssetSize,
	}
}

func (r ResolveParam) OutputDir() string {
	return r.outputDir
}

func (r ResolveParam) MaxAssetSize() int64 {
	return r.maxAssetSize
}

type AssetfulMarkdownDoc struct {
	content         []byte
	missingAssets   []url.URL
	unparseableURLs []string
	localAssets     []string
}

func NewAssetfulMarkdownDoc(content []byte, missingAssets []url.URL, unparseableURLs []string, localAssets []string) AssetfulMarkdownDoc {
	return AssetfulMarkdownDoc{
		content:         content,
		missingAssets:   missingAssets,
		unparseableURLs: unparseableURLs,
		localAssets:     localAssets,
	}
}

func (a AssetfulMarkdownDoc) Content() []byte {
	return a.content
}

func (a AssetfulMarkdownDoc) MissingAssets() []url.URL {
	return a.missingAssets
}

func (a AssetfulMarkdownDoc) UnparseableURLs() []string {
	return a.unparseableURLs
}

func (a AssetfulMarkdownDoc) LocalAssets() []string {
	return a.localAssets
}
