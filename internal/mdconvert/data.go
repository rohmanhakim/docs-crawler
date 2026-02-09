package mdconvert

// Representation

type ConversionResult struct {
	markdownContent []byte
	linkRefs        []LinkRef
}

func NewConversionResult(
	markdownContent []byte,
	linkRefs []LinkRef,
) ConversionResult {
	return ConversionResult{
		markdownContent: markdownContent,
		linkRefs:        linkRefs,
	}
}

func (c *ConversionResult) GetMarkdownContent() []byte {
	return c.markdownContent
}

func (c *ConversionResult) GetLinkRefs() []LinkRef {
	return c.linkRefs
}

type LinkKind string

const (
	KindNavigation LinkKind = "navigation"
	KindImage      LinkKind = "image"
	KindAnchor     LinkKind = "anchor"
)

type LinkRef struct {
	raw  string
	kind LinkKind
}

func NewLinkRef(
	raw string,
	kind LinkKind,
) LinkRef {
	return LinkRef{
		raw:  raw,
		kind: kind,
	}
}

func (l *LinkRef) GetRaw() string {
	return l.raw
}

func (l *LinkRef) GetKind() LinkKind {
	return l.kind
}
