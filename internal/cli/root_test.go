package cmd_test

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cmd "github.com/rohmanhakim/docs-crawler/internal/cli"
	"github.com/rohmanhakim/docs-crawler/internal/config"
)

// defaultTestURLs returns a default set of test URLs for use in tests
func defaultTestURLs() []url.URL {
	return []url.URL{
		{Scheme: "https", Host: "example.com"},
	}
}

// TestInitConfigNoFlags tests that initConfig returns a Config with default values when only seed URLs are provided
func TestInitConfigNoFlags(t *testing.T) {
	cmd.ResetFlags()

	testURLs := defaultTestURLs()
	cfg, err := cmd.InitConfigWithError(testURLs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	defaultCfg, err := config.WithDefault(baseURL).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	// Verify that the returned config matches the default config for non-overridden values
	if cfg.MaxDepth() != defaultCfg.MaxDepth() {
		t.Errorf("Expected MaxDepth %d, got %d", defaultCfg.MaxDepth(), cfg.MaxDepth())
	}
	if cfg.Concurrency() != defaultCfg.Concurrency() {
		t.Errorf("Expected Concurrency %d, got %d", defaultCfg.Concurrency(), cfg.Concurrency())
	}
	if cfg.OutputDir() != defaultCfg.OutputDir() {
		t.Errorf("Expected OutputDir %s, got %s", defaultCfg.OutputDir(), cfg.OutputDir())
	}
	if cfg.DryRun() != defaultCfg.DryRun() {
		t.Errorf("Expected DryRun %t, got %t", defaultCfg.DryRun(), cfg.DryRun())
	}
	if cfg.MaxPages() != defaultCfg.MaxPages() {
		t.Errorf("Expected MaxPages %d, got %d", defaultCfg.MaxPages(), cfg.MaxPages())
	}

	// Verify the seed URLs are correctly set
	if len(cfg.SeedURLs()) != len(testURLs) {
		t.Errorf("Expected %d SeedURLs, got %d", len(testURLs), len(cfg.SeedURLs()))
	}
}

// TestInitConfigWithEmptySeedUrls tests that InitConfigWithError returns error when seed URLs are empty
func TestInitConfigWithEmptySeedUrls(t *testing.T) {
	cmd.ResetFlags()

	_, err := cmd.InitConfigWithError([]url.URL{})
	if err == nil {
		t.Fatal("Expected error for empty seed URLs, got nil")
	}

	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Errorf("Expected ErrInvalidConfig, got: %v", err)
	}
}

// TestInitConfigWithMaxDepth tests that maxDepth flag is properly applied
func TestInitConfigWithMaxDepth(t *testing.T) {
	tests := []struct {
		name      string
		maxDepth  int
		expectErr bool
	}{
		{"Zero maxDepth", 0, false},
		{"Positive maxDepth", 10, false},
		{"Negative maxDepth", -1, false},
		{"Large maxDepth", 1000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()

			// We need to manually set the flag for testing
			cmd.SetMaxDepthForTest(tt.maxDepth)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// When maxDepth is 0, it should remain as default
			expectedDepth := tt.maxDepth
			if tt.maxDepth <= 0 {
				baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
				build, err := config.WithDefault(baseURL).Build()
				if err != nil {
					t.Errorf("should not have any error, got %d", err)
				}
				expectedDepth = build.MaxDepth()
			}

			if cfg.MaxDepth() != expectedDepth {
				t.Errorf("Expected MaxDepth %d, got %d", expectedDepth, cfg.MaxDepth())
			}
		})
	}
}

// TestInitConfigWithConcurrency tests that concurrency flag is properly applied
func TestInitConfigWithConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		expectErr   bool
	}{
		{"Zero concurrency", 0, false},
		{"Positive concurrency", 5, false},
		{"Negative concurrency", -1, false},
		{"Large concurrency", 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetConcurrencyForTest(tt.concurrency)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// When concurrency is 0, it should remain as default
			expectedConcurrency := tt.concurrency
			if tt.concurrency <= 0 {
				baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
				build, err := config.WithDefault(baseURL).Build()
				if err != nil {
					t.Errorf("should not have any error, got %d", err)
				}
				expectedConcurrency = build.Concurrency()
			}

			if cfg.Concurrency() != expectedConcurrency {
				t.Errorf("Expected Concurrency %d, got %d", expectedConcurrency, cfg.Concurrency())
			}
		})
	}
}

// TestInitConfigWithOutputDir tests that outputDir flag is properly applied
func TestInitConfigWithOutputDir(t *testing.T) {
	tests := []struct {
		name         string
		outputDir    string
		shouldChange bool
	}{
		{"Empty outputDir", "", false},
		{"Default outputDir", "output", false},
		{"Custom outputDir", "custom-output", true},
		{"Absolute path outputDir", "/tmp/output", true},
		{"Relative path outputDir", "./docs", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetOutputDirForTest(tt.outputDir)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
			build, err := config.WithDefault(baseURL).Build()
			if err != nil {
				t.Errorf("should not have any error, got %d", err)
			}
			defaultOutputDir := build.OutputDir()
			expectedOutputDir := defaultOutputDir
			if tt.shouldChange && tt.outputDir != "" && tt.outputDir != "output" {
				expectedOutputDir = tt.outputDir
			}

			if cfg.OutputDir() != expectedOutputDir {
				t.Errorf("Expected OutputDir %s, got %s", expectedOutputDir, cfg.OutputDir())
			}
		})
	}
}

// TestInitConfigWithDryRun tests that dryRun flag is properly applied
func TestInitConfigWithDryRun(t *testing.T) {
	tests := []struct {
		name           string
		dryRun         bool
		expectedDryRun bool
	}{
		{"DryRun true", true, true},
		{"DryRun false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetDryRunForTest(tt.dryRun)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if cfg.DryRun() != tt.expectedDryRun {
				t.Errorf("Expected DryRun %t, got %t", tt.expectedDryRun, cfg.DryRun())
			}
		})
	}
}

// TestInitConfigWithSeedURLs tests that seedURLs are properly parsed and applied
func TestInitConfigWithSeedURLs(t *testing.T) {
	tests := []struct {
		name        string
		seedURLs    []string
		expectError bool
		expectedLen int
	}{
		{"Single valid URL", []string{"https://example.com"}, false, 1},
		{"Multiple valid URLs", []string{"https://example.com", "https://docs.example.com"}, false, 2},
		{"Mixed protocols", []string{"https://example.com", "http://localhost:8080"}, false, 2},
		{"URLs with paths", []string{"https://example.com/docs", "https://example.com/api"}, false, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()

			// Parse seed URLs
			var parsedURLs []url.URL
			for _, urlStr := range tt.seedURLs {
				parsedURL, _ := url.Parse(urlStr)
				parsedURLs = append(parsedURLs, *parsedURL)
			}

			cfg, err := cmd.InitConfigWithError(parsedURLs)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				// Check that error message contains expected text
				if err != nil && !strings.Contains(err.Error(), "error parsing seed URL") {
					t.Errorf("Expected error about parsing seed URL, got: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if len(cfg.SeedURLs()) != tt.expectedLen {
					t.Errorf("Expected %d SeedURLs, got %d", tt.expectedLen, len(cfg.SeedURLs()))
				}

				// Verify URL parsing is correct
				for i, seedURL := range tt.seedURLs {
					expectedURL, _ := url.Parse(seedURL)
					if cfg.SeedURLs()[i].String() != expectedURL.String() {
						t.Errorf("Expected SeedURL[%d] to be %s, got %s", i, expectedURL.String(), cfg.SeedURLs()[i].String())
					}
				}
			}
		})
	}
}

// TestInitConfigWithPartialConfigFile tests loading config from a partial config file
func TestInitConfigWithPartialConfigFile(t *testing.T) {
	cmd.ResetFlags()

	// Create a temporary partial config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	// Partial config with seedUrls (required) and some other fields
	configContent := `{
		"seedUrls": [{"Scheme": "https", "Host": "test-docs.com", "Path": "/docs"}],
		"maxDepth": 10,
		"concurrency": 5,
		"outputDir": "test-output",
		"dryRun": true,
		"maxPages": 50,
		"userAgent": "test-agent",
		"randomSeed": 123456789,
		"allowedHosts": {"example.com": {}, "docs.example.com": {}},
		"allowedPathPrefix": ["/docs", "/api"]
	}`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cmd.SetConfigFileForTest(configFile)

	testURLs := defaultTestURLs()
	cfg, err := cmd.InitConfigWithError(testURLs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the config was loaded correctly with partial values
	if cfg.MaxDepth() != 10 {
		t.Errorf("Expected MaxDepth 10, got %d", cfg.MaxDepth())
	}
	if cfg.Concurrency() != 5 {
		t.Errorf("Expected Concurrency 5, got %d", cfg.Concurrency())
	}
	if cfg.OutputDir() != "test-output" {
		t.Errorf("Expected OutputDir 'test-output', got %s", cfg.OutputDir())
	}
	if !cfg.DryRun() {
		t.Errorf("Expected DryRun true, got false")
	}
	if cfg.MaxPages() != 50 {
		t.Errorf("Expected MaxPages 50, got %d", cfg.MaxPages())
	}
	if cfg.UserAgent() != "test-agent" {
		t.Errorf("Expected UserAgent 'test-agent', got %s", cfg.UserAgent())
	}
	if cfg.RandomSeed() != 123456789 {
		t.Errorf("Expected RandomSeed 123456789, got %d", cfg.RandomSeed())
	}
	// When using config file, seed URLs from file should be used
	if len(cfg.SeedURLs()) != 1 || cfg.SeedURLs()[0].String() != "https://test-docs.com/docs" {
		t.Errorf("Expected SeedURLs to be loaded from config, got %v", cfg.SeedURLs())
	}

	// Verify default fields are preserved (baseDelay, jitter, timeout should use defaults)
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}

	defaultCfg, err := config.WithDefault(baseURL).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.BaseDelay() != defaultCfg.BaseDelay() {
		t.Errorf("Expected BaseDelay to use default, got %v", cfg.BaseDelay())
	}
	if cfg.Jitter() != defaultCfg.Jitter() {
		t.Errorf("Expected Jitter to use default, got %v", cfg.Jitter())
	}
	if cfg.Timeout() != defaultCfg.Timeout() {
		t.Errorf("Expected Timeout to use default, got %v", cfg.Timeout())
	}
}

func TestInitConfigWithPartialConfigFileNoSeedUrls(t *testing.T) {
	cmd.ResetFlags()

	// Create a temporary partial config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	// Partial config without seedUrls (should fail)
	configContent := `{
		"maxDepth": 10,
		"concurrency": 5,
		"outputDir": "test-output",
		"dryRun": true,
		"maxPages": 50,
		"userAgent": "test-agent",
		"randomSeed": 123456789,
		"allowedHosts": {"example.com": {}, "docs.example.com": {}},
		"allowedPathPrefix": ["/docs", "/api"]
	}`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cmd.SetConfigFileForTest(configFile)

	testURLs := defaultTestURLs()
	_, err = cmd.InitConfigWithError(testURLs)
	if err == nil {
		t.Errorf("Should error")
	}
	if err != nil {
		if !errors.Is(err, config.ErrInvalidConfig) {
			t.Errorf("expected ErrInvalidConfig error, got: %v", err)
		}
	}
}

// TestInitConfigWithNonExistentFile tests behavior when config file doesn't exist
func TestInitConfigWithNonExistentFile(t *testing.T) {
	cmd.ResetFlags()

	nonExistentFile := "/path/that/does/not/exist/config.json"
	cmd.SetConfigFileForTest(nonExistentFile)

	testURLs := defaultTestURLs()
	_, err := cmd.InitConfigWithError(testURLs)
	if err == nil {
		t.Errorf("Expected error for non-existent config file, got none")
	}
	if err != nil && !strings.Contains(err.Error(), "config file does not exist") {
		t.Errorf("Expected error about non-existent config file, got: %v", err)
	}
}

// TestInitConfigWithInvalidConfigFile tests behavior with invalid config file
func TestInitConfigWithInvalidConfigFile(t *testing.T) {
	cmd.ResetFlags()

	// Create a temporary config file with invalid JSON
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.json")

	invalidJSON := `{invalid json content}`
	err := os.WriteFile(configFile, []byte(invalidJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cmd.SetConfigFileForTest(configFile)

	testURLs := defaultTestURLs()
	_, err = cmd.InitConfigWithError(testURLs)
	if err == nil {
		t.Errorf("Expected error for invalid config file, got none")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to parse config file") {
		t.Errorf("Expected error about parsing config file, got: %v", err)
	}
}

// TestInitConfigWithMultipleFlags tests combination of multiple CLI flags
func TestInitConfigWithMultipleFlags(t *testing.T) {
	tests := []struct {
		name           string
		maxDepth       int
		concurrency    int
		outputDir      string
		dryRun         bool
		expectedValues map[string]interface{}
	}{
		{
			name:        "All flags set with custom values",
			maxDepth:    7,
			concurrency: 8,
			outputDir:   "combined-output",
			dryRun:      true,
			expectedValues: map[string]interface{}{
				"MaxDepth":    7,
				"Concurrency": 8,
				"OutputDir":   "combined-output",
				"DryRun":      true,
			},
		},
		{
			name:        "Some flags default, some custom",
			maxDepth:    0,    // Should use default
			concurrency: 15,   // Custom
			outputDir:   "",   // Should use default
			dryRun:      true, // Custom
			expectedValues: map[string]interface{}{
				"MaxDepth":    0, // Will check against actual default
				"Concurrency": 15,
				"OutputDir":   "", // Will check against actual default
				"DryRun":      true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetMaxDepthForTest(tt.maxDepth)
			cmd.SetConcurrencyForTest(tt.concurrency)
			cmd.SetOutputDirForTest(tt.outputDir)
			cmd.SetDryRunForTest(tt.dryRun)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			for key, expectedValue := range tt.expectedValues {
				switch key {
				case "MaxDepth":
					expectedVal := expectedValue.(int)
					if expectedVal == 0 {
						baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
						build, err := config.WithDefault(baseURL).Build()
						if err != nil {
							t.Errorf("should not have any error, got %d", err)
						}
						expectedVal = build.MaxDepth()
					}
					if cfg.MaxDepth() != expectedVal {
						t.Errorf("Expected MaxDepth %d, got %d", expectedVal, cfg.MaxDepth())
					}
				case "Concurrency":
					if cfg.Concurrency() != expectedValue.(int) {
						t.Errorf("Expected Concurrency %d, got %d", expectedValue.(int), cfg.Concurrency())
					}
				case "OutputDir":
					expectedVal := expectedValue.(string)
					if expectedVal == "" {
						baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
						build, err := config.WithDefault(baseURL).Build()
						if err != nil {
							t.Errorf("should not have any error, got %d", err)
						}
						expectedVal = build.OutputDir()
					}
					if cfg.OutputDir() != expectedVal {
						t.Errorf("Expected OutputDir %s, got %s", expectedVal, cfg.OutputDir())
					}
				case "DryRun":
					if cfg.DryRun() != expectedValue.(bool) {
						t.Errorf("Expected DryRun %t, got %t", expectedValue.(bool), cfg.DryRun())
					}
				}
			}
		})
	}
}

// TestResetFlags tests that ResetFlags properly resets all flag values
func TestResetFlags(t *testing.T) {
	// First set some values
	cmd.SetConfigFileForTest("test.yaml")
	cmd.SetSeedURLsForTest([]string{"https://example.com"})
	cmd.SetMaxDepthForTest(10)
	cmd.SetConcurrencyForTest(5)
	cmd.SetOutputDirForTest("custom")
	cmd.SetDryRunForTest(true)

	// Reset flags
	cmd.ResetFlags()

	// Now test that InitConfig returns default values
	testURLs := defaultTestURLs()
	cfg, err := cmd.InitConfigWithError(testURLs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	defaultCfg, err := config.WithDefault(baseURL).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.MaxDepth() != defaultCfg.MaxDepth() {
		t.Errorf("After ResetFlags, expected MaxDepth %d, got %d", defaultCfg.MaxDepth(), cfg.MaxDepth())
	}
	if cfg.Concurrency() != defaultCfg.Concurrency() {
		t.Errorf("After ResetFlags, expected Concurrency %d, got %d", defaultCfg.Concurrency(), cfg.Concurrency())
	}
	if cfg.OutputDir() != defaultCfg.OutputDir() {
		t.Errorf("After ResetFlags, expected OutputDir %s, got %s", defaultCfg.OutputDir(), cfg.OutputDir())
	}
	if cfg.DryRun() != defaultCfg.DryRun() {
		t.Errorf("After ResetFlags, expected DryRun %t, got %t", defaultCfg.DryRun(), cfg.DryRun())
	}
	if cfg.MaxPages() != defaultCfg.MaxPages() {
		t.Errorf("After ResetFlags, expected MaxPages %d, got %d", defaultCfg.MaxPages(), cfg.MaxPages())
	}
}

// TestInitConfigCompleteIntegration tests a complete integration scenario
func TestInitConfigCompleteIntegration(t *testing.T) {
	cmd.ResetFlags()

	// Set up a complex scenario with multiple seed URLs and custom flags
	seedURLs := []url.URL{
		{Scheme: "https", Host: "docs.example.com"},
		{Scheme: "https", Host: "api.example.com", Path: "/v1"},
		{Scheme: "https", Host: "blog.example.com"},
	}
	cmd.SetMaxDepthForTest(12)
	cmd.SetConcurrencyForTest(7)
	cmd.SetOutputDirForTest("/tmp/docs-crawl")
	cmd.SetDryRunForTest(true)

	cfg, err := cmd.InitConfigWithError(seedURLs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify seed URLs
	if len(cfg.SeedURLs()) != len(seedURLs) {
		t.Errorf("Expected %d SeedURLs, got %d", len(seedURLs), len(cfg.SeedURLs()))
	}

	for i, expectedURL := range seedURLs {
		if cfg.SeedURLs()[i].String() != expectedURL.String() {
			t.Errorf("Expected SeedURL[%d] to be %s, got %s", i, expectedURL.String(), cfg.SeedURLs()[i].String())
		}
	}

	// Verify other settings
	if cfg.MaxDepth() != 12 {
		t.Errorf("Expected MaxDepth 12, got %d", cfg.MaxDepth())
	}
	if cfg.Concurrency() != 7 {
		t.Errorf("Expected Concurrency 7, got %d", cfg.Concurrency())
	}
	if cfg.OutputDir() != "/tmp/docs-crawl" {
		t.Errorf("Expected OutputDir '/tmp/docs-crawl', got %s", cfg.OutputDir())
	}
	if !cfg.DryRun() {
		t.Errorf("Expected DryRun true, got false")
	}
}

// TestInitConfigWithMaxPages tests that maxPages flag is properly applied
func TestInitConfigWithMaxPages(t *testing.T) {
	tests := []struct {
		name      string
		maxPages  int
		expectErr bool
	}{
		{"Zero maxPages", 0, false},
		{"Positive maxPages", 50, false},
		{"Negative maxPages", -1, false},
		{"Large maxPages", 10000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetMaxPagesForTest(tt.maxPages)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// When maxPages is 0 or negative, it should remain as default
			expectedMaxPages := tt.maxPages
			if tt.maxPages <= 0 {
				baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
				build, err := config.WithDefault(baseURL).Build()
				if err != nil {
					t.Errorf("should not have any error, got %d", err)
				}
				expectedMaxPages = build.MaxPages()
			}

			if cfg.MaxPages() != expectedMaxPages {
				t.Errorf("Expected MaxPages %d, got %d", expectedMaxPages, cfg.MaxPages())
			}
		})
	}
}

// TestInitConfigWithUserAgent tests that userAgent flag is properly applied
func TestInitConfigWithUserAgent(t *testing.T) {
	tests := []struct {
		name         string
		userAgent    string
		shouldChange bool
	}{
		{"Empty userAgent", "", false},
		{"Custom userAgent", "my-crawler/1.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetUserAgentForTest(tt.userAgent)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
			build, err := config.WithDefault(baseURL).Build()
			if err != nil {
				t.Errorf("should not have any error, got %d", err)
			}
			defaultUserAgent := build.UserAgent()
			expectedUserAgent := defaultUserAgent
			if tt.shouldChange && tt.userAgent != "" {
				expectedUserAgent = tt.userAgent
			}

			if cfg.UserAgent() != expectedUserAgent {
				t.Errorf("Expected UserAgent %s, got %s", expectedUserAgent, cfg.UserAgent())
			}
		})
	}
}

// TestInitConfigWithTimeout tests that timeout flag is properly applied
func TestInitConfigWithTimeout(t *testing.T) {
	tests := []struct {
		name      string
		timeout   time.Duration
		expectErr bool
	}{
		{"Zero timeout", 0, false},
		{"Positive timeout", time.Second * 30, false},
		{"Negative timeout", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetTimeoutForTest(tt.timeout)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// When timeout is 0 or negative, it should remain as default
			expectedTimeout := tt.timeout
			if tt.timeout <= 0 {
				baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
				build, err := config.WithDefault(baseURL).Build()
				if err != nil {
					t.Errorf("should not have any error, got %d", err)
				}
				expectedTimeout = build.Timeout()
			}

			if cfg.Timeout() != expectedTimeout {
				t.Errorf("Expected Timeout %v, got %v", expectedTimeout, cfg.Timeout())
			}
		})
	}
}

// TestInitConfigWithBaseDelay tests that baseDelay flag is properly applied
func TestInitConfigWithBaseDelay(t *testing.T) {
	tests := []struct {
		name      string
		baseDelay time.Duration
		expectErr bool
	}{
		{"Zero baseDelay", 0, false},
		{"Positive baseDelay", time.Second * 2, false},
		{"Negative baseDelay", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetBaseDelayForTest(tt.baseDelay)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// When baseDelay is 0 or negative, it should remain as default
			expectedBaseDelay := tt.baseDelay
			if tt.baseDelay <= 0 {
				baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
				build, err := config.WithDefault(baseURL).Build()
				if err != nil {
					t.Errorf("should not have any error, got %d", err)
				}
				expectedBaseDelay = build.BaseDelay()
			}

			if cfg.BaseDelay() != expectedBaseDelay {
				t.Errorf("Expected BaseDelay %v, got %v", expectedBaseDelay, cfg.BaseDelay())
			}
		})
	}
}

// TestInitConfigWithJitter tests that jitter flag is properly applied
func TestInitConfigWithJitter(t *testing.T) {
	tests := []struct {
		name      string
		jitter    time.Duration
		expectErr bool
	}{
		{"Zero jitter", 0, false},
		{"Positive jitter", time.Millisecond * 500, false},
		{"Negative jitter", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetJitterForTest(tt.jitter)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// When jitter is 0 or negative, it should remain as default
			expectedJitter := tt.jitter
			if tt.jitter <= 0 {
				baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
				build, err := config.WithDefault(baseURL).Build()
				if err != nil {
					t.Errorf("should not have any error, got %d", err)
				}
				expectedJitter = build.Jitter()
			}

			if cfg.Jitter() != expectedJitter {
				t.Errorf("Expected Jitter %v, got %v", expectedJitter, cfg.Jitter())
			}
		})
	}
}

// TestInitConfigWithRandomSeed tests that randomSeed flag is properly applied
func TestInitConfigWithRandomSeed(t *testing.T) {
	tests := []struct {
		name       string
		randomSeed int64
		expectErr  bool
	}{
		{"Zero randomSeed", 0, false},
		{"Positive randomSeed", 123456789, false},
		{"Negative randomSeed", -98765, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetRandomSeedForTest(tt.randomSeed)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// When randomSeed is 0, it should remain as default (current time)
			// Since we can't know current time exactly, we just check if it's non-zero
			// after ResetFlags+InitConfig if a value was provided
			if tt.randomSeed != 0 && cfg.RandomSeed() == 0 {
				t.Errorf("Expected RandomSeed to be set, got 0")
			}
		})
	}
}

// TestInitConfigWithAllowedHosts tests that allowedHosts flag is properly applied
func TestInitConfigWithAllowedHosts(t *testing.T) {
	tests := []struct {
		name         string
		allowedHosts []string
		expectLen    int
		expectedHost string
	}{
		{"Empty allowedHosts defaults to seed URL", []string{}, 1, "example.com"},
		{"Single allowedHost", []string{"custom.com"}, 1, "custom.com"},
		{"Multiple allowedHosts", []string{"example.com", "docs.example.com", "api.example.com"}, 3, ""},
		{"With empty strings", []string{"", "example.com", ""}, 1, "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetAllowedHostsForTest(tt.allowedHosts)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(cfg.AllowedHosts()) != tt.expectLen {
				t.Errorf("Expected %d AllowedHosts, got %d", tt.expectLen, len(cfg.AllowedHosts()))
			}

			// Verify expected host is in the set
			if tt.expectedHost != "" {
				if _, exists := cfg.AllowedHosts()[tt.expectedHost]; !exists {
					t.Errorf("Expected %s in AllowedHosts, got %v", tt.expectedHost, cfg.AllowedHosts())
				}
			}

			// Verify each non-empty host is properly added to the set
			for _, host := range tt.allowedHosts {
				if host != "" {
					if _, exists := cfg.AllowedHosts()[host]; !exists {
						t.Errorf("Expected %s in AllowedHosts", host)
					}
				}
			}
		})
	}
}

// TestInitConfigWithAllowedPathPrefix tests that allowedPathPrefix flag is properly applied
func TestInitConfigWithAllowedPathPrefix(t *testing.T) {
	tests := []struct {
		name              string
		allowedPathPrefix []string
		expectedLen       int
		isDefault         bool
	}{
		{"Empty allowedPathPrefix", []string{}, 1, true}, // Empty means use default
		{"Single allowedPathPrefix", []string{"/docs"}, 1, false},
		{"Multiple allowedPathPrefix", []string{"/docs", "/api", "/blog"}, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.ResetFlags()
			cmd.SetAllowedPathPrefixForTest(tt.allowedPathPrefix)

			testURLs := defaultTestURLs()
			cfg, err := cmd.InitConfigWithError(testURLs)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			expectedLen := tt.expectedLen
			if tt.isDefault {
				baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
				build, err := config.WithDefault(baseURL).Build()
				if err != nil {
					t.Errorf("should not have any error, got %d", err)
				}
				expectedLen = len(build.AllowedPathPrefix())
			}

			if len(cfg.AllowedPathPrefix()) != expectedLen {
				t.Errorf("Expected %d AllowedPathPrefix, got %d", expectedLen, len(cfg.AllowedPathPrefix()))
			}

			// Verify each path prefix matches
			for i, prefix := range tt.allowedPathPrefix {
				if i < len(cfg.AllowedPathPrefix()) && cfg.AllowedPathPrefix()[i] != prefix {
					t.Errorf("Expected AllowedPathPrefix[%d] %s, got %s", i, prefix, cfg.AllowedPathPrefix()[i])
				}
			}
		})
	}
}

// TestInitConfigAllowedHostsDefaultsToSeedUrls tests that allowedHosts defaults to seed URL hostnames
func TestInitConfigAllowedHostsDefaultsToSeedUrls(t *testing.T) {
	cmd.ResetFlags()

	// Multiple seed URLs - allowedHosts should default to all of them
	seedURLs := []url.URL{
		{Scheme: "https", Host: "docs.example.com"},
		{Scheme: "https", Host: "api.example.com", Path: "/v1"},
		{Scheme: "https", Host: "blog.example.com"},
	}

	cfg, err := cmd.InitConfigWithError(seedURLs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify allowedHosts defaults to all seed URL hostnames
	if len(cfg.AllowedHosts()) != 3 {
		t.Errorf("Expected 3 AllowedHosts, got %d", len(cfg.AllowedHosts()))
	}
	if _, exists := cfg.AllowedHosts()["docs.example.com"]; !exists {
		t.Errorf("Expected 'docs.example.com' in AllowedHosts, got %v", cfg.AllowedHosts())
	}
	if _, exists := cfg.AllowedHosts()["api.example.com"]; !exists {
		t.Errorf("Expected 'api.example.com' in AllowedHosts, got %v", cfg.AllowedHosts())
	}
	if _, exists := cfg.AllowedHosts()["blog.example.com"]; !exists {
		t.Errorf("Expected 'blog.example.com' in AllowedHosts, got %v", cfg.AllowedHosts())
	}
}

// TestInitConfigCompleteIntegrationWithAllFlags tests a complete integration scenario with all new flags
func TestInitConfigCompleteIntegrationWithAllFlags(t *testing.T) {

	cmd.ResetFlags()

	// Set up a complex scenario with all flags
	seedURLs := []url.URL{
		{Scheme: "https", Host: "docs.example.com"},
		{Scheme: "https", Host: "api.example.com", Path: "/v1"},
	}
	cmd.SetMaxDepthForTest(12)
	cmd.SetConcurrencyForTest(7)
	cmd.SetOutputDirForTest("/tmp/docs-crawl")
	cmd.SetDryRunForTest(true)
	cmd.SetMaxPagesForTest(1000)
	cmd.SetUserAgentForTest("custom-crawler/2.0")
	cmd.SetTimeoutForTest(time.Second * 45)
	cmd.SetBaseDelayForTest(time.Second * 3)
	cmd.SetJitterForTest(time.Millisecond * 750)
	cmd.SetRandomSeedForTest(987654321)
	cmd.SetAllowedHostsForTest([]string{"example.com", "api.example.com"})
	cmd.SetAllowedPathPrefixForTest([]string{"/docs", "/api"})

	cfg, err := cmd.InitConfigWithError(seedURLs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify all settings
	if len(cfg.SeedURLs()) != len(seedURLs) {
		t.Errorf("Expected %d SeedURLs, got %d", len(seedURLs), len(cfg.SeedURLs()))
	}
	for i, expectedURL := range seedURLs {
		if cfg.SeedURLs()[i].String() != expectedURL.String() {
			t.Errorf("Expected SeedURL[%d] to be %s, got %s", i, expectedURL.String(), cfg.SeedURLs()[i].String())
		}
	}
	if cfg.MaxDepth() != 12 {
		t.Errorf("Expected MaxDepth 12, got %d", cfg.MaxDepth())
	}
	if cfg.Concurrency() != 7 {
		t.Errorf("Expected Concurrency 7, got %d", cfg.Concurrency())
	}
	if cfg.OutputDir() != "/tmp/docs-crawl" {
		t.Errorf("Expected OutputDir '/tmp/docs-crawl', got %s", cfg.OutputDir())
	}
	if !cfg.DryRun() {
		t.Errorf("Expected DryRun true, got false")
	}
	if cfg.MaxPages() != 1000 {
		t.Errorf("Expected MaxPages 1000, got %d", cfg.MaxPages())
	}
	if cfg.UserAgent() != "custom-crawler/2.0" {
		t.Errorf("Expected UserAgent 'custom-crawler/2.0', got %s", cfg.UserAgent())
	}
	if cfg.Timeout() != time.Second*45 {
		t.Errorf("Expected Timeout 45s, got %v", cfg.Timeout())
	}
	if cfg.BaseDelay() != time.Second*3 {
		t.Errorf("Expected BaseDelay 3s, got %v", cfg.BaseDelay())
	}
	if cfg.Jitter() != time.Millisecond*750 {
		t.Errorf("Expected Jitter 750ms, got %v", cfg.Jitter())
	}
	if cfg.RandomSeed() != 987654321 {
		t.Errorf("Expected RandomSeed 987654321, got %d", cfg.RandomSeed())
	}
	if len(cfg.AllowedHosts()) != 2 {
		t.Errorf("Expected 2 AllowedHosts, got %d", len(cfg.AllowedHosts()))
	}
	if _, exists := cfg.AllowedHosts()["example.com"]; !exists {
		t.Errorf("Expected 'example.com' in AllowedHosts")
	}
	if len(cfg.AllowedPathPrefix()) != 2 {
		t.Errorf("Expected 2 AllowedPathPrefix, got %d", len(cfg.AllowedPathPrefix()))
	}
	if cfg.AllowedPathPrefix()[0] != "/docs" {
		t.Errorf("Expected AllowedPathPrefix[0] '/docs', got %s", cfg.AllowedPathPrefix()[0])
	}
	if cfg.AllowedPathPrefix()[1] != "/api" {
		t.Errorf("Expected AllowedPathPrefix[1] '/api', got %s", cfg.AllowedPathPrefix()[1])
	}
}
