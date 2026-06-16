// Package compositor assembles raw clangd LSP responses into RelationMaps —
// the composed relationship views that skein renders. This is where skein's
// core value lives: composing multiple point queries into one shape query.
//
// Vocabulary: a thread is the focal symbol; a bundle is a pinned collection;
// ply is the traversal depth. See docs/SPEC.md section 3.
package compositor

// Location is a file path (relative to the workspace root when possible)
// plus a 1-indexed line number, ready for display.
type Location struct {
	Path string
	Line int
}

// CalledInGroup is the set of call-site lines for one caller file.
type CalledInGroup struct {
	File  string
	Lines []int
}

// RelationMap is the composed, render-ready view of a thread's relationships.
type RelationMap struct {
	ThreadName string
	Kind       string // "method", "function", "class", "file"
	Ply        int
	DefinedAt  Location
	Signature  string // source line at DefinedAt, if available
	CalledIn   []CalledInGroup
	Calls      []string
}

// CalledInTotal returns the total number of call sites across all files.
func (m *RelationMap) CalledInTotal() int {
	total := 0
	for _, g := range m.CalledIn {
		total += len(g.Lines)
	}
	return total
}

// Member is a symbol nested under a class or file thread (a method, field,
// or — in file mode — a class containing its own members).
type Member struct {
	Name     string
	Kind     string // "method", "field", "class", "struct", "function", ...
	Children []Member
}

// ClassMap is the composed, render-ready view of a class thread: its
// location, base/derived classes, and members.
type ClassMap struct {
	ThreadName  string
	Kind        string // "class"
	DefinedAt   Location
	Inherits    []string
	InheritedBy []string
	Members     []Member
}

// FileMap is the composed, render-ready view of a file thread: every symbol
// defined in it, nested by lexical containment (e.g. a class's methods
// appear under that class).
type FileMap struct {
	ThreadName string // file path
	Kind       string // "file"
	Symbols    []Member
}
