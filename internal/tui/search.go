package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/fmbfs/skein/internal/lsp"
)

// maxSearchHistory is how many distinct past queries are remembered for
// up/down recall (docs/SPEC.md's "last 5 searches" — a shell-history-style
// convenience, not a persistent search log).
const maxSearchHistory = 5

// maxVisibleSearchResults caps how many results renderSearchBar draws
// below the input line. Without a cap, a broad query (e.g. a common method
// name across a big workspace) can return dozens of hits, and rendering
// all of them pushes the input line and footer hints out of the terminal's
// visible rows — reported directly by a user: "the typing box gets
// overwhelmed with the results and we cannot see what we are writing".
const maxVisibleSearchResults = 8

// searchState holds the bottom search bar / tangle-view state: the text
// input, the last query's results, which one is highlighted, and a small
// recall history of past queries.
type searchState struct {
	input   textinput.Model
	results []lsp.SymbolInformation
	cursor  int
	err     error

	history    []string // most recent first, capped at maxSearchHistory
	historyIdx int      // -1 when not currently browsing history
}

// newSearchState builds a fresh, unfocused search bar.
func newSearchState() searchState {
	ti := textinput.New()
	ti.Placeholder = "search workspace symbols…"
	ti.CharLimit = 128
	return searchState{input: ti, historyIdx: -1}
}

// reset clears query text and results, e.g. after a search is cancelled.
// History is deliberately left untouched — it should survive across
// searches within the same session.
func (s *searchState) reset() {
	s.input.SetValue("")
	s.input.Blur()
	s.results = nil
	s.cursor = 0
	s.err = nil
	s.historyIdx = -1
}

// pushHistory records query as the most recent search, moving it to the
// front if already present (so re-running a recent search doesn't create a
// duplicate entry), and trims to maxSearchHistory. Blank queries are
// ignored — there's nothing useful to recall.
func (s *searchState) pushHistory(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}
	for i, q := range s.history {
		if q == query {
			s.history = append(s.history[:i], s.history[i+1:]...)
			break
		}
	}
	s.history = append([]string{query}, s.history...)
	if len(s.history) > maxSearchHistory {
		s.history = s.history[:maxSearchHistory]
	}
	s.historyIdx = -1
}

// historyPrev recalls the next-older query into the input box (shell-style
// up-arrow recall). A no-op with no history.
func (s *searchState) historyPrev() {
	if len(s.history) == 0 {
		return
	}
	if s.historyIdx < len(s.history)-1 {
		s.historyIdx++
	}
	s.input.SetValue(s.history[s.historyIdx])
	s.input.CursorEnd()
}

// historyNext recalls the next-newer query, or clears the input once you
// step past the newest entry back to a blank line.
func (s *searchState) historyNext() {
	if s.historyIdx <= 0 {
		s.historyIdx = -1
		s.input.SetValue("")
		return
	}
	s.historyIdx--
	s.input.SetValue(s.history[s.historyIdx])
	s.input.CursorEnd()
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

	start, end := searchResultsWindow(len(s.results), s.cursor)
	shown := s.results[start:end]

	for i, r := range shown {
		b.WriteByte('\n')
		line := fmt.Sprintf("%s  %s", r.Name, mutedStyle.Render(symbolKindLabel(r.Kind)+" "+r.ContainerName))
		if start+i == s.cursor {
			line = selectedLineStyle.Render(line)
		}
		b.WriteString(line)
	}
	if hidden := len(s.results) - (end - start); hidden > 0 {
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render(fmt.Sprintf("… +%d more (↑/↓ to scroll) — refine your search", hidden)))
	}
	return b.String()
}

// searchResultsWindow returns the [start, end) slice bounds of the visible
// results window: at most maxVisibleSearchResults entries, always including
// cursor (so moving the selection past the cap scrolls the window instead
// of hiding the selected line off-screen).
func searchResultsWindow(total, cursor int) (start, end int) {
	if total <= maxVisibleSearchResults {
		return 0, total
	}
	start = cursor - maxVisibleSearchResults/2
	if start < 0 {
		start = 0
	}
	if start > total-maxVisibleSearchResults {
		start = total - maxVisibleSearchResults
	}
	return start, start + maxVisibleSearchResults
}

// searchBarLineCount returns how many lines renderSearchBar will draw for
// the current state — the input line, plus results/error/truncation lines
// — so the caller can shrink the map/detail panels by exactly that much
// and keep the whole layout within the terminal's height. Without this,
// a wide result set pushes the input line and footer hints off-screen
// (docs/SPEC.md's reported "typing box gets overwhelmed" bug).
func searchBarLineCount(s *searchState) int {
	lines := 1 // the input line itself
	if s.err != nil {
		return lines + 1
	}
	shown := len(s.results)
	if shown > maxVisibleSearchResults {
		shown = maxVisibleSearchResults
	}
	lines += shown
	if len(s.results) > maxVisibleSearchResults {
		lines++ // "+N more" line
	}
	return lines
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
