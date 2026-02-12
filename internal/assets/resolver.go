package assets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/rohmanhakim/docs-crawler/pkg/urlutil"
)

// imageRegex matches markdown image syntax: ![alt](url)
// Captures the alt text and URL separately
var imageRegex = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

/*
Responsibilities
- Resolve asset URLs
- Download assets locally
- Deduplicate via content hashing
- Rewrite Markdown references

Asset Policies
- Preserve original formats
- Stable local filenames
- Separate assets directory
- Missing assets reported, not fatal
*/
type Resolver interface {
	Resolve(
		ctx context.Context,
		pageUrl url.URL,
		conversionResult mdconvert.ConversionResult,
		resolveParam ResolveParam,
		retryParam retry.RetryParam,
	) (AssetfulMarkdownDoc, failure.ClassifiedError)
}

type LocalResolver struct {
	metadataSink  metadata.MetadataSink
	writtenAssets map[string]string // key: assetURL, value: contentHash
	hashToPath    map[string]string // key: contentHash, value: localPath (only for files actually written)
	httpClient    *http.Client
	userAgent     string
}

func NewLocalResolver(
	metadataSink metadata.MetadataSink,
	httpClient *http.Client,
	userAgent string,
) LocalResolver {
	return LocalResolver{
		metadataSink:  metadataSink,
		writtenAssets: make(map[string]string),
		hashToPath:    make(map[string]string),
		httpClient:    httpClient,
		userAgent:     userAgent,
	}
}

func (r *LocalResolver) WrittenAssets() map[string]string {
	return r.writtenAssets
}

func (r *LocalResolver) Resolve(
	ctx context.Context,
	pageUrl url.URL,
	conversionResult mdconvert.ConversionResult,
	resolveParam ResolveParam,
	retryParam retry.RetryParam,
) (AssetfulMarkdownDoc, failure.ClassifiedError) {
	// Derive host and scheme from pageUrl for resolving relative asset URLs
	host := pageUrl.Host
	scheme := pageUrl.Scheme

	fetchEventCallback := func(retryCount int, fetchResult AssetFetchResult) {
		url := fetchResult.URL()
		r.metadataSink.RecordAssetFetch(
			url.String(),
			fetchResult.Status(),
			fetchResult.Duration(),
			retryCount,
		)
	}

	// Asset callback - only called when actual new write happens
	assetCallback := func(localPath string) {
		r.metadataSink.RecordArtifact(
			metadata.ArtifactAsset,
			localPath,
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, pageUrl.String()),
			},
		)
	}

	assetfulMarkdownDoc, err := r.resolve(
		ctx,
		conversionResult,
		resolveParam,
		host,
		scheme,
		retryParam,
		fetchEventCallback,
		assetCallback,
	)

	// Record errors for missing URLs
	for urlStr, cause := range assetfulMarkdownDoc.MissingAssets() {
		r.metadataSink.RecordError(
			time.Now(),
			"assets",
			"Resolver.Resolve",
			mapAssetsErrorToMetadataCause(AssetsError{Cause: cause}),
			fmt.Sprintf("missing asset: %s", urlStr),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrMessage, urlStr),
				metadata.NewAttr(metadata.AttrURL, pageUrl.String()),
			},
		)
	}

	// Record errors for unparseable URLs
	for _, unparseableURL := range assetfulMarkdownDoc.UnparseableURLs() {
		r.metadataSink.RecordError(
			time.Now(),
			"assets",
			"Resolver.Resolve",
			metadata.CauseContentInvalid,
			fmt.Sprintf("unparseable asset URL: %s", unparseableURL),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrMessage, unparseableURL),
				metadata.NewAttr(metadata.AttrURL, pageUrl.String()),
			},
		)
	}

	// Record error for write failure (with polymorphism)
	if err != nil {
		var cause metadata.ErrorCause
		var details string

		var retryErr *retry.RetryError
		var assetsErr *AssetsError

		switch {
		case errors.As(err, &retryErr):
			cause = metadata.CauseRetryFailure
			details = retryErr.Error()
		case errors.As(err, &assetsErr):
			cause = mapAssetsErrorToMetadataCause(*assetsErr)
			details = assetsErr.Error()
		default:
			cause = metadata.CauseUnknown
			details = err.Error()
		}

		r.metadataSink.RecordError(
			time.Now(),
			"assets",
			"Resolver.Resolve",
			cause,
			details,
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrWritePath, resolveParam.OutputDir()),
				metadata.NewAttr(metadata.AttrURL, pageUrl.String()),
			},
		)
		return AssetfulMarkdownDoc{}, err
	}

	return assetfulMarkdownDoc, nil
}

func (r *LocalResolver) resolve(
	ctx context.Context,
	conversionResult mdconvert.ConversionResult,
	resolveParam ResolveParam,
	host string,
	scheme string,
	retryParam retry.RetryParam,
	fetchCallback func(int, AssetFetchResult),
	assetCallback func(string),
) (AssetfulMarkdownDoc, failure.ClassifiedError) {
	// Extract image URLs from link refs
	var imageURLs []url.URL
	var unparseableURLs []string
	for _, linkRef := range conversionResult.GetLinkRefs() {
		if linkRef.GetKind() == mdconvert.KindImage {
			u, err := url.Parse(linkRef.GetRaw())
			if err != nil {
				// Track unparseable URL
				unparseableURLs = append(unparseableURLs, linkRef.GetRaw())
				continue
			}
			imageURLs = append(imageURLs, *u)
		}
	}

	// Mechanically deduplicate the asset URLs
	deduplicatedAssetsUrls := r.mechanicalDeduplicate(imageURLs, host, scheme)

	// Track missing asset URLs for this call (with error cause)
	missingAssetErrors := make(map[string]AssetsErrorCause)

	// Check if there are URLs that need downloading
	if len(deduplicatedAssetsUrls) > 0 {
		// Create asset directory (decoupled from page - assets are shared)
		if err := r.ensureAssetDir(resolveParam.OutputDir()); err != nil {
			return AssetfulMarkdownDoc{}, err
		}

		// Fetch each asset with retry
		for _, assetURL := range deduplicatedAssetsUrls {
			result := r.fetchAssetWithRetry(ctx, assetURL, r.userAgent, retryParam, resolveParam.MaxAssetSize())

			// Calculate retry count (attempts - 1, since first try is not a retry)
			retryCount := result.Attempts() - 1

			if result.Err() != nil {
				// Record missing asset URL with error cause
				var assetsErr *AssetsError
				if errors.As(result.Err(), &assetsErr) {
					missingAssetErrors[assetURL.String()] = assetsErr.Cause
				} else {
					missingAssetErrors[assetURL.String()] = ErrCauseNetworkFailure
				}
				// Call fetchCallback even on failure with empty result (but with URL)
				fetchCallback(retryCount, NewAssetFetchResult(assetURL, 0, 0, nil))
				// Continue with next asset (missing assets are reported, not fatal)
				continue
			}
			// Call fetchCallback on success
			fetchResult := result.Value()
			fetchCallback(retryCount, fetchResult)

			// Hash the content using the configured hash algorithm
			assetData := fetchResult.Data()
			contentHash, hashErr := hashutil.HashBytes(assetData, resolveParam.HashAlgo())
			if hashErr != nil {
				// This should not happen with valid algorithms, but handle defensively
				missingAssetErrors[assetURL.String()] = ErrCauseHashError
				continue
			}

			// Get extension from asset URL
			extension := getFileExtension(assetURL.Path)

			// Check if content hash already exists (content-hash deduplication)
			if existingPath := r.findPathByHash(contentHash); existingPath != "" {
				// Content already written from different URL, add new URL entry with same hash
				// DON'T call assetCallback - no new write happened
				r.writtenAssets[assetURL.String()] = contentHash
				continue
			}

			// Write asset to disk (pass original URL path for filename)
			localPath, err := r.writeAsset(resolveParam.OutputDir(), assetURL.Path, contentHash, extension, assetData)
			if err != nil {
				// Write failed - don't update writtenAssets, asset remains "pending"
				var assetsErr *AssetsError
				if errors.As(err, &assetsErr) {
					missingAssetErrors[assetURL.String()] = assetsErr.Cause
				} else {
					missingAssetErrors[assetURL.String()] = ErrCauseWriteFailure
				}
				continue
			}

			// Record successfully written asset: URL -> contentHash
			r.writtenAssets[assetURL.String()] = contentHash

			// Store hash -> path mapping for content-hash deduplication lookups
			r.hashToPath[contentHash] = localPath

			// Call assetCallback ONLY for actual new writes (not content-hash dedups)
			assetCallback(localPath)
		}
	}

	// Construct local asset paths for the current document's image URLs
	currentDocumentAssets := r.constructLocalPaths(imageURLs, host, scheme)

	// Build localAssets slice from map values
	var localAssets []string
	for _, localPath := range currentDocumentAssets {
		localAssets = append(localAssets, localPath)
	}

	// Get content from constructDocument
	content := r.constructDocument(conversionResult.GetMarkdownContent(), currentDocumentAssets)

	// Create fully populated AssetfulMarkdownDoc
	resolvedDoc := NewAssetfulMarkdownDoc(content, missingAssetErrors, unparseableURLs, localAssets)
	return resolvedDoc, nil
}

// findPathByHash finds the stored path for a content hash.
// This is used for content-hash deduplication.
// Returns empty string if no file was written for this hash.
func (r *LocalResolver) findPathByHash(hash string) string {
	return r.hashToPath[hash]
}

func (r *LocalResolver) mechanicalDeduplicate(urls []url.URL, host string, scheme string) []url.URL {
	var deduplicated []url.URL
	// Track URLs within this call to deduplicate within the same page
	seenInThisCall := make(map[string]bool)

	for _, u := range urls {
		// Step 1: Resolve relative to absolute
		resolved := urlutil.Resolve(u, scheme, host)

		// Step 2: Normalize/Canonicalize
		canonical := urlutil.Canonicalize(resolved)
		canonicalKey := canonical.String()

		// Step 3: Deduplicate using writtenAssets map keys AND seenInThisCall
		// Skip if already in writtenAssets (from previous calls) OR already seen in this call
		if _, exists := r.writtenAssets[canonicalKey]; exists {
			continue
		}
		if seenInThisCall[canonicalKey] {
			continue
		}

		// Mark as seen and add to result
		seenInThisCall[canonicalKey] = true
		deduplicated = append(deduplicated, canonical)
	}

	return deduplicated
}

func (r *LocalResolver) ensureAssetDir(outputDir string) failure.ClassifiedError {
	assetsDir := filepath.Join(outputDir, "assets", "images")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return &AssetsError{
			Message:   fmt.Sprintf("%v", err),
			Retryable: false,
			Cause:     ErrCausePathError,
		}
	}
	return nil
}

func (r *LocalResolver) fetchAssetWithRetry(
	ctx context.Context,
	fetchUrl url.URL,
	userAgent string,
	retryParam retry.RetryParam,
	maxAssetSize int64,
) retry.Result[AssetFetchResult] {
	fetchTask := func() (AssetFetchResult, failure.ClassifiedError) {
		return r.performFetch(ctx, fetchUrl, userAgent, maxAssetSize)
	}

	result := retry.Retry(retryParam, fetchTask)

	return result
}

func (r *LocalResolver) performFetch(ctx context.Context, fetchUrl url.URL, userAgent string, maxAssetSize int64) (AssetFetchResult, failure.ClassifiedError) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchUrl.String(), nil)
	if err != nil {
		return AssetFetchResult{}, &AssetsError{
			Message:   fmt.Sprintf("failed to create request: %v", err),
			Retryable: false,
			Cause:     ErrCauseNetworkFailure,
		}
	}

	// Apply headers for asset fetching
	headers := assetRequestHeaders(userAgent)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	startTime := time.Now()
	resp, err := r.httpClient.Do(req)
	duration := time.Since(startTime)
	if err != nil {
		return AssetFetchResult{}, &AssetsError{
			Message:   fmt.Sprintf("request failed: %v", err),
			Retryable: true,
			Cause:     ErrCauseNetworkFailure,
		}
	}
	defer resp.Body.Close()

	// Check Content-Length before downloading
	if resp.ContentLength > maxAssetSize {
		return AssetFetchResult{}, &AssetsError{
			Message:   fmt.Sprintf("asset too large: %d bytes (max %d)", resp.ContentLength, maxAssetSize),
			Retryable: false,
			Cause:     ErrCauseAssetTooLarge,
		}
	}

	// Handle HTTP status codes
	switch {
	case resp.StatusCode >= 500:
		return AssetFetchResult{}, &AssetsError{
			Message:   fmt.Sprintf("server error: %d", resp.StatusCode),
			Retryable: true,
			Cause:     ErrCauseRequest5xx,
		}

	case resp.StatusCode == 429:
		return AssetFetchResult{}, &AssetsError{
			Message:   "rate limited (429)",
			Retryable: true,
			Cause:     ErrCauseRequestTooMany,
		}

	case resp.StatusCode == 403:
		return AssetFetchResult{}, &AssetsError{
			Message:   "access forbidden (403)",
			Retryable: false,
			Cause:     ErrCauseRequestPageForbidden,
		}

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return AssetFetchResult{}, &AssetsError{
			Message:   fmt.Sprintf("client error: %d", resp.StatusCode),
			Retryable: false,
			Cause:     ErrCauseRequestPageForbidden,
		}

	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return AssetFetchResult{}, &AssetsError{
			Message:   fmt.Sprintf("redirect error: %d", resp.StatusCode),
			Retryable: false,
			Cause:     ErrCauseRedirectLimitExceeded,
		}
	}

	// Read with hard limit to protect against:
	// - Content-Length = -1 (unknown/omitted)
	// - Incorrect/malicious Content-Length values
	// - Streaming responses that exceed maxAssetSize
	limitedReader := io.LimitReader(resp.Body, maxAssetSize+1) // +1 to detect overflow
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return AssetFetchResult{}, &AssetsError{
			Message:   fmt.Sprintf("failed to read response body: %v", err),
			Retryable: true,
			Cause:     ErrCauseReadResponseBodyError,
		}
	}

	// Check if we hit the limit (body exceeds maxAssetSize)
	if int64(len(body)) > maxAssetSize {
		return AssetFetchResult{}, &AssetsError{
			Message:   fmt.Sprintf("asset too large: exceeded max %d bytes", maxAssetSize),
			Retryable: false,
			Cause:     ErrCauseAssetTooLarge,
		}
	}

	return NewAssetFetchResult(fetchUrl, resp.StatusCode, duration, body), nil
}

func (r *LocalResolver) writeAsset(outputDir string, originalPath string, contentHash string, extension string, data []byte) (string, failure.ClassifiedError) {
	localPath := buildAssetPath(originalPath, contentHash, extension)
	filePath := filepath.Join(outputDir, localPath)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		// Check if disk is full
		if errors.Is(err, syscall.ENOSPC) {
			return "", &AssetsError{
				Message:   fmt.Sprintf("disk full: %v", err),
				Retryable: true,
				Cause:     ErrCauseDiskFull,
			}
		}
		// Other write failures
		return "", &AssetsError{
			Message:   fmt.Sprintf("write failed: %v", err),
			Retryable: false,
			Cause:     ErrCauseWriteFailure,
		}
	}
	return localPath, nil
}

func (r *LocalResolver) constructLocalPaths(imageUrls []url.URL, host string, scheme string) map[string]string {
	localPaths := make(map[string]string)

	for _, imgURL := range imageUrls {
		// Store the raw URL string (as it appears in markdown)
		rawURLStr := imgURL.String()

		// Resolve relative to absolute and canonicalize (same logic as mechanicalDeduplicate)
		resolved := urlutil.Resolve(imgURL, scheme, host)
		canonical := urlutil.Canonicalize(resolved)
		canonicalURLStr := canonical.String()

		// Look up content hash in writtenAssets using canonical URL
		if contentHash, exists := r.writtenAssets[canonicalURLStr]; exists {
			// First try to find existing path for this content hash (content-hash dedup)
			localPath := r.findPathByHash(contentHash)
			if localPath == "" {
				// No existing path found, build new path
				extension := getFileExtension(canonical.Path)
				localPath = buildAssetPath(canonical.Path, contentHash, extension)
			}

			// Map the RAW URL (as it appears in markdown) to the local path
			localPaths[rawURLStr] = localPath
		}
		// Skip if not in writtenAssets (failed download) - raw URL won't be in map
	}

	return localPaths
}

func (r *LocalResolver) constructDocument(inputDoc []byte, localMapping map[string]string) []byte {
	// Use regex to find and replace image URLs in markdown
	content := imageRegex.ReplaceAllStringFunc(string(inputDoc), func(match string) string {
		// Extract URL from the match using the regex
		submatches := imageRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			// Should not happen, but keep original if it does
			return match
		}

		altText := submatches[1] // The alt text
		url := submatches[2]     // The URL

		// Check if this URL should be replaced (successful download only)
		if localPath, exists := localMapping[url]; exists {
			return "![" + altText + "](" + localPath + ")"
		}

		// URL not in mapping (failed download), keep original
		return match
	})

	return []byte(content)
}

func assetRequestHeaders(userAgent string) map[string]string {
	return map[string]string{
		"User-Agent":      userAgent,
		"Accept":          "image/webp,image/apng,image/*,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.5",
		"DNT":             "1",
		"Connection":      "keep-alive",
	}
}

// getFileExtension extracts the file extension from a path, or empty string if none
func getFileExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	// Remove the leading dot
	return strings.TrimPrefix(ext, ".")
}

// buildAssetPath builds the relative path for an asset using the format:
// assets/images/<original-name>-<short-hash>.<ext>
// Example: assets/images/logo-a3f7b2c.png
func buildAssetPath(originalPath string, contentHash string, extension string) string {
	// Extract basename without extension from original path
	base := filepath.Base(originalPath)
	nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))

	// Sanitize filename: keep only safe characters
	safeName := sanitizeFilename(nameWithoutExt)
	if safeName == "" {
		safeName = "asset"
	}

	// Use first 7 chars of hash (like git) for readability
	shortHash := contentHash
	if len(contentHash) > 7 {
		shortHash = contentHash[:7]
	}

	// Build filename: <name>-<short-hash>.<ext>
	filename := safeName + "-" + shortHash
	if extension != "" {
		filename = filename + "." + extension
	}

	return filepath.Join("assets", "images", filename)
}

// sanitizeFilename removes or replaces unsafe characters from a filename
func sanitizeFilename(name string) string {
	// Replace unsafe characters with underscore
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	result := name
	for _, char := range unsafe {
		result = strings.ReplaceAll(result, char, "_")
	}
	// Limit length to avoid overly long filenames
	if len(result) > 100 {
		result = result[:100]
	}
	return result
}
