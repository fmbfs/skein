package compositor

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fmbfs/skein/internal/lsp"
)

// fakeClient is a hand-rolled stand-in for *lsp.Client. workspaceSymbolSeq
// lets a test simulate clangd's incremental indexing by returning a
// different result on each successive call — this is what reproduces the
// "partial index snapshot" bug (see TestFindWorkspaceSymbol_WaitsForStableCount).
type fakeClient struct {
	workspaceSymbolSeq [][]lsp.SymbolInformation
	workspaceSymbolHit int

	definitions   map[string][]lsp.Location // key: "path:line:char"
	prepareItems  map[string][]lsp.CallHierarchyItem
	incomingCalls map[string][]lsp.CallHierarchyIncomingCall
	outgoingErr   error // simulates clangd's "method not found" for outgoingCalls

	openedFiles []string
}

func (f *fakeClient) WorkspaceSymbol(query string) ([]lsp.SymbolInformation, error) {
	if f.workspaceSymbolHit >= len(f.workspaceSymbolSeq) {
		return f.workspaceSymbolSeq[len(f.workspaceSymbolSeq)-1], nil
	}
	result := f.workspaceSymbolSeq[f.workspaceSymbolHit]
	f.workspaceSymbolHit++
	return result, nil
}

func (f *fakeClient) DidOpen(path, text string) error {
	f.openedFiles = append(f.openedFiles, path)
	return nil
}

func posKey(path string, pos lsp.Position) string {
	return fmt.Sprintf("%s:%d:%d", path, pos.Line, pos.Character)
}

func (f *fakeClient) Definition(path string, pos lsp.Position) ([]lsp.Location, error) {
	return f.definitions[posKey(path, pos)], nil
}

func (f *fakeClient) PrepareCallHierarchy(path string, pos lsp.Position) ([]lsp.CallHierarchyItem, error) {
	return f.prepareItems[posKey(path, pos)], nil
}

func (f *fakeClient) IncomingCalls(item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, error) {
	return f.incomingCalls[item.Name], nil
}

func (f *fakeClient) OutgoingCalls(item lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, error) {
	if f.outgoingErr != nil {
		return nil, f.outgoingErr
	}
	return nil, nil
}

func (f *fakeClient) DocumentSymbol(path string) ([]lsp.SymbolInformation, error) { return nil, nil }

func (f *fakeClient) PrepareTypeHierarchy(path string, pos lsp.Position) ([]lsp.TypeHierarchyItem, error) {
	return nil, nil
}

func (f *fakeClient) Supertypes(item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error) {
	return nil, nil
}

func (f *fakeClient) Subtypes(item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error) {
	return nil, nil
}

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// TestBuild_PicksConcreteImplOverInterfaceDeclaration is the regression test
// for the exact fixture scenario in tests/fixtures/simple_cpp: an interface
// method (header, pure virtual) and its concrete override (source file) both
// match by name. bestDefinition must prefer the source-file candidate.
func TestBuild_PicksConcreteImplOverInterfaceDeclaration(t *testing.T) {
	dir := t.TempDir()
	headerPath := writeTempFile(t, dir, "iface.hpp", "virtual void processFrame(RawBuffer&) = 0;\n")
	implPath := writeTempFile(t, dir, "pipeline.cpp", "void Pipeline::processFrame(RawBuffer& buf) {\n    acquire();\n}\n")

	client := &fakeClient{
		workspaceSymbolSeq: [][]lsp.SymbolInformation{{
			{
				Name: "processFrame", Kind: lsp.SymbolKindMethod,
				Location: lsp.Location{URI: "file://" + headerPath, Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 17}}},
			},
			{
				Name: "processFrame", Kind: lsp.SymbolKindMethod,
				Location: lsp.Location{URI: "file://" + implPath, Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 15}}},
			},
		}},
		prepareItems: map[string][]lsp.CallHierarchyItem{
			posKey(implPath, lsp.Position{Line: 0, Character: 15}): {{
				Name: "Pipeline::processFrame", Kind: lsp.SymbolKindMethod, URI: "file://" + implPath,
				Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 2, Character: 1}},
			}},
		},
		incomingCalls: map[string][]lsp.CallHierarchyIncomingCall{
			"Pipeline::processFrame": {{
				From:       lsp.CallHierarchyItem{Name: "main", URI: "file://" + implPath},
				FromRanges: []lsp.Range{{Start: lsp.Position{Line: 10, Character: 4}}},
			}},
		},
		outgoingErr: fmt.Errorf("method not found"),
	}

	mc := &MethodCompositor{base{Client: client, RootDir: dir}}
	rm, err := mc.Build("processFrame", 1)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if rm.DefinedAt.Path != "pipeline.cpp" {
		t.Errorf("DefinedAt.Path = %q, want pipeline.cpp (the concrete impl, not the header)", rm.DefinedAt.Path)
	}
	if rm.DefinedAt.Line != 1 {
		t.Errorf("DefinedAt.Line = %d, want 1", rm.DefinedAt.Line)
	}
	if got, want := rm.Calls, []string{"acquire()"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Calls (outgoing fallback scan) = %v, want %v", got, want)
	}
	if len(rm.CalledIn) != 1 || rm.CalledInTotal() != 1 {
		t.Errorf("CalledIn = %+v, want one group with one line", rm.CalledIn)
	}
}

// TestFindWorkspaceSymbol_WaitsForStableCount is the regression test for the
// real bug found against a real-world client class: the first
// workspace/symbol poll to see *any* match can be a partial index snapshot
// (e.g. only the header declaration), with the .cpp definition(s) landing a
// moment later. findWorkspaceSymbol must wait for the result count to
// stabilize, not return on the first non-empty hit.
func TestFindWorkspaceSymbol_WaitsForStableCount(t *testing.T) {
	headerOnly := []lsp.SymbolInformation{{Name: "foo", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file:///a.h"}}}
	full := []lsp.SymbolInformation{
		headerOnly[0],
		{Name: "foo", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file:///a.cpp"}},
	}

	client := &fakeClient{
		workspaceSymbolSeq: [][]lsp.SymbolInformation{
			{},         // not indexed yet
			headerOnly, // partial: only the header declaration so far
			full,       // stabilized: the .cpp definition has landed too
			full,
			full,
			full,
		},
	}

	mc := &MethodCompositor{base{Client: client, RootDir: t.TempDir()}}
	symbols, err := mc.findWorkspaceSymbol("foo")
	if err != nil {
		t.Fatalf("findWorkspaceSymbol error: %v", err)
	}
	if len(symbols) != 2 {
		t.Fatalf("findWorkspaceSymbol returned %d symbols, want 2 (should have waited past the partial 1-result snapshot)", len(symbols))
	}
}

func TestGroupIncoming(t *testing.T) {
	calls := []lsp.CallHierarchyIncomingCall{
		{
			From:       lsp.CallHierarchyItem{URI: "file:///main.cpp"},
			FromRanges: []lsp.Range{{Start: lsp.Position{Line: 7}}, {Start: lsp.Position{Line: 8}}},
		},
		{
			From:       lsp.CallHierarchyItem{URI: "file:///scheduler.cpp"},
			FromRanges: []lsp.Range{{Start: lsp.Position{Line: 76}}},
		},
	}
	groups := groupIncoming(calls, func(p string) string { return filepath.Base(p) })

	want := []CalledInGroup{
		{File: "main.cpp", Lines: []int{8, 9}},
		{File: "scheduler.cpp", Lines: []int{77}},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Errorf("groupIncoming = %+v, want %+v", groups, want)
	}
}

func TestScanCallExpressions_ExcludesOwnNameAndKeywords(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "rule.cpp", `bool Rule::evaluate() const {
    if (isReady()) {
        return helper();
    }
    return evaluate_other();
}
`)
	calls := scanCallExpressions(path, lsp.Range{Start: lsp.Position{Line: 0}, End: lsp.Position{Line: 5}}, "evaluate")

	want := []string{"isReady()", "helper()", "evaluate_other()"}
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("scanCallExpressions = %v, want %v (should exclude the keyword 'if' and the own bare name 'evaluate')", calls, want)
	}
}
