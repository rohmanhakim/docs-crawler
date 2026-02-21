// Package treeprinter provides a utility for printing tree-structured output
// with ASCII tree symbols.
package treeprinter

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	treeBranch  = "├── "
	treeLeaf    = "└── "
	treeIndent  = "│   "
	emptyIndent = "    "
)

// Node represents a single node in the tree output.
type Node struct {
	content  string
	children []Node
	isLast   bool
}

// TreePrinter manages tree-structured output with parent-child relationships.
// It buffers children until the parent is complete, then prints everything
// in the correct order with proper indentation.
type TreePrinter struct {
	mu       sync.Mutex
	writer   io.Writer
	current  *Node
	children []string // buffered child lines for current parent
}

// NewTreePrinter creates a new TreePrinter that writes to stdout.
func NewTreePrinter() *TreePrinter {
	return &TreePrinter{
		writer: os.Stdout,
	}
}

// NewTreePrinterWithWriter creates a new TreePrinter with a custom writer.
func NewTreePrinterWithWriter(w io.Writer) *TreePrinter {
	return &TreePrinter{
		writer: w,
	}
}

// StartParent begins a new parent node with the given content.
// Any previous parent's children are finalized before starting the new parent.
// The format string and args follow fmt.Printf conventions.
func (tp *TreePrinter) StartParent(format string, args ...any) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Finalize any previous parent's children
	tp.flushChildren()

	// Print the parent line
	tp.printLine(format, args...)

	// Reset children buffer for new parent
	tp.children = nil
}

// AddChild adds a child node to the current parent.
// Children are buffered until EndParent() is called or a new parent starts.
// The format string and args follow fmt.Printf conventions.
func (tp *TreePrinter) AddChild(format string, args ...any) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	content := fmt.Sprintf(format, args...)
	tp.children = append(tp.children, content)
}

// EndParent finalizes the current parent's children, printing them
// with proper tree prefixes (├── for all but last, └── for last).
func (tp *TreePrinter) EndParent() {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.flushChildren()
}

// Flush finalizes any remaining buffered output.
// Call this at the end of output to ensure all children are printed.
func (tp *TreePrinter) Flush() {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.flushChildren()
}

// flushChildren prints all buffered children with proper tree prefixes.
// Must be called with mutex held.
func (tp *TreePrinter) flushChildren() {
	n := len(tp.children)
	for i, child := range tp.children {
		isLast := i == n-1
		prefix := treeBranch
		if isLast {
			prefix = treeLeaf
		}
		tp.printLine("%s%s", prefix, child)
	}
	tp.children = nil
}

// printLine writes a line to the output writer.
// Must be called with mutex held.
func (tp *TreePrinter) printLine(format string, args ...any) {
	content := fmt.Sprintf(format, args...)
	// Ensure single newline at end
	content = strings.TrimSuffix(content, "\n")
	fmt.Fprintln(tp.writer, content)
}

// PrintStandalone prints a line outside the tree structure.
// Use for events like SKIP or STATS that don't belong to a parent.
func (tp *TreePrinter) PrintStandalone(format string, args ...any) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Finalize any pending children first
	tp.flushChildren()
	tp.printLine(format, args...)
}

// TreeNode provides a more flexible API for building arbitrary tree structures.
// This is useful when the tree structure is known ahead of time.
type TreeNode struct {
	content  string
	children []*TreeNode
}

// NewTreeNode creates a new tree node with the given content.
func NewTreeNode(content string) *TreeNode {
	return &TreeNode{
		content:  content,
		children: nil,
	}
}

// AddChild adds a child node and returns the child for chaining.
func (n *TreeNode) AddChild(child *TreeNode) *TreeNode {
	n.children = append(n.children, child)
	return child
}

// Render writes the tree node and its children to the writer.
// The prefix is used for indentation (empty for root).
// The isLast parameter determines the branch symbol.
func (n *TreeNode) Render(w io.Writer, prefix string, isLast bool) {
	// Print this node
	connector := treeBranch
	if isLast {
		connector = treeLeaf
	}
	fmt.Fprintf(w, "%s%s%s\n", prefix, connector, n.content)

	// Prepare prefix for children
	childPrefix := prefix
	if isLast {
		childPrefix += emptyIndent
	} else {
		childPrefix += treeIndent
	}

	// Render children
	for i, child := range n.children {
		childIsLast := i == len(n.children)-1
		child.Render(w, childPrefix, childIsLast)
	}
}

// String returns the tree as a string.
func (n *TreeNode) String() string {
	var sb strings.Builder
	n.Render(&sb, "", true)
	return sb.String()
}
