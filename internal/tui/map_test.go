package tui

import (
	"strings"
	"testing"

	"github.com/fmbfs/skein/internal/compositor"
)

func TestFlatten(t *testing.T) {
	nodes := []Node{
		{Label: "a", Children: []Node{
			{Label: "a1"},
			{Label: "a2"},
		}},
		{Label: "b"},
	}
	flat := flatten(nodes)
	if len(flat) != 4 {
		t.Fatalf("expected 4 flattened nodes, got %d", len(flat))
	}
	if flat[0].node.Label != "a" || flat[0].depth != 0 {
		t.Errorf("flat[0] = %+v, want label=a depth=0", flat[0])
	}
	if flat[1].node.Label != "a1" || flat[1].depth != 1 {
		t.Errorf("flat[1] = %+v, want label=a1 depth=1", flat[1])
	}
	if flat[2].node.Label != "a2" || flat[2].depth != 1 || !flat[2].last[len(flat[2].last)-1] {
		t.Errorf("flat[2] = %+v, want label=a2 depth=1 last", flat[2])
	}
	if flat[3].node.Label != "b" || flat[3].depth != 0 {
		t.Errorf("flat[3] = %+v, want label=b depth=0", flat[3])
	}
}

func TestFlattenEmpty(t *testing.T) {
	if got := flatten(nil); len(got) != 0 {
		t.Errorf("flatten(nil) = %v, want empty", got)
	}
}

func TestViewport(t *testing.T) {
	tests := []struct {
		name               string
		total, cursor, ht  int
		wantStart, wantEnd int
	}{
		{"fits entirely", 5, 2, 10, 0, 5},
		{"zero height", 5, 2, 0, 0, 5},
		{"cursor near start", 10, 1, 4, 0, 4},
		{"cursor near end", 10, 9, 4, 6, 10},
		{"cursor in middle", 20, 10, 6, 7, 13},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := viewport(tt.total, tt.cursor, tt.ht)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("viewport(%d,%d,%d) = (%d,%d), want (%d,%d)",
					tt.total, tt.cursor, tt.ht, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestRenderMapEmpty(t *testing.T) {
	got := renderMap(nil, 0, 10)
	if !strings.Contains(got, "empty") {
		t.Errorf("renderMap(nil) = %q, want it to mention empty", got)
	}
}

func TestRenderMapHighlightsCursor(t *testing.T) {
	nodes := []Node{{Label: "one"}, {Label: "two"}}
	got := renderMap(nodes, 1, 10)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[1], "two") {
		t.Errorf("line 1 = %q, want it to contain 'two'", lines[1])
	}
}

func TestBuildMethodTree(t *testing.T) {
	rm := &compositor.RelationMap{
		ThreadName: "Foo",
		Kind:       "method",
		DefinedAt:  compositor.Location{Path: "a.cpp", Line: 10},
		Signature:  "void Foo()",
		CalledIn: []compositor.CalledInGroup{
			{File: "b.cpp", Lines: []int{5, 6}},
		},
		Calls:     []string{"Bar", "Baz"},
		Container: "MyClass",
	}
	nodes := buildMethodTree(rm)
	if len(nodes) != 4 {
		t.Fatalf("expected 4 top-level nodes (defined/calledin/calls/container), got %d: %+v", len(nodes), nodes)
	}

	defined := nodes[0]
	if defined.Label != "defined in" || len(defined.Children) != 2 {
		t.Errorf("defined node = %+v, want label 'defined in' with 2 children", defined)
	}
	if defined.Children[0].Follow != followFile || defined.Children[0].Target != "a.cpp" {
		t.Errorf("defined location child = %+v, want followFile target a.cpp", defined.Children[0])
	}

	calledIn := nodes[1]
	if calledIn.Direction != directionIncoming {
		t.Errorf("calledIn.Direction = %v, want directionIncoming", calledIn.Direction)
	}
	if len(calledIn.Children) != 1 || len(calledIn.Children[0].Children) != 2 {
		t.Errorf("calledIn children = %+v, want 1 file with 2 line children", calledIn.Children)
	}

	calls := nodes[2]
	if calls.Direction != directionOutgoing || len(calls.Children) != 2 {
		t.Errorf("calls node = %+v, want directionOutgoing with 2 children", calls)
	}
	if calls.Children[0].Follow != followMethod || calls.Children[0].Target != "Bar" {
		t.Errorf("calls.Children[0] = %+v, want followMethod target Bar", calls.Children[0])
	}

	container := nodes[3]
	if container.Follow != followClass || container.Target != "MyClass" {
		t.Errorf("container node = %+v, want followClass target MyClass", container)
	}
}

// TestBuildMethodTree_CalleeTargetStripsParensAndQualifier is the
// regression test for a real bug found in code review: rm.Calls entries
// always carry a "()" suffix (and sometimes a "Class::" qualifier prefix —
// see compositor.formatOutgoing/scanCallExpressions), but Node.Target is
// what gets passed straight to MethodCompositor.Build's exact-match lookup
// (findWorkspaceSymbol's `s.Name == name` filter). clangd's workspace/symbol
// never returns names with a "()" suffix or "::" qualifier, so following an
// outgoing-call node always failed with "no method or function named ...".
// The display Label must keep the decorated form; only Target needs to be
// bare and resolvable.
func TestBuildMethodTree_CalleeTargetStripsParensAndQualifier(t *testing.T) {
	rm := &compositor.RelationMap{
		ThreadName: "Foo",
		Calls:      []string{"acquire()", "Pipeline::processFrame()"},
	}
	nodes := buildMethodTree(rm)
	calls := nodes[0]
	if calls.Children[0].Label != "acquire()" || calls.Children[0].Target != "acquire" {
		t.Errorf("calls.Children[0] = %+v, want Label 'acquire()' Target 'acquire'", calls.Children[0])
	}
	if calls.Children[1].Label != "Pipeline::processFrame()" || calls.Children[1].Target != "processFrame" {
		t.Errorf("calls.Children[1] = %+v, want Label 'Pipeline::processFrame()' Target 'processFrame'", calls.Children[1])
	}
}

func TestBuildMethodTreeMinimal(t *testing.T) {
	rm := &compositor.RelationMap{ThreadName: "Foo"}
	nodes := buildMethodTree(rm)
	if len(nodes) != 0 {
		t.Errorf("expected no nodes for an empty RelationMap, got %+v", nodes)
	}
}

func TestBuildClassTree(t *testing.T) {
	cm := &compositor.ClassMap{
		ThreadName:  "Base",
		DefinedAt:   compositor.Location{Path: "base.h", Line: 1},
		Inherits:    []string{"Root"},
		InheritedBy: []string{"Derived"},
		Members: []compositor.Member{
			{Name: "DoIt", Kind: "method"},
		},
	}
	nodes := buildClassTree(cm)
	if len(nodes) != 4 {
		t.Fatalf("expected 4 top-level nodes, got %d: %+v", len(nodes), nodes)
	}
	if nodes[0].Follow != followFile {
		t.Errorf("defined-in node = %+v, want followFile", nodes[0])
	}
	if nodes[1].Direction != directionOutgoing || nodes[1].Children[0].Target != "Root" {
		t.Errorf("inherits node = %+v", nodes[1])
	}
	if nodes[2].Direction != directionIncoming || nodes[2].Children[0].Target != "Derived" {
		t.Errorf("inheritedBy node = %+v", nodes[2])
	}
	if len(nodes[3].Children) != 1 || nodes[3].Children[0].Follow != followMethod {
		t.Errorf("members node = %+v", nodes[3])
	}
}

func TestBuildFileTree(t *testing.T) {
	fm := &compositor.FileMap{
		ThreadName: "foo.cpp",
		Symbols: []compositor.Member{
			{Name: "MyClass", Kind: "class", Children: []compositor.Member{
				{Name: "Method1", Kind: "method"},
			}},
			{Name: "FreeFn", Kind: "function"},
		},
	}
	nodes := buildFileTree(fm)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 top-level nodes, got %d", len(nodes))
	}
	class := nodes[0]
	if class.Follow != followClass || class.Target != "MyClass" {
		t.Errorf("class node = %+v, want followClass target MyClass", class)
	}
	if len(class.Children) != 1 || class.Children[0].Follow != followMethod || class.Children[0].ClassCtx != "MyClass" {
		t.Errorf("class.Children[0] = %+v, want followMethod with ClassCtx MyClass", class.Children[0])
	}
	fn := nodes[1]
	if fn.Follow != followMethod || fn.ClassCtx != "" {
		t.Errorf("free function node = %+v, want followMethod with empty ClassCtx", fn)
	}
}

func TestRenderLineSelectedDirectionColouredNodeDoesNotBleedColour(t *testing.T) {
	// Regression test: a selected outgoing/incoming-tagged node used to
	// render its label through incomingStyle/outgoingStyle *and then*
	// wrap that already-ANSI-coloured string in selectedLineStyle. Because
	// lipgloss just concatenates raw escape codes, the label's own
	// embedded colour code came later in the string than
	// selectedLineStyle's foreground code and silently overrode it —
	// so a selected coloured row lost the guaranteed-contrast colour the
	// pink-highlight fix relied on. Selected rows must render the label
	// exclusively through selectedLineStyle, with no direction colouring
	// mixed in.
	fn := flatNode{node: &Node{Label: "callee", Direction: directionOutgoing}}
	got := renderLine(fn, true)
	want := selectedLineStyle.Render("callee")
	if got != want {
		t.Errorf("renderLine(selected, outgoing) = %q, want %q (selectedLineStyle only, no outgoingStyle colour mixed in)", got, want)
	}
}

func TestRenderLineDimsNonFollowableNeutralNodes(t *testing.T) {
	followable := flatNode{node: &Node{Label: "processFrame [method]", Follow: followMethod}}
	notFollowable := flatNode{node: &Node{Label: "counter_ [field]", Follow: followNone}}

	gotFollowable := renderLine(followable, false)
	gotNotFollowable := renderLine(notFollowable, false)

	if gotFollowable != "processFrame [method]" {
		t.Errorf("renderLine(followable) = %q, want the plain label undimmed", gotFollowable)
	}
	wantDimmed := mutedStyle.Render("counter_ [field]")
	if gotNotFollowable != wantDimmed {
		t.Errorf("renderLine(non-followable neutral) = %q, want %q (dimmed via mutedStyle so it's visually distinct from a followable row)", gotNotFollowable, wantDimmed)
	}
}

func TestRenderMapDeepTreeAllBranches(t *testing.T) {
	// Exercise every renderLine branch: multi-level depth, both "last" and
	// "not last" siblings, and both incoming/outgoing colour-coding.
	nodes := []Node{
		{Label: "root-a", Direction: directionIncoming, Children: []Node{
			{Label: "child-a1", Direction: directionOutgoing},
			{Label: "child-a2", Children: []Node{
				{Label: "grandchild"},
			}},
		}},
		{Label: "root-b"},
	}
	got := renderMap(nodes, 0, 20)
	for _, want := range []string{"root-a", "child-a1", "child-a2", "grandchild", "root-b", "├──", "└──"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderMap output missing %q:\n%s", want, got)
		}
	}
}
