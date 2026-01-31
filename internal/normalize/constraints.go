package normalize

import (
	"errors"
	"fmt"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal"
	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
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

RAG-Oriented Constraints
- Logical section boundaries preserved
- Code blocks and tables are atomic
- Chunk sizes predictable
*/

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
	assetfulMarkdownDoc assets.AssetfulMarkdownDoc,
) (NormalizedMarkdownDoc, internal.ClassifiedError) {
	normalizedMarkdown, err := normalize()
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
				metadata.NewAttr(metadata.AttrField, fmt.Sprintf("H1: %v", "the-violated-h1-invariant-value")),
			},
		)
		return NormalizedMarkdownDoc{}, normalizationError
	}
	return normalizedMarkdown, nil
}

func normalize() (NormalizedMarkdownDoc, error) {
	return NormalizedMarkdownDoc{}, nil
}
