package robots

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/robots/cache"
)

/*
RobotsFetcher

Responsibilities:
- Fetch robots.txt per host using net/http
- Parse robots.txt content into structured format
- Map parsed response to ruleSet for decision making
- Handle HTTP errors and status codes according to spec
- Cache fetched results using the provided Cache implementation

The Fetcher returns a parsed RobotsResponse that can be mapped to ruleSet.
It does not make decisions about URL permissions.
*/

// RobotsFetcher fetches and parses robots.txt files from hosts.
type RobotsFetcher struct {
	httpClient *http.Client
	userAgent  string
	cache      cache.Cache
}

// RobotsFetchResult represents the result of fetching a robots.txt file.
type RobotsFetchResult struct {
	Response    RobotsResponse
	FetchedAt   time.Time
	SourceURL   string
	HTTPStatus  int
	ContentType string
}

// cachedResult is a serializable representation of RobotsFetchResult for cache storage.
type cachedResult struct {
	Response    RobotsResponse `json:"response"`
	FetchedAt   time.Time      `json:"fetched_at"`
	SourceURL   string         `json:"source_url"`
	HTTPStatus  int            `json:"http_status"`
	ContentType string         `json:"content_type"`
}

// NewRobotsFetcher creates a new RobotsFetcher with the given dependencies.
// The cache parameter is optional - if nil, no caching will be performed.
func NewRobotsFetcher(
	metadataSink metadata.MetadataSink,
	userAgent string,
	cache cache.Cache,
) *RobotsFetcher {
	return &RobotsFetcher{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  userAgent,
		cache:      cache,
	}
}

// NewRobotsFetcherWithClient creates a new RobotsFetcher with a custom HTTP client.
// This is useful for testing.
// The cache parameter is optional - if nil, no caching will be performed.
func NewRobotsFetcherWithClient(
	metadataSink metadata.MetadataSink,
	userAgent string,
	httpClient *http.Client,
	cache cache.Cache,
) *RobotsFetcher {
	return &RobotsFetcher{
		httpClient: httpClient,
		userAgent:  userAgent,
		cache:      cache,
	}
}

// cacheKey generates a cache key for the given scheme and hostname.
func cacheKey(scheme, hostname string) string {
	return fmt.Sprintf("%s://%s/robots.txt", scheme, hostname)
}

// serializeResult converts a RobotsFetchResult to a JSON string for cache storage.
func serializeResult(result RobotsFetchResult) (string, error) {
	cached := cachedResult{
		Response:    result.Response,
		FetchedAt:   result.FetchedAt,
		SourceURL:   result.SourceURL,
		HTTPStatus:  result.HTTPStatus,
		ContentType: result.ContentType,
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// deserializeResult converts a JSON string from cache to a RobotsFetchResult.
func deserializeResult(data string) (RobotsFetchResult, error) {
	var cached cachedResult
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		return RobotsFetchResult{}, err
	}
	return RobotsFetchResult{
		Response:    cached.Response,
		FetchedAt:   cached.FetchedAt,
		SourceURL:   cached.SourceURL,
		HTTPStatus:  cached.HTTPStatus,
		ContentType: cached.ContentType,
	}, nil
}

// Fetch retrieves the robots.txt file from the given host.
// The hostname should be in the form "example.com" or "example.com:8080".
// The scheme (http/https) must be provided to construct the URL.
// If a cache is configured, it will check the cache first and store results after fetching.
func (f *RobotsFetcher) Fetch(ctx context.Context, scheme, hostname string) (RobotsFetchResult, *RobotsError) {
	// Check cache first if available
	if f.cache != nil {
		key := cacheKey(scheme, hostname)
		if cachedData, found := f.cache.Get(key); found {
			if result, err := deserializeResult(cachedData); err == nil {
				return result, nil
			}
			// If deserialization fails, continue with fetch
		}
	}

	start := time.Now()
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", scheme, hostname)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return RobotsFetchResult{}, &RobotsError{
			Message:   fmt.Sprintf("failed to create request: %v", err),
			Retryable: false,
			Cause:     ErrCausePreFetchFailure,
		}
	}

	// Set browser-like headers
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/plain,text/html,*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := f.httpClient.Do(req)

	if err != nil {
		return RobotsFetchResult{}, &RobotsError{
			Message:   fmt.Sprintf("failed to fetch robots.txt: %v", err),
			Retryable: true,
			Cause:     ErrCauseHttpFetchFailure,
		}
	}
	defer resp.Body.Close()

	var result RobotsFetchResult
	var parsingError *RobotsError

	// Handle status codes according to spec
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Success - parse the robots.txt
		result, parsingError = f.parseSuccessfulResponse(resp, hostname, robotsURL)

	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		// Redirects should be followed by http.Client automatically
		// If we get here, it means there were too many redirects or a redirect loop
		return RobotsFetchResult{}, &RobotsError{
			Message:   fmt.Sprintf("redirect loop or too many redirects for %s", robotsURL),
			Retryable: true,
			Cause:     ErrCauseHttpTooManyRedirects,
		}

	case resp.StatusCode == 429:
		// Too Many Requests - treat as server error per spec
		return RobotsFetchResult{}, &RobotsError{
			Message:   fmt.Sprintf("rate limited (429) when fetching %s", robotsURL),
			Retryable: true,
			Cause:     ErrCauseHttpTooManyRequests,
		}

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// 4xx errors (except 429) mean no robots.txt exists
		// Return an empty response indicating no restrictions
		result = RobotsFetchResult{
			Response: RobotsResponse{
				Host:       hostname,
				Sitemaps:   []string{},
				UserAgents: []UserAgentGroup{},
			},
			FetchedAt:   start,
			SourceURL:   robotsURL,
			HTTPStatus:  resp.StatusCode,
			ContentType: resp.Header.Get("Content-Type"),
		}

	case resp.StatusCode >= 500:
		// 5xx errors are server errors - should retry
		return RobotsFetchResult{}, &RobotsError{
			Message:   fmt.Sprintf("server error (%d) when fetching %s", resp.StatusCode, robotsURL),
			Retryable: true,
			Cause:     ErrCauseHttpServerError,
		}

	default:
		return RobotsFetchResult{}, &RobotsError{
			Message:   fmt.Sprintf("unexpected status code %d for %s", resp.StatusCode, robotsURL),
			Retryable: true,
			Cause:     ErrCauseHttpUnexpectedStatus,
		}
	}

	if parsingError != nil {
		return RobotsFetchResult{}, parsingError
	}

	// Store successful result in cache
	if f.cache != nil {
		key := cacheKey(scheme, hostname)
		if cachedData, err := serializeResult(result); err == nil {
			f.cache.Put(key, cachedData)
		}
	}

	return result, nil
}

func (f *RobotsFetcher) parseSuccessfulResponse(resp *http.Response, hostname, sourceURL string) (RobotsFetchResult, *RobotsError) {
	// Limit reading to 500 KiB per spec
	const maxSize = 500 * 1024
	limitedReader := io.LimitReader(resp.Body, maxSize+1)

	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return RobotsFetchResult{}, &RobotsError{
			Message:   fmt.Sprintf("failed to read robots.txt body: %v", err),
			Retryable: true,
			Cause:     ErrCauseParseError,
		}
	}

	// Check if content exceeded max size
	if len(content) > maxSize {
		// Trim to max size as per spec
		content = content[:maxSize]
	}

	parsed := ParseRobotsTxt(string(content), hostname)

	return RobotsFetchResult{
		Response:    parsed,
		FetchedAt:   time.Now(),
		SourceURL:   sourceURL,
		HTTPStatus:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}

// ParseRobotsTxt parses robots.txt content into a structured format.
// This is exported for testing purposes.
func ParseRobotsTxt(content, hostname string) RobotsResponse {
	response := RobotsResponse{
		Host:       hostname,
		Sitemaps:   []string{},
		UserAgents: []UserAgentGroup{},
	}

	scanner := bufio.NewScanner(strings.NewReader(content))

	var currentGroup *UserAgentGroup
	var globalGroup UserAgentGroup // For rules without specific user-agent
	hasGlobalGroup := false

	for scanner.Scan() {
		line := scanner.Text()

		// Remove comments (everything after #)
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}

		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse field:value format
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue // Invalid line, skip
		}

		field := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		value := strings.TrimSpace(line[colonIdx+1:])

		switch field {
		case "user-agent":
			// Handle user-agent line
			if currentGroup == nil {
				// First user-agent in a new group
				currentGroup = &UserAgentGroup{
					UserAgents: []string{value},
					Allows:     []PathRule{},
					Disallows:  []PathRule{},
				}
			} else if len(currentGroup.Allows) == 0 && len(currentGroup.Disallows) == 0 && currentGroup.CrawlDelay == nil {
				// No rules yet, add to current group (multiple user-agents for same rules)
				currentGroup.UserAgents = append(currentGroup.UserAgents, value)
			} else {
				// Save the previous group and start a new one
				response.UserAgents = append(response.UserAgents, *currentGroup)
				currentGroup = &UserAgentGroup{
					UserAgents: []string{value},
					Allows:     []PathRule{},
					Disallows:  []PathRule{},
				}
			}

		case "allow":
			if currentGroup != nil {
				currentGroup.Allows = append(currentGroup.Allows, PathRule{
					Path: value,
				})
			} else {
				// Rule without user-agent - treat as global
				globalGroup.Allows = append(globalGroup.Allows, PathRule{Path: value})
				hasGlobalGroup = true
			}

		case "disallow":
			if currentGroup != nil {
				currentGroup.Disallows = append(currentGroup.Disallows, PathRule{
					Path: value,
				})
			} else {
				// Rule without user-agent - treat as global
				globalGroup.Disallows = append(globalGroup.Disallows, PathRule{Path: value})
				hasGlobalGroup = true
			}

		case "crawl-delay":
			if currentGroup != nil {
				// Parse as seconds (integer or float)
				var seconds float64
				if _, err := fmt.Sscanf(value, "%f", &seconds); err == nil && seconds >= 0 {
					delay := time.Duration(seconds * float64(time.Second))
					currentGroup.CrawlDelay = &delay
				}
			}

		case "sitemap":
			// Sitemaps are global and not tied to any user-agent
			if value != "" {
				response.Sitemaps = append(response.Sitemaps, value)
			}
		}
	}

	// Don't forget the last group
	if currentGroup != nil {
		if len(currentGroup.Allows) > 0 || len(currentGroup.Disallows) > 0 || currentGroup.CrawlDelay != nil || len(currentGroup.UserAgents) > 0 {
			response.UserAgents = append(response.UserAgents, *currentGroup)
		}
	}

	// Add global group if it has content
	if hasGlobalGroup && (len(globalGroup.Allows) > 0 || len(globalGroup.Disallows) > 0) {
		globalGroup.UserAgents = []string{"*"}
		response.UserAgents = append([]UserAgentGroup{globalGroup}, response.UserAgents...)
	}

	return response
}

func (f *RobotsFetcher) UserAgent() string {
	return f.userAgent
}

func (f *RobotsFetcher) HttpClient() *http.Client {
	return f.httpClient
}

func (f *RobotsFetcher) Cache() cache.Cache {
	return f.cache
}
