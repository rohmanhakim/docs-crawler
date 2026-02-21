package stagedump

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"golang.org/x/net/html"
)

// FileDumper is an implementation of Dumper that writes stage outputs
// to files in a specified directory. Each URL gets its own subdirectory
// named by URL hash, containing numbered files for each stage.
type FileDumper struct {
	outputDir string
	dryRun    bool // if true, asset resolver images were not actually downloaded
}

// NewFileDumper creates a new FileDumper that writes to the specified directory.
// The dryRun parameter indicates whether the crawler is in dry-run mode,
// which is used for informational purposes in output.
func NewFileDumper(outputDir string, dryRun bool) *FileDumper {
	return &FileDumper{
		outputDir: outputDir,
		dryRun:    dryRun,
	}
}

// DumpFetcherOutput dumps raw HTML from fetcher.
// File: <outputDir>/<url-hash>/01_fetcher.html
func (d *FileDumper) DumpFetcherOutput(url string, body []byte) error {
	return d.dumpFile(url, "01_fetcher.html", body)
}

// DumpExtractorOutput dumps extracted HTML DOM.
// File: <outputDir>/<url-hash>/02_extractor.html
func (d *FileDumper) DumpExtractorOutput(url string, node *html.Node) error {
	content := renderHTMLNode(node)
	return d.dumpFile(url, "02_extractor.html", content)
}

// DumpSanitizerOutput dumps sanitized HTML DOM.
// File: <outputDir>/<url-hash>/03_sanitizer.html
func (d *FileDumper) DumpSanitizerOutput(url string, node *html.Node) error {
	content := renderHTMLNode(node)
	return d.dumpFile(url, "03_sanitizer.html", content)
}

// DumpMDConvertOutput dumps converted markdown.
// File: <outputDir>/<url-hash>/04_mdconvert.md
func (d *FileDumper) DumpMDConvertOutput(url string, content []byte) error {
	return d.dumpFile(url, "04_mdconvert.md", content)
}

// DumpAssetResolverOutput dumps markdown after asset resolution.
// File: <outputDir>/<url-hash>/05_asset_resolver.md
func (d *FileDumper) DumpAssetResolverOutput(url string, content []byte) error {
	return d.dumpFile(url, "05_asset_resolver.md", content)
}

// dumpFile writes content to a file in a URL-specific subdirectory.
// The subdirectory name is derived from the URL hash for deterministic grouping.
func (d *FileDumper) dumpFile(url string, filename string, content []byte) error {
	// Compute URL hash for directory name
	urlHash, err := hashutil.HashBytes([]byte(url), hashutil.HashAlgoSHA256)
	if err != nil {
		return fmt.Errorf("failed to hash URL: %w", err)
	}

	// Use first 12 characters of hash for readable directory name
	dirHash := urlHash[:12]

	// Create URL-specific directory
	dirPath := filepath.Join(d.outputDir, dirHash)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create stage dump directory: %w", err)
	}

	// Write file
	filePath := filepath.Join(dirPath, filename)
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to write stage dump file: %w", err)
	}

	return nil
}

// renderHTMLNode renders an html.Node to a byte slice.
func renderHTMLNode(node *html.Node) []byte {
	if node == nil {
		return []byte("<!-- nil node -->")
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, node); err != nil {
		return []byte(fmt.Sprintf("<!-- render error: %v -->", err))
	}
	return buf.Bytes()
}
