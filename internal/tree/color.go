package tree

import (
	"os"

	"github.com/mattn/go-isatty"
)

// ANSI SGR codes for the two relationship directions, matching the TUI's
// incoming/outgoing colour scheme (see internal/tui/style.go): cyan for
// things that call/reference *this* symbol, green for things *this* symbol
// calls/references. Kept as plain ANSI escapes (not lipgloss) so this
// package has no TUI dependency.
const (
	ansiReset    = "\x1b[0m"
	ansiIncoming = "\x1b[36m" // cyan: called in / inherited by
	ansiOutgoing = "\x1b[32m" // green: calls / inherits
)

// renderOpts holds the rendering configuration built up by Option values.
// Zero value renders monochrome output, so existing callers that pass no
// options are unaffected.
type renderOpts struct {
	colour bool
}

// Option configures optional rendering behaviour for Print/PrintClass.
type Option func(*renderOpts)

// WithColour enables or disables ANSI colour in the rendered tree. Callers
// are responsible for deciding whether colour is appropriate (e.g. checking
// that stdout is a TTY and that --no-color/NO_COLOR were not requested) —
// this package makes no such decisions itself.
func WithColour(enabled bool) Option {
	return func(o *renderOpts) { o.colour = enabled }
}

func resolveOpts(opts []Option) renderOpts {
	var ro renderOpts
	for _, opt := range opts {
		opt(&ro)
	}
	return ro
}

// colourize wraps s in the given ANSI code when colour is enabled, otherwise
// returns s unchanged.
func (ro renderOpts) colourize(code, s string) string {
	if !ro.colour {
		return s
	}
	return code + s + ansiReset
}

func (ro renderOpts) incoming(s string) string { return ro.colourize(ansiIncoming, s) }
func (ro renderOpts) outgoing(s string) string { return ro.colourize(ansiOutgoing, s) }

// AutoColour decides whether draw mode should emit ANSI colour, following
// docs/SPEC.md section 7's rule that "colour [is] auto-disabled when stdout
// is not a TTY", plus two explicit overrides: the --no-color CLI flag
// (noColour) and the https://no-color.org NO_COLOR environment variable
// convention, either of which forces colour off regardless of TTY status.
func AutoColour(out *os.File, noColour bool) bool {
	if noColour {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isatty.IsTerminal(out.Fd()) || isatty.IsCygwinTerminal(out.Fd())
}
