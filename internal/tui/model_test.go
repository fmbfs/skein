package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fmbfs/skein/internal/compositor"
	"github.com/fmbfs/skein/internal/lsp"
)

func newTestModel() Model {
	m := New(nil, "/root", "", "")
	m.width, m.height = 80, 24
	return m
}

func TestNewSetsUpTangleBundle(t *testing.T) {
	m := New(nil, "/root", "", "")
	if len(m.bundles) != 1 || m.bundles[0].name != "tangle" {
		t.Fatalf("New() bundles = %+v, want a single tangle bundle", m.bundles)
	}
	if !m.showIn || !m.showOut {
		t.Error("New() should default to showing both directions")
	}
}

// TestInitNoPendingSymbolNudgesIndexer is the regression test for a bare
// `skein` launch (no CLI symbol) leaving clangd's indexer never nudged: a
// user going straight to interactive search (`/`) would then find every
// search silently returns nothing forever, since clangd's background index
// never populates without at least one textDocument/didOpen (see
// compositor.NudgeIndexer's doc comment). Init() must still return a
// non-nil cmd in this case so the nudge actually happens.
func TestInitNoPendingSymbolNudgesIndexer(t *testing.T) {
	m := New(nil, "/root", "", "")
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() with no pending symbol should still return a non-nil cmd (indexer nudge)")
	}
	msg := cmd()
	if _, ok := msg.(indexerNudgedMsg); !ok {
		t.Errorf("Init()'s cmd produced %T, want indexerNudgedMsg", msg)
	}
}

func TestInitWithPendingSymbolReturnsCmd(t *testing.T) {
	m := New(nil, "/root", "Foo", "")
	if cmd := m.Init(); cmd == nil {
		t.Error("Init() with a pending symbol should return a non-nil cmd")
	}
}

func TestInitWithPendingAndPinnedReturnsBatchedCmd(t *testing.T) {
	m := New(nil, "/root", "Foo", "Bar")
	if cmd := m.Init(); cmd == nil {
		t.Error("Init() with pending + pinned symbols should return a non-nil cmd")
	}
}

func TestApplyThreadLoadedError(t *testing.T) {
	m := newTestModel()
	got, cmd := m.applyThreadLoaded(threadLoadedMsg{err: errors.New("boom")})
	gm := got.(Model)
	if gm.err == nil || gm.err.Error() != "boom" {
		t.Errorf("expected err to be set, got %v", gm.err)
	}
	if cmd != nil {
		t.Error("expected nil cmd on error")
	}
}

func TestApplyThreadLoadedReplacesActiveThread(t *testing.T) {
	m := newTestModel()
	state := threadState{name: "Foo", kind: "method"}
	got, _ := m.applyThreadLoaded(threadLoadedMsg{state: state})
	gm := got.(Model)
	if gm.bundles[gm.activeBundle].thread.name != "Foo" {
		t.Errorf("active thread = %+v, want name Foo", gm.bundles[gm.activeBundle].thread)
	}
}

func TestApplyThreadLoadedPinningCreatesNewBundle(t *testing.T) {
	m := newTestModel()
	state := threadState{name: "Foo", kind: "method"}
	got, _ := m.applyThreadLoaded(threadLoadedMsg{state: state, pinning: true, label: "Foo"})
	gm := got.(Model)
	if len(gm.bundles) != 2 {
		t.Fatalf("expected a new bundle to be pinned, got %d bundles", len(gm.bundles))
	}
	if gm.activeBundle != 1 || gm.bundles[1].name != "Foo" {
		t.Errorf("pinned bundle = %+v active=%d, want name Foo at index 1", gm.bundles[1], gm.activeBundle)
	}
}

func TestHandleKeyQuit(t *testing.T) {
	m := newTestModel()
	got, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	gm := got.(Model)
	if !gm.quitting {
		t.Error("expected quitting to be true after 'q'")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd after 'q'")
	}
}

func TestHandleKeyHelpToggle(t *testing.T) {
	m := newTestModel()
	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	gm := got.(Model)
	if !gm.help {
		t.Error("expected help to toggle on")
	}
	got2, _ := gm.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	gm2 := got2.(Model)
	if gm2.help {
		t.Error("expected help to toggle back off")
	}
}

func TestHandleKeySearchFocus(t *testing.T) {
	m := newTestModel()
	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	gm := got.(Model)
	if gm.focus != focusSearch {
		t.Errorf("focus = %v, want focusSearch", gm.focus)
	}
}

func TestHandleKeyTabCyclesFocus(t *testing.T) {
	m := newTestModel()
	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	gm := got.(Model)
	if gm.focus != focusDetail {
		t.Errorf("focus after one tab = %v, want focusDetail", gm.focus)
	}
}

func TestHandleKeyToggleDirections(t *testing.T) {
	m := newTestModel()
	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	gm := got.(Model)
	if gm.showIn {
		t.Error("expected showIn to toggle off")
	}
	got2, _ := gm.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	gm2 := got2.(Model)
	if gm2.showOut {
		t.Error("expected showOut to toggle off")
	}
}

func TestHandleKeyPinAndCloseBundle(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{name: "Foo", kind: "method"}

	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	gm := got.(Model)
	if len(gm.bundles) != 2 {
		t.Fatalf("expected pin to create a new bundle, got %d", len(gm.bundles))
	}

	got2, _ := gm.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	gm2 := got2.(Model)
	if len(gm2.bundles) != 1 {
		t.Errorf("expected close to remove the pinned bundle, got %d bundles", len(gm2.bundles))
	}
}

func TestHandleKeyCycleBundle(t *testing.T) {
	m := newTestModel()
	m.bundles = append(m.bundles, bundle{name: "second"})

	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	gm := got.(Model)
	if gm.activeBundle != 1 {
		t.Errorf("activeBundle = %d, want 1 after ']'", gm.activeBundle)
	}

	got2, _ := gm.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	gm2 := got2.(Model)
	if gm2.activeBundle != 0 {
		t.Errorf("activeBundle = %d, want 0 after '['", gm2.activeBundle)
	}
}

func TestHandleKeyUpDownMovesCursor(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{
		name: "Foo", kind: "method",
		nodes: []Node{{Label: "a"}, {Label: "b"}, {Label: "c"}},
	}

	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	gm := got.(Model)
	if gm.bundles[0].thread.cursor != 1 {
		t.Errorf("cursor = %d, want 1 after 'j'", gm.bundles[0].thread.cursor)
	}

	got2, _ := gm.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	gm2 := got2.(Model)
	if gm2.bundles[0].thread.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after 'k'", gm2.bundles[0].thread.cursor)
	}
}

func TestFollowNoneIsNoop(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{
		name: "Foo", kind: "method",
		nodes: []Node{{Label: "unfollowable"}},
	}
	got, cmd := m.follow()
	gm := got.(Model)
	if cmd != nil {
		t.Error("expected nil cmd following a non-followable node")
	}
	if len(gm.bundles[0].back) != 0 {
		t.Error("expected no spool entry to be pushed for a no-op follow")
	}
}

func TestFollowMethodPushesSpoolAndReturnsCmd(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{
		name: "Foo", kind: "method",
		nodes: []Node{{Label: "Bar", Follow: followMethod, Target: "Bar"}},
	}
	got, cmd := m.follow()
	gm := got.(Model)
	if cmd == nil {
		t.Error("expected a non-nil cmd for a followable node")
	}
	if len(gm.bundles[0].back) != 1 {
		t.Errorf("expected the previous thread to be pushed onto the spool, back=%+v", gm.bundles[0].back)
	}
}

// TestGotoSelectedNoneIsNoop confirms g on a node with no GotoPath and no
// Follow target (e.g. a section header) does nothing.
func TestGotoSelectedNoneIsNoop(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{
		name: "Foo", kind: "method",
		nodes: []Node{{Label: "header"}},
	}
	_, cmd := m.gotoSelected()
	if cmd != nil {
		t.Error("expected nil cmd for a goto-less, follow-less node")
	}
}

// TestGotoSelectedWithKnownPathReturnsCmd covers the fast path: a node
// that already carries a GotoPath (definitions, call sites) should jump
// straight to an editor-launching cmd without needing an LSP round trip.
func TestGotoSelectedWithKnownPathReturnsCmd(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{
		name: "Foo", kind: "method",
		nodes: []Node{{Label: "defined in foo.cpp:12", GotoPath: "foo.cpp", GotoLine: 12}},
	}
	_, cmd := m.gotoSelected()
	if cmd == nil {
		t.Fatal("expected a non-nil cmd for a node with a known GotoPath")
	}
}

// TestGotoSelectedResolveFallbackReturnsCmd covers a symbol-target node
// with no baked-in location (e.g. a call or member) — it must fall back
// to gotoResolveCmd instead of being a no-op.
func TestGotoSelectedResolveFallbackReturnsCmd(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{
		name: "Foo", kind: "method",
		nodes: []Node{{Label: "calls Bar", Follow: followMethod, Target: "Bar"}},
	}
	_, cmd := m.gotoSelected()
	if cmd == nil {
		t.Fatal("expected a non-nil cmd falling back to workspace/symbol resolution")
	}
}

// TestGotoSelectedOutOfBoundsCursorIsNoop mirrors follow()'s cursor guard.
func TestGotoSelectedOutOfBoundsCursorIsNoop(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{name: "Foo", kind: "method", cursor: 5}
	_, cmd := m.gotoSelected()
	if cmd != nil {
		t.Error("expected nil cmd for an out-of-bounds cursor")
	}
}

func TestSpoolBackForwardReset(t *testing.T) {
	m := newTestModel()
	origin := threadState{name: "origin"}
	next := threadState{name: "next"}
	m.bundles[0] = bundle{name: "tangle", origin: origin, thread: origin}
	m.bundles[0].thread = next
	m.bundles[0].back = []threadState{origin}

	back := m.spoolBack()
	if back.bundles[0].thread.name != "origin" {
		t.Errorf("after spoolBack, thread = %q, want origin", back.bundles[0].thread.name)
	}
	if len(back.bundles[0].fwd) != 1 {
		t.Errorf("expected forward history to gain an entry, got %+v", back.bundles[0].fwd)
	}

	fwd := back.spoolForward()
	if fwd.bundles[0].thread.name != "next" {
		t.Errorf("after spoolForward, thread = %q, want next", fwd.bundles[0].thread.name)
	}

	reset := fwd.spoolReset()
	if reset.bundles[0].thread.name != "origin" {
		t.Errorf("after spoolReset, thread = %q, want origin", reset.bundles[0].thread.name)
	}
	if len(reset.bundles[0].back) != 0 || len(reset.bundles[0].fwd) != 0 {
		t.Errorf("spoolReset should clear back/fwd history, got back=%+v fwd=%+v",
			reset.bundles[0].back, reset.bundles[0].fwd)
	}
}

func TestSpoolBackNoopWhenEmpty(t *testing.T) {
	m := newTestModel()
	got := m.spoolBack()
	if got.bundles[0].thread.name != m.bundles[0].thread.name {
		t.Error("spoolBack with empty history should be a no-op")
	}
}

func TestAdjustPlyOnlyAppliesToMethodThreads(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{name: "Foo", kind: "class"}
	_, cmd := m.adjustPly(1)
	if cmd != nil {
		t.Error("adjustPly should no-op for non-method threads")
	}
}

func TestAdjustPlyClampsRange(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{name: "Foo", kind: "method", ply: maxPly}
	_, cmd := m.adjustPly(1)
	if cmd != nil {
		t.Error("adjustPly should no-op when already at maxPly")
	}

	m.bundles[0].thread.ply = minPly
	_, cmd = m.adjustPly(-1)
	if cmd != nil {
		t.Error("adjustPly should no-op when already at minPly")
	}

	m.bundles[0].thread.ply = 2
	_, cmd = m.adjustPly(1)
	if cmd == nil {
		t.Error("adjustPly should return a cmd for a valid ply change")
	}
}

func TestClampCursor(t *testing.T) {
	nodes := []Node{{Label: "a"}, {Label: "b"}}
	if got := clampCursor(-1, nodes); got != 0 {
		t.Errorf("clampCursor(-1) = %d, want 0", got)
	}
	if got := clampCursor(5, nodes); got != 1 {
		t.Errorf("clampCursor(5) = %d, want 1", got)
	}
	if got := clampCursor(0, nil); got != 0 {
		t.Errorf("clampCursor with no nodes = %d, want 0", got)
	}
}

func TestLocationString(t *testing.T) {
	if got := locationString(compositor.Location{}); got != "" {
		t.Errorf("locationString(empty) = %q, want empty", got)
	}
	got := locationString(compositor.Location{Path: "a.cpp", Line: 5})
	if got != "a.cpp:5" {
		t.Errorf("locationString = %q, want a.cpp:5", got)
	}
}

func TestThreadStateFromRelationMap(t *testing.T) {
	rm := &compositor.RelationMap{
		ThreadName: "Foo",
		Kind:       "method",
		DefinedAt:  compositor.Location{Path: "a.cpp", Line: 1},
		Signature:  "void Foo()",
		Container:  "MyClass",
		Ambiguous:  []string{"Other"},
	}
	ts := threadStateFromRelationMap(rm, "MyClass", 2, "")
	if ts.name != "Foo" || ts.classFilter != "MyClass" || ts.ply != 2 || ts.container != "MyClass" {
		t.Errorf("threadStateFromRelationMap = %+v", ts)
	}
	if ts.definedAt != "a.cpp:1" {
		t.Errorf("definedAt = %q, want a.cpp:1", ts.definedAt)
	}
}

func TestThreadStateFromRelationMap_Warning(t *testing.T) {
	rm := &compositor.RelationMap{ThreadName: "Foo", Kind: "method"}
	ts := threadStateFromRelationMap(rm, "", 1, "some truncation warning")
	if ts.warning != "some truncation warning" {
		t.Errorf("warning = %q, want %q", ts.warning, "some truncation warning")
	}
}

func TestThreadStateFromClassMap(t *testing.T) {
	cm := &compositor.ClassMap{ThreadName: "MyClass", Kind: "class"}
	ts := threadStateFromClassMap(cm, "")
	if ts.name != "MyClass" || ts.kind != "class" {
		t.Errorf("threadStateFromClassMap = %+v", ts)
	}
}

func TestThreadStateFromFileMap(t *testing.T) {
	fm := &compositor.FileMap{ThreadName: "foo.cpp", Kind: "file"}
	ts := threadStateFromFileMap(fm, "")
	if ts.name != "foo.cpp" || ts.kind != "file" {
		t.Errorf("threadStateFromFileMap = %+v", ts)
	}
}

func TestCombineWarnings(t *testing.T) {
	got := combineWarnings("", compositor.TruncationWarning("a"), "", compositor.TruncationWarning("b"))
	if got != "a b" {
		t.Errorf("combineWarnings = %q, want %q", got, "a b")
	}
	if got := combineWarnings("", ""); got != "" {
		t.Errorf("combineWarnings(all empty) = %q, want empty", got)
	}
}

func TestHandleKeyDigitJumpsToBundle(t *testing.T) {
	m := newTestModel()
	m.bundles = append(m.bundles, bundle{name: "b1"}, bundle{name: "b2"})
	m.activeBundle = 0
	newModel, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	got := newModel.(Model)
	if got.activeBundle != 2 {
		t.Errorf("activeBundle after pressing '3' = %d, want 2", got.activeBundle)
	}
}

func TestHandleKeyDigitBeyondBundleCountIsNoop(t *testing.T) {
	m := newTestModel()
	m.activeBundle = 0
	newModel, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("9")})
	got := newModel.(Model)
	if got.activeBundle != 0 {
		t.Errorf("activeBundle after pressing '9' with only 1 bundle = %d, want 0 (no-op)", got.activeBundle)
	}
}

func TestPinCurrent(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{name: "Foo"}
	got := m.pinCurrent()
	if len(got.bundles) != 2 || got.bundles[1].name != "Foo" {
		t.Errorf("pinCurrent bundles = %+v, want a new bundle named Foo", got.bundles)
	}
	if !got.bundles[1].pinned {
		t.Error("pinCurrent should mark the new bundle as pinned")
	}
	if got.activeBundle != 1 {
		t.Errorf("activeBundle after pinCurrent = %d, want 1 (switch to the new tab)", got.activeBundle)
	}
}

// TestPinCurrentAlwaysCreatesNewBundle replaces the old toggle-to-unpin
// behaviour: p now always stacks a new tab, even when pressed again from
// an already-pinned tab, so repeated pinning (e.g. to compare the same
// symbol at different ply/filter settings) reliably works instead of
// sometimes closing the tab you just made.
func TestPinCurrentAlwaysCreatesNewBundle(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{name: "Foo"}
	m = m.pinCurrent() // pin -> now on the new "Foo" tab
	if len(m.bundles) != 2 {
		t.Fatalf("expected 2 bundles after pinning, got %d", len(m.bundles))
	}

	m = m.pinCurrent() // press p again on the pinned tab -> stacks another
	if len(m.bundles) != 3 {
		t.Errorf("bundles after re-pressing p on a pinned tab = %+v, want 3 (stack, not unpin)", m.bundles)
	}
}

// TestUnpinCurrentClosesPinnedTab is the regression test for the reported
// "we are unable to unpin" bug: u must close the active bundle when it's
// a pinned tab.
func TestUnpinCurrentClosesPinnedTab(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{name: "Foo"}
	m = m.pinCurrent()
	if len(m.bundles) != 2 {
		t.Fatalf("expected 2 bundles after pinning, got %d", len(m.bundles))
	}
	m = m.unpinCurrent()
	if len(m.bundles) != 1 {
		t.Errorf("bundles after unpinCurrent = %+v, want 1", m.bundles)
	}
}

// TestUnpinCurrentIsNoopOnTangleTab confirms the original "tangle" entry
// bundle can never be unpinned (there's nothing to unpin there).
func TestUnpinCurrentIsNoopOnTangleTab(t *testing.T) {
	m := newTestModel()
	m = m.unpinCurrent()
	if len(m.bundles) != 1 {
		t.Errorf("bundles after unpinCurrent on tangle tab = %+v, want unchanged 1", m.bundles)
	}
}

// TestPinCurrentIsNoopOnTangleTab is the regression test for the
// "multiple empty tangle bundles" bug reported in handoff.md
// (tui-fix-multiple-empty-tangle-bundles): pressing p before any thread is
// selected must not stack duplicate, unusable "[tangle]" tabs.
func TestPinCurrentIsNoopOnTangleTab(t *testing.T) {
	m := newTestModel()
	m = m.pinCurrent()
	if len(m.bundles) != 1 {
		t.Errorf("bundles after pinCurrent on tangle tab = %+v, want unchanged 1 (no-op)", m.bundles)
	}
	if m.activeBundle != 0 {
		t.Errorf("activeBundle after pinCurrent on tangle tab = %d, want unchanged 0", m.activeBundle)
	}
}

func TestFollowSearchResultNoSelection(t *testing.T) {
	m := newTestModel()
	got, cmd := m.followSearchResult()
	if cmd != nil {
		t.Error("followSearchResult with no selection should return a nil cmd")
	}
	_ = got
}

func TestUpdateWindowSizeMsg(t *testing.T) {
	m := New(nil, "/root", "", "")
	got, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	gm := got.(Model)
	if gm.width != 100 || gm.height != 40 {
		t.Errorf("width/height = %d/%d, want 100/40", gm.width, gm.height)
	}
}

// TestUpdateGotoResolvedMsgError surfaces a failed workspace/symbol
// resolution as the status-bar error, rather than silently dropping it.
func TestUpdateGotoResolvedMsgError(t *testing.T) {
	m := newTestModel()
	got, cmd := m.Update(gotoResolvedMsg{err: errors.New("not found")})
	gm := got.(Model)
	if gm.err == nil {
		t.Error("expected err to be set on a failed goto resolution")
	}
	if cmd != nil {
		t.Error("expected nil cmd on a failed goto resolution")
	}
}

// TestUpdateGotoResolvedMsgSuccessReturnsExecCmd confirms a successful
// resolution launches the editor (a non-nil cmd) instead of erroring.
func TestUpdateGotoResolvedMsgSuccessReturnsExecCmd(t *testing.T) {
	m := newTestModel()
	got, cmd := m.Update(gotoResolvedMsg{path: "foo.cpp", line: 12})
	gm := got.(Model)
	if gm.err != nil {
		t.Errorf("expected no err on a successful goto resolution, got %v", gm.err)
	}
	if cmd == nil {
		t.Error("expected a non-nil cmd (tea.ExecProcess) on a successful goto resolution")
	}
}

// TestUpdateGotoDoneMsgSurfacesExecError confirms a failed editor
// subprocess (nonzero exit, binary not found) surfaces as the status-bar
// error rather than vanishing silently.
func TestUpdateGotoDoneMsgSurfacesExecError(t *testing.T) {
	m := newTestModel()
	got, _ := m.Update(gotoDoneMsg{err: errors.New("exit status 1")})
	gm := got.(Model)
	if gm.err == nil {
		t.Error("expected err to be set after a failed editor subprocess")
	}
}

// TestUpdateGotoDoneMsgNoErrorIsNoop confirms a clean editor exit doesn't
// spuriously set an error.
func TestUpdateGotoDoneMsgNoErrorIsNoop(t *testing.T) {
	m := newTestModel()
	got, cmd := m.Update(gotoDoneMsg{})
	gm := got.(Model)
	if gm.err != nil {
		t.Errorf("expected no err after a clean editor exit, got %v", gm.err)
	}
	if cmd != nil {
		t.Error("expected nil cmd after a clean editor exit")
	}
}

// TestHandleKeyClearsStaleErr is the regression test for a reported bug:
// switching tabs (or pressing any other key) kept showing a stale error
// banner from an earlier, unrelated failed action even though the new
// tab's content loaded and switched correctly.
func TestHandleKeyClearsStaleErr(t *testing.T) {
	m := newTestModel()
	m.bundles = append(m.bundles, bundle{name: "second"})
	m.err = errors.New("stale error from a previous action")

	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	gm := got.(Model)
	if gm.err != nil {
		t.Errorf("expected err to be cleared after switching tabs, got %v", gm.err)
	}
}

func TestUpdateSearchResultsMsg(t *testing.T) {
	m := newTestModel()
	got, _ := m.Update(searchResultsMsg{err: errors.New("oops")})
	gm := got.(Model)
	if gm.search.err == nil {
		t.Error("expected search.err to be set")
	}
}

func TestHandleSearchKeyEscape(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.input.SetValue("query")
	m.search.results = []lsp.SymbolInformation{{Name: "A"}}
	got, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	gm := got.(Model)
	if gm.focus != focusMap {
		t.Errorf("focus after esc = %v, want focusMap", gm.focus)
	}
	if gm.search.input.Value() != "" || gm.search.results != nil {
		t.Error("expected escape to reset the search state")
	}
}

// TestHandleSearchKeyEscapePushesHistory confirms cancelling a search with
// Escape still remembers the typed query for later recall via up/down —
// not just searches that were actually followed.
func TestHandleSearchKeyEscapePushesHistory(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.input.SetValue("query")
	got, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	gm := got.(Model)
	if len(gm.search.history) != 1 || gm.search.history[0] != "query" {
		t.Errorf("history after escape = %v, want [query]", gm.search.history)
	}
}

func TestHandleSearchKeyResultNav(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.results = []lsp.SymbolInformation{{Name: "A"}, {Name: "B"}}

	got, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyDown})
	gm := got.(Model)
	if gm.search.cursor != 1 {
		t.Errorf("cursor = %d, want 1 after ResultDown", gm.search.cursor)
	}

	got2, _ := gm.handleSearchKey(tea.KeyMsg{Type: tea.KeyUp})
	gm2 := got2.(Model)
	if gm2.search.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after ResultUp", gm2.search.cursor)
	}
}

// TestHandleSearchKeyResultNavFallsBackToHistory confirms that when there
// are no results to navigate (nothing typed, or the last query came back
// empty), ResultUp/ResultDown instead recall past searches — the
// shell-history behaviour requested directly by a user ("we need to be
// able to see last searched using arrows").
func TestHandleSearchKeyResultNavFallsBackToHistory(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.pushHistory("earlier-query")
	// No results currently loaded.

	got, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyUp})
	gm := got.(Model)
	if gm.search.input.Value() != "earlier-query" {
		t.Errorf("input after ResultUp with no results = %q, want %q", gm.search.input.Value(), "earlier-query")
	}

	got2, _ := gm.handleSearchKey(tea.KeyMsg{Type: tea.KeyDown})
	gm2 := got2.(Model)
	if gm2.search.input.Value() != "" {
		t.Errorf("input after ResultDown past newest = %q, want empty", gm2.search.input.Value())
	}
}

func TestHandleSearchKeyTypingUpdatesInputAndSearches(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.input.Focus()

	got, cmd := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	gm := got.(Model)
	if gm.search.input.Value() != "f" {
		t.Errorf("search input value = %q, want %q", gm.search.input.Value(), "f")
	}
	if cmd == nil {
		t.Error("expected a non-nil cmd once a non-empty query is typed")
	}
}

// TestHandleSearchKeyTyping_BumpsGeneration is the regression test for the
// "search vs load time is too long" fix: every keystroke must bump
// search.generation, and produce a debounce cmd rather than firing
// searchCmd directly, so a fast typing burst collapses to one eventual
// workspace/symbol query instead of one per keystroke.
func TestHandleSearchKeyTyping_BumpsGeneration(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.input.Focus()
	startGen := m.search.generation

	got, cmd := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	gm := got.(Model)
	if gm.search.generation != startGen+1 {
		t.Errorf("generation = %d, want %d after one keystroke", gm.search.generation, startGen+1)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil debounce cmd")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want tea.BatchMsg wrapping the debounce cmd", msg)
	}
	var dbg searchDebounceMsg
	found := false
	for _, sub := range batch {
		if sub == nil {
			continue
		}
		if d, ok := sub().(searchDebounceMsg); ok {
			dbg = d
			found = true
		}
	}
	if !found {
		t.Fatal("expected a searchDebounceMsg among the batched cmds (debounced, not an immediate searchResultsMsg)")
	}
	if dbg.gen != gm.search.generation || dbg.query != "f" {
		t.Errorf("searchDebounceMsg = %+v, want gen=%d query=%q", dbg, gm.search.generation, "f")
	}
}

// TestUpdateSearchDebounceMsg_StaleGenerationDropped confirms a
// searchDebounceMsg tagged with an older generation (i.e. the user kept
// typing after it was scheduled) is dropped instead of firing a query.
func TestUpdateSearchDebounceMsg_StaleGenerationDropped(t *testing.T) {
	m := newTestModel()
	m.search.generation = 5

	got, cmd := m.Update(searchDebounceMsg{gen: 3, query: "stale"})
	gm := got.(Model)
	if cmd != nil {
		t.Error("expected a nil cmd for a stale (superseded) debounce trigger")
	}
	if gm.search.generation != 5 {
		t.Errorf("generation = %d, want unchanged 5", gm.search.generation)
	}
}

// TestUpdateSearchDebounceMsg_CurrentGenerationFiresQuery confirms a
// still-current searchDebounceMsg issues the actual searchCmd.
func TestUpdateSearchDebounceMsg_CurrentGenerationFiresQuery(t *testing.T) {
	m := newTestModel()
	m.search.generation = 2

	_, cmd := m.Update(searchDebounceMsg{gen: 2, query: "foo"})
	if cmd == nil {
		t.Fatal("expected a non-nil cmd for a current-generation debounce trigger")
	}
}

// TestUpdateSearchResultsMsg_StaleGenerationDropped confirms a
// searchResultsMsg from a superseded query doesn't clobber newer search
// state (e.g. results for query "f" arriving after the user has already
// typed "foo").
func TestUpdateSearchResultsMsg_StaleGenerationDropped(t *testing.T) {
	m := newTestModel()
	m.search.generation = 2
	m.search.results = []lsp.SymbolInformation{{Name: "current"}}

	got, _ := m.Update(searchResultsMsg{gen: 1, results: []lsp.SymbolInformation{{Name: "stale"}}})
	gm := got.(Model)
	if len(gm.search.results) != 1 || gm.search.results[0].Name != "current" {
		t.Errorf("results = %+v, want unchanged (stale result dropped)", gm.search.results)
	}
}

func TestHandleSearchKeyClearingQueryDropsResults(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.input.Focus()
	m.search.input.SetValue("f")
	m.search.results = []lsp.SymbolInformation{{Name: "A"}}

	got, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	gm := got.(Model)
	if gm.search.results != nil {
		t.Errorf("expected results to be cleared once the query is empty, got %+v", gm.search.results)
	}
}

func TestFollowSearchResultDispatchesByKind(t *testing.T) {
	m := newTestModel()
	m.search.results = []lsp.SymbolInformation{{Name: "MyClass", Kind: lsp.SymbolKindClass}}
	got, cmd := m.followSearchResult()
	gm := got.(Model)
	if cmd == nil {
		t.Error("expected a non-nil cmd for a matched class result")
	}
	if gm.focus != focusMap {
		t.Errorf("focus after following a search result = %v, want focusMap", gm.focus)
	}

	m2 := newTestModel()
	m2.search.results = []lsp.SymbolInformation{{Name: "DoIt", Kind: lsp.SymbolKindMethod}}
	_, cmd2 := m2.followSearchResult()
	if cmd2 == nil {
		t.Error("expected a non-nil cmd for a matched method result")
	}
}

// TestFollowSearchResultPushesHistory confirms selecting a result records
// the typed query for later recall via up/down arrow history.
func TestFollowSearchResultPushesHistory(t *testing.T) {
	m := newTestModel()
	m.search.input.SetValue("MyClass")
	m.search.results = []lsp.SymbolInformation{{Name: "MyClass", Kind: lsp.SymbolKindClass}}
	got, _ := m.followSearchResult()
	gm := got.(Model)
	if len(gm.search.history) != 1 || gm.search.history[0] != "MyClass" {
		t.Errorf("history after followSearchResult = %v, want [MyClass]", gm.search.history)
	}
}

// TestFollowSearchResultPushesSpool is the regression test for a bug found
// in code review: followSearchResult replaced the active bundle's thread
// without pushing it onto the spool first (unlike follow()), so `u` (back)
// couldn't return to wherever the user was before opening search.
func TestFollowSearchResultPushesSpool(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{name: "existing-thread", kind: "method"}
	m.search.results = []lsp.SymbolInformation{{Name: "MyClass", Kind: lsp.SymbolKindClass}}

	got, _ := m.followSearchResult()
	gm := got.(Model)
	back := gm.bundles[gm.activeBundle].back
	if len(back) != 1 || back[0].name != "existing-thread" {
		t.Errorf("back spool after followSearchResult = %+v, want [existing-thread]", back)
	}
}

// TestFollowSearchResultFromTangleDoesNotPushEmptySpool confirms the
// tangle's no-op entry state isn't spooled — there's nothing meaningful to
// go "back" to before the very first thread.
func TestFollowSearchResultFromTangleDoesNotPushEmptySpool(t *testing.T) {
	m := newTestModel() // bundles[0].thread defaults to tangleState()
	m.search.results = []lsp.SymbolInformation{{Name: "MyClass", Kind: lsp.SymbolKindClass}}

	got, _ := m.followSearchResult()
	gm := got.(Model)
	back := gm.bundles[gm.activeBundle].back
	if len(back) != 0 {
		t.Errorf("back spool after following from tangle = %+v, want empty", back)
	}
}
