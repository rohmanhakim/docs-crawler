package stagedump

import "golang.org/x/net/html"

// Dumper dumps intermediate stage outputs for debugging.
// The scheduler calls these methods at each pipeline stage.
// This interface allows for clean injection of debugging behavior
// without polluting the pipeline components themselves.
type Dumper interface {
	// DumpFetcherOutput dumps raw HTML from fetcher.
	// Called after a successful fetch.
	DumpFetcherOutput(url string, body []byte) error

	// DumpExtractorOutput dumps extracted HTML DOM.
	// Called after successful DOM extraction.
	DumpExtractorOutput(url string, node *html.Node) error

	// DumpSanitizerOutput dumps sanitized HTML DOM.
	// Called after successful HTML sanitization.
	DumpSanitizerOutput(url string, node *html.Node) error

	// DumpMDConvertOutput dumps converted markdown.
	// Called after successful markdown conversion.
	DumpMDConvertOutput(url string, content []byte) error

	// DumpAssetResolverOutput dumps markdown after asset resolution.
	// Called after successful asset resolution.
	DumpAssetResolverOutput(url string, content []byte) error
}

// NoOpDumper is a no-operation implementation of Dumper.
// It's the default implementation when stage dumping is disabled,
// ensuring zero overhead when not in use.
type NoOpDumper struct{}

// NewNoOpDumper creates a new NoOpDumper.
func NewNoOpDumper() *NoOpDumper {
	return &NoOpDumper{}
}

func (d *NoOpDumper) DumpFetcherOutput(_ string, _ []byte) error {
	return nil
}

func (d *NoOpDumper) DumpExtractorOutput(_ string, _ *html.Node) error {
	return nil
}

func (d *NoOpDumper) DumpSanitizerOutput(_ string, _ *html.Node) error {
	return nil
}

func (d *NoOpDumper) DumpMDConvertOutput(_ string, _ []byte) error {
	return nil
}

func (d *NoOpDumper) DumpAssetResolverOutput(_ string, _ []byte) error {
	return nil
}
