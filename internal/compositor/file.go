package compositor

import (
	"github.com/fmbfs/skein/internal/lsp"
)

// FileCompositor builds a FileMap for a file thread: every symbol defined
// in it, nested by lexical containment.
type FileCompositor struct {
	base
}

// NewFileCompositor constructs a FileCompositor against a real clangd client.
func NewFileCompositor(client *lsp.Client, rootDir string) *FileCompositor {
	return &FileCompositor{base{Client: client, RootDir: rootDir}}
}

// Build returns every symbol defined in path, with members nested under
// their containing class/struct.
func (f *FileCompositor) Build(path string) (*FileMap, error) {
	if err := f.openFile(path); err != nil {
		return nil, err
	}

	symbols, err := f.Client.DocumentSymbol(path)
	if err != nil {
		return nil, err
	}

	return &FileMap{
		ThreadName: f.relPath(path),
		Kind:       "file",
		Symbols:    nestByContainment(symbols),
	}, nil
}

// nestByContainment groups a flat documentSymbol result into a tree by
// range containment: any symbol whose range falls inside a container-kind
// symbol's range (class/struct/namespace) becomes that container's child.
// Each symbol is assigned to its smallest enclosing container (so a method
// inside a nested class doesn't get attached to the outer class). Top-level
// symbols (not contained by anything) stay at the root. Original source
// order is preserved at every level.
//
// One level of grandchildren (a class nested inside a class) collapses into
// the outer class's direct children for v0.1 — acceptable for the common
// case; full recursive nesting is future work.
func nestByContainment(symbols []lsp.SymbolInformation) []Member {
	parent := make([]int, len(symbols)) // index into symbols, or -1 for top-level
	for i := range parent {
		parent[i] = -1
	}

	for i, s := range symbols {
		if !isContainerKind(s.Kind) {
			continue
		}
		for j, candidate := range symbols {
			if j == i || !rangeContains(s.Location.Range, candidate.Location.Range) {
				continue
			}
			// Smallest enclosing container: prefer i over whatever's
			// currently assigned if i's range is tighter (nested inside it).
			if parent[j] == -1 || rangeContains(symbols[parent[j]].Location.Range, s.Location.Range) {
				parent[j] = i
			}
		}
	}

	children := make(map[int][]int)
	var roots []int
	for i := range symbols {
		if parent[i] == -1 {
			roots = append(roots, i)
		} else {
			children[parent[i]] = append(children[parent[i]], i)
		}
	}

	var build func(i int) Member
	build = func(i int) Member {
		m := Member{Name: symbols[i].Name, Kind: kindName(symbols[i].Kind)}
		for _, c := range children[i] {
			m.Children = append(m.Children, build(c))
		}
		return m
	}

	members := make([]Member, 0, len(roots))
	for _, i := range roots {
		members = append(members, build(i))
	}
	return members
}

func isContainerKind(k lsp.SymbolKind) bool {
	return k == lsp.SymbolKindClass || k == lsp.SymbolKindStruct || k == lsp.SymbolKindNamespace
}
