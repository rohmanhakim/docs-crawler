package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile           string
	seedURLs          []string
	maxDepth          int
	concurrency       int
	outputDir         string
	dryRun            bool
	maxPages          int
	userAgent         string
	timeout           time.Duration
	baseDelay         time.Duration
	jitter            time.Duration
	randomSeed        int64
	allowedHosts      []string
	allowedPathPrefix []string
)

// parseStringSliceToSet converts a string slice to a map[string]struct{} set
func parseStringSliceToSet(strings []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, s := range strings {
		if s != "" {
			set[s] = struct{}{}
		}
	}
	return set
}

// parseSeedURLs converts a string slice of URLs to []url.URL
func parseSeedURLs(urlStrings []string) ([]url.URL, error) {
	if len(urlStrings) == 0 {
		return nil, fmt.Errorf("seed URLs cannot be empty")
	}

	var urls []url.URL
	for _, urlStr := range urlStrings {
		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing seed URL %s: %w", urlStr, err)
		}
		urls = append(urls, *parsedURL)
	}
	return urls, nil
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "docs-crawler",
	Short: "A local-only documentation crawler.",
	Long: `docs-crawler is a CLI application that crawls static documentation
websites and converts their content into clean, semantically faithful Markdown,
optimized for LLM Retrieval-Augmented Generation (RAG) workflows.

This tool aims to provide a deterministic and repeatable crawl process,
producing high-quality Markdown suitable for embedding and retrieval.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if seed URLs are provided
		if len(seedURLs) == 0 {
			fmt.Fprintf(os.Stderr, "Error: --seed-url is required. Please provide at least one seed URL to start crawling.\n")
			cmd.Usage()
			os.Exit(1)
		}

		// Parse seed URLs
		parsedURLs, err := parseSeedURLs(seedURLs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

		// Build config using initConfig with parsed seed URLs
		cfg := InitConfig(parsedURLs)

		// Display configuration for verification
		fmt.Printf("Configuration initialized successfully\n")
		if len(cfg.SeedURLs()) > 0 {
			var urls []string
			for _, u := range cfg.SeedURLs() {
				urls = append(urls, u.String())
			}
			fmt.Printf("Seed URLs: %s\n", strings.Join(urls, ", "))
		}
		if len(cfg.AllowedHosts()) > 0 {
			var hosts []string
			for host := range cfg.AllowedHosts() {
				hosts = append(hosts, host)
			}
			fmt.Printf("Allowed Hosts: %s\n", strings.Join(hosts, ", "))
		}
		if len(cfg.AllowedPathPrefix()) > 0 {
			fmt.Printf("Allowed Path Prefixes: %s\n", strings.Join(cfg.AllowedPathPrefix(), ", "))
		}
		fmt.Printf("Max Depth: %d\n", cfg.MaxDepth())
		fmt.Printf("Max Pages: %d\n", cfg.MaxPages())
		fmt.Printf("Concurrency: %d\n", cfg.Concurrency())
		fmt.Printf("Base Delay: %v\n", cfg.BaseDelay())
		fmt.Printf("Jitter: %v\n", cfg.Jitter())
		fmt.Printf("Random Seed: %d\n", cfg.RandomSeed())
		fmt.Printf("Timeout: %v\n", cfg.Timeout())
		fmt.Printf("User Agent: %s\n", cfg.UserAgent())
		fmt.Printf("Output Directory: %s\n", cfg.OutputDir())
		fmt.Printf("Dry Run: %t\n", cfg.DryRun())
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be available to all subcommands in the docs-crawler application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config-file", "", "config file path (e.g., /home/myuser/config.json)")
	rootCmd.PersistentFlags().StringArrayVar(&seedURLs, "seed-url", []string{}, "one or more starting URLs (can be repeated)")
	rootCmd.PersistentFlags().IntVar(&maxDepth, "max-depth", 5, "maximum link depth from seed URL")
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", 3, "number of concurrent fetch workers")
	rootCmd.PersistentFlags().StringVar(&outputDir, "output-dir", "output", "root output directory for crawled content")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "crawl without writing output")
	rootCmd.PersistentFlags().IntVar(&maxPages, "max-pages", 0, "maximum number of pages to fetch (0 for unlimited)")
	rootCmd.PersistentFlags().StringVar(&userAgent, "user-agent", "", "user agent string for HTTP requests")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 0, "timeout for HTTP requests")
	rootCmd.PersistentFlags().DurationVar(&baseDelay, "base-delay", 0, "base delay between HTTP requests to the same host")
	rootCmd.PersistentFlags().DurationVar(&jitter, "jitter", 0, "random jitter added to base delay")
	rootCmd.PersistentFlags().Int64Var(&randomSeed, "random-seed", 0, "seed for random number generation (0 for current time)")
	rootCmd.PersistentFlags().StringArrayVar(&allowedHosts, "allowed-host", []string{}, "explicit hostname allowlist (defaults to seed host)")
	rootCmd.PersistentFlags().StringArrayVar(&allowedPathPrefix, "allowed-path-prefix", []string{}, "restrict crawl to paths like `/docs`, `/guide`")
}

// InitConfig reads in config file and ENV variables if set.
// seedUrls is a mandatory parameter and must contain at least one valid URL.
func InitConfig(seedUrls []url.URL) config.Config {
	cfg, err := InitConfigWithError(seedUrls)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	return cfg
}

// InitConfigWithError reads in config file and ENV variables if set, returning any errors.
// seedUrls is a mandatory parameter and must contain at least one valid URL.
// This makes it easier to test error cases.
func InitConfigWithError(seedUrls []url.URL) (config.Config, error) {
	if len(seedUrls) == 0 {
		return config.Config{}, fmt.Errorf("%w: seedUrls cannot be empty", config.ErrInvalidConfig)
	}

	if cfgFile != "" {
		fmt.Printf("Initializing config from file: %s\n", cfgFile)
		cfg, err := config.WithConfigFile(cfgFile)
		if err != nil {
			return cfg, fmt.Errorf("error initializing config from file: %w", err)
		}
		return cfg, nil
	}

	// Build config from CLI flags using the With... functions with method chaining
	fmt.Println("No config file specified. Using default flag values or environment variables")

	// Start with default config using provided seed URLs and apply overrides using method chaining
	configBuilder := config.WithDefault(seedUrls)

	// Override with CLI flag values where provided
	if maxDepth > 0 {
		configBuilder = configBuilder.WithMaxDepth(maxDepth)
	}

	if concurrency > 0 {
		configBuilder = configBuilder.WithConcurrency(concurrency)
	}

	if outputDir != "" && outputDir != "output" {
		configBuilder = configBuilder.WithOutputDir(outputDir)
	}

	if dryRun {
		configBuilder = configBuilder.WithDryRun(dryRun)
	}

	if maxPages > 0 {
		configBuilder = configBuilder.WithMaxPages(maxPages)
	}

	if userAgent != "" {
		configBuilder = configBuilder.WithUserAgent(userAgent)
	}

	if timeout > 0 {
		configBuilder = configBuilder.WithTimeout(timeout)
	}

	if baseDelay > 0 {
		configBuilder = configBuilder.WithBaseDelay(baseDelay)
	}

	if jitter > 0 {
		configBuilder = configBuilder.WithJitter(jitter)
	}

	if randomSeed != 0 {
		configBuilder = configBuilder.WithRandomSeed(randomSeed)
	}

	if len(allowedHosts) > 0 {
		configBuilder = configBuilder.WithAllowedHosts(parseStringSliceToSet(allowedHosts))
	}

	if len(allowedPathPrefix) > 0 {
		configBuilder = configBuilder.WithAllowedPathPrefix(allowedPathPrefix)
	}

	cfg, err := configBuilder.Build()
	if err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func ResetFlags() {
	cfgFile = ""
	seedURLs = []string{}
	maxDepth = 0
	concurrency = 0
	outputDir = ""
	dryRun = false
	maxPages = 0
	userAgent = ""
	timeout = 0
	baseDelay = 0
	jitter = 0
	randomSeed = 0
	allowedHosts = []string{}
	allowedPathPrefix = []string{}
}

// Test helper functions to set flag values from tests
func SetConfigFileForTest(path string) {
	cfgFile = path
}

func SetSeedURLsForTest(urls []string) {
	seedURLs = urls
}

func SetMaxDepthForTest(depth int) {
	maxDepth = depth
}

func SetConcurrencyForTest(conc int) {
	concurrency = conc
}

func SetOutputDirForTest(dir string) {
	outputDir = dir
}

func SetDryRunForTest(dry bool) {
	dryRun = dry
}

func SetMaxPagesForTest(pages int) {
	maxPages = pages
}

func SetUserAgentForTest(agent string) {
	userAgent = agent
}

func SetTimeoutForTest(t time.Duration) {
	timeout = t
}

func SetBaseDelayForTest(delay time.Duration) {
	baseDelay = delay
}

func SetJitterForTest(j time.Duration) {
	jitter = j
}

func SetRandomSeedForTest(seed int64) {
	randomSeed = seed
}

func SetAllowedHostsForTest(hosts []string) {
	allowedHosts = hosts
}

func SetAllowedPathPrefixForTest(prefixes []string) {
	allowedPathPrefix = prefixes
}
