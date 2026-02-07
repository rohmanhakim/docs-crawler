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
	// maximum attempt during retry
	maxAttempt int
	// initial delay for backoff
	backoffInitialDuration time.Duration
	// multiplier during exponential backoff
	backoffMultiplier float64
	// capped maximum delay for backoff to stop exponential multiplication
	backoffMaxDuration time.Duration

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

	//===============
	// Extraction
	//===============
	// BodySpecificityBias is the threshold for preferring a child container over <body>.
	// If a child node's score is >= BodySpecificityBias * bodyScore, the child is preferred.
	// Default: 0.75 (75%)
	bodySpecificityBias float64
	// LinkDensityThreshold is the maximum ratio of link text to total text before
	// applying a penalty. Higher values allow more link-heavy content.
	// Default: 0.80 (80%)
	linkDensityThreshold float64
	// ScoreMultiplierNonWhitespaceDivisor is the divisor for calculating text score.
	// Score gets +1 point per NonWhitespaceDivisor characters.
	// Default: 50.0
	scoreMultiplierNonWhitespaceDivisor float64
	// ScoreMultiplierParagraphs is the score multiplier for each paragraph element.
	// Default: 5.0
	scoreMultiplierParagraphs float64
	// ScoreMultiplierHeadings is the score multiplier for each heading element (h1-h3).
	// Default: 10.0
	scoreMultiplierHeadings float64
	// ScoreMultiplierCodeBlocks is the score multiplier for each code block.
	// Default: 15.0
	scoreMultiplierCodeBlocks float64
	// ScoreMultiplierListItems is the score multiplier for each list item.
	// Default: 2.0
	scoreMultiplierListItems float64
	// ThresholdMinNonWhitespace is the minimum number of non-whitespace characters
	// required for content to be considered meaningful.
	// Default: 50
	thresholdMinNonWhitespace int
	// ThresholdMinHeadings is the minimum number of headings required.
	// Headings are optional but valuable.
	// Default: 0
	thresholdMinHeadings int
	// ThresholdMinParagraphsOrCode is the minimum number of paragraphs OR code blocks
	// required for content to be considered meaningful.
	// Default: 1
	thresholdMinParagraphsOrCode int
	// ThresholdMaxLinkDensity is the maximum ratio of link text to total text before
	// content is considered navigation-only and rejected.
	// Default: 0.8 (80%)
	thresholdMaxLinkDensity float64
}

type configDTO struct {
	SeedURLs               []url.URL           `json:"seedUrls"`
	AllowedHosts           map[string]struct{} `json:"allowedHosts,omitempty"`
	AllowedPathPrefix      []string            `json:"allowedPathPrefix,omitempty"`
	MaxDepth               int                 `json:"maxDepth,omitempty"`
	MaxPages               int                 `json:"maxPages,omitempty"`
	Concurrency            int                 `json:"concurrency,omitempty"`
	BaseDelay              time.Duration       `json:"baseDelay,omitempty"`
	Jitter                 time.Duration       `json:"jitter,omitempty"`
	RandomSeed             int64               `json:"randomSeed,omitempty"`
	MaxAttempt             int                 `json:"maxAttempt,omitempty"`
	BackoffInitialDuration time.Duration       `json:"backoffInitialDuration,omitempty"`
	BackoffMultiplier      float64             `json:"backoffMultiplier,omitempty"`
	BackoffMaxDuration     time.Duration       `json:"backoffMaxDuration,omitempty"`
	Timeout                time.Duration       `json:"timeout,omitempty"`
	UserAgent              string              `json:"userAgent,omitempty"`
	OutputDir              string              `json:"outputDir,omitempty"`
	DryRun                 bool                `json:"dryRun,omitempty"`
	// Extraction parameters
	BodySpecificityBias                 float64 `json:"bodySpecificityBias,omitempty"`
	LinkDensityThreshold                float64 `json:"linkDensityThreshold,omitempty"`
	ScoreMultiplierNonWhitespaceDivisor float64 `json:"scoreMultiplierNonWhitespaceDivisor,omitempty"`
	ScoreMultiplierParagraphs           float64 `json:"scoreMultiplierParagraphs,omitempty"`
	ScoreMultiplierHeadings             float64 `json:"scoreMultiplierHeadings,omitempty"`
	ScoreMultiplierCodeBlocks           float64 `json:"scoreMultiplierCodeBlocks,omitempty"`
	ScoreMultiplierListItems            float64 `json:"scoreMultiplierListItems,omitempty"`
	ThresholdMinNonWhitespace           int     `json:"thresholdMinNonWhitespace,omitempty"`
	ThresholdMinHeadings                int     `json:"thresholdMinHeadings,omitempty"`
	ThresholdMinParagraphsOrCode        int     `json:"thresholdMinParagraphsOrCode,omitempty"`
	ThresholdMaxLinkDensity             float64 `json:"thresholdMaxLinkDensity,omitempty"`
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
	if dto.MaxAttempt != 0 {
		cfg.maxAttempt = dto.MaxAttempt
	}
	if dto.BackoffInitialDuration != 0 {
		cfg.backoffInitialDuration = dto.BackoffInitialDuration
	}
	if dto.BackoffMultiplier != 0 {
		cfg.backoffMultiplier = dto.BackoffMultiplier
	}
	if dto.BackoffMaxDuration != 0 {
		cfg.backoffMaxDuration = dto.BackoffMaxDuration
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

	// Extraction parameters - only override if non-zero value is provided
	// For float64, we check if value is not 0 (which is also the zero value)
	if dto.BodySpecificityBias != 0 {
		cfg.bodySpecificityBias = dto.BodySpecificityBias
	}
	if dto.LinkDensityThreshold != 0 {
		cfg.linkDensityThreshold = dto.LinkDensityThreshold
	}
	if dto.ScoreMultiplierNonWhitespaceDivisor != 0 {
		cfg.scoreMultiplierNonWhitespaceDivisor = dto.ScoreMultiplierNonWhitespaceDivisor
	}
	if dto.ScoreMultiplierParagraphs != 0 {
		cfg.scoreMultiplierParagraphs = dto.ScoreMultiplierParagraphs
	}
	if dto.ScoreMultiplierHeadings != 0 {
		cfg.scoreMultiplierHeadings = dto.ScoreMultiplierHeadings
	}
	if dto.ScoreMultiplierCodeBlocks != 0 {
		cfg.scoreMultiplierCodeBlocks = dto.ScoreMultiplierCodeBlocks
	}
	if dto.ScoreMultiplierListItems != 0 {
		cfg.scoreMultiplierListItems = dto.ScoreMultiplierListItems
	}
	if dto.ThresholdMinNonWhitespace != 0 {
		cfg.thresholdMinNonWhitespace = dto.ThresholdMinNonWhitespace
	}
	// Note: ThresholdMinHeadings can be 0 (which is a valid value), so we don't check for non-zero
	cfg.thresholdMinHeadings = dto.ThresholdMinHeadings
	if dto.ThresholdMinParagraphsOrCode != 0 {
		cfg.thresholdMinParagraphsOrCode = dto.ThresholdMinParagraphsOrCode
	}
	if dto.ThresholdMaxLinkDensity != 0 {
		cfg.thresholdMaxLinkDensity = dto.ThresholdMaxLinkDensity
	}

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
		maxDepth:               3,
		maxPages:               100,
		concurrency:            10,
		baseDelay:              time.Second,
		jitter:                 time.Millisecond * 500,
		randomSeed:             time.Now().UnixNano(),
		maxAttempt:             10,
		backoffInitialDuration: 100 * time.Millisecond,
		backoffMultiplier:      2.0,
		backoffMaxDuration:     10 * time.Second,
		timeout:                time.Second * 10,
		userAgent:              "docs-crawler/1.0",
		outputDir:              "output",
		dryRun:                 false,
		// Extraction defaults
		bodySpecificityBias:                 0.75,
		linkDensityThreshold:                0.80,
		scoreMultiplierNonWhitespaceDivisor: 50.0,
		scoreMultiplierParagraphs:           5.0,
		scoreMultiplierHeadings:             10.0,
		scoreMultiplierCodeBlocks:           15.0,
		scoreMultiplierListItems:            2.0,
		thresholdMinNonWhitespace:           50,
		thresholdMinHeadings:                0,
		thresholdMinParagraphsOrCode:        1,
		thresholdMaxLinkDensity:             0.8,
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

func (c *Config) WithMaxAttempt(attempts int) *Config {
	c.maxAttempt = attempts
	return c
}

func (c *Config) WithBackoffInitialDuration(duration time.Duration) *Config {
	c.backoffInitialDuration = duration
	return c
}

func (c *Config) WithBackoffMultiplier(multiplier float64) *Config {
	c.backoffMultiplier = multiplier
	return c
}

func (c *Config) WithBackoffMaxDuration(duration time.Duration) *Config {
	c.backoffMaxDuration = duration
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

func (c *Config) WithBodySpecificityBias(bias float64) *Config {
	c.bodySpecificityBias = bias
	return c
}

func (c *Config) WithLinkDensityThreshold(threshold float64) *Config {
	c.linkDensityThreshold = threshold
	return c
}

func (c *Config) WithScoreMultiplierNonWhitespaceDivisor(divisor float64) *Config {
	c.scoreMultiplierNonWhitespaceDivisor = divisor
	return c
}

func (c *Config) WithScoreMultiplierParagraphs(multiplier float64) *Config {
	c.scoreMultiplierParagraphs = multiplier
	return c
}

func (c *Config) WithScoreMultiplierHeadings(multiplier float64) *Config {
	c.scoreMultiplierHeadings = multiplier
	return c
}

func (c *Config) WithScoreMultiplierCodeBlocks(multiplier float64) *Config {
	c.scoreMultiplierCodeBlocks = multiplier
	return c
}

func (c *Config) WithScoreMultiplierListItems(multiplier float64) *Config {
	c.scoreMultiplierListItems = multiplier
	return c
}

func (c *Config) WithThresholdMinNonWhitespace(min int) *Config {
	c.thresholdMinNonWhitespace = min
	return c
}

func (c *Config) WithThresholdMinHeadings(min int) *Config {
	c.thresholdMinHeadings = min
	return c
}

func (c *Config) WithThresholdMinParagraphsOrCode(min int) *Config {
	c.thresholdMinParagraphsOrCode = min
	return c
}

func (c *Config) WithThresholdMaxLinkDensity(max float64) *Config {
	c.thresholdMaxLinkDensity = max
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

func (c Config) MaxAttempt() int {
	return c.maxAttempt
}

func (c Config) BackoffInitialDuration() time.Duration {
	return c.backoffInitialDuration
}

func (c Config) BackoffMultiplier() float64 {
	return c.backoffMultiplier
}

func (c Config) BackoffMaxDuration() time.Duration {
	return c.backoffMaxDuration
}

func (c Config) BodySpecificityBias() float64 {
	return c.bodySpecificityBias
}

func (c Config) LinkDensityThreshold() float64 {
	return c.linkDensityThreshold
}

func (c Config) ScoreMultiplierNonWhitespaceDivisor() float64 {
	return c.scoreMultiplierNonWhitespaceDivisor
}

func (c Config) ScoreMultiplierParagraphs() float64 {
	return c.scoreMultiplierParagraphs
}

func (c Config) ScoreMultiplierHeadings() float64 {
	return c.scoreMultiplierHeadings
}

func (c Config) ScoreMultiplierCodeBlocks() float64 {
	return c.scoreMultiplierCodeBlocks
}

func (c Config) ScoreMultiplierListItems() float64 {
	return c.scoreMultiplierListItems
}

func (c Config) ThresholdMinNonWhitespace() int {
	return c.thresholdMinNonWhitespace
}

func (c Config) ThresholdMinHeadings() int {
	return c.thresholdMinHeadings
}

func (c Config) ThresholdMinParagraphsOrCode() int {
	return c.thresholdMinParagraphsOrCode
}

func (c Config) ThresholdMaxLinkDensity() float64 {
	return c.thresholdMaxLinkDensity
}
