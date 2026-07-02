package tui

import "github.com/charmbracelet/lipgloss"

// Colour palette — lazygit-inspired: understated background, saturated
// accents reserved for direction-coding and focus state (see docs/SPEC.md
// section 5 for the incoming/outgoing/bidirectional colour meanings).
const (
	colourIncoming    = lipgloss.Color("14") // cyan
	colourOutgoing    = lipgloss.Color("10") // green
	colourMuted       = lipgloss.Color("242")
	colourAccent      = lipgloss.Color("205")
	colourBorder      = lipgloss.Color("240")
	colourBorderFocus = lipgloss.Color("205")
	colourError       = lipgloss.Color("9")
	colourWarning     = lipgloss.Color("11") // yellow
)

// Note: docs/SPEC.md section 5 also defines a "bidirectional" (yellow)
// colour for a symbol that appears in both a thread's calls and calledIn —
// i.e. mutual recursion. RelationMap.CalledIn only carries file+line (no
// callee symbol name, see compositor.Location), so there's no data to match
// a Calls entry against a caller symbol today — deferred alongside the
// other explicitly-scoped-out items (see detail.go's hover note).

var (
	statusBarStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colourAccent).
			Padding(0, 1)

	bundleTabStyle = lipgloss.NewStyle().
			Foreground(colourMuted).
			Padding(0, 1)

	bundleTabActiveStyle = bundleTabStyle.
				Foreground(lipgloss.Color("0")).
				Background(colourAccent).
				Bold(true)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourBorder).
			Padding(0, 1)

	panelFocusStyle = panelStyle.
			BorderForeground(colourBorderFocus)

	selectedLineStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("0")).
				Background(colourAccent)

	incomingStyle = lipgloss.NewStyle().Foreground(colourIncoming)
	outgoingStyle = lipgloss.NewStyle().Foreground(colourOutgoing)
	mutedStyle    = lipgloss.NewStyle().Foreground(colourMuted)
	errorStyle    = lipgloss.NewStyle().Foreground(colourError).Bold(true)
	warningStyle  = lipgloss.NewStyle().Foreground(colourWarning)

	searchBarStyle = lipgloss.NewStyle().
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(colourBorder)

	keyHintStyle = lipgloss.NewStyle().
			Foreground(colourMuted).
			Padding(0, 1)

	keyHintKeyStyle = lipgloss.NewStyle().
			Foreground(colourAccent).
			Bold(true)
)
