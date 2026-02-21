package treeprinter

import (
	"bytes"
	"strings"
	"testing"
)

func TestTreePrinter_SingleParentWithChildren(t *testing.T) {
	var buf bytes.Buffer
	tp := NewTreePrinterWithWriter(&buf)

	tp.StartParent("[FETCH] https://example.com/page.html")
	tp.AddChild("[extract] - success")
	tp.AddChild("[sanitize] - success")
	tp.AddChild("[convert] - success")
	tp.EndParent()

	expected := `[FETCH] https://example.com/page.html
├── [extract] - success
├── [sanitize] - success
└── [convert] - success
`

	if got := buf.String(); got != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestTreePrinter_MultipleParents(t *testing.T) {
	var buf bytes.Buffer
	tp := NewTreePrinterWithWriter(&buf)

	// First parent
	tp.StartParent("[FETCH] https://example.com/page1.html")
	tp.AddChild("[extract] - success")
	tp.EndParent()

	// Second parent
	tp.StartParent("[FETCH] https://example.com/page2.html")
	tp.AddChild("[extract] - success")
	tp.AddChild("[sanitize] - failed")
	tp.EndParent()

	expected := `[FETCH] https://example.com/page1.html
└── [extract] - success
[FETCH] https://example.com/page2.html
├── [extract] - success
└── [sanitize] - failed
`

	if got := buf.String(); got != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestTreePrinter_EmptyParent(t *testing.T) {
	var buf bytes.Buffer
	tp := NewTreePrinterWithWriter(&buf)

	tp.StartParent("[FETCH] https://example.com/page.html")
	tp.EndParent()

	expected := "[FETCH] https://example.com/page.html\n"

	if got := buf.String(); got != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestTreePrinter_StandalonePrint(t *testing.T) {
	var buf bytes.Buffer
	tp := NewTreePrinterWithWriter(&buf)

	tp.StartParent("[FETCH] https://example.com/page.html")
	tp.AddChild("[extract] - success")
	tp.EndParent()

	tp.PrintStandalone("[SKIP] https://example.com/admin - robots_disallow")

	// Another parent after standalone
	tp.StartParent("[FETCH] https://example.com/other.html")
	tp.AddChild("[extract] - success")
	tp.EndParent()

	expected := `[FETCH] https://example.com/page.html
└── [extract] - success
[SKIP] https://example.com/admin - robots_disallow
[FETCH] https://example.com/other.html
└── [extract] - success
`

	if got := buf.String(); got != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestTreePrinter_Flush(t *testing.T) {
	var buf bytes.Buffer
	tp := NewTreePrinterWithWriter(&buf)

	tp.StartParent("[FETCH] https://example.com/page.html")
	tp.AddChild("[extract] - success")
	// Don't call EndParent, just Flush
	tp.Flush()

	expected := `[FETCH] https://example.com/page.html
└── [extract] - success
`

	if got := buf.String(); got != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestTreePrinter_FormatArgs(t *testing.T) {
	var buf bytes.Buffer
	tp := NewTreePrinterWithWriter(&buf)

	tp.StartParent("[FETCH] %s - %d", "https://example.com/page.html", 200)
	tp.AddChild("[%s] - %s", "extract", "success")
	tp.EndParent()

	expected := `[FETCH] https://example.com/page.html - 200
└── [extract] - success
`

	if got := buf.String(); got != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestTreeNode_Render(t *testing.T) {
	root := NewTreeNode("[FETCH] https://example.com/page.html")
	root.AddChild(NewTreeNode("[extract] - success"))
	root.AddChild(NewTreeNode("[sanitize] - success"))
	convertChild := root.AddChild(NewTreeNode("[convert] - success"))
	convertChild.AddChild(NewTreeNode("[ARTIFACT] image.png"))
	root.AddChild(NewTreeNode("[normalize] - success"))

	var buf bytes.Buffer
	root.Render(&buf, "", true)

	output := buf.String()
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")

	// Verify structure
	if len(lines) != 6 {
		t.Errorf("expected 6 lines, got %d: %v", len(lines), lines)
	}

	// Root gets └── prefix when isLast=true
	if !strings.HasPrefix(lines[0], "└── [FETCH]") {
		t.Errorf("expected └── prefix for root when isLast=true, got: %s", lines[0])
	}

	// First children should have ├── or └──
	if !strings.HasPrefix(lines[1], "    ├── [extract]") {
		t.Errorf("expected '    ├──' prefix, got: %s", lines[1])
	}

	// Last top-level child should have └──
	if !strings.HasPrefix(lines[5], "    └── [normalize]") {
		t.Errorf("expected '    └──' prefix for last child, got: %s", lines[5])
	}

	// Nested child should have │   prefix
	if !strings.HasPrefix(lines[4], "    │   └── [ARTIFACT]") {
		t.Errorf("expected '    │   └──' prefix for nested child, got: %s", lines[4])
	}
}

func TestTreeNode_String(t *testing.T) {
	root := NewTreeNode("root")
	root.AddChild(NewTreeNode("child1"))
	root.AddChild(NewTreeNode("child2"))

	output := root.String()

	if !strings.Contains(output, "root") {
		t.Error("output should contain 'root'")
	}
	if !strings.Contains(output, "child1") {
		t.Error("output should contain 'child1'")
	}
	if !strings.Contains(output, "child2") {
		t.Error("output should contain 'child2'")
	}
}

func TestTreePrinter_NewlineTrimming(t *testing.T) {
	var buf bytes.Buffer
	tp := NewTreePrinterWithWriter(&buf)

	// Content with trailing newline should be trimmed
	tp.StartParent("[FETCH] https://example.com/page.html\n")
	tp.AddChild("[extract] - success\n")
	tp.EndParent()

	// Should only have one newline per line
	expected := `[FETCH] https://example.com/page.html
└── [extract] - success
`

	if got := buf.String(); got != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}
