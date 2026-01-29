package storage

import (
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
)

/*
Responsibilities
- Persist Markdown files
- Write assets
- Ensure deterministic filenames

Output Characteristics
- Stable directory layout
- Idempotent writes
- Overwrite-safe reruns
*/

type Sink struct {
	cfg           config.Config
	normalizedDoc normalize.NormalizedMarkdownDoc
}

func NewSink(
	cfg config.Config,
	normalizedDoc normalize.NormalizedMarkdownDoc,
) WriteResult {
	return WriteResult{}
}
