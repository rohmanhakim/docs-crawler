package normalize

import (
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
)

// RAG Shaping

type NormalizedMarkdownDoc struct {
	frontmatter Frontmatter
	content     []byte
}

// Frontmatter returns the frontmatter of the normalized document.
func (n NormalizedMarkdownDoc) Frontmatter() Frontmatter {
	return n.frontmatter
}

// Content returns the normalized markdown content.
func (n NormalizedMarkdownDoc) Content() []byte {
	return n.content
}

// NewNormalizedMarkdownDoc creates a new immutable NormalizedMarkdownDoc.
func NewNormalizedMarkdownDoc(frontmatter Frontmatter, content []byte) NormalizedMarkdownDoc {
	return NormalizedMarkdownDoc{
		frontmatter: frontmatter,
		content:     content,
	}
}

type Frontmatter struct {
	title          string
	sourceURL      string
	canonicalURL   string
	crawlDepth     int
	section        string
	docID          string
	contentHash    string
	fetchedAt      time.Time
	crawlerVersion string
}

// NewFrontmatter creates a new immutable Frontmatter with all fields populated.
// All data must be gathered and validated before construction.
func NewFrontmatter(
	title string,
	sourceURL string,
	canonicalURL string,
	crawlDepth int,
	section string,
	docID string,
	contentHash string,
	fetchedAt time.Time,
	crawlerVersion string,
) Frontmatter {
	return Frontmatter{
		title:          title,
		sourceURL:      sourceURL,
		canonicalURL:   canonicalURL,
		crawlDepth:     crawlDepth,
		section:        section,
		docID:          docID,
		contentHash:    contentHash,
		fetchedAt:      fetchedAt,
		crawlerVersion: crawlerVersion,
	}
}

// Title returns the document title.
func (f Frontmatter) Title() string {
	return f.title
}

// SourceURL returns the original source URL.
func (f Frontmatter) SourceURL() string {
	return f.sourceURL
}

// CanonicalURL returns the canonicalized URL.
func (f Frontmatter) CanonicalURL() string {
	return f.canonicalURL
}

// CrawlDepth returns the crawl depth.
func (f Frontmatter) CrawlDepth() int {
	return f.crawlDepth
}

// Section returns the logical section derived from URL path.
func (f Frontmatter) Section() string {
	return f.section
}

// DocID returns the document ID (hash of canonical URL).
func (f Frontmatter) DocID() string {
	return f.docID
}

// ContentHash returns the hash of the normalized markdown content.
func (f Frontmatter) ContentHash() string {
	return f.contentHash
}

// FetchedAt returns the timestamp when the document was fetched.
func (f Frontmatter) FetchedAt() time.Time {
	return f.fetchedAt
}

// CrawlerVersion returns the crawler version.
func (f Frontmatter) CrawlerVersion() string {
	return f.crawlerVersion
}

type NormalizeParam struct {
	appVersion          string
	fetchedAt           time.Time
	hashAlgo            hashutil.HashAlgo
	crawlDepth          int
	allowedPathPrefixes []string
}

func NewNormalizeParam(
	appVersion string,
	fetchedAt time.Time,
	hashAlgo hashutil.HashAlgo,
	crawlDepth int,
	allowedPathPrefixes []string,
) NormalizeParam {
	return NormalizeParam{
		appVersion:          appVersion,
		fetchedAt:           fetchedAt,
		hashAlgo:            hashAlgo,
		crawlDepth:          crawlDepth,
		allowedPathPrefixes: allowedPathPrefixes,
	}
}
