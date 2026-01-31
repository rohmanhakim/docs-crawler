package mdconvert

import (
	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/sanitizer"
)

/*
Design Principles
- Semantic fidelity over visual fidelity
- No inferred structure
- No code reformatting
- GitHub-Flavored Markdown compatibility

Conversion Rules
- One H1 per document
- Headings map directly
- Code blocks preserved verbatim
- Tables converted structurally
- Links and images rewritten as relative paths

Inline styles and raw HTML are avoided.
*/

type Rule struct {
	cfg              config.Config
	sanitizedHTMLDoc sanitizer.SanitizedHTMLDoc
}

func NewRule() Rule {
	return Rule{}
}

func (r *Rule) Convert(
	sanitizedHTMLDoc sanitizer.SanitizedHTMLDoc,
) MarkdownDoc {
	return convert()
}

func convert() MarkdownDoc {
	return MarkdownDoc{}
}
