package compositor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fmbfs/skein/internal/lsp"
)

// workspaceSymbolTimeout bounds how long a Build waits for clangd's
// background index to populate after a cold start. Every `skein draw`
// invocation spawns a fresh clangd with no persistent index, so the first
// workspace/symbol query right after the LSP handshake will often return
// nothing — indexing a project (even a tiny one) takes a beat. Retrying is
// cheaper than the alternative (a long-lived clangd daemon), which is out
// of scope for v0.1.
const workspaceSymbolTimeout = 10 * time.Second

// languageClient is the subset of *lsp.Client that the compositors need.
// Extracted as an interface so tests can substitute a fake instead of
// spawning real clangd — see method_test.go.
type languageClient interface {
	WorkspaceSymbol(query string) ([]lsp.SymbolInformation, error)
	DidOpen(path, text string) error
	Definition(path string, pos lsp.Position) ([]lsp.Location, error)
	PrepareCallHierarchy(path string, pos lsp.Position) ([]lsp.CallHierarchyItem, error)
	IncomingCalls(item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, error)
	OutgoingCalls(item lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, error)
	DocumentSymbol(path string) ([]lsp.SymbolInformation, error)
	PrepareTypeHierarchy(path string, pos lsp.Position) ([]lsp.TypeHierarchyItem, error)
	Supertypes(item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error)
	Subtypes(item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error)
}

// base holds the client/root-dir pair and helpers shared by every
// compositor (method, class, file). Embed it rather than duplicating these.
type base struct {
	Client  languageClient
	RootDir string
}

func (b *base) openFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return b.Client.DidOpen(path, string(content))
}

func (b *base) relPath(abs string) string {
	rel, err := filepath.Rel(b.RootDir, abs)
	if err != nil {
		return abs
	}
	return rel
}

// findWorkspaceSymbol retries workspace/symbol until a name-matching result
// appears, then keeps polling a little longer until the result count
// stabilises (or workspaceSymbolTimeout elapses).
//
// Indexing is incremental: the first poll to see *any* match can be a
// partial snapshot — e.g. only the header declaration indexed so far, with
// the concrete .cpp implementation(s) landing a moment later. Returning
// immediately on the first hit can hand the caller only the declaration,
// when a source-file definition was about to show up. Confirmed empirically
// on a real-world client class: the index briefly contained only the
// interface's pure-virtual declaration before the concrete class's and its
// PIMPL's .cpp definitions appeared ~1s later.
func (b *base) findWorkspaceSymbol(name string) ([]lsp.SymbolInformation, error) {
	deadline := time.Now().Add(workspaceSymbolTimeout)
	var lastCount = -1
	var stableSince time.Time

	for {
		symbols, err := b.Client.WorkspaceSymbol(name)
		if err != nil {
			return nil, err
		}
		found := false
		for _, s := range symbols {
			if s.Name == name {
				found = true
				break
			}
		}

		if found {
			if len(symbols) != lastCount {
				lastCount = len(symbols)
				stableSince = time.Now()
			} else if time.Since(stableSince) >= 800*time.Millisecond {
				return symbols, nil
			}
		}

		if time.Now().After(deadline) {
			return symbols, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func isSourceFile(path string) bool {
	switch filepath.Ext(path) {
	case ".cpp", ".cc", ".cxx", ".c":
		return true
	default:
		return false
	}
}
