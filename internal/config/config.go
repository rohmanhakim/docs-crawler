package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"
)

type Config struct {
	//===============
	//  Crawl scope
	//===============
	// Initial pages to give to the crawler to begin discovering and traversing other pages.
	seedURLs []url.URL
	// Whitelisted hostname. Empty means all hostnames are allowed
	allowedHosts map[string]struct{}
	// Which URL path segments are permitted to be fetched and traversed, even if the links are on the same domain
	allowedPathPrefix []string

	//===============
	// Limits
	//===============
	// Maximum number of hyperlink hops from a seed (root) URL
	maxDepth int
	// Maximum number of total documents are allowed to be fetched
	maxPages int

	//===============
	// Politeness
	//===============
	// Maximum number of crawl worker goroutines processing URLs concurrently;
	// it does not control OS threads or CPU parallelism.
	concurrency int
	// Minimum, fixed waiting time you enforce between two HTTP requests to the same host.
	baseDelay time.Duration
	// Randomized variation added on top of the base delay.
	// Intentional randomness applied to timing.
	jitter time.Duration
	// Controls the random number generator
	randomSeed int64

	//===============
	// Fetch
	//===============
	// Maximum time of a single fetch request in millisecond
	timeout time.Duration
	// User agent that will be used in the request header. In raw string
	userAgent string

	//===============
	// Output
	//===============
	// Root directory in which to store the resulting markdown files
	outputDir string
	// Whether the program will simulates what it would do without
	// actually performing any irreversible or side-effecting actions
	dryRun bool
}

type configDTO struct {
	SeedURLs          []url.URL           `json:"seedUrls"`
	AllowedHosts      map[string]struct{} `json:"allowedHosts,omitempty"`
	AllowedPathPrefix []string            `json:"allowedPathPrefix,omitempty"`
	MaxDepth          int                 `json:"maxDepth,omitempty"`
	MaxPages          int                 `json:"maxPages,omitempty"`
	Concurrency       int                 `json:"concurrency,omitempty"`
	BaseDelay         time.Duration       `json:"baseDelay,omitempty"`
	Jitter            time.Duration       `json:"jitter,omitempty"`
	RandomSeed        int64               `json:"randomSeed,omitempty"`
	Timeout           time.Duration       `json:"timeout,omitempty"`
	UserAgent         string              `json:"userAgent,omitempty"`
	OutputDir         string              `json:"outputDir,omitempty"`
	DryRun            bool                `json:"dryRun,omitempty"`
}

func newConfigFromDTO(dto configDTO) (Config, error) {

	// Start with default config
	cfg, err := WithDefault(dto.SeedURLs).Build()
	if err != nil {
		return Config{}, err
	}

	// AllowedHosts can be empty - if so, default to seed URLs hostnames
	if len(dto.AllowedHosts) > 0 {
		cfg.allowedHosts = dto.AllowedHosts
	}

	// AllowedPathPrefix can be empty - always use DTO values
	cfg.allowedPathPrefix = dto.AllowedPathPrefix

	// For other fields, only override if non-zero value is provided
	if dto.MaxDepth != 0 {
		cfg.maxDepth = dto.MaxDepth
	}
	if dto.MaxPages != 0 {
		cfg.maxPages = dto.MaxPages
	}
	if dto.Concurrency != 0 {
		cfg.concurrency = dto.Concurrency
	}
	if dto.BaseDelay != 0 {
		cfg.baseDelay = dto.BaseDelay
	}
	if dto.Jitter != 0 {
		cfg.jitter = dto.Jitter
	}
	if dto.RandomSeed != 0 {
		cfg.randomSeed = dto.RandomSeed
	}
	if dto.Timeout != 0 {
		cfg.timeout = dto.Timeout
	}
	if dto.UserAgent != "" {
		cfg.userAgent = dto.UserAgent
	}
	if dto.OutputDir != "" {
		cfg.outputDir = dto.OutputDir
	}
	// DryRun is a boolean, check if explicitly set (we use the DTO value as-is since bool zero value is false)
	cfg.dryRun = dto.DryRun

	return cfg, nil
}

func WithConfigFile(path string) (Config, error) {
	_, err := os.Stat(path)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %s", ErrFileDoesNotExist, err.Error())
	}
	configContent, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %s", ErrReadConfigFail, err.Error())
	}
	cfgDTO := configDTO{}

	err = json.Unmarshal(configContent, &cfgDTO)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %s", ErrConfigParsingFail, err.Error())
	}

	cfg, err := newConfigFromDTO(cfgDTO)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// WithDefault creates a new Config with the provided seed URLs and default values for all other fields.
// seedUrls is mandatory and must not be empty - an error will be returned if it is.
func WithDefault(seedUrls []url.URL) *Config {
	defaultConfig := Config{
		seedURLs:     seedUrls,
		allowedHosts: map[string]struct{}{},
		allowedPathPrefix: []string{
			"/",
		},
		maxDepth:    3,
		maxPages:    100,
		concurrency: 10,
		baseDelay:   time.Second,
		jitter:      time.Millisecond * 500,
		randomSeed:  time.Now().UnixNano(),
		timeout:     time.Second * 10,
		userAgent:   "docs-crawler/1.0",
		outputDir:   "output",
		dryRun:      false,
	}
	return &defaultConfig
}

func (c *Config) WithSeedUrls(urls []url.URL) *Config {
	c.seedURLs = urls
	return c
}

func (c *Config) WithAllowedHosts(hosts map[string]struct{}) *Config {
	c.allowedHosts = hosts
	return c
}

func (c *Config) WithAllowedPathPrefix(prefixes []string) *Config {
	c.allowedPathPrefix = prefixes
	return c
}

func (c *Config) WithMaxDepth(depth int) *Config {
	c.maxDepth = depth
	return c
}

func (c *Config) WithMaxPages(pages int) *Config {
	c.maxPages = pages
	return c
}

func (c *Config) WithConcurrency(concurrency int) *Config {
	c.concurrency = concurrency
	return c
}

func (c *Config) WithBaseDelay(delay time.Duration) *Config {
	c.baseDelay = delay
	return c
}

func (c *Config) WithJitter(jitter time.Duration) *Config {
	c.jitter = jitter
	return c
}

func (c *Config) WithRandomSeed(seed int64) *Config {
	c.randomSeed = seed
	return c
}

func (c *Config) WithTimeout(timeout time.Duration) *Config {
	c.timeout = timeout
	return c
}

func (c *Config) WithUserAgent(agent string) *Config {
	c.userAgent = agent
	return c
}

func (c *Config) WithOutputDir(outputDir string) *Config {
	c.outputDir = outputDir
	return c
}

func (c *Config) WithDryRun(dryRun bool) *Config {
	c.dryRun = dryRun
	return c
}

func (c *Config) Build() (Config, error) {
	if len(c.seedURLs) == 0 {
		return Config{}, fmt.Errorf("%w: seedUrls cannot be empty", ErrInvalidConfig)
	}

	// If allowedHosts is empty, default to seed URLs hostnames
	if len(c.allowedHosts) == 0 {
		c.allowedHosts = make(map[string]struct{})
		for _, u := range c.seedURLs {
			if u.Host != "" {
				c.allowedHosts[u.Host] = struct{}{}
			}
		}
	}

	return *c, nil
}

func (c Config) SeedURLs() []url.URL {
	urls := make([]url.URL, len(c.seedURLs))
	copy(urls, c.seedURLs)
	return urls
}

func (c Config) AllowedHosts() map[string]struct{} {
	hosts := make(map[string]struct{})
	for k, v := range c.allowedHosts {
		hosts[k] = v
	}
	return hosts
}

func (c Config) AllowedPathPrefix() []string {
	prefixes := make([]string, len(c.allowedPathPrefix))
	copy(prefixes, c.allowedPathPrefix)
	return prefixes
}

func (c Config) MaxDepth() int {
	return c.maxDepth
}

func (c Config) MaxPages() int {
	return c.maxPages
}

func (c Config) Concurrency() int {
	return c.concurrency
}

func (c Config) BaseDelay() time.Duration {
	return c.baseDelay
}

func (c Config) Jitter() time.Duration {
	return c.jitter
}

func (c Config) RandomSeed() int64 {
	return c.randomSeed
}

func (c Config) Timeout() time.Duration {
	return c.timeout
}

func (c Config) UserAgent() string {
	return c.userAgent
}

func (c Config) OutputDir() string {
	return c.outputDir
}

func (c Config) DryRun() bool {
	return c.dryRun
}
