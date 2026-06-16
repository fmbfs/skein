package compositor

import (
	"fmt"

	"github.com/fmbfs/skein/internal/lsp"
)

// dedupeTypeNames returns the names from items, collapsing entries that
// share the same name and source location. clangd's type hierarchy can
// list the same base/derived class twice under distinct symbolIDs when a
// CRTP-style template is involved — confirmed empirically on a real-world
// class -> EnableMakeUnique<T> base: two different symbolIDs, identical name,
// URI, and range. Visually and semantically that's one entry to a skein
// user, regardless of why clangd's index has two.
func dedupeTypeNames(items []lsp.TypeHierarchyItem) []string {
	type key struct {
		name string
		uri  string
		rng  lsp.Range
	}
	seen := make(map[key]bool, len(items))
	var names []string
	for _, item := range items {
		k := key{item.Name, item.URI, item.Range}
		if seen[k] {
			continue
		}
		seen[k] = true
		names = append(names, item.Name)
	}
	return names
}

// ClassCompositor builds a ClassMap for a class/struct thread: where it's
// defined, its base/derived classes, and its members.
type ClassCompositor struct {
	base
}

// NewClassCompositor constructs a ClassCompositor against a real clangd client.
func NewClassCompositor(client *lsp.Client, rootDir string) *ClassCompositor {
	return &ClassCompositor{base{Client: client, RootDir: rootDir}}
}

// Build resolves name to a class/struct symbol and composes its ClassMap.
func (c *ClassCompositor) Build(name string) (*ClassMap, error) {
	_ = c.nudgeIndexer() // best-effort; findWorkspaceSymbol still retries either way

	symbols, err := c.findWorkspaceSymbol(name)
	if err != nil {
		return nil, fmt.Errorf("workspace/symbol %q: %w", name, err)
	}

	var sym *lsp.SymbolInformation
	for i := range symbols {
		if symbols[i].Name == name && (symbols[i].Kind == lsp.SymbolKindClass || symbols[i].Kind == lsp.SymbolKindStruct) {
			sym = &symbols[i]
			break
		}
	}
	if sym == nil {
		return nil, fmt.Errorf("no class or struct named %q found in workspace", name)
	}

	path, err := lsp.URIToPath(sym.Location.URI)
	if err != nil {
		return nil, err
	}
	if err := c.openFile(path); err != nil {
		return nil, err
	}

	cm := &ClassMap{
		ThreadName: name,
		Kind:       kindName(sym.Kind),
		DefinedAt: Location{
			Path: c.relPath(path),
			Line: sym.Location.Range.Start.Line + 1,
		},
	}

	items, err := c.Client.PrepareTypeHierarchy(path, sym.Location.Range.Start)
	if err == nil && len(items) > 0 {
		item := items[0]
		if super, err := c.Client.Supertypes(item); err == nil {
			cm.Inherits = dedupeTypeNames(super)
		}
		if sub, err := c.Client.Subtypes(item); err == nil {
			cm.InheritedBy = dedupeTypeNames(sub)
		}
	}

	docSymbols, err := c.Client.DocumentSymbol(path)
	if err == nil {
		// workspace/symbol gives a narrow name-span range (just enough to
		// anchor textDocument/* point queries); documentSymbol separately
		// reports the full body extent, which is what containment checks
		// need. Find the matching full-extent entry rather than reusing
		// sym.Location.Range directly.
		container := containerRange(docSymbols, name, sym.Location.Range.Start)
		cm.Members = membersWithin(docSymbols, container, name)
	}

	return cm, nil
}

// containerRange finds the documentSymbol entry named name whose range
// contains atPos, and returns its (full-extent) range. Falls back to a
// zero Range if no match is found.
func containerRange(all []lsp.SymbolInformation, name string, atPos lsp.Position) lsp.Range {
	for _, s := range all {
		if s.Name != name {
			continue
		}
		if posWithin(s.Location.Range, atPos) {
			return s.Location.Range
		}
	}
	return lsp.Range{}
}

func posWithin(r lsp.Range, p lsp.Position) bool {
	afterStart := p.Line > r.Start.Line || (p.Line == r.Start.Line && p.Character >= r.Start.Character)
	beforeEnd := p.Line < r.End.Line || (p.Line == r.End.Line && p.Character <= r.End.Character)
	return afterStart && beforeEnd
}

// membersWithin returns the symbols from all (a flat documentSymbol result)
// that fall strictly inside container, excluding the container itself and
// any symbol with the same name+range as the container (some servers list
// nested-class members alongside the class; v0.1 takes the simple
// containment-only view rather than tracking lexical depth precisely).
func membersWithin(all []lsp.SymbolInformation, container lsp.Range, containerName string) []Member {
	var members []Member
	for _, s := range all {
		if s.Name == containerName && s.Location.Range == container {
			continue
		}
		if rangeContains(container, s.Location.Range) {
			members = append(members, Member{Name: s.Name, Kind: kindName(s.Kind)})
		}
	}
	return members
}

// rangeContains reports whether outer strictly contains inner (inner is not
// equal to outer — used to exclude a symbol from being its own member).
func rangeContains(outer, inner lsp.Range) bool {
	if inner == outer {
		return false
	}
	startsAfter := inner.Start.Line > outer.Start.Line ||
		(inner.Start.Line == outer.Start.Line && inner.Start.Character >= outer.Start.Character)
	endsBefore := inner.End.Line < outer.End.Line ||
		(inner.End.Line == outer.End.Line && inner.End.Character <= outer.End.Character)
	return startsAfter && endsBefore
}
