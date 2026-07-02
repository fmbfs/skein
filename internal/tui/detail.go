package tui

import (
	"fmt"
	"strings"
)

// detailFor renders the right panel's content: an expansion of the current
// thread's composed data plus context for the node currently selected in
// the map panel.
//
// Note: docs/SPEC.md section 11 lists textDocument/hover as a detail-panel
// source. skein's compositor layer intentionally reduces LSP locations to
// file+line (no column) to keep composed types simple and testable, so a
// live hover call (which needs a precise position) isn't wired up yet —
// deferred alongside the other explicitly-marked v2/v3 items in SPEC.md.
// The detail panel instead surfaces everything already composed: the
// thread's own data, plus the selected map node's own label/target.
func detailFor(t *threadState, visible []Node) string {
	if t == nil {
		return mutedStyle.Render("no thread selected — press / to search")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s  [%s]\n\n", t.name, t.kind)

	if t.signature != "" {
		fmt.Fprintf(&b, "%s\n\n", t.signature)
	}
	if t.definedAt != "" {
		fmt.Fprintf(&b, "defined at:\n  %s\n\n", t.definedAt)
	}
	if t.container != "" {
		fmt.Fprintf(&b, "container:\n  %s\n\n", t.container)
	}
	if len(t.ambiguous) > 0 {
		fmt.Fprintf(&b, "also found in:\n  %s\n\n", strings.Join(t.ambiguous, ", "))
	}

	flat := flatten(visible)
	if t.cursor >= 0 && t.cursor < len(flat) {
		n := flat[t.cursor].node
		fmt.Fprintf(&b, "selected:\n  %s\n", n.Label)
		if n.Follow != followNone {
			b.WriteString(followHint(n.Follow) + "\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func followHint(k followKind) string {
	switch k {
	case followMethod:
		return mutedStyle.Render("[follow with <enter> — opens as method thread]")
	case followClass:
		return mutedStyle.Render("[follow with <enter> — opens as class thread]")
	case followFile:
		return mutedStyle.Render("[follow with <enter> — opens as file thread]")
	default:
		return ""
	}
}
