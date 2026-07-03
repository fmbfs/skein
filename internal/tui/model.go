package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fmbfs/skein/internal/compositor"
	"github.com/fmbfs/skein/internal/lsp"
)

// focusArea is which of the three panels (docs/SPEC.md section 4) currently
// receives key input; <tab> cycles map -> detail -> search -> map.
type focusArea int

const (
	focusMap focusArea = iota
	focusDetail
	focusSearch
)

const maxPly = 3
const minPly = 1

// defaultTUIStrandLimit is the strand-limit truncation budget used by the
// TUI (docs/SPEC.md section 6's default of 50). The TUI has no --strands
// flag of its own (that's a draw-mode-only concept), so it always applies
// this default.
const defaultTUIStrandLimit = 50

// threadState is the composed, render-ready snapshot of one thread
// (docs/SPEC.md vocabulary): the map panel's Node tree, plus everything
// the detail panel needs. Snapshotting means switching bundles or
// navigating the spool doesn't re-query clangd.
type threadState struct {
	name        string
	kind        string // "method", "class", "file", "tangle"
	classFilter string
	ply         int
	nodes       []Node
	cursor      int
	signature   string
	definedAt   string
	container   string
	ambiguous   []string
	warning     string // non-empty when strand-limit truncation dropped content
}

// tangleState is the empty, no-thread entry state (docs/SPEC.md's
// "tangle" vocabulary entry): a fresh bundle before any thread is chosen.
func tangleState() threadState {
	return threadState{name: "tangle", kind: "tangle"}
}

// Model is the bubbletea root model for the skein TUI (docs/SPEC.md
// section 4). It owns the LSP client for the whole session (opened once in
// main.go, closed on quit) and dispatches all composed queries through
// tea.Cmd so clangd's blocking round-trips never stall the render loop —
// see internal/lsp.Client's doc comment for that design decision.
type Model struct {
	client  *lsp.Client
	rootDir string

	width, height int

	bundles      []bundle
	activeBundle int

	showIn  bool
	showOut bool
	focus   focusArea
	search  searchState
	help    bool
	status  string
	err     error

	pendingInitial string
	pendingPinned  string

	quitting bool
}

// New builds the initial Model. initialSymbol, if non-empty, is resolved
// as the first thread (skein <symbol>); pinned, if non-empty, is a second
// symbol pre-pinned into its own bundle tab (skein <symbol> <pinned>) —
// docs/SPEC.md section 3.2.
func New(client *lsp.Client, rootDir, initialSymbol, pinned string) Model {
	return Model{
		client:         client,
		rootDir:        rootDir,
		showIn:         true,
		showOut:        true,
		search:         newSearchState(),
		bundles:        []bundle{{name: "tangle", thread: tangleState(), origin: tangleState()}},
		pendingInitial: initialSymbol,
		pendingPinned:  pinned,
	}
}

// Init kicks off resolution of the initial symbol (and pinned symbol, if
// any) passed on the command line. When no initial symbol was given (a bare
// `skein` launch straight into interactive search), it instead nudges
// clangd's indexer directly — resolveGeneric would otherwise be the only
// thing that ever does this, so without it every search from a cold start
// returns nothing indefinitely (see compositor.NudgeIndexer's doc comment).
func (m Model) Init() tea.Cmd {
	if m.pendingInitial == "" {
		return nudgeIndexerCmd(m.client, m.rootDir)
	}
	cmds := []tea.Cmd{resolveGenericCmd(m.client, m.rootDir, m.pendingInitial, minPly)}
	if m.pendingPinned != "" {
		cmds = append(cmds, resolvePinnedCmd(m.client, m.rootDir, m.pendingPinned))
	}
	return tea.Batch(cmds...)
}

// indexerNudgedMsg is a no-op result message for nudgeIndexerCmd — nothing
// in the UI needs to react to it (it's best-effort background work), but a
// concrete message type keeps the tea.Cmd contract explicit and testable
// rather than silently returning nil.
type indexerNudgedMsg struct{}

func nudgeIndexerCmd(client *lsp.Client, rootDir string) tea.Cmd {
	return func() tea.Msg {
		_ = compositor.NudgeIndexer(client, rootDir) // best-effort
		return indexerNudgedMsg{}
	}
}

// --- messages -----------------------------------------------------------

// threadLoadedMsg carries the result of resolving/composing a thread —
// whether from the initial CLI symbol, a follow, or a search selection.
// pinning, when true, means "create a new bundle for this result" rather
// than replacing the active bundle's thread (used for the `skein foo bar`
// pre-pinned second thread).
type threadLoadedMsg struct {
	state   threadState
	err     error
	pinning bool
	label   string // display name for a newly-pinned bundle
}

// searchResultsMsg carries a workspace/symbol query's results back to the
// search bar. gen ties the result back to the keystroke that triggered it
// (via searchDebounceMsg) so a slow, superseded query can't clobber the
// input state with stale results after the user has kept typing.
type searchResultsMsg struct {
	results []lsp.SymbolInformation
	err     error
	gen     int
}

// searchDebounceMsg fires searchDebounceDelay after a search-box keystroke.
// If gen no longer matches the search state's current generation (i.e. the
// user typed again before the delay elapsed), it's a stale trigger and is
// dropped instead of issuing a query — this is what actually collapses a
// fast typist's keystrokes down to a single workspace/symbol round trip
// instead of firing (and fully serialising, per *lsp.Client's one-call-at-
// a-time contract) one query per keystroke, which is what made rapid
// typing feel like it was "overwhelmed" and lagging behind ("search vs
// load time is too long").
type searchDebounceMsg struct {
	gen   int
	query string
}

// searchDebounceDelay is how long search waits after the last keystroke
// before actually querying clangd. Short enough to still feel instant for
// a pause mid-word, long enough to collapse a normal typing burst into one
// round trip instead of one per character.
const searchDebounceDelay = 120 * time.Millisecond

// gotoResolvedMsg carries the result of resolving a symbol-target Node's
// location via workspace/symbol, for the g ("goto") key on nodes that had
// no GotoPath baked in at build time (calls, members, inherits/
// inheritedBy — see Node's doc comment in map.go).
type gotoResolvedMsg struct {
	path string
	line int
	err  error
}

// gotoDoneMsg surfaces any error from the editor subprocess itself
// (nonzero exit, binary not found, etc.) once it exits.
type gotoDoneMsg struct {
	err error
}

// --- commands -------------------------------------------------------------

func resolveGenericCmd(client *lsp.Client, rootDir, name string, ply int) tea.Cmd {
	return func() tea.Msg { return resolveGeneric(client, rootDir, name, ply, false) }
}

func resolvePinnedCmd(client *lsp.Client, rootDir, name string) tea.Cmd {
	return func() tea.Msg { return resolveGeneric(client, rootDir, name, minPly, true) }
}

func resolveGeneric(client *lsp.Client, rootDir, name string, ply int, pinning bool) threadLoadedMsg {
	matches, err := compositor.ResolveSymbol(client, rootDir, name)
	if err != nil {
		return threadLoadedMsg{err: fmt.Errorf("workspace/symbol %q: %w", name, err)}
	}
	if len(matches) == 0 {
		return threadLoadedMsg{err: fmt.Errorf("no symbol found matching %q", name)}
	}
	best := matches[0]
	var msg threadLoadedMsg
	if followKindForSymbol(best.Kind) == followClass {
		msg = loadClass(client, rootDir, best.Name)
	} else {
		msg = loadMethod(client, rootDir, best.Name, "", ply)
	}
	msg.pinning = pinning
	msg.label = best.Name
	return msg
}

// gotoResolveCmd looks up target's definition via workspace/symbol so the
// g key can jump to a symbol-target Node that had no location baked in at
// build time (calls, members, inherits/inheritedBy). Reuses the same
// *lsp.Client the rest of the session already benefits from, including
// internal/compositor/shared.go's index-warm fast path, so this is a
// single fast round trip after the workspace's first lookup.
func gotoResolveCmd(client *lsp.Client, target string) tea.Cmd {
	return func() tea.Msg {
		matches, err := client.WorkspaceSymbol(target)
		if err != nil {
			return gotoResolvedMsg{err: fmt.Errorf("workspace/symbol %q: %w", target, err)}
		}
		if len(matches) == 0 {
			return gotoResolvedMsg{err: fmt.Errorf("no symbol found matching %q", target)}
		}
		best := matches[0]
		path, err := lsp.URIToPath(best.Location.URI)
		if err != nil {
			return gotoResolvedMsg{err: fmt.Errorf("resolve %q location: %w", target, err)}
		}
		return gotoResolvedMsg{path: path, line: best.Location.Range.Start.Line + 1}
	}
}

// gotoExecCallback turns the editor subprocess's exit error (if any) into
// a gotoDoneMsg for Update to surface as a status-bar error once it exits.
func gotoExecCallback(err error) tea.Msg {
	return gotoDoneMsg{err: err}
}

func loadMethodCmd(client *lsp.Client, rootDir, name, classFilter string, ply int) tea.Cmd {
	return func() tea.Msg { return loadMethod(client, rootDir, name, classFilter, ply) }
}

func loadMethod(client *lsp.Client, rootDir, name, classFilter string, ply int) threadLoadedMsg {
	if ply < minPly {
		ply = minPly
	}
	if ply > maxPly {
		ply = maxPly
	}
	mc := compositor.NewMethodCompositor(client, rootDir)
	rm, err := mc.Build(name, classFilter, ply)
	if err != nil {
		return threadLoadedMsg{err: err}
	}
	warning := combineWarnings(
		rm.TruncateCalledIn(defaultTUIStrandLimit),
		rm.TruncateCalls(defaultTUIStrandLimit),
	)
	return threadLoadedMsg{state: threadStateFromRelationMap(rm, classFilter, ply, warning)}
}

func loadClassCmd(client *lsp.Client, rootDir, name string) tea.Cmd {
	return func() tea.Msg { return loadClass(client, rootDir, name) }
}

func loadClass(client *lsp.Client, rootDir, name string) threadLoadedMsg {
	cc := compositor.NewClassCompositor(client, rootDir)
	cm, err := cc.Build(name)
	if err != nil {
		return threadLoadedMsg{err: err}
	}
	warning := string(cm.TruncateMembers(defaultTUIStrandLimit))
	return threadLoadedMsg{state: threadStateFromClassMap(cm, warning)}
}

func loadFileCmd(client *lsp.Client, rootDir, relOrAbsPath string) tea.Cmd {
	return func() tea.Msg {
		path := relOrAbsPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(rootDir, path)
		}
		fc := compositor.NewFileCompositor(client, rootDir)
		fm, err := fc.Build(path)
		if err != nil {
			return threadLoadedMsg{err: err}
		}
		warning := string(fm.TruncateSymbols(defaultTUIStrandLimit))
		return threadLoadedMsg{state: threadStateFromFileMap(fm, warning)}
	}
}

func searchCmd(client *lsp.Client, query string, gen int) tea.Cmd {
	return func() tea.Msg {
		results, err := client.WorkspaceSymbol(query)
		return searchResultsMsg{results: results, err: err, gen: gen}
	}
}

// searchDebounceCmd schedules a searchDebounceMsg after searchDebounceDelay,
// tagged with gen (the search generation at keystroke time) so a later
// keystroke's own debounce can supersede it. See searchDebounceMsg's doc
// comment.
func searchDebounceCmd(gen int, query string) tea.Cmd {
	return tea.Tick(searchDebounceDelay, func(time.Time) tea.Msg {
		return searchDebounceMsg{gen: gen, query: query}
	})
}

func threadStateFromRelationMap(rm *compositor.RelationMap, classFilter string, ply int, warning string) threadState {
	return threadState{
		name:        rm.ThreadName,
		kind:        rm.Kind,
		classFilter: classFilter,
		ply:         ply,
		nodes:       buildMethodTree(rm),
		signature:   rm.Signature,
		definedAt:   locationString(rm.DefinedAt),
		container:   rm.Container,
		ambiguous:   rm.Ambiguous,
		warning:     warning,
	}
}

func threadStateFromClassMap(cm *compositor.ClassMap, warning string) threadState {
	return threadState{
		name:      cm.ThreadName,
		kind:      cm.Kind,
		nodes:     buildClassTree(cm),
		definedAt: locationString(cm.DefinedAt),
		warning:   warning,
	}
}

func threadStateFromFileMap(fm *compositor.FileMap, warning string) threadState {
	return threadState{
		name:    fm.ThreadName,
		kind:    fm.Kind,
		nodes:   buildFileTree(fm),
		warning: warning,
	}
}

// combineWarnings joins any non-empty truncation warnings with a space,
// since a single load can truncate more than one section (e.g. both
// CalledIn and Calls) and both should be visible to the user.
func combineWarnings(warnings ...compositor.TruncationWarning) string {
	var parts []string
	for _, w := range warnings {
		if w != "" {
			parts = append(parts, string(w))
		}
	}
	return strings.Join(parts, " ")
}

func locationString(loc compositor.Location) string {
	if loc.Path == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", loc.Path, loc.Line)
}

// --- Update -----------------------------------------------------------

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case threadLoadedMsg:
		return m.applyThreadLoaded(msg)

	case searchResultsMsg:
		if msg.gen != m.search.generation {
			// Superseded by a later keystroke — drop it so a slow query
			// can't overwrite what the user has typed since.
			return m, nil
		}
		m.search.err = msg.err
		m.search.results = msg.results
		m.search.cursor = 0
		return m, nil

	case searchDebounceMsg:
		if msg.gen != m.search.generation {
			return m, nil // a newer keystroke already superseded this trigger
		}
		return m, searchCmd(m.client, msg.query, msg.gen)

	case gotoResolvedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		return m, tea.ExecProcess(editorCommand(msg.path, msg.line), gotoExecCallback)

	case gotoDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) applyThreadLoaded(msg threadLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.status = msg.err.Error()
		return m, nil
	}
	m.err = nil

	if msg.pinning {
		name := msg.label
		if name == "" {
			name = msg.state.name
		}
		m.bundles, m.activeBundle = pinBundle(m.bundles, name, msg.state)
		return m, nil
	}

	b := &m.bundles[m.activeBundle]
	b.thread = msg.state
	if len(b.back) == 0 {
		b.origin = msg.state
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.focus == focusSearch {
		return m.handleSearchKey(msg)
	}

	// Any keypress dismisses a stale error banner from an earlier, unrelated
	// action (e.g. an ambiguous follow) — without this, switching tabs via
	// [ / ] or a digit jump kept showing that old error even though the new
	// tab's content loaded and switched correctly, making tab-switching look
	// broken when it wasn't.
	m.err = nil

	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Help):
		m.help = !m.help
		return m, nil
	case key.Matches(msg, keys.Search):
		m.focus = focusSearch
		m.search.input.Focus()
		return m, nil
	case key.Matches(msg, keys.Escape):
		m.help = false
		return m, nil
	case key.Matches(msg, keys.Tab):
		m.focus = (m.focus + 1) % 3
		return m, nil
	case key.Matches(msg, keys.Up):
		t := m.activeThread()
		visible := filterByDirection(t.nodes, m.showIn, m.showOut)
		t.cursor = clampCursor(t.cursor-1, visible)
		return m, nil
	case key.Matches(msg, keys.Down):
		t := m.activeThread()
		visible := filterByDirection(t.nodes, m.showIn, m.showOut)
		t.cursor = clampCursor(t.cursor+1, visible)
		return m, nil
	case key.Matches(msg, keys.Follow):
		return m.follow()
	case key.Matches(msg, keys.Goto):
		return m.gotoSelected()
	case key.Matches(msg, keys.Back):
		return m.spoolBack(), nil
	case key.Matches(msg, keys.Forward):
		return m.spoolForward(), nil
	case key.Matches(msg, keys.Reset):
		return m.spoolReset(), nil
	case key.Matches(msg, keys.Pin):
		return m.pinCurrent(), nil
	case key.Matches(msg, keys.Unpin):
		return m.unpinCurrent(), nil
	case key.Matches(msg, keys.PrevBundle):
		m.activeBundle = cycleBundle(m.activeBundle, len(m.bundles), -1)
		return m, nil
	case key.Matches(msg, keys.NextBundle):
		m.activeBundle = cycleBundle(m.activeBundle, len(m.bundles), 1)
		return m, nil
	case key.Matches(msg, keys.CloseBundle):
		m.bundles, m.activeBundle = closeBundle(m.bundles, m.activeBundle, m.activeBundle)
		return m, nil
	case key.Matches(msg, keys.PlyUp):
		return m.adjustPly(1)
	case key.Matches(msg, keys.PlyDown):
		return m.adjustPly(-1)
	case key.Matches(msg, keys.ToggleIn):
		m.showIn = !m.showIn
		return m, nil
	case key.Matches(msg, keys.ToggleOut):
		m.showOut = !m.showOut
		return m, nil
	case isBundleDigitKey(msg):
		m.activeBundle = jumpToBundle(m.activeBundle, len(m.bundles), int(msg.String()[0]-'0'))
		return m, nil
	}
	return m, nil
}

// isBundleDigitKey reports whether msg is a single "1".."9" keypress, used
// for docs/SPEC.md section 7's bundle-jump shortcut. Bounds are re-checked
// by jumpToBundle itself; this just filters to plausible digit runes so the
// switch above doesn't have to enumerate nine separate key.Binding cases.
func isBundleDigitKey(msg tea.KeyMsg) bool {
	s := msg.String()
	return len(s) == 1 && s[0] >= '1' && s[0] <= '9'
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.search.pushHistory(m.search.input.Value())
		m.search.reset()
		m.focus = focusMap
		return m, nil
	case key.Matches(msg, keys.ResultUp):
		// With no results to navigate (nothing typed yet, or the last
		// query came back empty), up/down instead recall past searches —
		// shell-history style — so a recent query can be re-run without
		// retyping it (docs/SPEC.md's "last 5 searches" request).
		if len(m.search.results) == 0 {
			m.search.historyPrev()
			return m, nil
		}
		m.search.moveCursor(-1)
		return m, nil
	case key.Matches(msg, keys.ResultDown):
		if len(m.search.results) == 0 {
			m.search.historyNext()
			return m, nil
		}
		m.search.moveCursor(1)
		return m, nil
	case key.Matches(msg, keys.SelectResult):
		return m.followSearchResult()
	}

	var cmd tea.Cmd
	m.search.input, cmd = m.search.input.Update(msg)
	query := strings.TrimSpace(m.search.input.Value())
	m.search.generation++
	if query == "" {
		m.search.results = nil
		return m, cmd
	}
	return m, tea.Batch(cmd, searchDebounceCmd(m.search.generation, query))
}

func (m Model) followSearchResult() (tea.Model, tea.Cmd) {
	sel := m.search.selected()
	if sel == nil {
		return m, nil
	}
	m.search.pushHistory(m.search.input.Value())
	m.search.reset()
	m.focus = focusMap

	// Push the current thread onto the spool before replacing it, mirroring
	// follow()'s behaviour — otherwise `u` (back) can't return to wherever
	// the user was before opening search. Skip for the tangle's empty entry
	// state: there's nothing meaningful to spool back to.
	b := &m.bundles[m.activeBundle]
	if b.thread.kind != "tangle" {
		b.back = append(b.back, b.thread)
		b.fwd = nil
	}

	name := sel.Name
	if followKindForSymbol(sel.Kind) == followClass {
		return m, loadClassCmd(m.client, m.rootDir, name)
	}
	return m, loadMethodCmd(m.client, m.rootDir, name, sel.ContainerName, minPly)
}

// follow dispatches the selected map node's <enter> action, per its
// followKind (docs/SPEC.md's "follow" vocabulary entry). Addresses the
// cursor against the same direction-filtered node list the map panel is
// currently rendering, so <enter> always follows what's visibly selected.
func (m Model) follow() (tea.Model, tea.Cmd) {
	t := m.activeThread()
	visible := filterByDirection(t.nodes, m.showIn, m.showOut)
	flat := flatten(visible)
	if t.cursor < 0 || t.cursor >= len(flat) {
		return m, nil
	}
	n := flat[t.cursor].node
	if n.Follow == followNone {
		return m, nil
	}

	b := &m.bundles[m.activeBundle]
	b.back = append(b.back, b.thread)
	b.fwd = nil

	switch n.Follow {
	case followMethod:
		return m, loadMethodCmd(m.client, m.rootDir, n.Target, n.ClassCtx, minPly)
	case followClass:
		return m, loadClassCmd(m.client, m.rootDir, n.Target)
	case followFile:
		return m, loadFileCmd(m.client, m.rootDir, n.Target)
	case followNone:
		// unreachable — guarded above
	}
	return m, nil
}

// gotoSelected opens the currently-selected Node's location in the user's
// default editor (docs/SPEC.md `g` key). Mirrors follow()'s cursor
// addressing: get the active thread, filter/flatten it by the current
// direction toggles, and address the selected Node by cursor. Nodes that
// already carry a known GotoPath (definitions, call sites) jump straight
// to tea.ExecProcess; symbol-target nodes with no baked-in location
// (calls, members, inherits/inheritedBy) go through gotoResolveCmd first.
func (m Model) gotoSelected() (tea.Model, tea.Cmd) {
	t := m.activeThread()
	visible := filterByDirection(t.nodes, m.showIn, m.showOut)
	flat := flatten(visible)
	if t.cursor < 0 || t.cursor >= len(flat) {
		return m, nil
	}
	n := flat[t.cursor].node
	if n.GotoPath != "" {
		return m, tea.ExecProcess(editorCommand(n.GotoPath, n.GotoLine), gotoExecCallback)
	}
	if n.Follow == followNone {
		return m, nil
	}
	return m, gotoResolveCmd(m.client, n.Target)
}

func (m Model) spoolBack() Model {
	b := &m.bundles[m.activeBundle]
	if len(b.back) == 0 {
		return m
	}
	last := len(b.back) - 1
	prev := b.back[last]
	b.back = b.back[:last]
	b.fwd = append(b.fwd, b.thread)
	b.thread = prev
	return m
}

func (m Model) spoolForward() Model {
	b := &m.bundles[m.activeBundle]
	if len(b.fwd) == 0 {
		return m
	}
	last := len(b.fwd) - 1
	next := b.fwd[last]
	b.fwd = b.fwd[:last]
	b.back = append(b.back, b.thread)
	b.thread = next
	return m
}

func (m Model) spoolReset() Model {
	b := &m.bundles[m.activeBundle]
	b.back = nil
	b.fwd = nil
	b.thread = b.origin
	return m
}

// pinCurrent snapshots the active thread into a brand-new bundle tab
// (docs/SPEC.md `p` key), without disturbing the tab it was pinned from.
// p always creates a new tab now — the earlier toggle-to-unpin behaviour
// was replaced by a dedicated u (unpin) key so repeated p presses (e.g.
// to compare the same symbol at different ply/filter settings) reliably
// stack tabs instead of sometimes closing the one you just made.
func (m Model) pinCurrent() Model {
	t := *m.activeThread()
	m.bundles, m.activeBundle = pinBundle(m.bundles, t.name, t)
	return m
}

// unpinCurrent closes the active bundle (docs/SPEC.md `u` key) if and only
// if it's a pinned tab (bundle.pinned) — a no-op on the original "tangle"
// entry bundle, which has nothing to unpin.
func (m Model) unpinCurrent() Model {
	if !m.bundles[m.activeBundle].pinned {
		return m
	}
	m.bundles, m.activeBundle = closeBundle(m.bundles, m.activeBundle, m.activeBundle)
	return m
}

// adjustPly changes the active method thread's traversal depth and
// re-queries clangd at the new depth (ply only applies to method threads —
// class/file compositors don't take a ply argument).
func (m Model) adjustPly(delta int) (tea.Model, tea.Cmd) {
	t := m.activeThread()
	if t.kind != "method" {
		return m, nil
	}
	newPly := t.ply + delta
	if newPly < minPly || newPly > maxPly {
		return m, nil
	}
	return m, loadMethodCmd(m.client, m.rootDir, t.name, t.classFilter, newPly)
}

// activeThread returns a pointer to the active bundle's current thread,
// for in-place cursor/field mutation within Update handlers.
func (m *Model) activeThread() *threadState {
	return &m.bundles[m.activeBundle].thread
}

func clampCursor(c int, nodes []Node) int {
	total := len(flatten(nodes))
	if total == 0 {
		return 0
	}
	if c < 0 {
		return 0
	}
	if c >= total {
		return total - 1
	}
	return c
}
