package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model — docs/SPEC.md section 4's layout: status bar,
// bundle tabs, map+detail split, search bar, key hints footer.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "loading…"
	}

	t := m.activeThread()

	var b strings.Builder
	b.WriteString(m.renderStatusBar(t))
	b.WriteByte('\n')
	b.WriteString(renderBundleTabs(m.bundles, m.activeBundle))
	b.WriteByte('\n')

	showSearch := m.focus == focusSearch || len(m.search.results) > 0
	reserved := 0
	if showSearch {
		reserved = searchBarLineCount(&m.search)
	}

	if m.help {
		b.WriteString(m.renderHelp())
	} else {
		b.WriteString(m.renderPanels(t, reserved))
	}
	b.WriteByte('\n')

	if showSearch {
		b.WriteString(renderSearchBar(&m.search, m.width))
		b.WriteByte('\n')
	}

	b.WriteString(hints(m.focus, len(m.search.results) > 0))
	return b.String()
}

func (m Model) renderStatusBar(t *threadState) string {
	strands := len(flatten(t.nodes))
	spool := len(m.bundles[m.activeBundle].back) + 1
	status := fmt.Sprintf("skein  ·  %s  [%s]  ply:%d  strands:%d  spool:%d",
		t.name, t.kind, t.ply, strands, spool)
	if t.warning != "" {
		status = fmt.Sprintf("%s  ·  %s", status, warningStyle.Render(t.warning))
	}
	if m.err != nil {
		status = fmt.Sprintf("skein  ·  %s", errorStyle.Render(m.err.Error()))
	}
	return statusBarStyle.Width(m.width).Render(status)
}

func (m Model) renderPanels(t *threadState, reserved int) string {
	panelWidth := (m.width - 4) / 2
	panelHeight := m.height - 6 - reserved
	if panelHeight < 3 {
		panelHeight = 3
	}

	mapStyle := panelStyle
	detailStyle := panelStyle
	switch m.focus {
	case focusMap:
		mapStyle = panelFocusStyle
	case focusDetail:
		detailStyle = panelFocusStyle
	case focusSearch:
		// neither panel is focused while the search bar has focus
	}

	visible := filterByDirection(t.nodes, m.showIn, m.showOut)
	left := mapStyle.Width(panelWidth).Height(panelHeight).Render(renderMap(visible, t.cursor, panelHeight-2, t.kind))
	right := detailStyle.Width(panelWidth).Height(panelHeight).Render(detailFor(t, visible))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderHelp() string {
	rows := [][2]string{
		{"q / ctrl+c", "quit"},
		{"/", "focus search bar"},
		{"esc", "clear search / close help"},
		{"?", "toggle this help"},
		{"tab", "cycle focus: map -> detail -> search"},
		{"j/↓, k/↑", "move selection"},
		{"enter, l", "follow selected node"},
		{"u, h", "back (spool)"},
		{"ctrl+r", "forward (spool)"},
		{"r", "reset to first thread"},
		{"p", "pin current thread to a new bundle tab (press again on a pinned tab to unpin)"},
		{"[ / ]", "previous / next bundle tab"},
		{"x", "close/unpin current bundle tab"},
		{"1-9", "jump to bundle tab by number"},
		{"+/-", "increase/decrease ply (1-3)"},
		{"i", "toggle incoming edges"},
		{"o", "toggle outgoing edges"},
	}
	var b strings.Builder
	b.WriteString(statusBarStyle.Render("Keybindings"))
	b.WriteByte('\n')
	for _, r := range rows {
		fmt.Fprintf(&b, "  %-14s %s\n", keyHintKeyStyle.Render(r[0]), r[1])
	}
	return strings.TrimRight(b.String(), "\n")
}

// filterByDirection drops incoming/outgoing-tagged nodes (and their
// subtrees) per the current filter toggles — docs/SPEC.md section 5:
// shortcuts act as filters (show/hide), not mode switches.
func filterByDirection(nodes []Node, showIn, showOut bool) []Node {
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if n.Direction == directionIncoming && !showIn {
			continue
		}
		if n.Direction == directionOutgoing && !showOut {
			continue
		}
		filtered := n
		if len(n.Children) > 0 {
			filtered.Children = filterByDirection(n.Children, showIn, showOut)
		}
		out = append(out, filtered)
	}
	return out
}
