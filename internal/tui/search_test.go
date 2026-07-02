package tui

import (
	"strings"
	"testing"

	"github.com/fmbfs/skein/internal/lsp"
)

func TestSearchStateMoveCursorClampsBounds(t *testing.T) {
	s := newSearchState()
	s.results = []lsp.SymbolInformation{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	s.moveCursor(-5)
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want clamped to 0", s.cursor)
	}
	s.moveCursor(1)
	if s.cursor != 1 {
		t.Errorf("cursor = %d, want 1", s.cursor)
	}
	s.moveCursor(10)
	if s.cursor != 2 {
		t.Errorf("cursor = %d, want clamped to 2", s.cursor)
	}
}

func TestSearchStateMoveCursorNoResults(t *testing.T) {
	s := newSearchState()
	s.moveCursor(1)
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0 with no results", s.cursor)
	}
}

func TestSearchStateSelected(t *testing.T) {
	s := newSearchState()
	if s.selected() != nil {
		t.Error("selected() with no results should be nil")
	}
	s.results = []lsp.SymbolInformation{{Name: "only"}}
	got := s.selected()
	if got == nil || got.Name != "only" {
		t.Errorf("selected() = %v, want &{Name: only}", got)
	}
}

func TestSearchStateReset(t *testing.T) {
	s := newSearchState()
	s.input.SetValue("query")
	s.results = []lsp.SymbolInformation{{Name: "x"}}
	s.cursor = 1
	s.err = errBoom

	s.reset()
	if s.input.Value() != "" || s.results != nil || s.cursor != 0 || s.err != nil {
		t.Errorf("reset() left state = %+v", s)
	}
}

var errBoom = &searchTestErr{}

type searchTestErr struct{}

func (*searchTestErr) Error() string { return "boom" }

func TestRenderSearchBarShowsResultsAndError(t *testing.T) {
	s := newSearchState()
	s.input.SetValue("foo")
	s.results = []lsp.SymbolInformation{
		{Name: "FooBar", Kind: lsp.SymbolKindMethod, ContainerName: "MyClass"},
	}
	got := renderSearchBar(&s, 40)
	if !strings.Contains(got, "FooBar") {
		t.Errorf("renderSearchBar = %q, want it to contain FooBar", got)
	}

	s2 := newSearchState()
	s2.err = errBoom
	got2 := renderSearchBar(&s2, 40)
	if !strings.Contains(got2, "boom") {
		t.Errorf("renderSearchBar with error = %q, want it to contain the error text", got2)
	}
}

func TestSymbolKindLabel(t *testing.T) {
	tests := []struct {
		kind lsp.SymbolKind
		want string
	}{
		{lsp.SymbolKindClass, "[class]"},
		{lsp.SymbolKindStruct, "[class]"},
		{lsp.SymbolKindMethod, "[method]"},
		{lsp.SymbolKindFunction, "[method]"},
		{lsp.SymbolKindConstructor, "[method]"},
		{lsp.SymbolKindNamespace, "[namespace]"},
		{lsp.SymbolKindField, "[field]"},
		{lsp.SymbolKindProperty, "[field]"},
		{lsp.SymbolKindVariable, "[variable]"},
		{lsp.SymbolKindConstant, "[variable]"},
		{lsp.SymbolKind(999), "[symbol]"},
	}
	for _, tt := range tests {
		if got := symbolKindLabel(tt.kind); got != tt.want {
			t.Errorf("symbolKindLabel(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestFollowKindForSymbol(t *testing.T) {
	tests := []struct {
		kind lsp.SymbolKind
		want followKind
	}{
		{lsp.SymbolKindClass, followClass},
		{lsp.SymbolKindStruct, followClass},
		{lsp.SymbolKindMethod, followMethod},
		{lsp.SymbolKindFunction, followMethod},
		{lsp.SymbolKindConstructor, followMethod},
		{lsp.SymbolKindVariable, followMethod}, // best-effort default
	}
	for _, tt := range tests {
		if got := followKindForSymbol(tt.kind); got != tt.want {
			t.Errorf("followKindForSymbol(%d) = %v, want %v", tt.kind, got, tt.want)
		}
	}
}
