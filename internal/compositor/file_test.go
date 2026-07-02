package compositor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fmbfs/skein/internal/lsp"
)

// TestFileBuild_NestsMembersByRangeContainment reproduces the fixture's
// pipeline.hpp: a struct (RawBuffer) with one field, an interface
// (IProcessor) with two members, and a class (Pipeline) with three members.
// clangd's documentSymbol returns a flat list (confirmed empirically); the
// compositor must infer nesting from range containment.
func TestFileBuild_NestsMembersByRangeContainment(t *testing.T) {
	rootDir := t.TempDir()
	includeDir := filepath.Join(rootDir, "include")
	if err := os.MkdirAll(includeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := writeTempFile(t, includeDir, "pipeline.hpp", "// stub content, openFile just needs a real file\n")

	client := &classFakeClient{
		documentSymbols: map[string][]lsp.SymbolInformation{
			path: {
				{Name: "RawBuffer", Kind: lsp.SymbolKindStruct, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 5}, End: lsp.Position{Line: 7}}}},
				{Name: "size", Kind: lsp.SymbolKindField, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 6}, End: lsp.Position{Line: 6}}}},
				{Name: "IProcessor", Kind: lsp.SymbolKindClass, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 10}, End: lsp.Position{Line: 14}}}},
				{Name: "~IProcessor", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 12}, End: lsp.Position{Line: 12}}}},
				{Name: "processFrame", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 13}, End: lsp.Position{Line: 13}}}},
				{Name: "Pipeline", Kind: lsp.SymbolKindClass, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 17}, End: lsp.Position{Line: 23}}}},
				{Name: "processFrame", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 19}, End: lsp.Position{Line: 19}}}},
				{Name: "acquire", Kind: lsp.SymbolKindMethod, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 20}, End: lsp.Position{Line: 20}}}},
				{Name: "counter_", Kind: lsp.SymbolKindField, Location: lsp.Location{URI: "file://" + path, Range: lsp.Range{Start: lsp.Position{Line: 22}, End: lsp.Position{Line: 22}}}},
			},
		},
	}

	fc := &FileCompositor{base{Client: client, RootDir: rootDir}}
	fm, err := fc.Build(path)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := []Member{
		{Name: "RawBuffer", Kind: "struct", Children: []Member{
			{Name: "size", Kind: "field"},
		}},
		{Name: "IProcessor", Kind: "class", Children: []Member{
			{Name: "~IProcessor", Kind: "method"},
			{Name: "processFrame", Kind: "method"},
		}},
		{Name: "Pipeline", Kind: "class", Children: []Member{
			{Name: "processFrame", Kind: "method"},
			{Name: "acquire", Kind: "method"},
			{Name: "counter_", Kind: "field"},
		}},
	}
	if !reflect.DeepEqual(fm.Symbols, want) {
		t.Errorf("Symbols = %+v, want %+v", fm.Symbols, want)
	}
	if fm.ThreadName != "include/pipeline.hpp" {
		t.Errorf("ThreadName = %q, want include/pipeline.hpp (relative to RootDir)", fm.ThreadName)
	}
}

func TestNewFileCompositor(t *testing.T) {
	fc := NewFileCompositor(nil, "/root")
	if fc.RootDir != "/root" {
		t.Errorf("RootDir = %q, want %q", fc.RootDir, "/root")
	}
}
