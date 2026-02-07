package extractor

// KnownDocSelectors contains framework-specific documentation container selectors.
// These are used as Layer 2 heuristic when semantic containers (Layer 1) fail.
//
// Each slice contains selectors for a specific documentation framework or platform,
// ordered by specificity and reliability based on trained data.
//
//nolint:gochecknoglobals // This is a static lookup table that must be global
var KnownDocSelectors = map[string][]string{
	"generic": {
		// Core documentation selectors (framework-agnostic)
		".content",
		".doc-content",
		".markdown-body",
		"#docs-content",
		".rst-content",
		".theme-doc-markdown",
		".md-content",
	},
	"docusaurus": {
		// Docusaurus (Meta/Facebook documentation framework)
		".theme-doc-markdown",
		".docMainContainer",
	},
	"gitbook": {
		// GitBook platform
		".book-body",
		".markdown-section",
	},
	"mkdocs": {
		// MkDocs (Python-based)
		".md-content",
		".md-main__inner",
	},
	"sphinx": {
		// Sphinx/ReadTheDocs (Python documentation)
		".rst-content",
		".document",
	},
	"vuepress": {
		// VuePress (Vue.js documentation)
		".theme-default-content",
		".content__default",
	},
	"docsify": {
		// Docsify (client-side rendering)
		"#main",
		".content",
	},
	"hexo": {
		// Hexo blog framework (often used for docs)
		".post-content",
		".article-content",
	},
	"jekyll": {
		// Jekyll (GitHub Pages)
		".post-content",
		".entry-content",
	},
}

// getAllSelectors returns a flattened, prioritized list of all known documentation selectors.
// Order matters: generic selectors are checked first, then framework-specific in priority order.
func getAllSelectors() []string {
	// Priority order for framework categories
	frameworkOrder := []string{
		"generic",
		"docusaurus",
		"sphinx",
		"mkdocs",
		"gitbook",
		"vuepress",
		"docsify",
		"hexo",
		"jekyll",
	}

	var allSelectors []string
	seen := make(map[string]bool)

	for _, framework := range frameworkOrder {
		selectors := KnownDocSelectors[framework]
		for _, selector := range selectors {
			if !seen[selector] {
				seen[selector] = true
				allSelectors = append(allSelectors, selector)
			}
		}
	}

	return allSelectors
}

// mergeSelectors combines default selectors with user-provided custom selectors,
// deduplicating to ensure each selector appears only once.
func mergeSelectors(defaultSelectors, customSelectors []string) []string {
	seen := make(map[string]bool)
	var merged []string

	// Add defaults first
	for _, selector := range defaultSelectors {
		if !seen[selector] {
			seen[selector] = true
			merged = append(merged, selector)
		}
	}

	// Add custom selectors, skipping duplicates
	for _, selector := range customSelectors {
		if !seen[selector] {
			seen[selector] = true
			merged = append(merged, selector)
		}
	}

	return merged
}
