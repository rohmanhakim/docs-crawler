package assets

import (
	"errors"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
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
	metadataSink metadata.MetadataSink
}

func NewResolver(
	metadataSink metadata.MetadataSink,
) Resolver {
	return Resolver{
		metadataSink: metadataSink,
	}
}

func (r *Resolver) Resolve(
	markdownDoc mdconvert.MarkdownDoc,
) (AssetfulMarkdownDoc, failure.ClassifiedError) {
	assetfulMarkdownDoc, err := resolve()
	if err != nil {
		var assetsError *AssetsError
		errors.As(err, &assetsError)
		r.metadataSink.RecordError(
			time.Now(),
			"assets",
			"Resolver.Resolve",
			mapAssetsErrorToMetadataCause(*assetsError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrAssetURL, "https://the-failed-image-url"),
			},
		)
		return AssetfulMarkdownDoc{}, assetsError
	}
	return assetfulMarkdownDoc, nil
}

func resolve() (AssetfulMarkdownDoc, error) {
	return AssetfulMarkdownDoc{}, nil
}
