package normalize

import (
	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/config"
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
	cfg                 config.Config
	assetfulMarkdownDoc assets.AssetfulMarkdownDoc
}

func NewMarkdownConstraint(
	cfg config.Config,
	assetfulMarkdownDoc assets.AssetfulMarkdownDoc,
) NormalizedMarkdownDoc {
	return NormalizedMarkdownDoc{}
}
