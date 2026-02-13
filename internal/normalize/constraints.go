package normalize

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/rohmanhakim/docs-crawler/pkg/urlutil"
)

/*
Responsibilities
- Inject frontmatter
- Enforce structural rules
- Prepare documents for RAG chunking

Frontmatter Fields
- Title
- Source URL
- Crawl depth
- Section or category
- etc

RAG-Oriented Constraints
- Logical section boundaries preserved
- Code blocks and tables are atomic
- Chunk sizes predictable
*/

type Constraint interface {
	Normalize(
		fetchUrl url.URL,
		assetfulMarkdownDoc assets.AssetfulMarkdownDoc,
		normalizeParam NormalizeParam,
	) (NormalizedMarkdownDoc, failure.ClassifiedError)
}

type MarkdownConstraint struct {
	metadataSink metadata.MetadataSink
}

func NewMarkdownConstraint(
	metadataSink metadata.MetadataSink,
) MarkdownConstraint {
	return MarkdownConstraint{
		metadataSink: metadataSink,
	}
}

func (m *MarkdownConstraint) Normalize(
	fetchUrl url.URL,
	assetfulMarkdownDoc assets.AssetfulMarkdownDoc,
	normalizeParam NormalizeParam,
) (NormalizedMarkdownDoc, failure.ClassifiedError) {
	normalizedMarkdown, err := normalize(fetchUrl, assetfulMarkdownDoc, normalizeParam)
	if err != nil {
		var normalizationError *NormalizationError
		errors.As(err, &normalizationError)
		m.metadataSink.RecordError(
			time.Now(),
			"normalize",
			"MarkdownConstraint.Normalize",
			mapNormalizationErrorToMetadataCause(*normalizationError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, fetchUrl.String()),
			},
		)
		return NormalizedMarkdownDoc{}, normalizationError
	}
	return normalizedMarkdown, nil
}

func normalize(
	fetchUrl url.URL,
	inputDoc assets.AssetfulMarkdownDoc,
	normalizeParam NormalizeParam,
) (NormalizedMarkdownDoc, failure.ClassifiedError) {
	content := inputDoc.Content()

	// Step 1: Validate structure before generating frontmatter
	if err := validateStructure(content); err != nil {
		return NormalizedMarkdownDoc{}, err
	}

	// Step 2: Generate frontmatter (assumes valid structure)
	frontmatter, err := generateFrontmatter(fetchUrl, inputDoc, normalizeParam)
	if err != nil {
		return NormalizedMarkdownDoc{}, err
	}

	// Return normalized document with both frontmatter and content
	return NewNormalizedMarkdownDoc(frontmatter, content), nil
}

// validateStructure validates the Markdown document structure according to
// normalization invariants N1, N3, N4, N5, and N6.
// It uses AST parsing for correctness.
func validateStructure(content []byte) failure.ClassifiedError {
	// Check for empty content (Invariant N1 prerequisite)
	if len(bytes.TrimSpace(content)) == 0 {
		return &NormalizationError{
			Message:   "markdown content is empty",
			Retryable: false,
			Cause:     ErrCauseEmptyContent,
		}
	}

	// Parse markdown into AST
	p := parser.New()
	doc := markdown.Parse(content, p)

	// Collect headings and validate structure via AST walk
	var headings []*ast.Heading
	var hasContentBeforeH1 bool
	var insideCodeBlock bool

	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		switch n := node.(type) {
		case *ast.Heading:
			if entering {
				// Check if heading is inside a code block (Invariant N6)
				if insideCodeBlock {
					return ast.Terminate
				}
				headings = append(headings, n)
			}

		case *ast.CodeBlock:
			if entering {
				insideCodeBlock = true
			} else {
				insideCodeBlock = false
			}

		case *ast.Text, *ast.Paragraph, *ast.List, *ast.Table:
			if entering {
				// Track if we have content before first H1
				if len(headings) == 0 {
					hasContentBeforeH1 = true
				}
			}
		}

		return ast.GoToNext
	})

	// Check if heading inside code block was detected
	if insideCodeBlock {
		return &NormalizationError{
			Message:   "heading detected inside code block",
			Retryable: false,
			Cause:     ErrCauseBrokenAtomicBlock,
		}
	}

	// Validate N1: Exactly one H1
	h1Count := 0
	for _, h := range headings {
		if h.Level == 1 {
			h1Count++
		}
	}

	if h1Count == 0 {
		return &NormalizationError{
			Message:   "document has no H1 heading",
			Retryable: false,
			Cause:     ErrCauseBrokenH1Invariant,
		}
	}

	if h1Count > 1 {
		return &NormalizationError{
			Message:   fmt.Sprintf("document has %d H1 headings, expected exactly one", h1Count),
			Retryable: false,
			Cause:     ErrCauseBrokenH1Invariant,
		}
	}

	// Validate N4: No orphan content before H1
	if hasContentBeforeH1 {
		return &NormalizationError{
			Message:   "content exists before first H1 heading",
			Retryable: false,
			Cause:     ErrCauseOrphanContent,
		}
	}

	// Validate N3: No skipped heading levels
	prevLevel := 0
	for _, h := range headings {
		// Check for level skip (N3)
		if h.Level > prevLevel+1 && prevLevel != 0 {
			return &NormalizationError{
				Message:   fmt.Sprintf("heading level skipped: H%d follows H%d", h.Level, prevLevel),
				Retryable: false,
				Cause:     ErrCauseSkippedHeadingLevels,
			}
		}

		prevLevel = h.Level
	}

	return nil
}

func generateFrontmatter(
	fetchUrl url.URL,
	inputDoc assets.AssetfulMarkdownDoc,
	normalizeParam NormalizeParam,
) (Frontmatter, failure.ClassifiedError) {
	content := inputDoc.Content()

	// Extract title from content (assumes exactly one H1 exists after validation)
	title, err := extractTitle(content)
	if err != nil {
		return Frontmatter{}, err
	}

	// Get source URL
	sourceURL := fetchUrl.String()

	// Compute canonical URL
	canonicalURL := urlutil.Canonicalize(fetchUrl)

	// Derive section from canonical URL path (stripping allowedPathPrefixes first)
	section, err := deriveSection(canonicalURL, normalizeParam.allowedPathPrefixes)
	if err != nil {
		return Frontmatter{}, err
	}

	// Compute docID (hash of canonical URL)
	canonicalURLStr := canonicalURL.String()
	docIDHash, hashErr := hashutil.HashBytes([]byte(canonicalURLStr), normalizeParam.hashAlgo)
	if hashErr != nil {
		return Frontmatter{}, &NormalizationError{
			Message:   fmt.Sprintf("failed to compute doc_id: %v", hashErr),
			Retryable: false,
			Cause:     ErrCauseHashComputationFailed,
		}
	}
	docID := string(normalizeParam.hashAlgo) + ":" + docIDHash

	// Compute contentHash (hash of markdown content)
	contentHashValue, hashErr := hashutil.HashBytes(content, normalizeParam.hashAlgo)
	if hashErr != nil {
		return Frontmatter{}, &NormalizationError{
			Message:   fmt.Sprintf("failed to compute content_hash: %v", hashErr),
			Retryable: false,
			Cause:     ErrCauseHashComputationFailed,
		}
	}
	contentHash := string(normalizeParam.hashAlgo) + ":" + contentHashValue

	// Gather remaining fields from normalizeParam
	fetchedAt := normalizeParam.fetchedAt
	crawlerVersion := normalizeParam.appVersion
	crawlDepth := normalizeParam.crawlDepth

	// Construct immutable Frontmatter
	return NewFrontmatter(
		title,
		sourceURL,
		canonicalURLStr,
		crawlDepth,
		section,
		docID,
		contentHash,
		fetchedAt,
		crawlerVersion,
	), nil
}

// deriveSection extracts the first meaningful path segment from the URL.
// Per frontmatter.md Section 4, section is derived from the first path segment
// after stripping any matching allowedPathPrefix.
//
// Algorithm:
// 1. Check if path starts with any allowedPathPrefix (case-sensitive, exact match)
// 2. If yes, strip that prefix from path
// 3. Take the first remaining path segment as the section
// 4. If no prefix matches, use the first segment of the full path
func deriveSection(canonicalURL url.URL, allowedPathPrefixes []string) (string, failure.ClassifiedError) {
	path := canonicalURL.Path
	if path == "" || path == "/" {
		return "", &NormalizationError{
			Message:   "URL path is empty, cannot derive section",
			Retryable: false,
			Cause:     ErrCauseSectionDerivationFailed,
		}
	}

	// Try to strip matching allowedPathPrefix
	for _, prefix := range allowedPathPrefixes {
		if prefix == "" {
			continue
		}
		// Ensure prefix starts with /
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		// Check if path starts with this prefix
		if strings.HasPrefix(path, prefix) {
			// Strip the prefix
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}

	// Remove leading slash and split by /
	path = strings.TrimPrefix(path, "/")

	// If nothing remains after stripping prefix, error
	if path == "" {
		return "", &NormalizationError{
			Message:   "URL path has no segments after stripping allowedPathPrefix",
			Retryable: false,
			Cause:     ErrCauseSectionDerivationFailed,
		}
	}

	segments := strings.Split(path, "/")

	// Return first non-empty segment
	for _, segment := range segments {
		if segment != "" {
			return segment, nil
		}
	}

	return "", &NormalizationError{
		Message:   "URL path has no valid segments",
		Retryable: false,
		Cause:     ErrCauseSectionDerivationFailed,
	}
}

// extractTitle extracts the title from the first H1 heading in markdown content.
// Per frontmatter.md, title must come from the top-most H1 heading.
// This function assumes validateStructure has already ensured exactly one H1 exists.
func extractTitle(content []byte) (string, failure.ClassifiedError) {
	lines := bytes.Split(content, []byte("\n"))

	for _, line := range lines {
		line = bytes.TrimSpace(line)

		// Look for H1: line starts with "# "
		if bytes.HasPrefix(line, []byte("# ")) {
			// Extract text after "# "
			title := string(line[2:])

			// Strip inline markdown formatting
			title = stripInlineMarkdown(title)

			// Trim whitespace
			title = strings.TrimSpace(title)

			if title == "" {
				return "", &NormalizationError{
					Message:   "H1 heading contains no text",
					Retryable: false,
					Cause:     ErrCauseTitleExtractionFailed,
				}
			}

			return title, nil
		}
	}

	// This should not happen if validateStructure passed
	return "", &NormalizationError{
		Message:   "no H1 heading found in document",
		Retryable: false,
		Cause:     ErrCauseTitleExtractionFailed,
	}
}

// stripInlineMarkdown removes common inline markdown formatting from text.
func stripInlineMarkdown(text string) string {
	// Remove backticks (inline code)
	text = strings.ReplaceAll(text, "`", "")

	// Remove bold markers
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")

	// Remove italic markers
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", "")

	// Remove link text markers but keep the text
	// This is a simplified approach - removes [ and ] characters
	text = strings.ReplaceAll(text, "[", "")
	text = strings.ReplaceAll(text, "]", "")

	return text
}
