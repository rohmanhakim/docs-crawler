package scheduler

import (
	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/extractor"
	"github.com/rohmanhakim/docs-crawler/internal/fetcher"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
)

/*
Scheduler Responsibilities

- Coordinate crawl lifecycle
- Enforce global limits (pages, depth)
- Manage graceful shutdown
- Aggregate crawl statistics
*/

type Scheduler struct {
	cfg                config.Config
	metadataRecorder   metadata.Recorder
	crawlingPolicy     frontier.CrawlingPolicy
	htmlFetcher        fetcher.HtmlFetcher
	domExtractor       extractor.DomExtractor
	htmlSanitizer      sanitizer.HtmlSanitizer
	markdownRule       mdconvert.Rule
	assetResolver      assets.Resolver
	markdownConstraint normalize.MarkdownConstraint
	storageSink        storage.Sink
}

func NewScheduler(cfg config.Config) Scheduler {
	return Scheduler{}
}
