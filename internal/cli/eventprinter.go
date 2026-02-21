package cmd

import (
	"fmt"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/treeprinter"
)

// EventPrinter handles tree-structured printing of metadata events.
// It tracks the current page URL to group related events under a parent fetch.
type EventPrinter struct {
	treePrinter *treeprinter.TreePrinter
	currentPage string // URL of the current page being processed
	hasChildren bool   // tracks if current page has any child events
}

// NewEventPrinter creates a new EventPrinter with the given TreePrinter.
func NewEventPrinter(tp *treeprinter.TreePrinter) *EventPrinter {
	return &EventPrinter{
		treePrinter: tp,
	}
}

// PrintEvent prints a metadata event in tree-structured format.
// Events are grouped by page URL:
//   - FETCH (kind=page) starts a new parent node
//   - PIPELINE events become children of the current page
//   - ARTIFACT events become children of the current page
//   - FETCH (kind=asset) becomes a child of the current page
//   - SKIP and STATS are printed standalone
//   - ERROR is a child if there's a current page context, otherwise standalone
func (ep *EventPrinter) PrintEvent(e metadata.Event) {
	switch e.Kind() {
	case metadata.EventKindFetch:
		ep.printFetch(e.Fetch())

	case metadata.EventKindArtifact:
		ep.printArtifact(e.Artifact())

	case metadata.EventKindPipeline:
		ep.printPipeline(e.Pipeline())

	case metadata.EventKindSkip:
		ep.printSkip(e.Skip())

	case metadata.EventKindError:
		ep.printError(e.Error())

	case metadata.EventKindStats:
		ep.printStats(e.Stats())
	}
}

// printFetch handles FETCH events, starting a new tree for page fetches.
func (ep *EventPrinter) printFetch(fetch *metadata.FetchEvent) {
	// Asset fetches are children of the current page
	if fetch.Kind() == metadata.KindAsset {
		ep.treePrinter.AddChild("[FETCH] %s - %d %s (%.3fs)",
			fetch.FetchURL(),
			fetch.HTTPStatus(),
			fetch.ContentType(),
			fetch.Duration().Seconds())
		ep.hasChildren = true
		return
	}

	// Page fetch starts a new parent tree
	// First finalize any previous parent
	if ep.currentPage != "" {
		ep.treePrinter.EndParent()
	}

	// Start new parent
	ep.treePrinter.StartParent("[FETCH] %s - %d %s (%.3fs, depth=%d)",
		fetch.FetchURL(),
		fetch.HTTPStatus(),
		fetch.ContentType(),
		fetch.Duration().Seconds(),
		fetch.CrawlDepth())

	ep.currentPage = fetch.FetchURL()
	ep.hasChildren = false
}

// printArtifact handles ARTIFACT events as children of the current page.
func (ep *EventPrinter) printArtifact(artifact *metadata.ArtifactRecord) {
	overwrite := ""
	if artifact.Overwrite() {
		overwrite = " (overwrite)"
	}
	ep.treePrinter.AddChild("[ARTIFACT] %s -> %s%s",
		truncateURL(artifact.SourceURL(), 50),
		artifact.WritePath(),
		overwrite)
	ep.hasChildren = true
}

// printPipeline handles PIPELINE events as children of the current page.
func (ep *EventPrinter) printPipeline(pipeline *metadata.PipelineEvent) {
	status := "success"
	if !pipeline.Success() {
		status = "failed"
	}
	linksInfo := ""
	if pipeline.LinksFound() > 0 {
		linksInfo = fmt.Sprintf(" (%d links)", pipeline.LinksFound())
	}
	ep.treePrinter.AddChild("[%s] %s%s",
		pipeline.Stage(),
		status,
		linksInfo)
	ep.hasChildren = true
}

// printSkip handles SKIP events as standalone lines.
func (ep *EventPrinter) printSkip(skip *metadata.SkipEvent) {
	// Finalize any current parent before printing standalone
	if ep.currentPage != "" {
		ep.treePrinter.EndParent()
		ep.currentPage = ""
	}
	ep.treePrinter.PrintStandalone("[SKIP] %s - %s",
		truncateURL(skip.SkippedURL(), 60),
		skip.Reason())
}

// printError handles ERROR events. If there's a current page context,
// the error becomes a child; otherwise it's standalone.
func (ep *EventPrinter) printError(err *metadata.ErrorRecord) {
	// Check if error has page URL context in attrs
	pageURL := findAttr(err.Attrs(), metadata.AttrPageURL)

	if pageURL != "" && pageURL == ep.currentPage {
		// Error belongs to current page, add as child
		ep.treePrinter.AddChild("[ERROR] %s.%s - %s",
			err.PackageName(),
			err.Action(),
			err.ErrorString())
		ep.hasChildren = true
	} else if ep.currentPage != "" {
		// We have a current page, add as child
		ep.treePrinter.AddChild("[ERROR] %s.%s - %s",
			err.PackageName(),
			err.Action(),
			err.ErrorString())
		ep.hasChildren = true
	} else {
		// No page context, print standalone
		ep.treePrinter.PrintStandalone("[ERROR] %s.%s - %v: %s",
			err.PackageName(),
			err.Action(),
			err.Cause(),
			err.ErrorString())
	}
}

// printStats handles STATS events, finalizing any current tree first.
func (ep *EventPrinter) printStats(stats *metadata.CrawlStats) {
	// Finalize any current parent
	if ep.currentPage != "" {
		ep.treePrinter.EndParent()
		ep.currentPage = ""
	}

	ep.treePrinter.PrintStandalone("")
	ep.treePrinter.PrintStandalone("--- CRAWL STATS ---")
	ep.treePrinter.PrintStandalone("Started:           %s", stats.StartedAt().Format(time.RFC3339))
	ep.treePrinter.PrintStandalone("Finished:          %s", stats.FinishedAt().Format(time.RFC3339))
	ep.treePrinter.PrintStandalone("Duration:          %s", stats.FinishedAt().Sub(stats.StartedAt()))
	ep.treePrinter.PrintStandalone("Visited Pages:     %d", stats.TotalVisitedPages())
	ep.treePrinter.PrintStandalone("Written Markdowns: %d", stats.TotalProcessedPages())
	ep.treePrinter.PrintStandalone("Errors:            %d", stats.TotalErrors())
	ep.treePrinter.PrintStandalone("Assets:            %d", stats.TotalAssets())
	ep.treePrinter.PrintStandalone("Retry Q:           %d", stats.ManualRetryQueueCount())
}

// Flush finalizes any remaining buffered output.
func (ep *EventPrinter) Flush() {
	if ep.currentPage != "" {
		ep.treePrinter.EndParent()
		ep.currentPage = ""
	}
	ep.treePrinter.Flush()
}

// truncateURL truncates a URL string to maxLen if it's longer.
func truncateURL(urlStr string, maxLen int) string {
	if len(urlStr) <= maxLen {
		return urlStr
	}
	return urlStr[:maxLen-3] + "..."
}

// findAttr finds an attribute value by key in the attribute slice.
func findAttr(attrs []metadata.Attribute, key metadata.AttributeKey) string {
	for _, attr := range attrs {
		if attr.Key() == key {
			return attr.Value()
		}
	}
	return ""
}
