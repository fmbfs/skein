package compositor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fmbfs/skein/internal/lsp"
)

// TestResolveSymbol_NudgesIndexerThenRetries is the regression test for the
// bug found while wiring generic symbol resolution outside of a specific
// compositor's Build (docs/SPEC.md's `skein draw -s <symbol>` and the TUI's
// initial-symbol resolution): a bare client.WorkspaceSymbol call never
// returns results because clangd's background index never starts without
// at least one textDocument/didOpen (see nudgeIndexer's doc comment).
// ResolveSymbol must nudge the indexer exactly like a compositor's Build
// does, then retry via the same findWorkspaceSymbol loop.
func TestResolveSymbol_NudgesIndexerThenRetries(t *testing.T) {
	dir := t.TempDir()
	srcPath := writeTempFile(t, dir, "pipeline.cpp", "void Pipeline::processFrame() {}\n")

	entries := []compileCommandsEntry{{Directory: dir, File: srcPath}}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compile_commands.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	client := &fakeClient{
		workspaceSymbolSeq: [][]lsp.SymbolInformation{{
			{Name: "processFrame", Kind: lsp.SymbolKindMethod,
				Location: lsp.Location{URI: "file://" + srcPath}},
		}},
	}

	symbols, err := ResolveSymbol(client, dir, "processFrame")
	if err != nil {
		t.Fatalf("ResolveSymbol returned error: %v", err)
	}
	if len(symbols) != 1 || symbols[0].Name != "processFrame" {
		t.Errorf("ResolveSymbol = %+v, want one processFrame symbol", symbols)
	}
	if len(client.openedFiles) != 1 || client.openedFiles[0] != srcPath {
		t.Errorf("ResolveSymbol didn't nudge the indexer: openedFiles = %v, want [%s]", client.openedFiles, srcPath)
	}
}

// TestResolveSymbol_NoSymbolFound confirms ResolveSymbol propagates an empty
// result (rather than erroring) when nothing matches, mirroring
// findWorkspaceSymbol's contract — callers decide what "not found" means.
func TestResolveSymbol_NoSymbolFound(t *testing.T) {
	dir := t.TempDir() // no compile_commands.json: nudgeIndexer fails silently (best-effort)
	client := &fakeClient{workspaceSymbolSeq: [][]lsp.SymbolInformation{{}}}

	symbols, err := ResolveSymbol(client, dir, "missingSymbol")
	if err != nil {
		t.Fatalf("ResolveSymbol returned error: %v", err)
	}
	if len(symbols) != 0 {
		t.Errorf("ResolveSymbol = %+v, want empty", symbols)
	}
}
