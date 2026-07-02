package tui

import (
	"strings"
	"testing"
)

func TestDetailForNilThread(t *testing.T) {
	got := detailFor(nil, nil)
	if !strings.Contains(got, "no thread selected") {
		t.Errorf("detailFor(nil) = %q, want it to mention no thread selected", got)
	}
}

func TestDetailForRendersThreadFields(t *testing.T) {
	th := &threadState{
		name:      "Foo",
		kind:      "method",
		signature: "void Foo()",
		definedAt: "a.cpp:10",
		container: "MyClass",
		ambiguous: []string{"OtherClass"},
		cursor:    0,
	}
	nodes := []Node{{Label: "child node", Follow: followMethod}}
	got := detailFor(th, nodes)

	for _, want := range []string{"Foo", "method", "void Foo()", "a.cpp:10", "MyClass", "OtherClass", "child node"} {
		if !strings.Contains(got, want) {
			t.Errorf("detailFor output missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "follow with <enter>") {
		t.Errorf("detailFor output missing follow hint:\n%s", got)
	}
}

func TestDetailForCursorOutOfRange(t *testing.T) {
	th := &threadState{name: "Foo", kind: "method", cursor: 99}
	got := detailFor(th, []Node{{Label: "only"}})
	if strings.Contains(got, "selected:") {
		t.Errorf("detailFor with out-of-range cursor should not render a selected section:\n%s", got)
	}
}

// TestDetailForNonFollowableNodeSaysSo is the regression test for a
// reported "follow mechanic" confusion: pressing <enter> on a
// non-followable node (e.g. a class field, or a section header) was a
// silent no-op with zero on-screen indication that nothing would happen.
// The detail panel must say so explicitly instead of just omitting the
// follow hint.
func TestDetailForNonFollowableNodeSaysSo(t *testing.T) {
	th := &threadState{name: "Foo", kind: "class", cursor: 0}
	got := detailFor(th, []Node{{Label: "counter_ [field]", Follow: followNone}})
	if !strings.Contains(got, "not followable") {
		t.Errorf("detailFor for a non-followable node = %q, want it to explicitly say so", got)
	}
}

func TestFollowHint(t *testing.T) {
	if got := followHint(followNone); got != "" {
		t.Errorf("followHint(followNone) = %q, want empty", got)
	}
	for _, k := range []followKind{followMethod, followClass, followFile} {
		if got := followHint(k); got == "" {
			t.Errorf("followHint(%v) = empty, want a hint", k)
		}
	}
}
