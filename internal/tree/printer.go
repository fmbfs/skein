// Package tree renders RelationMaps for the fast "draw" mode: a tree(1)-style
// Unicode tree to stdout, plus a JSON serialiser for scripting.
package tree

import (
	"fmt"
	"io"

	"github.com/fmbfs/skein/internal/compositor"
)

// Print renders a RelationMap as a tree(1)-style Unicode tree, matching the
// format documented in README.md and docs/SPEC.md. By default the output is
// monochrome; pass WithColour(true) to colourize "called in" (incoming, cyan)
// and "calls" (outgoing, green) entries. Callers decide when colour is
// appropriate (e.g. TTY detection, --no-color, NO_COLOR).
func Print(w io.Writer, rm *compositor.RelationMap, opts ...Option) {
	ro := resolveOpts(opts)
	fmt.Fprintf(w, "%s  [%s]  ply:%d\n", rm.ThreadName, rm.Kind, rm.Ply)

	type section func(isLast bool)
	var sections []section

	if rm.DefinedAt.Path != "" {
		sections = append(sections, func(isLast bool) { printDefinedIn(w, rm, isLast) })
	}
	if len(rm.CalledIn) > 0 {
		sections = append(sections, func(isLast bool) { printCalledIn(w, rm, ro, isLast) })
	}
	if len(rm.Calls) > 0 {
		sections = append(sections, func(isLast bool) { printCalls(w, rm, ro, isLast) })
	}

	for i, s := range sections {
		s(i == len(sections)-1)
	}
}

func branch(isLast bool) (connector, continuation string) {
	if isLast {
		return "└── ", "    "
	}
	return "├── ", "│   "
}

func printDefinedIn(w io.Writer, rm *compositor.RelationMap, isLast bool) {
	connector, cont := branch(isLast)
	fmt.Fprintf(w, "%sdefined in\n", connector)
	fmt.Fprintf(w, "%s└── %s :%d\n", cont, rm.DefinedAt.Path, rm.DefinedAt.Line)
	if rm.Signature != "" {
		fmt.Fprintf(w, "%s      %s\n", cont, rm.Signature)
	}
}

func printCalledIn(w io.Writer, rm *compositor.RelationMap, ro renderOpts, isLast bool) {
	connector, cont := branch(isLast)
	fmt.Fprintf(w, "%s%s\n", connector, ro.incoming(fmt.Sprintf("called in  (%d)", rm.CalledInTotal())))

	for fi, group := range rm.CalledIn {
		fileConnector, fileCont := branch(fi == len(rm.CalledIn)-1)
		fmt.Fprintf(w, "%s%s%s\n", cont, fileConnector, ro.incoming(group.File))
		for li, line := range group.Lines {
			lineConnector, _ := branch(li == len(group.Lines)-1)
			fmt.Fprintf(w, "%s%s%s\n", cont, fileCont, ro.incoming(fmt.Sprintf("%s:%d", lineConnector, line)))
		}
	}
}

func printCalls(w io.Writer, rm *compositor.RelationMap, ro renderOpts, isLast bool) {
	connector, cont := branch(isLast)
	fmt.Fprintf(w, "%s%s\n", connector, ro.outgoing("calls"))
	for i, callee := range rm.Calls {
		calleeConnector, _ := branch(i == len(rm.Calls)-1)
		fmt.Fprintf(w, "%s%s\n", cont, ro.outgoing(fmt.Sprintf("%s%s", calleeConnector, callee)))
	}
}
