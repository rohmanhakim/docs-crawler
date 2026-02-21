package stagedump_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/stagedump"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"golang.org/x/net/html"
)

func TestFileDumper_DumpFetcherOutput(t *testing.T) {
	tmpDir := t.TempDir()
	dumper := stagedump.NewFileDumper(tmpDir, false)

	testURL := "https://example.com/docs/page"
	testContent := []byte("<html><body>Test Content</body></html>")

	err := dumper.DumpFetcherOutput(testURL, testContent)
	if err != nil {
		t.Fatalf("DumpFetcherOutput failed: %v", err)
	}

	// Verify file created with correct path structure
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read output directory: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 subdirectory, got %d", len(entries))
	}

	// Verify the subdirectory name matches URL hash
	urlHash, _ := hashutil.HashBytes([]byte(testURL), hashutil.HashAlgoSHA256)
	expectedDirHash := urlHash[:12]
	if entries[0].Name() != expectedDirHash {
		t.Errorf("Expected directory name %s, got %s", expectedDirHash, entries[0].Name())
	}

	// Verify file exists and has correct content
	filePath := filepath.Join(tmpDir, entries[0].Name(), "01_fetcher.html")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read dumped file: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("Content mismatch: expected %s, got %s", testContent, content)
	}
}

func TestFileDumper_DumpAllStages(t *testing.T) {
	tmpDir := t.TempDir()
	dumper := stagedump.NewFileDumper(tmpDir, false)
	testURL := "https://example.com/page"

	// Create a simple HTML node for testing
	doc := &html.Node{
		Type: html.ElementNode,
		Data: "div",
	}
	textNode := &html.Node{
		Type: html.TextNode,
		Data: "Test",
	}
	doc.AppendChild(textNode)

	// Dump all stages for one URL
	if err := dumper.DumpFetcherOutput(testURL, []byte("<html>raw</html>")); err != nil {
		t.Fatalf("DumpFetcherOutput failed: %v", err)
	}
	if err := dumper.DumpExtractorOutput(testURL, doc); err != nil {
		t.Fatalf("DumpExtractorOutput failed: %v", err)
	}
	if err := dumper.DumpSanitizerOutput(testURL, doc); err != nil {
		t.Fatalf("DumpSanitizerOutput failed: %v", err)
	}
	if err := dumper.DumpMDConvertOutput(testURL, []byte("# Markdown")); err != nil {
		t.Fatalf("DumpMDConvertOutput failed: %v", err)
	}
	if err := dumper.DumpAssetResolverOutput(testURL, []byte("# With Assets")); err != nil {
		t.Fatalf("DumpAssetResolverOutput failed: %v", err)
	}

	// Verify all files in same directory (grouped by URL)
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read output directory: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 subdirectory, got %d", len(entries))
	}

	subdir := entries[0].Name()
	files, err := os.ReadDir(filepath.Join(tmpDir, subdir))
	if err != nil {
		t.Fatalf("Failed to read subdirectory: %v", err)
	}

	// Verify all 5 files exist
	expectedFiles := []string{
		"01_fetcher.html",
		"02_extractor.html",
		"03_sanitizer.html",
		"04_mdconvert.md",
		"05_asset_resolver.md",
	}

	if len(files) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(files))
	}

	// Verify file names
	fileNames := make(map[string]bool)
	for _, f := range files {
		fileNames[f.Name()] = true
	}
	for _, expected := range expectedFiles {
		if !fileNames[expected] {
			t.Errorf("Missing expected file: %s", expected)
		}
	}
}

func TestFileDumper_MultipleURLs(t *testing.T) {
	tmpDir := t.TempDir()
	dumper := stagedump.NewFileDumper(tmpDir, false)

	url1 := "https://example.com/a"
	url2 := "https://example.com/b"

	// Dump for different URLs
	if err := dumper.DumpFetcherOutput(url1, []byte("Content A")); err != nil {
		t.Fatalf("DumpFetcherOutput for URL1 failed: %v", err)
	}
	if err := dumper.DumpFetcherOutput(url2, []byte("Content B")); err != nil {
		t.Fatalf("DumpFetcherOutput for URL2 failed: %v", err)
	}

	// Should create two separate directories
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read output directory: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("Expected 2 subdirectories, got %d", len(entries))
	}
}

func TestFileDumper_DryRunMode(t *testing.T) {
	tmpDir := t.TempDir()
	dumper := stagedump.NewFileDumper(tmpDir, true) // dryRun = true

	testURL := "https://example.com/test"
	if err := dumper.DumpFetcherOutput(testURL, []byte("test")); err != nil {
		t.Fatalf("DumpFetcherOutput failed: %v", err)
	}

	// Files should still be created even in dry-run mode
	// (dryRun flag is informational, doesn't prevent dumping)
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read output directory: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 subdirectory, got %d", len(entries))
	}
}

func TestFileDumper_NilNode(t *testing.T) {
	tmpDir := t.TempDir()
	dumper := stagedump.NewFileDumper(tmpDir, false)

	testURL := "https://example.com/nil-test"

	// Dumping nil node should not crash
	err := dumper.DumpExtractorOutput(testURL, nil)
	if err != nil {
		t.Fatalf("DumpExtractorOutput with nil node failed: %v", err)
	}

	// Verify file exists with nil indicator
	entries, _ := os.ReadDir(tmpDir)
	subdir := entries[0].Name()
	filePath := filepath.Join(tmpDir, subdir, "02_extractor.html")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read dumped file: %v", err)
	}
	if !strings.Contains(string(content), "nil node") {
		t.Errorf("Expected nil node indicator, got: %s", content)
	}
}

func TestNoOpDumper_DoesNothing(t *testing.T) {
	dumper := stagedump.NewNoOpDumper()

	// All methods should return nil without error
	if err := dumper.DumpFetcherOutput("url", []byte("x")); err != nil {
		t.Errorf("DumpFetcherOutput should return nil, got: %v", err)
	}
	if err := dumper.DumpExtractorOutput("url", nil); err != nil {
		t.Errorf("DumpExtractorOutput should return nil, got: %v", err)
	}
	if err := dumper.DumpSanitizerOutput("url", nil); err != nil {
		t.Errorf("DumpSanitizerOutput should return nil, got: %v", err)
	}
	if err := dumper.DumpMDConvertOutput("url", []byte("x")); err != nil {
		t.Errorf("DumpMDConvertOutput should return nil, got: %v", err)
	}
	if err := dumper.DumpAssetResolverOutput("url", []byte("x")); err != nil {
		t.Errorf("DumpAssetResolverOutput should return nil, got: %v", err)
	}
}
