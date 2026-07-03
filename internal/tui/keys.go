package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap centralises every TUI keybinding in one place (docs/SPEC.md
// section 7), so the help overlay and the Update switch stay in sync by
// construction instead of by discipline.
type keyMap struct {
	Quit         key.Binding
	Search       key.Binding
	Escape       key.Binding
	Help         key.Binding
	Tab          key.Binding
	Up           key.Binding
	Down         key.Binding
	Follow       key.Binding
	SelectResult key.Binding
	ResultUp     key.Binding
	ResultDown   key.Binding
	Back         key.Binding
	Forward      key.Binding
	Reset        key.Binding
	Pin          key.Binding
	Unpin        key.Binding
	Goto         key.Binding
	PrevBundle   key.Binding
	NextBundle   key.Binding
	CloseBundle  key.Binding
	PlyUp        key.Binding
	PlyDown      key.Binding
	ToggleIn     key.Binding
	ToggleOut    key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "clear/back"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "cycle focus"),
	),
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "down"),
	),
	Follow: key.NewBinding(
		key.WithKeys("enter", "l"),
		key.WithHelp("enter/l", "follow"),
	),
	// SelectResult confirms the highlighted search hit — enter only, so
	// typing "l" in the search box still inserts the letter instead of
	// following (Follow's "l" alias only applies to the map panel).
	SelectResult: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	// ResultUp/ResultDown move the search-results cursor using arrow keys
	// only — "j"/"k" stay reserved for typing into the search box.
	ResultUp: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "prev result"),
	),
	ResultDown: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next result"),
	),
	Back: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "back"),
	),
	Forward: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "forward"),
	),
	Reset: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reset"),
	),
	Pin: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "pin"),
	),
	Unpin: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "unpin"),
	),
	Goto: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "goto"),
	),
	PrevBundle: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "prev bundle"),
	),
	NextBundle: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "next bundle"),
	),
	CloseBundle: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "close bundle"),
	),
	PlyUp: key.NewBinding(
		key.WithKeys("+", "="),
		key.WithHelp("+/-", "ply"),
	),
	PlyDown: key.NewBinding(
		key.WithKeys("-"),
		key.WithHelp("-", "ply"),
	),
	ToggleIn: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "toggle incoming"),
	),
	ToggleOut: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "toggle outgoing"),
	),
}

// hints returns the footer key-hint line shown at the bottom of the screen,
// tailored to the current focus (search bar shows fewer/different hints
// than the map panel). hasResults switches the ↑/↓ hint's description
// between "select result" and "recall search" depending on whether the
// search box currently has anything to navigate. bundleCount adds the
// [/] tab-switch hint (and its digit-jump note) only when there's more
// than one bundle open to switch between.
func hints(focus focusArea, hasResults bool, bundleCount int) string {
	if focus == focusSearch {
		upDownHint := "select result"
		if !hasResults {
			upDownHint = "recall search"
		}
		return renderHints([][2]string{
			{"↑/↓", upDownHint},
			{"enter", "open"},
			{"esc", "cancel"},
		})
	}
	pairs := [][2]string{
		{"enter/l", "follow"},
		{"g", "goto"},
		{"p", "pin"},
		{"u", "unpin"},
		{"x", "close"},
		{"h", "back"},
	}
	if bundleCount > 1 {
		pairs = append(pairs, [2]string{"[/]/1-9", "switch tab"})
	}
	pairs = append(pairs,
		[2]string{"/", "search"},
		[2]string{"?", "help"},
		[2]string{"q", "quit"},
	)
	return renderHints(pairs)
}

func renderHints(pairs [][2]string) string {
	out := ""
	for i, p := range pairs {
		if i > 0 {
			out += "  "
		}
		out += keyHintKeyStyle.Render(p[0]) + " " + keyHintStyle.Render(p[1])
	}
	return out
}
