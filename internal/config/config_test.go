package config_test

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/config"
)

func TestWithDefault(t *testing.T) {
	testURLs := []url.URL{
		{Scheme: "https", Host: "example.org"},
	}

	cfg := config.WithDefault(testURLs)

	if cfg == nil {
		t.Fatal("WithDefault() returned nil")
	}

	builtCfg, err := cfg.Build()

	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}

	// Verify SeedURLs
	if len(builtCfg.SeedURLs()) != 1 {
		t.Errorf("expected 1 seed URL, got %d", len(builtCfg.SeedURLs()))
	}

	// Verify AllowedHosts - should default to seed URL hostnames
	if len(builtCfg.AllowedHosts()) != 1 {
		t.Errorf("expected 1 allowed host, got %d", len(builtCfg.AllowedHosts()))
	}
	if _, ok := builtCfg.AllowedHosts()["example.org"]; !ok {
		t.Errorf("expected 'example.org' in AllowedHosts, got %v", builtCfg.AllowedHosts())
	}

	// Verify AllowedPathPrefix
	if len(builtCfg.AllowedPathPrefix()) != 1 || builtCfg.AllowedPathPrefix()[0] != "/" {
		t.Errorf("expected AllowedPathPrefix to be ['/'], got %v", builtCfg.AllowedPathPrefix())
	}

	// Verify numeric limits
	if builtCfg.MaxDepth() != 3 {
		t.Errorf("expected MaxDepth 3, got %d", builtCfg.MaxDepth())
	}
	if builtCfg.MaxPages() != 100 {
		t.Errorf("expected MaxPages 100, got %d", builtCfg.MaxPages())
	}
	if builtCfg.Concurrency() != 10 {
		t.Errorf("expected Concurrency 10, got %d", builtCfg.Concurrency())
	}

	// Verify durations
	if builtCfg.BaseDelay() != time.Second {
		t.Errorf("expected BaseDelay 1s, got %v", builtCfg.BaseDelay())
	}
	if builtCfg.Jitter() != 500*time.Millisecond {
		t.Errorf("expected Jitter 500ms, got %v", builtCfg.Jitter())
	}
	if builtCfg.Timeout() != 10*time.Second {
		t.Errorf("expected Timeout 10s, got %v", builtCfg.Timeout())
	}

	// Verify other fields
	if builtCfg.UserAgent() != "docs-crawler/1.0" {
		t.Errorf("expected UserAgent 'docs-crawler/1.0', got '%s'", builtCfg.UserAgent())
	}
	if builtCfg.OutputDir() != "output" {
		t.Errorf("expected OutputDir 'output', got '%s'", builtCfg.OutputDir())
	}
	if builtCfg.DryRun() != false {
		t.Errorf("expected DryRun false, got %v", builtCfg.DryRun())
	}

	// RandomSeed should be set (non-zero typically)
	if builtCfg.RandomSeed() == 0 {
		t.Error("expected RandomSeed to be set, got 0")
	}

	// Verify backoff and retry fields
	if builtCfg.MaxAttempt() != 10 {
		t.Errorf("expected MaxAttempt 10, got %d", builtCfg.MaxAttempt())
	}
	if builtCfg.BackoffInitialDuration() != 100*time.Millisecond {
		t.Errorf("expected BackoffInitialDuration 100ms, got %v", builtCfg.BackoffInitialDuration())
	}
	if builtCfg.BackoffMultiplier() != 2.0 {
		t.Errorf("expected BackoffMultiplier 2.0, got %f", builtCfg.BackoffMultiplier())
	}
	if builtCfg.BackoffMaxDuration() != 10*time.Second {
		t.Errorf("expected BackoffMaxDuration 10s, got %v", builtCfg.BackoffMaxDuration())
	}
}

func TestWithDefault_EmptySeedUrls(t *testing.T) {
	// WithDefault should accept empty seed URLs
	cfg := config.WithDefault([]url.URL{})

	if cfg == nil {
		t.Fatal("WithDefault() returned nil")
	}

	builtCfg, err := cfg.Build()
	if err == nil {
		t.Errorf("should error")
	}

	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig err, got %v", err)
	}

	// Verify SeedURLs is empty
	if len(builtCfg.SeedURLs()) != 0 {
		t.Errorf("expected 0 seed URL, got %d", len(builtCfg.SeedURLs()))
	}
}

func TestWithSeedUrls(t *testing.T) {
	testURLs := []url.URL{
		{Scheme: "https", Host: "example.org"},
		{Scheme: "http", Host: "test.com", Path: "/path"},
	}

	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithSeedUrls(testURLs).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}

	if len(cfg.SeedURLs()) != 2 {
		t.Errorf("expected 2 seed URLs, got %d", len(cfg.SeedURLs()))
	}
	if cfg.SeedURLs()[0].String() != "https://example.org" {
		t.Errorf("expected first URL 'https://example.org', got '%s'", cfg.SeedURLs()[0].String())
	}
	if cfg.SeedURLs()[1].String() != "http://test.com/path" {
		t.Errorf("expected second URL 'http://test.com/path', got '%s'", cfg.SeedURLs()[1].String())
	}

	// Verify other fields still have default values
	if cfg.MaxDepth() != 3 {
		t.Errorf("expected MaxDepth to remain default 3, got %d", cfg.MaxDepth())
	}
}

func TestWithAllowedHosts(t *testing.T) {
	testHosts := map[string]struct{}{
		"example.org": {},
		"test.com":    {},
	}

	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithAllowedHosts(testHosts).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if len(cfg.AllowedHosts()) != 2 {
		t.Errorf("expected 2 allowed hosts, got %d", len(cfg.AllowedHosts()))
	}
	if _, ok := cfg.AllowedHosts()["example.org"]; !ok {
		t.Error("expected 'example.org' in AllowedHosts")
	}
	if _, ok := cfg.AllowedHosts()["test.com"]; !ok {
		t.Error("expected 'test.com' in AllowedHosts")
	}

	// Verify SeedURLs still has the base value
	if len(cfg.SeedURLs()) != 1 || cfg.SeedURLs()[0].String() != "https://base.org" {
		t.Errorf("expected SeedURLs to remain at base value, got %v", cfg.SeedURLs())
	}
}

func TestAllowedHosts_DefaultsToSeedUrls(t *testing.T) {
	testURLs := []url.URL{
		{Scheme: "https", Host: "example.org"},
		{Scheme: "https", Host: "docs.example.com"},
	}

	cfg, err := config.WithDefault(testURLs).Build()
	if err != nil {
		t.Fatalf("should not have any error, got %v", err)
	}

	// Should have 2 allowed hosts from seed URLs
	if len(cfg.AllowedHosts()) != 2 {
		t.Errorf("expected 2 allowed hosts, got %d", len(cfg.AllowedHosts()))
	}
	if _, ok := cfg.AllowedHosts()["example.org"]; !ok {
		t.Errorf("expected 'example.org' in AllowedHosts, got %v", cfg.AllowedHosts())
	}
	if _, ok := cfg.AllowedHosts()["docs.example.com"]; !ok {
		t.Errorf("expected 'docs.example.com' in AllowedHosts, got %v", cfg.AllowedHosts())
	}
}

func TestAllowedHosts_WithExplicitHostsOverridesDefault(t *testing.T) {
	testURLs := []url.URL{
		{Scheme: "https", Host: "example.org"},
		{Scheme: "https", Host: "docs.example.com"},
	}

	explicitHosts := map[string]struct{}{
		"custom.com": {},
	}

	cfg, err := config.WithDefault(testURLs).WithAllowedHosts(explicitHosts).Build()
	if err != nil {
		t.Fatalf("should not have any error, got %v", err)
	}

	// Should only have the explicitly set allowed host
	if len(cfg.AllowedHosts()) != 1 {
		t.Errorf("expected 1 allowed host, got %d", len(cfg.AllowedHosts()))
	}
	if _, ok := cfg.AllowedHosts()["custom.com"]; !ok {
		t.Errorf("expected 'custom.com' in AllowedHosts, got %v", cfg.AllowedHosts())
	}
	// Should NOT have seed URL hosts
	if _, ok := cfg.AllowedHosts()["example.org"]; ok {
		t.Errorf("should not have 'example.org' in AllowedHosts when explicit hosts are set")
	}
}

func TestWithAllowedPathPrefix(t *testing.T) {
	testPrefixes := []string{"/docs", "/api", "/blog"}

	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithAllowedPathPrefix(testPrefixes).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if len(cfg.AllowedPathPrefix()) != 3 {
		t.Errorf("expected 3 path prefixes, got %d", len(cfg.AllowedPathPrefix()))
	}
	if cfg.AllowedPathPrefix()[0] != "/docs" || cfg.AllowedPathPrefix()[1] != "/api" || cfg.AllowedPathPrefix()[2] != "/blog" {
		t.Errorf("unexpected AllowedPathPrefix values: %v", cfg.AllowedPathPrefix())
	}
}

func TestWithMaxDepth(t *testing.T) {
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithMaxDepth(5).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.MaxDepth() != 5 {
		t.Errorf("expected MaxDepth 5, got %d", cfg.MaxDepth())
	}
}

func TestWithMaxPages(t *testing.T) {
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithMaxPages(500).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.MaxPages() != 500 {
		t.Errorf("expected MaxPages 500, got %d", cfg.MaxPages())
	}
}

func TestWithConcurrency(t *testing.T) {
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithConcurrency(20).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.Concurrency() != 20 {
		t.Errorf("expected Concurrency 20, got %d", cfg.Concurrency())
	}
}

func TestWithBaseDelay(t *testing.T) {
	testDelay := 2 * time.Second
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithBaseDelay(testDelay).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.BaseDelay() != testDelay {
		t.Errorf("expected BaseDelay %v, got %v", testDelay, cfg.BaseDelay())
	}
}

func TestWithJitter(t *testing.T) {
	testJitter := 1 * time.Second
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithJitter(testJitter).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.Jitter() != testJitter {
		t.Errorf("expected Jitter %v, got %v", testJitter, cfg.Jitter())
	}
}

func TestWithRandomSeed(t *testing.T) {
	testSeed := int64(12345)
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithRandomSeed(testSeed).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.RandomSeed() != testSeed {
		t.Errorf("expected RandomSeed %d, got %d", testSeed, cfg.RandomSeed())
	}
}

func TestWithMaxAttempt(t *testing.T) {
	testAttempts := 5
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithMaxAttempt(testAttempts).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.MaxAttempt() != testAttempts {
		t.Errorf("expected MaxAttempt %d, got %d", testAttempts, cfg.MaxAttempt())
	}
}

func TestWithBackoffInitialDuration(t *testing.T) {
	testDuration := 200 * time.Millisecond
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithBackoffInitialDuration(testDuration).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.BackoffInitialDuration() != testDuration {
		t.Errorf("expected BackoffInitialDuration %v, got %v", testDuration, cfg.BackoffInitialDuration())
	}
}

func TestWithBackoffMultiplier(t *testing.T) {
	testMultiplier := 1.5
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithBackoffMultiplier(testMultiplier).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.BackoffMultiplier() != testMultiplier {
		t.Errorf("expected BackoffMultiplier %f, got %f", testMultiplier, cfg.BackoffMultiplier())
	}
}

func TestWithBackoffMaxDuration(t *testing.T) {
	testDuration := 30 * time.Second
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithBackoffMaxDuration(testDuration).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.BackoffMaxDuration() != testDuration {
		t.Errorf("expected BackoffMaxDuration %v, got %v", testDuration, cfg.BackoffMaxDuration())
	}
}

func TestWithTimeout(t *testing.T) {
	testTimeout := 30 * time.Second
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithTimeout(testTimeout).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.Timeout() != testTimeout {
		t.Errorf("expected Timeout %v, got %v", testTimeout, cfg.Timeout())
	}
}

func TestWithUserAgent(t *testing.T) {
	testAgent := "CustomBot/2.0"
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithUserAgent(testAgent).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.UserAgent() != testAgent {
		t.Errorf("expected UserAgent '%s', got '%s'", testAgent, cfg.UserAgent())
	}
}

func TestWithOutputDir(t *testing.T) {
	testDir := "/custom/output/path"
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithOutputDir(testDir).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.OutputDir() != testDir {
		t.Errorf("expected OutputDir '%s', got '%s'", testDir, cfg.OutputDir())
	}
}

func TestWithDryRun(t *testing.T) {
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	cfg, err := config.WithDefault(baseURL).WithDryRun(true).Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if cfg.DryRun() != true {
		t.Errorf("expected DryRun true, got %v", cfg.DryRun())
	}
}

func TestBuild(t *testing.T) {
	baseURL := []url.URL{{Scheme: "https", Host: "base.org"}}
	original := config.WithDefault(baseURL)
	built, err := original.Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	// Verify Build returns the value, not pointer
	newBuilt, err := original.Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if newBuilt.SeedURLs()[0].String() != built.SeedURLs()[0].String() {
		t.Error("Build() did not return matching config")
	}

	// Modify built config should not affect original (value semantics)
	// Note: built is immutable now, so we can't modify it directly
	// We just verify the copy was made correctly
	newBuilt2, err := original.Build()
	if err != nil {
		t.Errorf("should not have any error, got %d", err)
	}
	if newBuilt2.MaxDepth() != 3 {
		t.Error("Build() appears to return reference, not value")
	}
}

func TestWithConfigFile_FileDoesNotExist(t *testing.T) {
	_, err := config.WithConfigFile("/nonexistent/path/config.json")

	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}

	if !errors.Is(err, config.ErrFileDoesNotExist) {
		t.Errorf("expected ErrFileDoesNotExist, got: %v", err)
	}
}

func TestWithConfigFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(configPath, []byte("{invalid json content}"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err = config.WithConfigFile(configPath)

	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	if !errors.Is(err, config.ErrConfigParsingFail) {
		t.Errorf("expected ErrConfigParsingFail, got: %v", err)
	}
}

func TestWithConfigFile_ValidCompleteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	err := os.WriteFile(configPath, []byte(completeConfigJson()), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	loadedConfig, err := config.WithConfigFile(configPath)

	if err != nil {
		t.Fatalf("unexpected error loading valid config: %v", err)
	}

	// Verify all values were loaded correctly
	if len(loadedConfig.SeedURLs()) != 2 ||
		loadedConfig.SeedURLs()[0].String() != "https://my-documentation.com/docs" ||
		loadedConfig.SeedURLs()[1].String() != "http://my-other-documentation.com/docs" {
		t.Errorf("unexpected SeedURLs: %v", loadedConfig.SeedURLs())
	}
	if loadedConfig.MaxDepth() != 5 {
		t.Errorf("expected MaxDepth 5, got %d", loadedConfig.MaxDepth())
	}
	if loadedConfig.MaxPages() != 200 {
		t.Errorf("expected MaxPages 200, got %d", loadedConfig.MaxPages())
	}
	if loadedConfig.Concurrency() != 20 {
		t.Errorf("expected Concurrency 20, got %d", loadedConfig.Concurrency())
	}
	if loadedConfig.UserAgent() != "TestBot/1.0" {
		t.Errorf("expected UserAgent 'TestBot/1.0', got '%s'", loadedConfig.UserAgent())
	}
	if loadedConfig.OutputDir() != "test_output" {
		t.Errorf("expected OutputDir 'test_output', got '%s'", loadedConfig.OutputDir())
	}
	if !loadedConfig.DryRun() {
		t.Errorf("expected DryRun true, got %v", loadedConfig.DryRun())
	}

	// Verify backoff and retry fields from complete config
	if loadedConfig.MaxAttempt() != 15 {
		t.Errorf("expected MaxAttempt 15, got %d", loadedConfig.MaxAttempt())
	}
	if loadedConfig.BackoffInitialDuration() != 200*time.Millisecond {
		t.Errorf("expected BackoffInitialDuration 200ms, got %v", loadedConfig.BackoffInitialDuration())
	}
	if loadedConfig.BackoffMultiplier() != 2.5 {
		t.Errorf("expected BackoffMultiplier 2.5, got %f", loadedConfig.BackoffMultiplier())
	}
	if loadedConfig.BackoffMaxDuration() != 20*time.Second {
		t.Errorf("expected BackoffMaxDuration 20s, got %v", loadedConfig.BackoffMaxDuration())
	}
}

func TestWithConfigFile_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.json")

	// Create a partial config - only override some fields (seedUrls is required)
	partialData := `{
		"seedUrls": [{"Scheme": "https", "Host": "partial-example.com"}],
		"maxDepth": 7,
		"userAgent": "PartialBot/1.0",
		"outputDir": "partial_output"
	}`

	err := os.WriteFile(configPath, []byte(partialData), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	loadedConfig, err := config.WithConfigFile(configPath)

	if err != nil {
		t.Fatalf("unexpected error loading partial config: %v", err)
	}

	// Verify overridden fields
	if loadedConfig.MaxDepth() != 7 {
		t.Errorf("expected MaxDepth 7, got %d", loadedConfig.MaxDepth())
	}
	if loadedConfig.UserAgent() != "PartialBot/1.0" {
		t.Errorf("expected UserAgent 'PartialBot/1.0', got '%s'", loadedConfig.UserAgent())
	}
	if loadedConfig.OutputDir() != "partial_output" {
		t.Errorf("expected OutputDir 'partial_output', got '%s'", loadedConfig.OutputDir())
	}
	if len(loadedConfig.SeedURLs()) != 1 || loadedConfig.SeedURLs()[0].String() != "https://partial-example.com" {
		t.Errorf("expected SeedURLs to be loaded from config, got %v", loadedConfig.SeedURLs())
	}

	// Verify default fields are preserved
	if loadedConfig.MaxPages() != 100 {
		t.Errorf("expected MaxPages to remain default 100, got %d", loadedConfig.MaxPages())
	}
	if loadedConfig.Concurrency() != 10 {
		t.Errorf("expected Concurrency to remain default 10, got %d", loadedConfig.Concurrency())
	}
}

func TestWithConfigFile_AllowedHostsDefaultsToSeedUrls(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "no_allowed_hosts.json")

	// Create a config without allowedHosts - should default to seed URLs
	configData := `{
		"seedUrls": [
			{"Scheme": "https", "Host": "docs.example.com"},
			{"Scheme": "https", "Host": "api.example.com"}
		],
		"maxDepth": 5
	}`

	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	loadedConfig, err := config.WithConfigFile(configPath)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	// Verify AllowedHosts defaults to seed URL hostnames
	if len(loadedConfig.AllowedHosts()) != 2 {
		t.Errorf("expected 2 allowed hosts, got %d", len(loadedConfig.AllowedHosts()))
	}
	if _, ok := loadedConfig.AllowedHosts()["docs.example.com"]; !ok {
		t.Errorf("expected 'docs.example.com' in AllowedHosts, got %v", loadedConfig.AllowedHosts())
	}
	if _, ok := loadedConfig.AllowedHosts()["api.example.com"]; !ok {
		t.Errorf("expected 'api.example.com' in AllowedHosts, got %v", loadedConfig.AllowedHosts())
	}
}

func TestWithConfigFile_PartialConfigNoSeedUrl(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.json")

	// Create a partial config - only override some fields (seedUrls is required)
	partialData := `{
		"maxDepth": 7,
		"userAgent": "PartialBot/1.0",
		"outputDir": "partial_output"
	}`

	err := os.WriteFile(configPath, []byte(partialData), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err = config.WithConfigFile(configPath)

	if err == nil {
		t.Fatalf("should error")
	}

	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig error, got: %v", err)
	}

}

func TestWithConfigFile_EmptyJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.json")

	err := os.WriteFile(configPath, []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err = config.WithConfigFile(configPath)

	// Empty JSON should return error because seedUrls is required
	if err == nil {
		t.Fatal("expected error for empty config without seedUrls, got nil")
	}

	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got: %v", err)
	}
}

// Note: Zero values in JSON with `omitempty` tags are omitted during marshaling,
// so they cannot override defaults. To set zero values, users must either:
// 1. Modify the Config struct after loading, or
// 2. Use a pointer type to distinguish between unset and zero values.

func completeConfigJson() string {
	return `
	{
    "seedUrls": [
        {
            "Scheme": "https",
            "Host": "my-documentation.com",
            "Path": "/docs"
        },
        {
            "Scheme": "http",
            "Host": "my-other-documentation.com",
            "Path": "/docs"
        }
    ],
    "allowedHosts": {
        "custom.com": {}
    },
    "allowedPathPrefix": [
        "/docs"
    ],
    "maxDepth": 5,
    "maxPages": 200,
    "concurrency": 20,
    "baseDelay": 2000000000,
    "jitter": 1000000000,
    "randomSeed": 42,
    "maxAttempt": 15,
    "backoffInitialDuration": 200000000,
    "backoffMultiplier": 2.5,
    "backoffMaxDuration": 20000000000,
    "timeout": 30000000000,
    "userAgent": "TestBot/1.0",
    "outputDir": "test_output",
    "dryRun": true
}
	`
}
