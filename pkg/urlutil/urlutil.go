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
