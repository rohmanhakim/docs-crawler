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
