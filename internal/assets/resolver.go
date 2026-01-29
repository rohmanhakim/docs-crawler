package assets

import (
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
)

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
type Resolver struct {
	cfg         config.Config
	markdownDoc mdconvert.MarkdownDoc
}

func NewResolver(
	cfg config.Config,
	markdownDoc mdconvert.MarkdownDoc,
) AssetfulMarkdownDoc {
	return AssetfulMarkdownDoc{}
}
