// Package tui implements the interactive exploration UI (the default mode),
// built on bubbletea + lipgloss. Visual design is inspired by lazygit.
//
// See docs/SPEC.md sections 4-7 for layout and keybindings.
package tui

import (
	"fmt"
	"strings"

	"github.com/fmbfs/skein/internal/compositor"
)

// followKind identifies which compositor to dispatch to when the user
// follows a Node — see docs/SPEC.md section 3 (thread kinds).
type followKind int

const (
	followNone followKind = iota
	followMethod
	followClass
	followFile
)

// direction labels a Node for direction-filter colouring and for the i/o
// toggle shortcuts (docs/SPEC.md section 5). directionNeutral covers
// section headers and members that aren't inherently a call edge.
type direction int

const (
	directionNeutral direction = iota
	directionIncoming
	directionOutgoing
)

// Node is the unified tree element the map panel renders, built by
// adapting whichever compositor result (RelationMap/ClassMap/FileMap) is
// current into one shape. Only Follow != followNone nodes react to <enter>.
type Node struct {
	Label     string
	Direction direction
	Follow    followKind
	Target    string // symbol/class/file name or path to follow into
	ClassCtx  string // class filter, to disambiguate a method follow
	Children  []Node
}

// flatNode is a Node flattened for cursor-addressed rendering/selection —
// bubbletea models need a flat, indexable list, not a recursive tree.
type flatNode struct {
	node  *Node
	depth int
	last  []bool // per-depth-level "is this branch the last sibling" flags
}

// flatten walks nodes depth-first into a flat, renderable slice.
func flatten(nodes []Node) []flatNode {
	var out []flatNode
	var walk func(ns []Node, depth int, prefix []bool)
	walk = func(ns []Node, depth int, prefix []bool) {
		for i := range ns {
			isLast := i == len(ns)-1
			line := append(append([]bool{}, prefix...), isLast)
			out = append(out, flatNode{node: &ns[i], depth: depth, last: line})
			if len(ns[i].Children) > 0 {
				walk(ns[i].Children, depth+1, line)
			}
		}
	}
	walk(nodes, 0, nil)
	return out
}

// renderMap renders the flattened node list as a tree(1)-style Unicode
// tree, highlighting the node at cursor and colour-coding by direction.
// kind is the owning thread's kind (docs/SPEC.md vocabulary); when it's
// "tangle" (the fresh, nothing-loaded-yet entry state) and there are no
// nodes, renderMap shows an onboarding hint instead of a bare "(empty)" —
// a first-time bare `skein` launch otherwise stares back at a blank pane
// with zero indication of what to do next (a real UX gap surfaced by
// direct user testing).
func renderMap(nodes []Node, cursor, height int, kind string) string {
	flat := flatten(nodes)
	if len(flat) == 0 {
		if kind == "tangle" {
			return mutedStyle.Render("press / to search for a symbol\n(class, method, or function)")
		}
		return mutedStyle.Render("(empty)")
	}

	var b strings.Builder
	start, end := viewport(len(flat), cursor, height)
	for i := start; i < end; i++ {
		fn := flat[i]
		b.WriteString(renderLine(fn, i == cursor))
		if i != end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// viewport computes the [start,end) window of a height-limited scrolling
// list that keeps cursor visible, clamped to the list bounds.
func viewport(total, cursor, height int) (int, int) {
	if height <= 0 || total <= height {
		return 0, total
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > total {
		end = total
		start = end - height
	}
	return start, end
}

func renderLine(fn flatNode, selected bool) string {
	var prefix strings.Builder
	for i := 0; i < fn.depth; i++ {
		switch {
		case i == fn.depth-1 && fn.last[i]:
			prefix.WriteString("└── ")
		case i == fn.depth-1:
			prefix.WriteString("├── ")
		case fn.last[i]:
			prefix.WriteString("    ")
		default:
			prefix.WriteString("│   ")
		}
	}

	// When selected, skip direction colouring entirely and render the raw
	// prefix+label text through selectedLineStyle alone. Nesting an
	// already-ANSI-coloured label (incomingStyle/outgoingStyle) inside
	// selectedLineStyle's Render call previously let the label's own
	// embedded foreground escape code override selectedLineStyle's
	// contrast-guaranteed foreground later in the same string — the
	// highlighted row silently lost its readable colour on any
	// incoming/outgoing-tagged node (found while investigating a reported
	// "hard to read pink highlight" bug: it also affected coloured rows,
	// not just neutral ones).
	if selected {
		return selectedLineStyle.Render(prefix.String() + fn.node.Label)
	}

	label := fn.node.Label
	switch fn.node.Direction {
	case directionIncoming:
		label = incomingStyle.Render(label)
	case directionOutgoing:
		label = outgoingStyle.Render(label)
	case directionNeutral:
		// A neutral (uncoloured) row is ambiguous on its own: section
		// headers like "members (3)" and non-followable leaves like a
		// class field render identically to followable leaves like a
		// method member — nothing on screen hinted which rows <enter>
		// would actually act on (reported directly: "the follow
		// mechanic" felt broken because nothing indicated what was
		// followable). Dim the ones <enter> can't do anything with, so
		// the followable rows stand out by contrast.
		if fn.node.Follow == followNone {
			label = mutedStyle.Render(label)
		}
	}

	return prefix.String() + label
}

// buildMethodTree adapts a composed RelationMap into the unified Node tree
// for a method/function thread (docs/SPEC.md section 4 example layout).
func buildMethodTree(rm *compositor.RelationMap) []Node {
	var nodes []Node

	if rm.DefinedAt.Path != "" {
		defined := Node{Label: "defined in"}
		loc := Node{
			Label:  fmt.Sprintf("%s :%d", rm.DefinedAt.Path, rm.DefinedAt.Line),
			Follow: followFile,
			Target: rm.DefinedAt.Path,
		}
		defined.Children = append(defined.Children, loc)
		if rm.Signature != "" {
			defined.Children = append(defined.Children, Node{Label: rm.Signature})
		}
		nodes = append(nodes, defined)
	}

	if len(rm.CalledIn) > 0 {
		calledIn := Node{
			Label:     fmt.Sprintf("called in (%d)", rm.CalledInTotal()),
			Direction: directionIncoming,
		}
		for _, group := range rm.CalledIn {
			if len(group.Lines) == 1 {
				// A single call site: fold the line number onto the file's
				// own line ("SafeDTC.cpp :21") instead of a separate child
				// row underneath it. A lone ":21" dangling below its file
				// with nothing else in that branch read as a rendering bug
				// (reported directly from a screenshot) rather than the
				// call site it actually was.
				calledIn.Children = append(calledIn.Children, Node{
					Label:     fmt.Sprintf("%s :%d", group.File, group.Lines[0]),
					Direction: directionIncoming,
					// Following a call site opens that file — skein doesn't
					// yet resolve the enclosing caller symbol at a bare
					// location (see docs/SPEC.md section 8, path-finding).
					Follow: followFile,
					Target: group.File,
				})
				continue
			}
			fileNode := Node{Label: group.File, Direction: directionIncoming}
			for _, line := range group.Lines {
				fileNode.Children = append(fileNode.Children, Node{
					Label:     fmt.Sprintf(":%d", line),
					Direction: directionIncoming,
					Follow:    followFile,
					Target:    group.File,
				})
			}
			calledIn.Children = append(calledIn.Children, fileNode)
		}
		nodes = append(nodes, calledIn)
	}

	if len(rm.Calls) > 0 {
		calls := Node{
			Label:     fmt.Sprintf("calls (%d)", len(rm.Calls)),
			Direction: directionOutgoing,
		}
		for _, callee := range rm.Calls {
			calls.Children = append(calls.Children, Node{
				Label:     callee,
				Direction: directionOutgoing,
				Follow:    followMethod,
				Target:    calleeTarget(callee),
			})
		}
		nodes = append(nodes, calls)
	}

	if rm.Container != "" {
		nodes = append(nodes, Node{
			Label:  "member of " + rm.Container,
			Follow: followClass,
			Target: rm.Container,
		})
	}

	return nodes
}

// calleeTarget derives a workspace/symbol-resolvable name from one of
// RelationMap.Calls's decorated display strings (e.g. "acquire()" or
// "Pipeline::processFrame()" — see compositor's formatOutgoing/
// scanCallExpressions). clangd's workspace/symbol never returns names with
// a "()" suffix or "Class::" qualifier, so following such a Node without
// stripping them always fails to resolve (regression: see
// TestBuildMethodTree_CalleeTargetStripsParensAndQualifier). The Label
// keeps the decorated form for display; only Target needs to be bare.
func calleeTarget(callee string) string {
	name := strings.TrimSuffix(callee, "()")
	if i := strings.LastIndex(name, "::"); i != -1 {
		name = name[i+2:]
	}
	return name
}

// buildClassTree adapts a composed ClassMap into the unified Node tree.
func buildClassTree(cm *compositor.ClassMap) []Node {
	var nodes []Node

	if cm.DefinedAt.Path != "" {
		nodes = append(nodes, Node{
			Label:  fmt.Sprintf("defined in %s :%d", cm.DefinedAt.Path, cm.DefinedAt.Line),
			Follow: followFile,
			Target: cm.DefinedAt.Path,
		})
	}

	if len(cm.Inherits) > 0 {
		inherits := Node{Label: "inherits", Direction: directionOutgoing}
		for _, base := range cm.Inherits {
			inherits.Children = append(inherits.Children, Node{
				Label: base, Direction: directionOutgoing,
				Follow: followClass, Target: base,
			})
		}
		nodes = append(nodes, inherits)
	}

	if len(cm.InheritedBy) > 0 {
		inheritedBy := Node{Label: "inherited by", Direction: directionIncoming}
		for _, derived := range cm.InheritedBy {
			inheritedBy.Children = append(inheritedBy.Children, Node{
				Label: derived, Direction: directionIncoming,
				Follow: followClass, Target: derived,
			})
		}
		nodes = append(nodes, inheritedBy)
	}

	if len(cm.Members) > 0 {
		members := Node{Label: fmt.Sprintf("members (%d)", len(cm.Members))}
		members.Children = buildMemberNodes(cm.Members, cm.ThreadName)
		nodes = append(nodes, members)
	}

	return nodes
}

// buildFileTree adapts a composed FileMap into the unified Node tree.
func buildFileTree(fm *compositor.FileMap) []Node {
	return buildMemberNodes(fm.Symbols, "")
}

// buildMemberNodes converts compositor.Member entries (methods/fields,
// possibly nested under classes) into followable Nodes. classCtx scopes a
// method follow to the immediately-enclosing class, if any, matching
// draw mode's -c disambiguation.
func buildMemberNodes(members []compositor.Member, classCtx string) []Node {
	nodes := make([]Node, 0, len(members))
	for _, m := range members {
		n := Node{Label: fmt.Sprintf("%s [%s]", m.Name, m.Kind)}
		switch m.Kind {
		case "method", "function", "constructor":
			n.Follow = followMethod
			n.Target = m.Name
			n.ClassCtx = classCtx
		case "class", "struct":
			n.Follow = followClass
			n.Target = m.Name
		}
		if len(m.Children) > 0 {
			childCtx := classCtx
			if m.Kind == "class" || m.Kind == "struct" {
				childCtx = m.Name
			}
			n.Children = buildMemberNodes(m.Children, childCtx)
		}
		nodes = append(nodes, n)
	}
	return nodes
}
