package mdconvert

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/PuerkitoBio/goquery"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
	"github.com/rohmanhakim/docs-crawler/pkg/debug"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"golang.org/x/net/html"
)

/*
Design Principles
- Semantic fidelity over visual fidelity
- No inferred structure
- No code reformatting
- GitHub-Flavored Markdown compatibility

Conversion Rules
- Headings map directly (h1-h6 to # - ######)
- Code blocks preserved verbatim
- Tables converted structurally (GFM)
- Links and images preserved as-is (no resolution)
- DOM order preserved

Inline styles and raw HTML are avoided.
*/

// ConvertRule defines the interface for converting sanitized HTML to Markdown.
// Implementations must ensure semantic fidelity and deterministic output.
type ConvertRule interface {
	Convert(sanitizedHTMLDoc sanitizer.SanitizedHTMLDoc, pageURL string) (ConversionResult, failure.ClassifiedError)
}

// Compile-time interface check
var _ ConvertRule = (*StrictConversionRule)(nil)

type StrictConversionRule struct {
	metadataSink metadata.MetadataSink
	debugLogger  debug.DebugLogger
}

func NewRule(metadataSink metadata.MetadataSink) *StrictConversionRule {
	return &StrictConversionRule{
		metadataSink: metadataSink,
		debugLogger:  debug.NewNoOpLogger(),
	}
}

// SetDebugLogger sets the debug logger for the converter.
// This is optional and defaults to NoOpLogger.
// If logger is nil, NoOpLogger is used as a safe default.
func (s *StrictConversionRule) SetDebugLogger(logger debug.DebugLogger) {
	if logger == nil {
		s.debugLogger = debug.NewNoOpLogger()
		return
	}
	s.debugLogger = logger
}

func (s *StrictConversionRule) Convert(
	sanitizedHTMLDoc sanitizer.SanitizedHTMLDoc,
	pageURL string,
) (ConversionResult, failure.ClassifiedError) {
	consversionResult, err := s.convert(sanitizedHTMLDoc.GetContentNode())
	if err != nil {
		var conversionError *ConversionError
		errors.As(err, &conversionError)

		s.metadataSink.RecordError(metadata.NewErrorRecord(
			time.Now(),
			"mdconvert",
			"StrictConversionRule.Convert",
			mapConversionErrorToMetadataCause(*conversionError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, pageURL),
			},
		))
		return ConversionResult{}, conversionError
	}

	s.metadataSink.RecordPipelineStage(metadata.NewPipelineEvent(
		metadata.StageConvert,
		pageURL,
		true,
		time.Now(),
		0,
	))
	return consversionResult, nil
}

// convert transforms a sanitized HTML node into a ConversionResult containing markdown content.
// It uses the html-to-markdown/v2 library for deterministic, semantic conversion.
func (s *StrictConversionRule) convert(htmlDoc *html.Node) (ConversionResult, *ConversionError) {
	// Handle nil node gracefully
	if htmlDoc == nil {
		return ConversionResult{}, NewConversionError(
			ErrCauseConversionFailure,
			"cannot convert nil HTML node",
		)
	}

	// Log converter creation if debug enabled
	if s.debugLogger.Enabled() {
		s.debugLogger.LogStep(context.TODO(), "mdconvert", "create_converter", debug.FieldMap{
			"plugins": []string{"base", "commonmark", "table"},
		})
	}

	// Create a converter with plugins for commonmark, base, and table support
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
		),
	)

	// Convert the HTML node to markdown
	markdown, err := conv.ConvertNode(htmlDoc)
	if err != nil {
		return ConversionResult{}, NewConversionError(
			ErrCauseConversionFailure,
			err.Error(),
		)
	}

	// Extract link refs from the HTML document using goquery
	linkRefs := extractLinkRefs(htmlDoc)

	// Log successful conversion if debug enabled
	if s.debugLogger.Enabled() {
		s.debugLogger.LogStep(context.TODO(), "mdconvert", "build_markdown", debug.FieldMap{
			"output_size_bytes": len(markdown),
			"links_count":       len(linkRefs),
		})
	}

	return NewConversionResult(markdown, linkRefs), nil
}

// extractLinkRefs walks the HTML DOM and extracts all link references.
// It finds <a> tags with href attributes and <img> tags with src attributes.
// LinkRefs are returned in document order.
func extractLinkRefs(htmlDoc *html.Node) []LinkRef {
	var linkRefs []LinkRef

	// Create goquery document from the HTML node
	doc := goquery.NewDocumentFromNode(htmlDoc)

	// Find all anchor tags with href attributes and image tags with src attributes
	// Using a single selector to preserve document order
	doc.Find("a[href], img[src]").Each(func(i int, s *goquery.Selection) {
		tagName := goquery.NodeName(s)
		switch tagName {
		case "a":
			href, exists := s.Attr("href")
			if exists {
				linkRef := toLinkRef("a", href)
				linkRefs = append(linkRefs, linkRef)
			}
		case "img":
			src, exists := s.Attr("src")
			if exists {
				linkRef := toLinkRef("img", src)
				linkRefs = append(linkRefs, linkRef)
			}
		}
	})

	return linkRefs
}

// toLinkRef creates a LinkRef from a tag name and raw URL value.
// It classifies the link based on tag type and URL pattern.
func toLinkRef(tagName, raw string) LinkRef {
	tagName = strings.ToLower(tagName)

	// Determine LinkKind based on tag and URL pattern
	var kind LinkKind
	switch tagName {
	case "img":
		kind = KindImage
	case "a":
		if strings.HasPrefix(raw, "#") {
			kind = KindAnchor
		} else {
			kind = KindNavigation
		}
	default:
		kind = KindNavigation
	}

	return NewLinkRef(raw, kind)
}
