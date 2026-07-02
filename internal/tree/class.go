package tree

import (
	"fmt"
	"io"

	"github.com/fmbfs/skein/internal/compositor"
)

// PrintClass renders a ClassMap as a tree(1)-style Unicode tree. By default
// the output is monochrome; pass WithColour(true) to colourize "inherited by"
// (incoming, cyan) and "inherits" (outgoing, green) entries.
func PrintClass(w io.Writer, cm *compositor.ClassMap, opts ...Option) {
	ro := resolveOpts(opts)
	fmt.Fprintf(w, "%s  [%s]\n", cm.ThreadName, cm.Kind)

	type section func(isLast bool)
	var sections []section

	if cm.DefinedAt.Path != "" {
		sections = append(sections, func(isLast bool) { printClassDefinedIn(w, cm, isLast) })
	}
	if len(cm.Inherits) > 0 {
		sections = append(sections, func(isLast bool) { printNameList(w, "inherits", cm.Inherits, ro.outgoing, isLast) })
	}
	if len(cm.InheritedBy) > 0 {
		sections = append(sections, func(isLast bool) { printNameList(w, "inherited by", cm.InheritedBy, ro.incoming, isLast) })
	}
	if len(cm.Members) > 0 {
		sections = append(sections, func(isLast bool) { printMembersSection(w, "members", cm.Members, isLast) })
	}

	for i, s := range sections {
		s(i == len(sections)-1)
	}
}

func printClassDefinedIn(w io.Writer, cm *compositor.ClassMap, isLast bool) {
	connector, cont := branch(isLast)
	fmt.Fprintf(w, "%sdefined in\n", connector)
	fmt.Fprintf(w, "%s└── %s :%d\n", cont, cm.DefinedAt.Path, cm.DefinedAt.Line)
}

func printNameList(w io.Writer, label string, names []string, colorFn func(string) string, isLast bool) {
	connector, cont := branch(isLast)
	fmt.Fprintf(w, "%s%s\n", connector, colorFn(label))
	for i, name := range names {
		c, _ := branch(i == len(names)-1)
		fmt.Fprintf(w, "%s%s\n", cont, colorFn(fmt.Sprintf("%s%s", c, name)))
	}
}

func printMembersSection(w io.Writer, label string, members []compositor.Member, isLast bool) {
	connector, cont := branch(isLast)
	fmt.Fprintf(w, "%s%s\n", connector, label)
	printMemberList(w, cont, members)
}

// printMemberList prints members at one indent level, recursing into any
// children (used by file mode, where classes nest their methods/fields).
func printMemberList(w io.Writer, prefix string, members []compositor.Member) {
	for i, m := range members {
		connector, cont := branch(i == len(members)-1)
		fmt.Fprintf(w, "%s%s%s\n", prefix, connector, memberLabel(m))
		if len(m.Children) > 0 {
			printMemberList(w, prefix+cont, m.Children)
		}
	}
}

func memberLabel(m compositor.Member) string {
	name := m.Name
	if m.Kind == "method" || m.Kind == "function" {
		name += "()"
	}
	return fmt.Sprintf("%s [%s]", name, m.Kind)
}
