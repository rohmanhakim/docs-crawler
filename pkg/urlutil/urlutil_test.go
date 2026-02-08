package urlutil

import (
	"net/url"
	"testing"
)

func TestCanonicalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trailing slash removed",
			input:    "https://docs.example.com/guide/",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "no trailing slash stays same",
			input:    "https://docs.example.com/guide",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "fragment removed",
			input:    "https://docs.example.com/guide#index",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "query parameters removed",
			input:    "https://docs.example.com/guide?utm_source=twitter",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "both fragment and query removed",
			input:    "https://docs.example.com/guide?utm_source=twitter#index",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "scheme lowercased",
			input:    "HTTPS://docs.example.com/guide",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "host lowercased",
			input:    "https://DOCS.EXAMPLE.COM/guide",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "scheme and host lowercased",
			input:    "HTTPS://DOCS.EXAMPLE.COM/GUIDE",
			expected: "https://docs.example.com/GUIDE",
		},
		{
			name:     "default http port removed",
			input:    "http://docs.example.com:80/guide",
			expected: "http://docs.example.com/guide",
		},
		{
			name:     "default https port removed",
			input:    "https://docs.example.com:443/guide",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "non-default port preserved",
			input:    "https://docs.example.com:8080/guide",
			expected: "https://docs.example.com:8080/guide",
		},
		{
			name:     "multiple trailing slashes removed",
			input:    "https://docs.example.com/guide///",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "root path preserved",
			input:    "https://docs.example.com/",
			expected: "https://docs.example.com/",
		},
		{
			name:     "root path without slash",
			input:    "https://docs.example.com",
			expected: "https://docs.example.com",
		},
		{
			name:     "complex path with fragment and query",
			input:    "https://docs.example.com/api/v1/users?id=123#section",
			expected: "https://docs.example.com/api/v1/users",
		},
		{
			name:     "path with uppercase preserved",
			input:    "https://docs.example.com/API/v1/Users",
			expected: "https://docs.example.com/API/v1/Users",
		},
		{
			name:     "http with non-standard port",
			input:    "http://docs.example.com:8080/path",
			expected: "http://docs.example.com:8080/path",
		},
		{
			name:     "empty query removed",
			input:    "https://docs.example.com/guide?",
			expected: "https://docs.example.com/guide",
		},
		{
			name:     "empty fragment removed",
			input:    "https://docs.example.com/guide#",
			expected: "https://docs.example.com/guide",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputURL, err := url.Parse(tt.input)
			if err != nil {
				t.Fatalf("failed to parse input URL %q: %v", tt.input, err)
			}

			result := Canonicalize(*inputURL)
			resultStr := result.String()

			if resultStr != tt.expected {
				t.Errorf("Canonicalize(%q) = %q, want %q", tt.input, resultStr, tt.expected)
			}
		})
	}
}

func TestCanonicalizeIdempotent(t *testing.T) {
	// Test that Canonicalize is idempotent: Canonicalize(Canonicalize(url)) == Canonicalize(url)
	testURLs := []string{
		"https://docs.example.com/guide/",
		"https://docs.example.com/guide?utm_source=twitter",
		"https://docs.example.com/guide#index",
		"HTTPS://DOCS.EXAMPLE.COM:443/GUIDE/?#",
		"http://example.com:80/path///",
	}

	for _, urlStr := range testURLs {
		t.Run(urlStr, func(t *testing.T) {
			inputURL, err := url.Parse(urlStr)
			if err != nil {
				t.Fatalf("failed to parse URL %q: %v", urlStr, err)
			}

			first := Canonicalize(*inputURL)
			second := Canonicalize(first)

			firstStr := first.String()
			secondStr := second.String()

			if firstStr != secondStr {
				t.Errorf("Canonicalize is not idempotent: first=%q, second=%q", firstStr, secondStr)
			}
		})
	}
}

func TestCanonicalizeDoesNotMutateInput(t *testing.T) {
	// Ensure the original URL is not modified
	input, _ := url.Parse("https://example.com/path/?query=1#frag")
	original := *input

	_ = Canonicalize(*input)

	if input.String() != original.String() {
		t.Error("Canonicalize mutated the input URL")
	}
}

func TestLowerASCII(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "hello"},
		{"HELLO", "hello"},
		{"hello", "hello"},
		{"HTTPS", "https"},
		{"MixedCASE", "mixedcase"},
		{"already-lower", "already-lower"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := lowerASCII(tt.input)
			if result != tt.expected {
				t.Errorf("lowerASCII(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStripTrailingSlash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/", "/path"},
		{"/path//", "/path"},
		{"/path///", "/path"},
		{"/path", "/path"},
		{"/", "/"},
		{"///", "/"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripTrailingSlash(tt.input)
			if result != tt.expected {
				t.Errorf("stripTrailingSlash(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name        string
		relativeURL string
		scheme      string
		host        string
		expected    string
	}{
		{
			name:        "path absolute URL",
			relativeURL: "/docs",
			scheme:      "https",
			host:        "example.com",
			expected:    "https://example.com/docs",
		},
		{
			name:        "path absolute with trailing slash",
			relativeURL: "/docs/",
			scheme:      "https",
			host:        "example.com",
			expected:    "https://example.com/docs/",
		},
		{
			name:        "path relative URL",
			relativeURL: "page.html",
			scheme:      "https",
			host:        "example.com",
			expected:    "https://example.com/page.html",
		},
		{
			name:        "already absolute URL - unchanged",
			relativeURL: "https://other.com/page",
			scheme:      "https",
			host:        "example.com",
			expected:    "https://other.com/page",
		},
		{
			name:        "absolute URL with http scheme - unchanged",
			relativeURL: "http://other.com/page",
			scheme:      "https",
			host:        "example.com",
			expected:    "http://other.com/page",
		},
		{
			name:        "URL with query string",
			relativeURL: "/search?q=test",
			scheme:      "https",
			host:        "example.com",
			expected:    "https://example.com/search?q=test",
		},
		{
			name:        "URL with fragment",
			relativeURL: "/docs#section",
			scheme:      "https",
			host:        "example.com",
			expected:    "https://example.com/docs#section",
		},
		{
			name:        "empty path",
			relativeURL: "",
			scheme:      "https",
			host:        "example.com",
			expected:    "https://example.com",
		},
		{
			name:        "root path",
			relativeURL: "/",
			scheme:      "https",
			host:        "example.com",
			expected:    "https://example.com/",
		},
		{
			name:        "nested path",
			relativeURL: "/api/v1/users",
			scheme:      "https",
			host:        "example.com",
			expected:    "https://example.com/api/v1/users",
		},
		{
			name:        "http scheme",
			relativeURL: "/docs",
			scheme:      "http",
			host:        "example.com",
			expected:    "http://example.com/docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relativeURL, err := url.Parse(tt.relativeURL)
			if err != nil {
				t.Fatalf("failed to parse relative URL %q: %v", tt.relativeURL, err)
			}

			result := Resolve(*relativeURL, tt.scheme, tt.host)
			resultStr := result.String()

			if resultStr != tt.expected {
				t.Errorf("Resolve(%q, %q, %q) = %q, want %q",
					tt.relativeURL, tt.scheme, tt.host, resultStr, tt.expected)
			}
		})
	}
}

func TestResolveDoesNotMutateInput(t *testing.T) {
	input, _ := url.Parse("/docs")
	original := *input

	_ = Resolve(*input, "https", "example.com")

	if input.String() != original.String() {
		t.Error("Resolve mutated the input URL")
	}
}

func TestFilterByHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		urls     []string
		expected []string
	}{
		{
			name: "mixed URLs - some matching",
			host: "example.com",
			urls: []string{
				"https://example.com/page1",
				"https://other.com/page",
				"https://example.com/page2",
				"http://different.com/",
			},
			expected: []string{
				"https://example.com/page1",
				"https://example.com/page2",
			},
		},
		{
			name: "all matching URLs",
			host: "example.com",
			urls: []string{
				"https://example.com/page1",
				"https://example.com/page2",
				"http://example.com/page3",
			},
			expected: []string{
				"https://example.com/page1",
				"https://example.com/page2",
				"http://example.com/page3",
			},
		},
		{
			name: "no matching URLs",
			host: "example.com",
			urls: []string{
				"https://other.com/page1",
				"https://different.com/page2",
			},
			expected: []string{},
		},
		{
			name:     "empty input",
			host:     "example.com",
			urls:     []string{},
			expected: []string{},
		},
		{
			name: "case insensitive matching",
			host: "EXAMPLE.COM",
			urls: []string{
				"https://example.com/page1",
				"https://EXAMPLE.COM/page2",
				"https://Other.com/page",
			},
			expected: []string{
				"https://example.com/page1",
				"https://EXAMPLE.COM/page2",
			},
		},
		{
			name: "handles www subdomain separately",
			host: "example.com",
			urls: []string{
				"https://example.com/page",
				"https://www.example.com/page",
			},
			expected: []string{
				"https://example.com/page",
			},
		},
		{
			name: "handles ports correctly",
			host: "example.com",
			urls: []string{
				"https://example.com/page",
				"https://example.com:8080/page",
				"https://other.com:8080/page",
			},
			expected: []string{
				"https://example.com/page",
				"https://example.com:8080/page",
			},
		},
		{
			name:     "single matching URL",
			host:     "example.com",
			urls:     []string{"https://example.com/single"},
			expected: []string{"https://example.com/single"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse input URLs
			urls := make([]url.URL, len(tt.urls))
			for i, u := range tt.urls {
				parsed, err := url.Parse(u)
				if err != nil {
					t.Fatalf("failed to parse URL %q: %v", u, err)
				}
				urls[i] = *parsed
			}

			result := FilterByHost(tt.host, urls)

			// Convert result to strings for comparison
			resultStrs := make([]string, len(result))
			for i, u := range result {
				resultStrs[i] = u.String()
			}

			if len(resultStrs) != len(tt.expected) {
				t.Errorf("FilterByHost(%q) returned %d URLs, want %d",
					tt.host, len(resultStrs), len(tt.expected))
			}

			for i, expected := range tt.expected {
				if i >= len(resultStrs) || resultStrs[i] != expected {
					t.Errorf("FilterByHost(%q)[%d] = %q, want %q",
						tt.host, i, resultStrs[i], expected)
				}
			}
		})
	}
}

func TestFilterByHostDoesNotMutateInput(t *testing.T) {
	urls := []url.URL{
		*mustParseURL("https://example.com/page1"),
		*mustParseURL("https://other.com/page2"),
	}
	original := make([]url.URL, len(urls))
	copy(original, urls)

	_ = FilterByHost("example.com", urls)

	for i, u := range urls {
		if u.String() != original[i].String() {
			t.Errorf("FilterByHost mutated input URL at index %d", i)
		}
	}
}

// mustParseURL is a test helper that parses a URL or panics
func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
