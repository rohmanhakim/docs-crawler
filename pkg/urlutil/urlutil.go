package urlutil

import "net/url"

// Canonicalize applies a deterministic normalization to a URL, producing a canonical form.
// It maps equivalent URL spellings to a single canonical representation.
//
// The normalization follows these rules:
//   - Scheme and host are lowercased
//   - Path is cleaned (trailing slashes removed, except for root "/")
//   - Fragments are removed
//   - Query parameters are removed
//   - Default ports are omitted (e.g., :80 for http, :443 for https)
//
// Properties:
//   - Pure: no state, no memory
//   - Deterministic: same input always produces same output
//   - Idempotent: Canonicalize(Canonicalize(url)) == Canonicalize(url)
//   - Context-free: does not depend on crawl history
func Canonicalize(sourceUrl url.URL) url.URL {
	// Create a copy to avoid mutating the original
	canonical := sourceUrl

	// Lowercase scheme and host
	canonical.Scheme = lowerASCII(canonical.Scheme)
	canonical.Host = lowerASCII(canonical.Host)

	// Remove default port if present
	if host, port := canonical.Hostname(), canonical.Port(); port != "" {
		if (canonical.Scheme == "http" && port == "80") ||
			(canonical.Scheme == "https" && port == "443") {
			canonical.Host = host
		}
	}

	// Clean the path: remove trailing slashes (except root)
	if len(canonical.Path) > 1 {
		canonical.Path = stripTrailingSlash(canonical.Path)
	}

	// Remove fragment (anchor)
	canonical.Fragment = ""
	canonical.RawFragment = ""

	// Remove query parameters
	canonical.RawQuery = ""
	canonical.ForceQuery = false

	return canonical
}

// lowerASCII converts ASCII characters to lowercase without allocating.
// This is faster than strings.ToLower for ASCII-only strings.
func lowerASCII(s string) string {
	var needsLower bool
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			needsLower = true
			break
		}
	}
	if !needsLower {
		return s
	}
	b := make([]byte, len(s))
	copy(b, s)
	for i := 0; i < len(b); i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// stripTrailingSlash removes trailing slashes from a path.
func stripTrailingSlash(path string) string {
	for len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}

// Resolve resolves a relative URL against a base URL constructed from scheme and host.
// If the relative URL is already absolute (has a scheme), it is returned as-is.
// The base URL is constructed as: scheme + "://" + host
//
// Examples:
//   - Resolve("/docs", "https", "example.com") → "https://example.com/docs"
//   - Resolve("page.html", "https", "example.com") from "https://example.com/dir/" → "https://example.com/dir/page.html"
//   - Resolve("https://other.com/page", "https", "example.com") → "https://other.com/page" (unchanged)
//
// Properties:
//   - Pure: no state, no memory
//   - Deterministic: same input always produces same output
//   - Handles protocol-relative URLs (//host/path)
func Resolve(relativeUrl url.URL, scheme string, host string) url.URL {
	// If the URL is already absolute (has a scheme), return it as-is
	if relativeUrl.Scheme != "" {
		return relativeUrl
	}

	// Construct the base URL from scheme and host
	baseURL := url.URL{
		Scheme: scheme,
		Host:   host,
	}

	// Resolve the relative URL against the base
	// url.ResolveReference handles path-absolute ("/path") and path-relative ("path") URLs
	resolved := baseURL.ResolveReference(&relativeUrl)

	return *resolved
}

// FilterByHost filters a slice of URLs to only include those from the specified host.
// Hostname comparison is case-insensitive.
//
// The host parameter should be just the hostname (e.g., "example.com"), not a full URL.
// URLs are matched by their Hostname() which excludes port numbers.
//
// Examples:
//   - FilterByHost("example.com", ["https://example.com/page", "https://other.com/page"])
//     → ["https://example.com/page"]
//
// Properties:
//   - Pure: no state, no memory
//   - Deterministic: same input always produces same output
//   - Case-insensitive hostname matching
func FilterByHost(host string, urls []url.URL) []url.URL {
	if len(urls) == 0 {
		return []url.URL{}
	}

	// Normalize target host to lowercase for case-insensitive comparison
	targetHost := lowerASCII(host)

	filtered := make([]url.URL, 0, len(urls))
	for _, u := range urls {
		// Compare hostnames case-insensitively
		if lowerASCII(u.Hostname()) == targetHost {
			filtered = append(filtered, u)
		}
	}

	return filtered
}
