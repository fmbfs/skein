package compositor

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fmbfs/skein/internal/lsp"
)

// fakeClient is a hand-rolled stand-in for *lsp.Client. workspaceSymbolSeq
// lets a test simulate clangd's incremental indexing by returning a
// different result on each successive call — this is what reproduces the
// "partial index snapshot" bug (see TestFindWorkspaceSymbol_WaitsForStableCount).
type fakeClient struct {
	workspaceSymbolSeq   [][]lsp.SymbolInformation
	workspaceSymbolHit   int
	workspaceSymbolCalls int // total WorkspaceSymbol invocations, unbounded by len(workspaceSymbolSeq)

	definitions   map[string][]lsp.Location // key: "path:line:char"
	prepareItems  map[string][]lsp.CallHierarchyItem
	incomingCalls map[string][]lsp.CallHierarchyIncomingCall
	outgoingErr   error // simulates clangd's "method not found" for outgoingCalls

	openedFiles []string

	// indexWarm mirrors lsp.Client's per-client warm-index flag (M1, skein
	// review M375/M379) so fakeClient satisfies the languageClient
	// interface without a shared/global registry.
	indexWarm bool
}

func (f *fakeClient) IsIndexWarm() bool {
	return f.indexWarm
}

func (f *fakeClient) MarkIndexWarm() {
	f.indexWarm = true
}

func (f *fakeClient) WorkspaceSymbol(query string) ([]lsp.SymbolInformation, error) {
	f.workspaceSymbolCalls++
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
	rm, err := mc.Build("processFrame", "", 1)
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
// stabilise, not return on the first non-empty hit.
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
			full,       // stabilised: the .cpp definition has landed too
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

// TestFindWorkspaceSymbol_WarmClientSkipsStabilisation is the regression
// test for the "search vs load time is too long" complaint: once a
// client's index has stabilised once, every subsequent
// findWorkspaceSymbol call for that same client must return on the very
// first WorkspaceSymbol call, without re-running the 300ms/800ms
// stabilisation loop.
func TestFindWorkspaceSymbol_WarmClientSkipsStabilisation(t *testing.T) {
	full := []lsp.SymbolInformation{
		{Name: "foo", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file:///a.cpp"}},
	}
	client := &fakeClient{
		workspaceSymbolSeq: [][]lsp.SymbolInformation{{}, full, full, full},
	}
	mc := &MethodCompositor{base{Client: client, RootDir: t.TempDir()}}

	// First call: cold, must go through the full stabilisation loop.
	if _, err := mc.findWorkspaceSymbol("foo"); err != nil {
		t.Fatalf("first findWorkspaceSymbol error: %v", err)
	}
	if !client.IsIndexWarm() {
		t.Fatalf("client should be marked warm after a stabilised resolution")
	}

	// Second call, same client: must resolve on a single WorkspaceSymbol
	// call rather than repolling for stability.
	other := []lsp.SymbolInformation{
		{Name: "bar", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file:///b.cpp"}},
	}
	client2 := &fakeClient{workspaceSymbolSeq: [][]lsp.SymbolInformation{other}}
	// Reuse client2 as a distinct fake but manually mark it warm to
	// simulate "already warm from a prior Build() on this same session".
	client2.MarkIndexWarm()

	callsBefore := client2.workspaceSymbolCalls
	symbols, err := (&MethodCompositor{base{Client: client2, RootDir: t.TempDir()}}).findWorkspaceSymbol("bar")
	if err != nil {
		t.Fatalf("findWorkspaceSymbol error: %v", err)
	}
	if got := client2.workspaceSymbolCalls - callsBefore; got != 1 {
		t.Fatalf("WorkspaceSymbol called %d times on a warm client, want exactly 1 (fast path)", got)
	}
	if len(symbols) != 1 || symbols[0].Name != "bar" {
		t.Fatalf("findWorkspaceSymbol returned %+v, want the single 'bar' symbol", symbols)
	}
}

// TestFindWorkspaceSymbol_WarmClientFallsBackWhenNotFound confirms that a
// warm client still falls through to the full stabilisation retry loop
// when the fast, single-shot WorkspaceSymbol call doesn't turn up a match
// yet — e.g. a symbol that was only just added and isn't indexed yet, even
// though the rest of the project's index is already warm.
func TestFindWorkspaceSymbol_WarmClientFallsBackWhenNotFound(t *testing.T) {
	full := []lsp.SymbolInformation{
		{Name: "freshlyAdded", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file:///new.cpp"}},
	}
	client := &fakeClient{
		// First call (fast path attempt): empty, not indexed yet.
		// Then the retry loop sees it appear and stabilise.
		workspaceSymbolSeq: [][]lsp.SymbolInformation{{}, full, full, full},
	}
	client.MarkIndexWarm()

	symbols, err := (&MethodCompositor{base{Client: client, RootDir: t.TempDir()}}).findWorkspaceSymbol("freshlyAdded")
	if err != nil {
		t.Fatalf("findWorkspaceSymbol error: %v", err)
	}
	if len(symbols) != 1 || symbols[0].Name != "freshlyAdded" {
		t.Fatalf("findWorkspaceSymbol returned %+v, want the single 'freshlyAdded' symbol", symbols)
	}
	if client.workspaceSymbolCalls < 2 {
		t.Fatalf("expected fallback to the retry loop (>=2 calls), got %d", client.workspaceSymbolCalls)
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
	groups := groupIncoming(calls, filepath.Base)

	want := []CalledInGroup{
		{File: "main.cpp", Lines: []int{8, 9}},
		{File: "scheduler.cpp", Lines: []int{77}},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Errorf("groupIncoming = %+v, want %+v", groups, want)
	}
}

// TestBuild_AmbiguousMethod_SurfacesOtherContainers is the regression test
// for the leveldb `draw -m Get` finding from project-agnostic validation:
// when two unrelated classes (DB and Version, in this fixture) both declare
// a method named "Get" and no -c filter is given, bestDefinition's
// "first source-file candidate wins" rule picks one of them by
// workspace/symbol response order — not by relevance. Build must at least
// surface the other container(s) via rm.Ambiguous so the caller (main.go)
// can warn the user and point at -c, instead of silently returning a
// possibly-wrong symbol with no indication another candidate existed.
func TestBuild_AmbiguousMethod_SurfacesOtherContainers(t *testing.T) {
	dir := t.TempDir()
	dbHeader := writeTempFile(t, dir, "db.h", "virtual Status Get(const Slice& key) = 0;\n")
	versionImpl := writeTempFile(t, dir, "version_set.cc", "Status Version::Get(const Slice& key) {\n  return ok();\n}\n")

	client := &fakeClient{
		workspaceSymbolSeq: [][]lsp.SymbolInformation{{
			{
				Name: "Get", Kind: lsp.SymbolKindMethod, ContainerName: "leveldb::DB",
				Location: lsp.Location{URI: "file://" + dbHeader, Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 15}}},
			},
			{
				Name: "Get", Kind: lsp.SymbolKindMethod, ContainerName: "leveldb::Version",
				Location: lsp.Location{URI: "file://" + versionImpl, Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 15}}},
			},
		}},
		prepareItems: map[string][]lsp.CallHierarchyItem{
			posKey(versionImpl, lsp.Position{Line: 0, Character: 15}): {{
				Name: "Version::Get", Kind: lsp.SymbolKindMethod, URI: "file://" + versionImpl,
				Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 2, Character: 1}},
			}},
		},
		outgoingErr: fmt.Errorf("method not found"),
	}

	mc := &MethodCompositor{base{Client: client, RootDir: dir}}
	rm, err := mc.Build("Get", "", 1)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if rm.Container != "leveldb::Version" {
		t.Errorf("Container = %q, want leveldb::Version (the source-file candidate)", rm.Container)
	}
	if want := []string{"leveldb::DB"}; !reflect.DeepEqual(rm.Ambiguous, want) {
		t.Errorf("Ambiguous = %v, want %v — caller needs this to warn the user another candidate exists", rm.Ambiguous, want)
	}
}

// TestBuild_ClassFilter_ScopesToRequestedContainer is the regression test
// for the -c fix: with a class filter, Build must resolve to the candidate
// in that container even when bestDefinition's source-file preference would
// otherwise have picked a different one, and Ambiguous must be empty since
// the caller already disambiguated.
func TestBuild_ClassFilter_ScopesToRequestedContainer(t *testing.T) {
	dir := t.TempDir()
	dbHeader := writeTempFile(t, dir, "db.h", "virtual Status Get(const Slice& key) = 0;\n")
	versionImpl := writeTempFile(t, dir, "version_set.cc", "Status Version::Get(const Slice& key) {\n  return ok();\n}\n")

	client := &fakeClient{
		workspaceSymbolSeq: [][]lsp.SymbolInformation{{
			{
				Name: "Get", Kind: lsp.SymbolKindMethod, ContainerName: "leveldb::DB",
				Location: lsp.Location{URI: "file://" + dbHeader, Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 15}}},
			},
			{
				Name: "Get", Kind: lsp.SymbolKindMethod, ContainerName: "leveldb::Version",
				Location: lsp.Location{URI: "file://" + versionImpl, Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 15}}},
			},
		}},
		prepareItems: map[string][]lsp.CallHierarchyItem{
			posKey(dbHeader, lsp.Position{Line: 0, Character: 15}): {{
				Name: "DB::Get", Kind: lsp.SymbolKindMethod, URI: "file://" + dbHeader,
				Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 0, Character: 40}},
			}},
		},
		outgoingErr: fmt.Errorf("method not found"),
	}

	mc := &MethodCompositor{base{Client: client, RootDir: dir}}
	rm, err := mc.Build("Get", "DB", 1)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if rm.DefinedAt.Path != "db.h" {
		t.Errorf("DefinedAt.Path = %q, want db.h (the -c DB scoped candidate)", rm.DefinedAt.Path)
	}
	if len(rm.Ambiguous) != 0 {
		t.Errorf("Ambiguous = %v, want empty — caller already disambiguated with -c", rm.Ambiguous)
	}
}

// TestBuild_ClassFilter_NoMatch_ListsAvailableContainers checks that an
// unmatched -c filter fails fast with the actual containers found, instead
// of a bare "not found" — the whole point of -c is helping the user pick
// correctly, so the error needs to show what was available.
func TestBuild_ClassFilter_NoMatch_ListsAvailableContainers(t *testing.T) {
	dir := t.TempDir()
	dbHeader := writeTempFile(t, dir, "db.h", "virtual Status Get(const Slice& key) = 0;\n")

	client := &fakeClient{
		workspaceSymbolSeq: [][]lsp.SymbolInformation{{
			{
				Name: "Get", Kind: lsp.SymbolKindMethod, ContainerName: "leveldb::DB",
				Location: lsp.Location{URI: "file://" + dbHeader, Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 15}}},
			},
		}},
	}

	mc := &MethodCompositor{base{Client: client, RootDir: dir}}
	_, err := mc.Build("Get", "NoSuchClass", 1)
	if err == nil {
		t.Fatal("Build returned nil error, want an error naming the available containers")
	}
	if got := err.Error(); !strings.Contains(got, "leveldb::DB") {
		t.Errorf("error = %q, want it to mention leveldb::DB (the actual container found)", got)
	}
}

func TestContainerMatches(t *testing.T) {
	cases := []struct {
		container, filter string
		want              bool
	}{
		{"leveldb::DB", "DB", true},
		{"leveldb::DB", "leveldb::DB", true},
		{"leveldb::DB", "Version", false},
		{"DB", "DB", true},
		{"testing::internal::BuiltInDefaultValue<int>", "BuiltInDefaultValue<int>", true},
	}
	for _, c := range cases {
		if got := containerMatches(c.container, c.filter); got != c.want {
			t.Errorf("containerMatches(%q, %q) = %v, want %v", c.container, c.filter, got, c.want)
		}
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

func TestNewMethodCompositor(t *testing.T) {
	mc := NewMethodCompositor(nil, "/root")
	if mc.RootDir != "/root" {
		t.Errorf("RootDir = %q, want %q", mc.RootDir, "/root")
	}
}

func TestFormatOutgoing(t *testing.T) {
	calls := []lsp.CallHierarchyOutgoingCall{
		{To: lsp.CallHierarchyItem{Name: "helper"}},
		{To: lsp.CallHierarchyItem{Name: "helper"}}, // duplicate -> deduped
		{To: lsp.CallHierarchyItem{Name: "method", Detail: "MyClass"}},
		{To: lsp.CallHierarchyItem{Name: "already::Qualified", Detail: "Other"}},
	}
	got := formatOutgoing(calls)
	want := []string{"helper()", "MyClass::method()", "already::Qualified()"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("formatOutgoing() = %v, want %v", got, want)
	}
}
