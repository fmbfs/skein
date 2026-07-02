package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/fmbfs/skein/internal/lsp"
)

// searchState holds the bottom search bar / tangle-view state: the text
// input, the last query's results, and which one is highlighted.
type searchState struct {
	input   textinput.Model
	results []lsp.SymbolInformation
	cursor  int
	err     error
}

// newSearchState builds a fresh, unfocused search bar.
func newSearchState() searchState {
	ti := textinput.New()
	ti.Placeholder = "search workspace symbols…"
	ti.CharLimit = 128
	return searchState{input: ti}
}

// reset clears query text and results, e.g. after a search is cancelled.
func (s *searchState) reset() {
	s.input.SetValue("")
	s.input.Blur()
	s.results = nil
	s.cursor = 0
	s.err = nil
}

// moveCursor shifts the highlighted result by delta, clamped to bounds.
func (s *searchState) moveCursor(delta int) {
	if len(s.results) == 0 {
		return
	}
	s.cursor += delta
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor >= len(s.results) {
		s.cursor = len(s.results) - 1
	}
}

// selected returns the currently-highlighted result, or nil if there are
// none (e.g. no query typed yet, or the last search returned nothing).
func (s *searchState) selected() *lsp.SymbolInformation {
	if s.cursor < 0 || s.cursor >= len(s.results) {
		return nil
	}
	return &s.results[s.cursor]
}

// renderSearchBar renders the input line plus, when active, a dropdown of
// results below it (docs/SPEC.md section 4: "bottom — always-available
// symbol search bar, fuzzy, workspace-wide").
func renderSearchBar(s *searchState, width int) string {
	var b strings.Builder
	b.WriteString(searchBarStyle.Width(width).Render("> " + s.input.View()))

	if s.err != nil {
		b.WriteByte('\n')
		b.WriteString(errorStyle.Render(s.err.Error()))
		return b.String()
	}

	for i, r := range s.results {
		b.WriteByte('\n')
		line := fmt.Sprintf("%s  %s", r.Name, mutedStyle.Render(symbolKindLabel(r.Kind)+" "+r.ContainerName))
		if i == s.cursor {
			line = selectedLineStyle.Render(line)
		}
		b.WriteString(line)
	}
	return b.String()
}

// symbolKindLabel renders an lsp.SymbolKind as a short lowercase tag for
// display, matching the vocabulary used elsewhere in the map panel.
func symbolKindLabel(k lsp.SymbolKind) string {
	switch k {
	case lsp.SymbolKindClass, lsp.SymbolKindStruct:
		return "[class]"
	case lsp.SymbolKindMethod, lsp.SymbolKindFunction, lsp.SymbolKindConstructor:
		return "[method]"
	case lsp.SymbolKindNamespace:
		return "[namespace]"
	case lsp.SymbolKindField, lsp.SymbolKindProperty:
		return "[field]"
	case lsp.SymbolKindVariable, lsp.SymbolKindConstant:
		return "[variable]"
	default:
		return "[symbol]"
	}
}

// followKindForSymbol maps a workspace/symbol result's Kind to the
// followKind used to dispatch the right compositor when the user selects
// it from tangle-view/search results.
func followKindForSymbol(k lsp.SymbolKind) followKind {
	switch k {
	case lsp.SymbolKindClass, lsp.SymbolKindStruct:
		return followClass
	case lsp.SymbolKindMethod, lsp.SymbolKindFunction, lsp.SymbolKindConstructor:
		return followMethod
	default:
		return followMethod // best-effort default; most workspace/symbol hits are callable
	}
}
