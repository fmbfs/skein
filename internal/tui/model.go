package tui

import (
	"fmt"
	"path/filepath"
	"strings"

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
// any) passed on the command line.
func (m Model) Init() tea.Cmd {
	if m.pendingInitial == "" {
		return nil
	}
	cmds := []tea.Cmd{resolveGenericCmd(m.client, m.rootDir, m.pendingInitial, minPly)}
	if m.pendingPinned != "" {
		cmds = append(cmds, resolvePinnedCmd(m.client, m.rootDir, m.pendingPinned))
	}
	return tea.Batch(cmds...)
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
// search bar.
type searchResultsMsg struct {
	results []lsp.SymbolInformation
	err     error
}

// --- commands -------------------------------------------------------------

func resolveGenericCmd(client *lsp.Client, rootDir, name string, ply int) tea.Cmd {
	return func() tea.Msg { return resolveGeneric(client, rootDir, name, ply, false) }
}

func resolvePinnedCmd(client *lsp.Client, rootDir, name string) tea.Cmd {
	return func() tea.Msg { return resolveGeneric(client, rootDir, name, minPly, true) }
}

func resolveGeneric(client *lsp.Client, rootDir, name string, ply int, pinning bool) threadLoadedMsg {
	matches, err := client.WorkspaceSymbol(name)
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
	return threadLoadedMsg{state: threadStateFromRelationMap(rm, classFilter, ply)}
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
	return threadLoadedMsg{state: threadStateFromClassMap(cm)}
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
		return threadLoadedMsg{state: threadStateFromFileMap(fm)}
	}
}

func searchCmd(client *lsp.Client, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := client.WorkspaceSymbol(query)
		return searchResultsMsg{results: results, err: err}
	}
}

func threadStateFromRelationMap(rm *compositor.RelationMap, classFilter string, ply int) threadState {
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
	}
}

func threadStateFromClassMap(cm *compositor.ClassMap) threadState {
	return threadState{
		name:      cm.ThreadName,
		kind:      cm.Kind,
		nodes:     buildClassTree(cm),
		definedAt: locationString(cm.DefinedAt),
	}
}

func threadStateFromFileMap(fm *compositor.FileMap) threadState {
	return threadState{
		name:  fm.ThreadName,
		kind:  fm.Kind,
		nodes: buildFileTree(fm),
	}
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
		m.search.err = msg.err
		m.search.results = msg.results
		m.search.cursor = 0
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
	case key.Matches(msg, keys.Back):
		return m.spoolBack(), nil
	case key.Matches(msg, keys.Forward):
		return m.spoolForward(), nil
	case key.Matches(msg, keys.Reset):
		return m.spoolReset(), nil
	case key.Matches(msg, keys.Pin):
		return m.pinCurrent(), nil
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
	}
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.search.reset()
		m.focus = focusMap
		return m, nil
	case key.Matches(msg, keys.ResultUp):
		m.search.moveCursor(-1)
		return m, nil
	case key.Matches(msg, keys.ResultDown):
		m.search.moveCursor(1)
		return m, nil
	case key.Matches(msg, keys.SelectResult):
		return m.followSearchResult()
	}

	var cmd tea.Cmd
	m.search.input, cmd = m.search.input.Update(msg)
	query := strings.TrimSpace(m.search.input.Value())
	if query == "" {
		m.search.results = nil
		return m, cmd
	}
	return m, tea.Batch(cmd, searchCmd(m.client, query))
}

func (m Model) followSearchResult() (tea.Model, tea.Cmd) {
	sel := m.search.selected()
	if sel == nil {
		return m, nil
	}
	m.search.reset()
	m.focus = focusMap
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
// (docs/SPEC.md `p` key) without disturbing the tab it was pinned from.
func (m Model) pinCurrent() Model {
	t := *m.activeThread()
	m.bundles, m.activeBundle = pinBundle(m.bundles, t.name, t)
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
