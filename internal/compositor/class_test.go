package compositor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fmbfs/skein/internal/lsp"
)

// classFakeClient extends fakeClient's WorkspaceSymbol/DidOpen behaviour
// with scriptable type-hierarchy and document-symbol responses, modelling
// the exact shapes confirmed empirically against clangd 18 on the fixture
// project (Pipeline : public IProcessor).
type classFakeClient struct {
	fakeClient
	prepareTypeItems map[string][]lsp.TypeHierarchyItem // key: posKey(path, pos)
	supertypes       map[string][]lsp.TypeHierarchyItem // key: item.Name
	subtypes         map[string][]lsp.TypeHierarchyItem // key: item.Name
	documentSymbols  map[string][]lsp.SymbolInformation  // key: path
}

func (c *classFakeClient) PrepareTypeHierarchy(path string, pos lsp.Position) ([]lsp.TypeHierarchyItem, error) {
	return c.prepareTypeItems[posKey(path, pos)], nil
}

func (c *classFakeClient) Supertypes(item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error) {
	return c.supertypes[item.Name], nil
}

func (c *classFakeClient) Subtypes(item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error) {
	return c.subtypes[item.Name], nil
}

func (c *classFakeClient) DocumentSymbol(path string) ([]lsp.SymbolInformation, error) {
	return c.documentSymbols[path], nil
}

// TestClassBuild_FixtureShape reproduces the exact Pipeline/IProcessor
// scenario confirmed against real clangd: a class with one base class, no
// derived classes, and three members whose ranges fall within the class's
// own range.
func TestClassBuild_FixtureShape(t *testing.T) {
	rootDir := t.TempDir()
	includeDir := filepath.Join(rootDir, "include")
	if err := os.MkdirAll(includeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := writeTempFile(t, includeDir, "pipeline.hpp", "// stub content, openFile just needs a real file\n")
	classPos := lsp.Position{Line: 17, Character: 6}
	// workspace/symbol returns a narrow name-span range (confirmed
	// empirically for methods too — see method.go's bestDefinition
	// comment); documentSymbol separately reports the full body extent.
	nameRange := lsp.Range{Start: classPos, End: lsp.Position{Line: 17, Character: 14}}
	classRange := lsp.Range{Start: lsp.Position{Line: 17, Character: 0}, End: lsp.Position{Line: 23, Character: 1}}

	client := &classFakeClient{
		fakeClient: fakeClient{
			workspaceSymbolSeq: [][]lsp.SymbolInformation{{
				{Name: "Pipeline", Kind: lsp.SymbolKindClass, Location: lsp.Location{URI: "file://" + path, Range: nameRange}},
			}},
		},
		prepareTypeItems: map[string][]lsp.TypeHierarchyItem{
			posKey(path, classPos): {{Name: "Pipeline", Kind: lsp.SymbolKindClass, URI: "file://" + path, Range: classRange, SelectionRange: classRange}},
		},
		supertypes: map[string][]lsp.TypeHierarchyItem{
			"Pipeline": {{Name: "IProcessor", Kind: lsp.SymbolKindClass}},
		},
		subtypes: map[string][]lsp.TypeHierarchyItem{
			"Pipeline": {}, // nothing derives from Pipeline
		},
		documentSymbols: map[string][]lsp.SymbolInformation{
			path: {
				{Name: "IProcessor", Kind: lsp.SymbolKindClass, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 10}, End: lsp.Position{Line: 14}}}},
				{Name: "Pipeline", Kind: lsp.SymbolKindClass, Location: lsp.Location{URI: "file://" + path, Range: classRange}},
				{Name: "processFrame", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 19}, End: lsp.Position{Line: 19}}}},
				{Name: "acquire", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 20}, End: lsp.Position{Line: 20}}}},
				{Name: "counter_", Kind: lsp.SymbolKindField, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 22}, End: lsp.Position{Line: 22}}}},
			},
		},
	}

	cc := &ClassCompositor{base{Client: client, RootDir: rootDir}}
	cm, err := cc.Build("Pipeline")
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if cm.DefinedAt.Path != "include/pipeline.hpp" || cm.DefinedAt.Line != 18 {
		t.Errorf("DefinedAt = %+v, want include/pipeline.hpp:18", cm.DefinedAt)
	}
	if got, want := cm.Inherits, []string{"IProcessor"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Inherits = %v, want %v", got, want)
	}
	if len(cm.InheritedBy) != 0 {
		t.Errorf("InheritedBy = %v, want empty", cm.InheritedBy)
	}

	wantMembers := []Member{
		{Name: "processFrame", Kind: "method"},
		{Name: "acquire", Kind: "method"},
		{Name: "counter_", Kind: "field"},
	}
	if !reflect.DeepEqual(cm.Members, wantMembers) {
		t.Errorf("Members = %+v, want %+v", cm.Members, wantMembers)
	}
}
