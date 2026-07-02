package tui

import (
	"fmt"
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
	s.historyIdx = 2

	s.reset()
	if s.input.Value() != "" || s.results != nil || s.cursor != 0 || s.err != nil || s.historyIdx != -1 {
		t.Errorf("reset() left state = %+v", s)
	}
}

func TestSearchStateResetKeepsHistory(t *testing.T) {
	s := newSearchState()
	s.pushHistory("keep-me")
	s.reset()
	if len(s.history) != 1 || s.history[0] != "keep-me" {
		t.Errorf("reset() should not clear history, got %v", s.history)
	}
}

func TestSearchStatePushHistoryDedupesAndCaps(t *testing.T) {
	s := newSearchState()
	for _, q := range []string{"one", "two", "three", "four", "five", "six"} {
		s.pushHistory(q)
	}
	want := []string{"six", "five", "four", "three", "two"}
	if len(s.history) != len(want) {
		t.Fatalf("history = %v, want length %d", s.history, len(want))
	}
	for i, q := range want {
		if s.history[i] != q {
			t.Errorf("history[%d] = %q, want %q (full: %v)", i, s.history[i], q, s.history)
		}
	}

	// Re-pushing an existing entry moves it to the front without growing
	// or duplicating the list.
	s.pushHistory("three")
	if len(s.history) != 5 {
		t.Fatalf("history len = %d after re-push, want 5", len(s.history))
	}
	if s.history[0] != "three" {
		t.Errorf("history[0] = %q, want %q after re-push", s.history[0], "three")
	}
	count := 0
	for _, q := range s.history {
		if q == "three" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("history contains %q %d times, want 1: %v", "three", count, s.history)
	}
}

func TestSearchStatePushHistoryIgnoresBlank(t *testing.T) {
	s := newSearchState()
	s.pushHistory("   ")
	if len(s.history) != 0 {
		t.Errorf("history = %v, want blank query ignored", s.history)
	}
}

func TestSearchStateHistoryPrevNextCycle(t *testing.T) {
	s := newSearchState()
	s.pushHistory("first")
	s.pushHistory("second")
	s.pushHistory("third")
	// history is now [third, second, first]

	s.historyPrev()
	if got := s.input.Value(); got != "third" {
		t.Errorf("after 1 historyPrev, input = %q, want %q", got, "third")
	}
	s.historyPrev()
	if got := s.input.Value(); got != "second" {
		t.Errorf("after 2 historyPrev, input = %q, want %q", got, "second")
	}
	s.historyPrev()
	if got := s.input.Value(); got != "first" {
		t.Errorf("after 3 historyPrev, input = %q, want %q", got, "first")
	}
	// Stepping further back than the oldest entry is a no-op.
	s.historyPrev()
	if got := s.input.Value(); got != "first" {
		t.Errorf("historyPrev past oldest = %q, want clamped to %q", got, "first")
	}

	s.historyNext()
	if got := s.input.Value(); got != "second" {
		t.Errorf("after historyNext, input = %q, want %q", got, "second")
	}
	s.historyNext()
	if got := s.input.Value(); got != "third" {
		t.Errorf("after 2nd historyNext, input = %q, want %q", got, "third")
	}
	// Stepping past the newest entry clears the input back to blank.
	s.historyNext()
	if got := s.input.Value(); got != "" {
		t.Errorf("historyNext past newest = %q, want empty", got)
	}
	if s.historyIdx != -1 {
		t.Errorf("historyIdx after clearing = %d, want -1", s.historyIdx)
	}
}

func TestSearchStateHistoryPrevNextNoHistoryIsNoOp(t *testing.T) {
	s := newSearchState()
	s.historyPrev()
	if s.input.Value() != "" {
		t.Errorf("historyPrev with no history should be a no-op, got %q", s.input.Value())
	}
	s.historyNext()
	if s.input.Value() != "" {
		t.Errorf("historyNext with no history should be a no-op, got %q", s.input.Value())
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

func TestSearchResultsWindowUnderCapShowsAll(t *testing.T) {
	start, end := searchResultsWindow(5, 2)
	if start != 0 || end != 5 {
		t.Errorf("searchResultsWindow(5, 2) = (%d, %d), want (0, 5)", start, end)
	}
}

func TestSearchResultsWindowClampsAtStart(t *testing.T) {
	start, end := searchResultsWindow(20, 0)
	if start != 0 || end != maxVisibleSearchResults {
		t.Errorf("searchResultsWindow(20, 0) = (%d, %d), want (0, %d)", start, end, maxVisibleSearchResults)
	}
}

func TestSearchResultsWindowClampsAtEnd(t *testing.T) {
	start, end := searchResultsWindow(20, 19)
	if end != 20 || start != 20-maxVisibleSearchResults {
		t.Errorf("searchResultsWindow(20, 19) = (%d, %d), want (%d, 20)", start, end, 20-maxVisibleSearchResults)
	}
}

func TestSearchResultsWindowCentersOnCursor(t *testing.T) {
	start, end := searchResultsWindow(20, 10)
	if end-start != maxVisibleSearchResults {
		t.Errorf("searchResultsWindow(20, 10) window size = %d, want %d", end-start, maxVisibleSearchResults)
	}
	if 10 < start || 10 >= end {
		t.Errorf("searchResultsWindow(20, 10) = (%d, %d), cursor 10 not in range", start, end)
	}
}

func TestRenderSearchBarTruncatesLargeResultSetsAndKeepsSelectionVisible(t *testing.T) {
	s := newSearchState()
	s.input.SetValue("x")
	for i := 0; i < 20; i++ {
		s.results = append(s.results, lsp.SymbolInformation{Name: fmt.Sprintf("Sym%d", i)})
	}
	s.cursor = 19

	got := renderSearchBar(&s, 60)
	if !strings.Contains(got, "Sym19") {
		t.Errorf("renderSearchBar should keep the selected (last) result visible, got:\n%s", got)
	}
	if !strings.Contains(got, "more") {
		t.Errorf("renderSearchBar should show a truncation indicator when results exceed the cap, got:\n%s", got)
	}
}

func TestSearchBarLineCount(t *testing.T) {
	s := newSearchState()
	if got := searchBarLineCount(&s); got != 1 {
		t.Errorf("searchBarLineCount with no results = %d, want 1 (just the input line)", got)
	}

	s.err = errBoom
	if got := searchBarLineCount(&s); got != 2 {
		t.Errorf("searchBarLineCount with error = %d, want 2", got)
	}
	s.err = nil

	s.results = []lsp.SymbolInformation{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	if got := searchBarLineCount(&s); got != 4 {
		t.Errorf("searchBarLineCount with 3 results = %d, want 4", got)
	}

	for i := 0; i < 20; i++ {
		s.results = append(s.results, lsp.SymbolInformation{Name: fmt.Sprintf("extra%d", i)})
	}
	want := 1 + maxVisibleSearchResults + 1 // input + capped results + "+N more" line
	if got := searchBarLineCount(&s); got != want {
		t.Errorf("searchBarLineCount with %d results = %d, want %d", len(s.results), got, want)
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
