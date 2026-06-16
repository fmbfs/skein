package compositor

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/fmbfs/skein/internal/lsp"
)

// MethodCompositor builds a RelationMap for a method/function thread:
// where it's defined, who calls it, and what it calls. (See README/SPEC —
// draw mode for a method does not include type-hierarchy/"inherits"; that's
// TUI-only.)
type MethodCompositor struct {
	base
}

// NewMethodCompositor constructs a MethodCompositor against a real clangd client.
func NewMethodCompositor(client *lsp.Client, rootDir string) *MethodCompositor {
	return &MethodCompositor{base{Client: client, RootDir: rootDir}}
}

// Build resolves name to a method/function symbol and composes its RelationMap.
func (m *MethodCompositor) Build(name string, ply int) (*RelationMap, error) {
	_ = m.nudgeIndexer() // best-effort; findWorkspaceSymbol still retries either way

	symbols, err := m.findWorkspaceSymbol(name)
	if err != nil {
		return nil, fmt.Errorf("workspace/symbol %q: %w", name, err)
	}

	var candidates []lsp.SymbolInformation
	for _, s := range symbols {
		if s.Name == name && (s.Kind == lsp.SymbolKindMethod || s.Kind == lsp.SymbolKindFunction) {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no method or function named %q found in workspace", name)
	}

	// Multiple classes can declare the same method name (e.g. an interface
	// and its implementation). Prefer a candidate whose definition resolves
	// into a source file — that's the concrete implementation, not just a
	// declaration. Falls back to the first candidate otherwise. Full
	// disambiguation (by class) is future scope, see SPEC.md `-c` flag.
	defPath, defPos, err := m.bestDefinition(candidates)
	if err != nil {
		return nil, err
	}

	items, err := m.Client.PrepareCallHierarchy(defPath, defPos)
	if err != nil {
		return nil, fmt.Errorf("prepareCallHierarchy for %q: %w", name, err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("clangd returned no call-hierarchy item for %q", name)
	}
	item := items[0]

	rm := &RelationMap{
		ThreadName: name,
		Kind:       "method",
		Ply:        ply,
		DefinedAt: Location{
			Path: m.relPath(defPath),
			Line: defPos.Line + 1,
		},
		Signature: sourceLine(defPath, defPos.Line),
	}

	if incoming, err := m.Client.IncomingCalls(item); err == nil {
		rm.CalledIn = groupIncoming(incoming, m.relPath)
	}
	rm.Calls = m.outgoingCallNames(item, name)

	return rm, nil
}

// outgoingCallNames prefers the real callHierarchy/outgoingCalls LSP method,
// but falls back to a textual scan of the function body when it's
// unavailable. As of clangd 18.1.8, outgoingCalls is unimplemented
// (responds "method not found", -32601) even though incomingCalls works —
// confirmed empirically against tests/fixtures/simple_cpp, not documented
// anywhere skein could have caught at design time. The fallback is a regex
// over `identifier(` shapes in the item's source range: it can't qualify
// names (no "Class::") and will misfire on macros/casts that look like
// calls, but it's a reasonable v0.1 approximation until clangd implements
// the real thing. ownName is the bare (unqualified) name originally
// searched for — item.Name is qualified (e.g. "Pipeline::processFrame") and
// won't match the regex's bare-identifier capture, so it can't be used to
// exclude the function's own signature line from the scan.
func (m *MethodCompositor) outgoingCallNames(item lsp.CallHierarchyItem, ownName string) []string {
	if outgoing, err := m.Client.OutgoingCalls(item); err == nil && len(outgoing) > 0 {
		return formatOutgoing(outgoing)
	}
	path, err := lsp.URIToPath(item.URI)
	if err != nil {
		return nil
	}
	return scanCallExpressions(path, item.Range, ownName)
}

// bestDefinition prefers a candidate whose own workspace/symbol location is
// already in a source file (.cpp/.cc/.cxx) — that's the concrete
// implementation. Only for header-only candidates (likely a declaration) does
// it attempt textDocument/definition to jump to a source-file body.
//
// Note: textDocument/definition has toggle semantics in clangd — calling it
// from a definition jumps to the matching declaration, not the other way
// round. So it must not be called on a candidate that's already in a source
// file, or it would bounce us back to the header.
func (m *MethodCompositor) bestDefinition(candidates []lsp.SymbolInformation) (path string, pos lsp.Position, err error) {
	type resolved struct {
		path string
		pos  lsp.Position
	}
	var fallback *resolved

	for _, sym := range candidates {
		declPath, err := lsp.URIToPath(sym.Location.URI)
		if err != nil {
			continue
		}
		if err := m.openFile(declPath); err != nil {
			continue
		}

		if isSourceFile(declPath) {
			return declPath, sym.Location.Range.Start, nil
		}

		defPath, defPos := declPath, sym.Location.Range.Start
		if defLocs, err := m.Client.Definition(declPath, sym.Location.Range.Start); err == nil && len(defLocs) > 0 {
			if p, err := lsp.URIToPath(defLocs[0].URI); err == nil {
				defPath, defPos = p, defLocs[0].Range.Start
				if defPath != declPath {
					if err := m.openFile(defPath); err != nil {
						continue
					}
				}
			}
		}

		if fallback == nil {
			fallback = &resolved{defPath, defPos}
		}
		if isSourceFile(defPath) {
			return defPath, defPos, nil
		}
	}

	if fallback != nil {
		return fallback.path, fallback.pos, nil
	}
	return "", lsp.Position{}, fmt.Errorf("could not resolve a definition for any candidate")
}

// callKeywords are control-flow/operator tokens that match the
// `identifier(` regex shape but aren't function calls.
var callKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true,
	"sizeof": true, "new": true, "delete": true, "catch": true, "throw": true,
	"static_cast": true, "dynamic_cast": true, "const_cast": true,
	"reinterpret_cast": true, "decltype": true, "typeid": true,
	"noexcept": true, "alignof": true, "static_assert": true,
}

var callExprPattern = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*\(`)

// scanCallExpressions textually scans r's line range in path for
// `identifier(` call-expression shapes, excluding control keywords and the
// function's own name (which otherwise self-matches on the signature line).
// See outgoingCallNames for why this exists.
func scanCallExpressions(path string, r lsp.Range, ownName string) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	start, end := r.Start.Line, r.End.Line
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	if start > end {
		return nil
	}
	body := strings.Join(lines[start:end+1], "\n")

	seen := map[string]bool{ownName: true}
	var out []string
	for _, match := range callExprPattern.FindAllStringSubmatch(body, -1) {
		name := match[1]
		if callKeywords[name] || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name+"()")
	}
	return out
}

func groupIncoming(calls []lsp.CallHierarchyIncomingCall, relPath func(string) string) []CalledInGroup {
	byFile := map[string][]int{}
	for _, call := range calls {
		path, err := lsp.URIToPath(call.From.URI)
		if err != nil {
			continue
		}
		path = relPath(path)
		for _, r := range call.FromRanges {
			byFile[path] = append(byFile[path], r.Start.Line+1)
		}
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)

	groups := make([]CalledInGroup, 0, len(files))
	for _, f := range files {
		lines := byFile[f]
		sort.Ints(lines)
		groups = append(groups, CalledInGroup{File: f, Lines: lines})
	}
	return groups
}

func formatOutgoing(calls []lsp.CallHierarchyOutgoingCall) []string {
	seen := map[string]bool{}
	var out []string
	for _, call := range calls {
		name := call.To.Name
		if call.To.Detail != "" && !strings.Contains(name, "::") {
			name = call.To.Detail + "::" + name
		}
		name += "()"
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func sourceLine(path string, zeroIndexedLine int) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(content), "\n")
	if zeroIndexedLine < 0 || zeroIndexedLine >= len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[zeroIndexedLine])
}
