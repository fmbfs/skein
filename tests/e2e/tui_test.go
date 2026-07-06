package e2e

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fmbfs/skein/internal/tui"
)

// drainCmd runs a tea.Cmd (unwrapping any tea.BatchMsg it produces) and
// returns every resulting concrete tea.Msg. Small helper so the tests below
// can drive Model.Update through a real async chain (nudge -> search
// debounce -> search results, or resolve -> threadLoaded) without
// re-deriving the batch-unwrapping logic each time.
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			out = append(out, drainCmd(sub)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// pumpModel feeds msg into m.Update, then keeps draining and feeding back
// any cmd the update produced, breadth-first, until the chain settles
// (no more cmds). Returns the final model. This is what makes it possible
// to drive Model through a real multi-step async flow (debounce -> query,
// resolve -> load) from outside the package using only its exported API.
func pumpModel(t *testing.T, m tui.Model, msg tea.Msg) tui.Model {
	t.Helper()
	pending := []tea.Msg{msg}
	for len(pending) > 0 {
		next := pending[0]
		pending = pending[1:]
		tm, cmd := m.Update(next)
		m = tm.(tui.Model)
		pending = append(pending, drainCmd(cmd)...)
	}
	return m
}

func runeKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// TestTUIBareLaunchSearchAndFollow is the E2E smoke test for a bare `skein`
// launch (no initial symbol on the CLI) against real clangd — the open
// item from handoff.md's tui-e2e-bare-launch-smoke-test todo, carried
// forward via the M375 skein review (projects/skein-review.md). It
// exercises the full stack the existing draw-mode E2E test doesn't touch
// at all: Model.Init's nudgeIndexerCmd cold-start path (issue #8's fix),
// the debounced search flow (searchDebounceCmd -> searchCmd ->
// searchResultsMsg), and following a search result into a loaded thread.
func TestTUIBareLaunchSearchAndFollow(t *testing.T) {
	requireClangdAndCmake(t)

	dir := fixtureDir(t)
	buildDir := buildFixture(t, dir)
	client := newFixtureClient(t, buildDir)
	defer client.Close()

	// Bare launch: no initial symbol, no pinned symbol.
	m := tui.New(client, buildDir, "", "")
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = m2.(tui.Model)

	// Init() on a bare launch must return the indexer-nudge cmd (not a
	// resolveGeneric cmd, since there's nothing to resolve yet) — running
	// it is what stops "search vs load time is too long" from a cold
	// start (issue #8).
	for _, msg := range drainCmd(m.Init()) {
		m = pumpModel(t, m, msg)
	}

	// Focus the search bar ("/") and type a query that must resolve
	// against the fixture via a real workspace/symbol round trip.
	m = pumpModel(t, m, runeKey("/"))
	m = pumpModel(t, m, runeKey("processFrame"))

	view := m.View()
	if !strings.Contains(view, "processFrame") {
		t.Fatalf("TUI view after searching %q = %q, want it to list a matching result", "processFrame", view)
	}

	// Select the first (only) result and follow it.
	m = pumpModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	finalView := m.View()
	if !strings.Contains(finalView, "processFrame") {
		t.Fatalf("TUI view after following the search result = %q, want the loaded processFrame thread", finalView)
	}
	// Note: the original bundle tab keeps its initial "tangle" label even
	// once a thread is loaded into it via search-follow — bundle.name is
	// only set on creation (New/pinBundle), never renamed by
	// applyThreadLoaded's non-pinning branch. That's existing, deliberate
	// behaviour (only `p` creates a freshly-named tab), not a defect this
	// smoke test is checking for.
}

// TestTUIResolve_NoCompileCommands is the negative-path E2E case flagged in
// M2 (skein-review.md): launching against a root with no
// compile_commands.json must surface a clean "not found" error through the
// normal threadLoadedMsg path, not hang or panic. clangd itself tolerates a
// missing compile database (confirmed empirically: it starts and answers
// workspace/symbol with an empty result, it does not error), so this
// exercises skein's own error surfacing, not a clangd crash.
func TestTUIResolve_NoCompileCommands(t *testing.T) {
	requireClangdAndCmake(t)

	emptyDir := t.TempDir() // no compile_commands.json here at all
	client := newFixtureClient(t, emptyDir)
	defer client.Close()

	// An initial symbol drives Init() down the resolveGenericCmd path
	// (which must yield an error threadLoadedMsg), rather than the
	// bare-launch nudge path (best-effort/silent by design).
	m := tui.New(client, emptyDir, "processFrame", "")
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = m2.(tui.Model)

	for _, msg := range drainCmd(m.Init()) {
		m = pumpModel(t, m, msg)
	}

	view := m.View()
	if !strings.Contains(view, `no symbol found matching "processFrame"`) {
		t.Fatalf("TUI view after resolving against an empty root = %q, want a \"no symbol found\" error surfaced in the status bar", view)
	}
}
