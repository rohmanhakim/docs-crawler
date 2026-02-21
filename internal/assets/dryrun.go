package assets

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/rohmanhakim/docs-crawler/pkg/urlutil"
)

/*
DryRunResolver is an asset resolver implementation that simulates asset resolution
without making network requests or writing files to disk.

It follows the same contract as LocalResolver:
- Extracts image URLs from markdown
- Deduplicates URLs mechanically
- Computes deterministic local paths
- Rewrites markdown links
- Records artifact metadata

This allows the scheduler to run unchanged while suppressing
both network requests and disk writes for assets.
*/

type DryRunResolver struct {
	metadataSink  metadata.MetadataSink
	writtenAssets map[string]string // key: canonical asset URL, value: synthetic hash
	hashToPath    map[string]string // key: synthetic hash, value: localPath
	httpClient    *http.Client      // kept for interface compatibility, not used
	userAgent     string
}

func NewDryRunResolver(
	metadataSink metadata.MetadataSink,
) *DryRunResolver {
	return &DryRunResolver{
		metadataSink:  metadataSink,
		writtenAssets: make(map[string]string),
		hashToPath:    make(map[string]string),
	}
}

// Init initializes the resolver with an HTTP client and user agent.
// In dry-run mode, the HTTP client is stored but not used for network requests.
func (r *DryRunResolver) Init(httpClient *http.Client, userAgent string) {
	r.httpClient = httpClient
	r.userAgent = userAgent
}

func (r *DryRunResolver) WrittenAssets() map[string]string {
	return r.writtenAssets
}

func (r *DryRunResolver) Resolve(
	ctx context.Context,
	pageUrl url.URL,
	conversionResult mdconvert.ConversionResult,
	resolveParam ResolveParam,
	_ retry.RetryParam, // unused in dry-run - no retries needed
) (AssetfulMarkdownDoc, failure.ClassifiedError) {
	// Derive host and scheme from pageUrl for resolving relative asset URLs
	host := pageUrl.Host
	scheme := pageUrl.Scheme

	assetfulMarkdownDoc, err := r.resolve(
		ctx,
		conversionResult,
		resolveParam,
		host,
		scheme,
	)

	// Record errors for unparseable URLs
	for _, unparseableURL := range assetfulMarkdownDoc.UnparseableURLs() {
		r.metadataSink.RecordError(metadata.NewErrorRecord(
			time.Now(),
			"assets",
			"DryRunResolver.Resolve",
			metadata.CauseContentInvalid,
			fmt.Sprintf("unparseable asset URL: %s", unparseableURL),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrAssetURL, unparseableURL),
				metadata.NewAttr(metadata.AttrURL, pageUrl.String()),
			},
		))
	}

	if err != nil {
		r.metadataSink.RecordError(metadata.NewErrorRecord(
			time.Now(),
			"assets",
			"DryRunResolver.Resolve",
			metadata.CauseUnknown,
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrWritePath, resolveParam.OutputDir()),
				metadata.NewAttr(metadata.AttrURL, pageUrl.String()),
			},
		))
		return AssetfulMarkdownDoc{}, err
	}

	return assetfulMarkdownDoc, nil
}

func (r *DryRunResolver) resolve(
	_ context.Context, // unused in dry-run
	conversionResult mdconvert.ConversionResult,
	resolveParam ResolveParam,
	host string,
	scheme string,
) (AssetfulMarkdownDoc, failure.ClassifiedError) {
	// Extract image URLs from link refs
	var imageURLs []url.URL
	var unparseableURLs []string
	for _, linkRef := range conversionResult.GetLinkRefs() {
		if linkRef.GetKind() == mdconvert.KindImage {
			u, err := url.Parse(linkRef.GetRaw())
			if err != nil {
				unparseableURLs = append(unparseableURLs, linkRef.GetRaw())
				continue
			}
			imageURLs = append(imageURLs, *u)
		}
	}

	// Mechanically deduplicate the asset URLs
	deduplicatedAssetsUrls := r.mechanicalDeduplicate(imageURLs, host, scheme)

	// Process each asset (compute paths without fetching)
	for _, assetURL := range deduplicatedAssetsUrls {
		// Compute canonical URL (without query params) for storage key
		canonicalAssetURL := urlutil.Canonicalize(assetURL)
		canonicalKey := canonicalAssetURL.String()

		// Generate a synthetic content hash based on URL
		// This ensures deterministic paths without actual content
		syntheticHash, _ := hashutil.HashBytes([]byte(assetURL.String()), resolveParam.HashAlgo())

		// Get extension from asset URL
		extension := getFileExtension(assetURL.Path)

		// Build the local path
		localPath := buildAssetPath(assetURL.Path, syntheticHash, extension)

		// Record successfully "resolved" asset
		r.writtenAssets[canonicalKey] = syntheticHash
		r.hashToPath[syntheticHash] = localPath

		// Record artifact metadata (without actual bytes)
		r.metadataSink.RecordArtifact(metadata.NewArtifactRecord(
			metadata.ArtifactAsset,
			localPath,
			assetURL.String(),
			syntheticHash,
			false, // overwrite = false (no actual file written in dry-run)
			0,     // bytes = 0 (no actual content in dry-run)
			time.Now(),
		))
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
	// In dry-run mode, there are no missing assets (we simulate success for all)
	resolvedDoc := NewAssetfulMarkdownDoc(content, make(map[string]AssetsErrorCause), unparseableURLs, localAssets)
	return resolvedDoc, nil
}

func (r *DryRunResolver) mechanicalDeduplicate(urls []url.URL, host string, scheme string) []url.URL {
	var deduplicated []url.URL
	seenInThisCall := make(map[string]bool)

	for _, u := range urls {
		// Step 1: Resolve relative to absolute
		resolved := urlutil.Resolve(u, scheme, host)

		// Step 2: Normalize/Canonicalize for deduplication key
		canonical := urlutil.Canonicalize(resolved)
		canonicalKey := canonical.String()

		// Step 3: Deduplicate using writtenAssets map keys AND seenInThisCall
		if _, exists := r.writtenAssets[canonicalKey]; exists {
			continue
		}
		if seenInThisCall[canonicalKey] {
			continue
		}

		// Mark as seen and add to result
		seenInThisCall[canonicalKey] = true
		deduplicated = append(deduplicated, resolved)
	}

	return deduplicated
}

func (r *DryRunResolver) constructLocalPaths(imageUrls []url.URL, host string, scheme string) map[string]string {
	localPaths := make(map[string]string)

	for _, imgURL := range imageUrls {
		// Store the raw URL string (as it appears in markdown)
		rawURLStr := imgURL.String()

		// Resolve relative to absolute and canonicalize
		resolved := urlutil.Resolve(imgURL, scheme, host)
		canonical := urlutil.Canonicalize(resolved)
		canonicalURLStr := canonical.String()

		// Look up synthetic hash in writtenAssets
		if syntheticHash, exists := r.writtenAssets[canonicalURLStr]; exists {
			// Find path for this hash
			localPath := r.hashToPath[syntheticHash]
			if localPath == "" {
				// Build new path if not found
				extension := getFileExtension(canonical.Path)
				localPath = buildAssetPath(canonical.Path, syntheticHash, extension)
			}
			localPaths[rawURLStr] = localPath
		}
	}

	return localPaths
}

func (r *DryRunResolver) constructDocument(inputDoc []byte, localMapping map[string]string) []byte {
	// Use regex to find and replace image URLs in markdown
	content := imageRegex.ReplaceAllStringFunc(string(inputDoc), func(match string) string {
		submatches := imageRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		altText := submatches[1]
		url := submatches[2]

		if localPath, exists := localMapping[url]; exists {
			return "![" + altText + "](" + localPath + ")"
		}

		return match
	})

	return []byte(content)
}

// getFileExtension extracts the file extension from a path
func getFileExtension(path string) string {
	ext := filepath.Ext(path)
	if len(ext) > 0 {
		return ext[1:] // remove leading dot
	}
	return ""
}
