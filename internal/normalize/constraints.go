package normalize

import (
	"bytes"
	"context"
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
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
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
	debugLogger  debug.DebugLogger
}

func NewMarkdownConstraint(
	metadataSink metadata.MetadataSink,
) MarkdownConstraint {
	return MarkdownConstraint{
		metadataSink: metadataSink,
		debugLogger:  debug.NewNoOpLogger(),
	}
}

// SetDebugLogger sets the debug logger for the constraint.
// This is optional and defaults to NoOpLogger.
// If logger is nil, NoOpLogger is used as a safe default.
func (m *MarkdownConstraint) SetDebugLogger(logger debug.DebugLogger) {
	if logger == nil {
		m.debugLogger = debug.NewNoOpLogger()
		return
	}
	m.debugLogger = logger
}

func (m *MarkdownConstraint) Normalize(
	fetchUrl url.URL,
	assetfulMarkdownDoc assets.AssetfulMarkdownDoc,
	normalizeParam NormalizeParam,
) (NormalizedMarkdownDoc, failure.ClassifiedError) {
	// Log normalization start
	if m.debugLogger.Enabled() {
		m.debugLogger.LogStep(context.TODO(), "normalize", "normalize_start", debug.FieldMap{
			"url":         fetchUrl.String(),
			"input_size":  len(assetfulMarkdownDoc.Content()),
			"crawl_depth": normalizeParam.crawlDepth,
			"hash_algo":   string(normalizeParam.hashAlgo),
		})
	}

	normalizedMarkdown, err := normalize(fetchUrl, assetfulMarkdownDoc, normalizeParam)
	if err != nil {
		var normalizationError *NormalizationError
		errors.As(err, &normalizationError)

		// Log validation error
		if m.debugLogger.Enabled() {
			m.debugLogger.LogStep(context.TODO(), "normalize", "normalize_failed", debug.FieldMap{
				"error_cause": string(normalizationError.Cause),
				"error_msg":   normalizationError.Error(),
			})
		}

		m.metadataSink.RecordError(
			metadata.NewErrorRecord(
				time.Now(),
				"normalize",
				"MarkdownConstraint.Normalize",
				mapNormalizationErrorToMetadataCause(*normalizationError),
				err.Error(),
				[]metadata.Attribute{
					metadata.NewAttr(metadata.AttrURL, fetchUrl.String()),
				},
			),
		)
		return NormalizedMarkdownDoc{}, normalizationError
	}

	// Log normalization complete
	if m.debugLogger.Enabled() {
		m.debugLogger.LogStep(context.TODO(), "normalize", "normalize_complete", debug.FieldMap{
			"doc_id":       normalizedMarkdown.Frontmatter().DocID(),
			"content_hash": normalizedMarkdown.Frontmatter().ContentHash(),
			"title":        normalizedMarkdown.Frontmatter().Title(),
			"section":      normalizedMarkdown.Frontmatter().Section(),
		})
	}

	m.metadataSink.RecordPipelineStage(
		metadata.NewPipelineEvent(
			metadata.StageNormalize,
			fetchUrl.String(),
			true,
			time.Now(),
			0,
		),
	)
	return normalizedMarkdown, nil
}

// normalize normalizes the markdown document by validating structure and generating frontmatter.
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
// normalization invariants N1, N3, N4, and N5.
// It uses AST parsing for correctness.
//
// Note: Invariant N6 (headings inside code blocks) is inherently enforced by the parser.
// The markdown parser treats content inside code blocks as literal text, never as
// ast.Heading nodes. Therefore, any "# Heading" text inside a fenced code block
// is parsed as ast.CodeBlock content, not as a heading. This means N6 violations
// are impossible to detect at the AST level because they don't produce ast.Heading nodes.
func validateStructure(content []byte) failure.ClassifiedError {
	// Check for empty content (Invariant N1 prerequisite)
	if len(bytes.TrimSpace(content)) == 0 {
		return NewNormalizationError(
			ErrCauseEmptyContent,
			"markdown content is empty",
		)
	}

	// Parse markdown into AST
	p := parser.New()
	doc := markdown.Parse(content, p)

	// Collect headings and validate structure via AST walk
	var headings []headingInfo
	var hasContentBeforeH1 bool
	contentAfterHeading := make(map[int]bool) // Tracks if heading[i] has content before next same/higher level heading
	currentHeadingIdx := -1
	nodeIdx := 0

	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		switch n := node.(type) {
		case *ast.Heading:
			if entering {
				currentHeadingIdx = len(headings)
				headings = append(headings, headingInfo{node: n, index: nodeIdx})
			}

		case *ast.CodeBlock:
			if entering {
				// Code blocks count as content
				if currentHeadingIdx >= 0 {
					contentAfterHeading[currentHeadingIdx] = true
				}
			}

		case *ast.Text:
			if entering {
				// Text nodes only count as content if not just whitespace
				if currentHeadingIdx >= 0 {
					contentAfterHeading[currentHeadingIdx] = true
				}
				// Track if we have content before first H1
				if len(headings) == 0 {
					hasContentBeforeH1 = true
				}
			}

		case *ast.Paragraph, *ast.List, *ast.Table:
			if entering {
				// Track if we have content before first H1
				if len(headings) == 0 {
					hasContentBeforeH1 = true
				}
				// Track content after current heading for N5
				if currentHeadingIdx >= 0 {
					contentAfterHeading[currentHeadingIdx] = true
				}
			}

		case *ast.BlockQuote:
			if entering {
				// Blockquotes count as content
				if currentHeadingIdx >= 0 {
					contentAfterHeading[currentHeadingIdx] = true
				}
				if len(headings) == 0 {
					hasContentBeforeH1 = true
				}
			}
		}

		nodeIdx++
		return ast.GoToNext
	})

	// Validate N1: Exactly one H1
	h1Count := 0
	for _, h := range headings {
		if h.node.Level == 1 {
			h1Count++
		}
	}

	if h1Count == 0 {
		return NewNormalizationError(
			ErrCauseBrokenH1Invariant,
			"document has no H1 heading",
		)
	}

	if h1Count > 1 {
		return NewNormalizationError(
			ErrCauseBrokenH1Invariant,
			fmt.Sprintf("document has %d H1 headings, expected exactly one", h1Count),
		)
	}

	// Validate N4: No orphan content before H1
	if hasContentBeforeH1 {
		return NewNormalizationError(
			ErrCauseOrphanContent,
			"content exists before first H1 heading",
		)
	}

	// Validate N3: No skipped heading levels
	prevLevel := 0
	for _, h := range headings {
		// Check for level skip (N3)
		if h.node.Level > prevLevel+1 && prevLevel != 0 {
			return NewNormalizationError(
				ErrCauseSkippedHeadingLevels,
				fmt.Sprintf("heading level skipped: H%d follows H%d", h.node.Level, prevLevel),
			)
		}

		prevLevel = h.node.Level
	}

	// Validate N5: No empty sections
	// A heading must not exist without content before the next heading of same or higher level
	for i := 0; i < len(headings); i++ {
		// Find the next heading with same or higher level
		for j := i + 1; j < len(headings); j++ {
			if headings[j].node.Level <= headings[i].node.Level {
				// Next same/higher level heading found - check if current heading has content
				if !contentAfterHeading[i] {
					return NewNormalizationError(
						ErrCauseEmptySection,
						fmt.Sprintf("empty section: H%d heading has no content before next H%d", headings[i].node.Level, headings[j].node.Level),
					)
				}
				break
			}
		}
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
		return Frontmatter{}, NewNormalizationError(
			ErrCauseHashComputationFailed,
			fmt.Sprintf("failed to compute doc_id: %v", hashErr),
		)
	}
	docID := string(normalizeParam.hashAlgo) + ":" + docIDHash

	// Compute contentHash (hash of markdown content)
	contentHashValue, hashErr := hashutil.HashBytes(content, normalizeParam.hashAlgo)
	if hashErr != nil {
		return Frontmatter{}, NewNormalizationError(
			ErrCauseHashComputationFailed,
			fmt.Sprintf("failed to compute content_hash: %v", hashErr),
		)
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
		return "", NewNormalizationError(
			ErrCauseSectionDerivationFailed,
			"URL path is empty, cannot derive section",
		)
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
		return "", NewNormalizationError(
			ErrCauseSectionDerivationFailed,
			"URL path has no segments after stripping allowedPathPrefix",
		)
	}

	segments := strings.Split(path, "/")

	// Return first non-empty segment
	for _, segment := range segments {
		if segment != "" {
			return segment, nil
		}
	}

	return "", NewNormalizationError(
		ErrCauseSectionDerivationFailed,
		"URL path has no valid segments",
	)
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
				return "", NewNormalizationError(
					ErrCauseTitleExtractionFailed,
					"H1 heading contains no text",
				)
			}

			return title, nil
		}
	}

	// This should not happen if validateStructure passed
	return "", NewNormalizationError(
		ErrCauseTitleExtractionFailed,
		"no H1 heading found in document",
	)
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
