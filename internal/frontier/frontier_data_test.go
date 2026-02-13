package frontier_test

import (
	"testing"
	"time"

	"net/url"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
)

func TestNewCrawlToken(t *testing.T) {
	tests := []struct {
		name  string
		u     url.URL
		depth int
	}{
		{
			name:  "simple http url with depth 0",
			u:     url.URL{Scheme: "http", Host: "example.com", Path: "/"},
			depth: 0,
		},
		{
			name:  "https url with positive depth",
			u:     url.URL{Scheme: "https", Host: "example.com", Path: "/page"},
			depth: 2,
		},
		{
			name:  "url with query parameters",
			u:     url.URL{Scheme: "http", Host: "example.com", Path: "/search", RawQuery: "q=test"},
			depth: 1,
		},
		{
			name:  "url with large depth",
			u:     url.URL{Scheme: "https", Host: "deep.example.com", Path: "/a/b/c/d/e"},
			depth: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := frontier.NewCrawlToken(tt.u, tt.depth)

			if token.URL() != tt.u {
				t.Errorf("URL() = %v, want %v", token.URL(), tt.u)
			}

			if token.Depth() != tt.depth {
				t.Errorf("Depth() = %v, want %v", token.Depth(), tt.depth)
			}
		})
	}
}

func TestCrawlAdmissionCandidate_TargetURL(t *testing.T) {
	tests := []struct {
		name string
		u    url.URL
	}{
		{
			name: "simple http url",
			u:    url.URL{Scheme: "http", Host: "example.com", Path: "/"},
		},
		{
			name: "https url with path",
			u:    url.URL{Scheme: "https", Host: "example.com", Path: "/page"},
		},
		{
			name: "url with query and fragment",
			u:    url.URL{Scheme: "http", Host: "example.com", Path: "/search", RawQuery: "q=test", Fragment: "section"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := frontier.NewCrawlAdmissionCandidate(
				tt.u,
				frontier.SourceSeed,
				frontier.NewDiscoveryMetadata(0, nil),
			)

			if candidate.TargetURL() != tt.u {
				t.Errorf("TargetURL() = %v, want %v", candidate.TargetURL(), tt.u)
			}
		})
	}
}

func TestCrawlAdmissionCandidate_SourceContext(t *testing.T) {
	tests := []struct {
		name          string
		sourceContext frontier.SourceContext
	}{
		{
			name:          "seed source",
			sourceContext: frontier.SourceSeed,
		},
		{
			name:          "crawl source",
			sourceContext: frontier.SourceCrawl,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := frontier.NewCrawlAdmissionCandidate(
				url.URL{Scheme: "http", Host: "example.com"},
				tt.sourceContext,
				frontier.NewDiscoveryMetadata(0, nil),
			)

			if candidate.SourceContext() != tt.sourceContext {
				t.Errorf("SourceContext() = %v, want %v", candidate.SourceContext(), tt.sourceContext)
			}
		})
	}
}

func TestCrawlAdmissionCandidate_DiscoveryMetadata(t *testing.T) {
	tests := []struct {
		name          string
		depth         int
		delayOverride *time.Duration
		expectedDepth int
		expectedDelay *time.Duration
	}{
		{
			name:          "zero depth with nil delay override",
			depth:         0,
			delayOverride: nil,
			expectedDepth: 0,
			expectedDelay: nil,
		},
		{
			name:          "positive depth with nil delay override",
			depth:         2,
			delayOverride: nil,
			expectedDepth: 2,
			expectedDelay: nil,
		},
		{
			name:          "depth with non-nil delay override",
			depth:         1,
			delayOverride: func() *time.Duration { d := time.Duration(500 * time.Millisecond); return &d }(),
			expectedDepth: 1,
			expectedDelay: func() *time.Duration { d := time.Duration(500 * time.Millisecond); return &d }(),
		},
		{
			name:          "zero depth with zero delay override",
			depth:         0,
			delayOverride: func() *time.Duration { d := time.Duration(0); return &d }(),
			expectedDepth: 0,
			expectedDelay: func() *time.Duration { d := time.Duration(0); return &d }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := frontier.NewDiscoveryMetadata(tt.depth, tt.delayOverride)

			if metadata.Depth() != tt.expectedDepth {
				t.Errorf("Depth() = %v, want %v", metadata.Depth(), tt.expectedDepth)
			}

			gotDelay := metadata.DelayOverride()
			if tt.expectedDelay == nil && gotDelay != nil {
				t.Errorf("DelayOverride() = %v, want nil", gotDelay)
			} else if tt.expectedDelay != nil && gotDelay == nil {
				t.Errorf("DelayOverride() = %v, want %v", gotDelay, tt.expectedDelay)
			} else if tt.expectedDelay != nil && gotDelay != nil {
				if *gotDelay != *tt.expectedDelay {
					t.Errorf("DelayOverride() = %v, want %v", *gotDelay, *tt.expectedDelay)
				}
			}
		})
	}
}

func TestCrawlAdmissionCandidate_CreatedWithDiscoveryMetadata(t *testing.T) {
	tests := []struct {
		name          string
		targetURL     url.URL
		sourceContext frontier.SourceContext
		depth         int
		delayOverride *time.Duration
	}{
		{
			name:          "candidate with zero depth and nil delay",
			targetURL:     url.URL{Scheme: "http", Host: "example.com", Path: "/"},
			sourceContext: frontier.SourceSeed,
			depth:         0,
			delayOverride: nil,
		},
		{
			name:          "candidate with positive depth",
			targetURL:     url.URL{Scheme: "https", Host: "example.org", Path: "/page"},
			sourceContext: frontier.SourceCrawl,
			depth:         5,
			delayOverride: nil,
		},
		{
			name:          "candidate with non-nil delay override",
			targetURL:     url.URL{Scheme: "http", Host: "deep.example.com", Path: "/a/b/c"},
			sourceContext: frontier.SourceSeed,
			depth:         3,
			delayOverride: func() *time.Duration { d := 250 * time.Millisecond; return &d }(),
		},
		{
			name:          "candidate with zero duration delay",
			targetURL:     url.URL{Scheme: "https", Host: "example.net", Path: "/test"},
			sourceContext: frontier.SourceCrawl,
			depth:         1,
			delayOverride: func() *time.Duration { d := time.Duration(0); return &d }(),
		},
		{
			name:          "candidate with large depth and 1 second delay",
			targetURL:     url.URL{Scheme: "http", Host: "example.com", Path: "/deep/path"},
			sourceContext: frontier.SourceSeed,
			depth:         100,
			delayOverride: func() *time.Duration { d := time.Second; return &d }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the discovery metadata that will be passed to the candidate
			expectedMetadata := frontier.NewDiscoveryMetadata(tt.depth, tt.delayOverride)

			// Create candidate with NewCrawlAdmissionCandidate
			candidate := frontier.NewCrawlAdmissionCandidate(
				tt.targetURL,
				tt.sourceContext,
				expectedMetadata,
			)

			// Retrieve the DiscoveryMetadata from the candidate
			gotMetadata := candidate.DiscoveryMetadata()

			// Assert that the returned metadata's depth matches
			if gotMetadata.Depth() != tt.depth {
				t.Errorf("DiscoveryMetadata().Depth() = %v, want %v", gotMetadata.Depth(), tt.depth)
			}

			// Assert that the returned metadata's delayOverride matches
			if tt.delayOverride == nil {
				if gotMetadata.DelayOverride() != nil {
					t.Errorf("DiscoveryMetadata().DelayOverride() = %v, want nil", gotMetadata.DelayOverride())
				}
			} else {
				if gotMetadata.DelayOverride() == nil {
					t.Errorf("DiscoveryMetadata().DelayOverride() = %v, want non-nil", gotMetadata.DelayOverride())
				} else if *gotMetadata.DelayOverride() != *tt.delayOverride {
					t.Errorf("DiscoveryMetadata().DelayOverride() = %v, want %v", *gotMetadata.DelayOverride(), *tt.delayOverride)
				}
			}
		})
	}
}
