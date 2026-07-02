package compositor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/fmbfs/skein/internal/lsp"
)

// --- class.go error paths --------------------------------------------------

// erroringWorkspaceSymbolClient wraps fakeClient to simulate a clangd
// transport failure on workspace/symbol.
type erroringWorkspaceSymbolClient struct {
	fakeClient
	err error
}

func (c *erroringWorkspaceSymbolClient) WorkspaceSymbol(query string) ([]lsp.SymbolInformation, error) {
	return nil, c.err
}

func TestClassBuild_WorkspaceSymbolError(t *testing.T) {
	client := &erroringWorkspaceSymbolClient{err: errors.New("clangd transport error")}
	cc := &ClassCompositor{base{Client: client, RootDir: t.TempDir()}}
	_, err := cc.Build("Pipeline")
	if err == nil {
		t.Fatal("Build returned nil error, want propagated workspace/symbol error")
	}
}

func TestClassBuild_NoClassFound(t *testing.T) {
	client := &fakeClient{workspaceSymbolSeq: [][]lsp.SymbolInformation{{
		{Name: "Pipeline", Kind: lsp.SymbolKindMethod}, // wrong kind: not a class/struct
	}}}
	cc := &ClassCompositor{base{Client: client, RootDir: t.TempDir()}}
	_, err := cc.Build("Pipeline")
	if err == nil {
		t.Fatal("Build returned nil error, want 'no class or struct named' error")
	}
}

func TestClassBuild_BadURIScheme(t *testing.T) {
	client := &fakeClient{workspaceSymbolSeq: [][]lsp.SymbolInformation{{
		{Name: "Pipeline", Kind: lsp.SymbolKindClass, Location: lsp.Location{URI: "http://not-a-file-uri"}},
	}}}
	cc := &ClassCompositor{base{Client: client, RootDir: t.TempDir()}}
	_, err := cc.Build("Pipeline")
	if err == nil {
		t.Fatal("Build returned nil error, want URIToPath scheme error")
	}
}

func TestClassBuild_OpenFileError(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.hpp")
	client := &fakeClient{workspaceSymbolSeq: [][]lsp.SymbolInformation{{
		{Name: "Pipeline", Kind: lsp.SymbolKindClass, Location: lsp.Location{URI: "file://" + missing}},
	}}}
	cc := &ClassCompositor{base{Client: client, RootDir: dir}}
	_, err := cc.Build("Pipeline")
	if err == nil {
		t.Fatal("Build returned nil error, want openFile error for missing file")
	}
}

func TestContainerRange_NoMatch(t *testing.T) {
	all := []lsp.SymbolInformation{
		{Name: "Other", Location: lsp.Location{Range: lsp.Range{Start: lsp.Position{Line: 0}, End: lsp.Position{Line: 5}}}},
	}
	got := containerRange(all, "Pipeline", lsp.Position{Line: 2})
	if got != (lsp.Range{}) {
		t.Errorf("containerRange = %+v, want zero Range when no name match", got)
	}
}

func TestRangeContains_EqualRangesReturnsFalse(t *testing.T) {
	r := lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 5}}
	if rangeContains(r, r) {
		t.Error("rangeContains(r, r) = true, want false (a range doesn't strictly contain itself)")
	}
}

// --- file.go error paths ----------------------------------------------------

func TestFileBuild_OpenFileError(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.cpp")
	fc := &FileCompositor{base{Client: &fakeClient{}, RootDir: dir}}
	_, err := fc.Build(missing)
	if err == nil {
		t.Fatal("Build returned nil error, want openFile error for missing file")
	}
}

type documentSymbolErrClient struct {
	fakeClient
	err error
}

func (c *documentSymbolErrClient) DocumentSymbol(path string) ([]lsp.SymbolInformation, error) {
	return nil, c.err
}

func TestFileBuild_DocumentSymbolError(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "a.cpp", "// stub\n")
	client := &documentSymbolErrClient{err: errors.New("documentSymbol failed")}
	fc := &FileCompositor{base{Client: client, RootDir: dir}}
	_, err := fc.Build(path)
	if err == nil {
		t.Fatal("Build returned nil error, want propagated documentSymbol error")
	}
}

// --- indexer.go error paths -------------------------------------------------

func TestNudgeIndexer_MissingComposeCommandsFile(t *testing.T) {
	dir := t.TempDir() // no compile_commands.json at all
	b := &base{Client: &fakeClient{}, RootDir: dir}
	if err := b.nudgeIndexer(); err == nil {
		t.Fatal("nudgeIndexer returned nil error, want os.ReadFile error")
	}
}

func TestNudgeIndexer_BadJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compile_commands.json"), []byte("not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &base{Client: &fakeClient{}, RootDir: dir}
	if err := b.nudgeIndexer(); err == nil {
		t.Fatal("nudgeIndexer returned nil error, want JSON unmarshal error")
	}
}

func TestNudgeIndexer_EmptyEntries(t *testing.T) {
	dir := t.TempDir()
	data, err := json.Marshal([]compileCommandsEntry{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compile_commands.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{}
	b := &base{Client: client, RootDir: dir}
	if err := b.nudgeIndexer(); err != nil {
		t.Fatalf("nudgeIndexer returned error, want nil for empty entries: %v", err)
	}
	if len(client.openedFiles) != 0 {
		t.Errorf("openedFiles = %v, want empty (nothing to nudge)", client.openedFiles)
	}
}

func TestNudgeIndexer_RelativeFilePath(t *testing.T) {
	dir := t.TempDir()
	srcPath := writeTempFile(t, dir, "main.cpp", "int main() {}\n")
	entries := []compileCommandsEntry{{Directory: dir, File: "main.cpp"}} // relative File
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compile_commands.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{}
	b := &base{Client: client, RootDir: dir}
	if err := b.nudgeIndexer(); err != nil {
		t.Fatalf("nudgeIndexer returned error: %v", err)
	}
	if len(client.openedFiles) != 1 || client.openedFiles[0] != srcPath {
		t.Errorf("openedFiles = %v, want [%s] (relative File joined with Directory)", client.openedFiles, srcPath)
	}
}

// --- method.go error paths ---------------------------------------------------

func TestMethodBuild_WorkspaceSymbolError(t *testing.T) {
	client := &erroringWorkspaceSymbolClient{err: errors.New("clangd transport error")}
	mc := &MethodCompositor{base{Client: client, RootDir: t.TempDir()}}
	_, err := mc.Build("foo", "", 1)
	if err == nil {
		t.Fatal("Build returned nil error, want propagated workspace/symbol error")
	}
}

func TestMethodBuild_NoCandidates(t *testing.T) {
	client := &fakeClient{workspaceSymbolSeq: [][]lsp.SymbolInformation{{
		{Name: "foo", Kind: lsp.SymbolKindClass}, // wrong kind
	}}}
	mc := &MethodCompositor{base{Client: client, RootDir: t.TempDir()}}
	_, err := mc.Build("foo", "", 1)
	if err == nil {
		t.Fatal("Build returned nil error, want 'no method or function named' error")
	}
}

func TestBestDefinition_AllOpenFileErrorsReturnsError(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.cpp")
	client := &fakeClient{workspaceSymbolSeq: [][]lsp.SymbolInformation{{
		{Name: "foo", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file://" + missing}},
	}}}
	mc := &MethodCompositor{base{Client: client, RootDir: dir}}
	_, err := mc.Build("foo", "", 1)
	if err == nil {
		t.Fatal("Build returned nil error, want bestDefinition failure when every candidate's file is unreadable")
	}
}

type prepareCallHierarchyErrClient struct {
	fakeClient
	err error
}

func (c *prepareCallHierarchyErrClient) PrepareCallHierarchy(path string, pos lsp.Position) ([]lsp.CallHierarchyItem, error) {
	return nil, c.err
}

func TestMethodBuild_PrepareCallHierarchyError(t *testing.T) {
	dir := t.TempDir()
	implPath := writeTempFile(t, dir, "foo.cpp", "void foo() {}\n")
	client := &prepareCallHierarchyErrClient{
		fakeClient: fakeClient{workspaceSymbolSeq: [][]lsp.SymbolInformation{{
			{Name: "foo", Kind: lsp.SymbolKindFunction, Location: lsp.Location{URI: "file://" + implPath}},
		}}},
		err: errors.New("prepareCallHierarchy failed"),
	}
	mc := &MethodCompositor{base{Client: client, RootDir: dir}}
	_, err := mc.Build("foo", "", 1)
	if err == nil {
		t.Fatal("Build returned nil error, want propagated prepareCallHierarchy error")
	}
}

func TestMethodBuild_NoCallHierarchyItems(t *testing.T) {
	dir := t.TempDir()
	implPath := writeTempFile(t, dir, "foo.cpp", "void foo() {}\n")
	client := &fakeClient{workspaceSymbolSeq: [][]lsp.SymbolInformation{{
		{Name: "foo", Kind: lsp.SymbolKindFunction, Location: lsp.Location{URI: "file://" + implPath}},
	}}}
	mc := &MethodCompositor{base{Client: client, RootDir: dir}}
	_, err := mc.Build("foo", "", 1)
	if err == nil {
		t.Fatal("Build returned nil error, want 'clangd returned no call-hierarchy item' error")
	}
}

func TestOutgoingCallNames_BadURIFallsBackToNil(t *testing.T) {
	mc := &MethodCompositor{base{Client: &fakeClient{}, RootDir: t.TempDir()}}
	item := lsp.CallHierarchyItem{URI: "http://not-a-file-uri"}
	got := mc.outgoingCallNames(item, "foo")
	if got != nil {
		t.Errorf("outgoingCallNames = %v, want nil when the item's URI can't be converted to a path", got)
	}
}

func TestScanCallExpressions_FileReadError(t *testing.T) {
	got := scanCallExpressions("/no/such/file.cpp", lsp.Range{}, "foo")
	if got != nil {
		t.Errorf("scanCallExpressions = %v, want nil on read error", got)
	}
}

func TestScanCallExpressions_StartAfterEnd(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "a.cpp", "line0\nline1\nline2\n")
	got := scanCallExpressions(path, lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 0}}, "foo")
	if got != nil {
		t.Errorf("scanCallExpressions = %v, want nil when start > end", got)
	}
}

func TestSourceLine_FileReadError(t *testing.T) {
	if got := sourceLine("/no/such/file.cpp", 0); got != "" {
		t.Errorf("sourceLine = %q, want empty on read error", got)
	}
}

func TestSourceLine_OutOfRange(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "a.cpp", "only one line\n")
	if got := sourceLine(path, 99); got != "" {
		t.Errorf("sourceLine = %q, want empty for an out-of-range line", got)
	}
	if got := sourceLine(path, -1); got != "" {
		t.Errorf("sourceLine = %q, want empty for a negative line", got)
	}
}

func TestGroupIncoming_BadURISkipped(t *testing.T) {
	calls := []lsp.CallHierarchyIncomingCall{
		{From: lsp.CallHierarchyItem{URI: "http://not-a-file-uri"}, FromRanges: []lsp.Range{{Start: lsp.Position{Line: 1}}}},
		{From: lsp.CallHierarchyItem{URI: "file:///main.cpp"}, FromRanges: []lsp.Range{{Start: lsp.Position{Line: 7}}}},
	}
	groups := groupIncoming(calls, filepath.Base)
	want := []CalledInGroup{{File: "main.cpp", Lines: []int{8}}}
	if fmt.Sprint(groups) != fmt.Sprint(want) {
		t.Errorf("groupIncoming = %+v, want %+v (bad-URI entry skipped)", groups, want)
	}
}

// --- shared.go error paths ---------------------------------------------------

func TestFindWorkspaceSymbol_ClientError(t *testing.T) {
	client := &erroringWorkspaceSymbolClient{err: errors.New("transport error")}
	b := &base{Client: client, RootDir: t.TempDir()}
	_, err := b.findWorkspaceSymbol("foo")
	if err == nil {
		t.Fatal("findWorkspaceSymbol returned nil error, want propagated client error")
	}
}

func TestOpenFile_ReadError(t *testing.T) {
	b := &base{Client: &fakeClient{}, RootDir: t.TempDir()}
	if err := b.openFile("/no/such/file.cpp"); err == nil {
		t.Fatal("openFile returned nil error, want os.ReadFile error for a missing file")
	}
}
