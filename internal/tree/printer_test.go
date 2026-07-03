package tree

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fmbfs/skein/internal/compositor"
)

func TestPrint_HeaderOnly(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{ThreadName: "foo", Kind: "function", Ply: 1}
	Print(&buf, rm)

	got := buf.String()
	want := "foo  [function]  ply:1\n"
	if got != want {
		t.Errorf("Print() = %q, want %q", got, want)
	}
}

func TestPrint_DefinedInWithSignature(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{
		ThreadName: "foo",
		Kind:       "function",
		Ply:        1,
		DefinedAt:  compositor.Location{Path: "a.cpp", Line: 10},
		Signature:  "void foo()",
	}
	Print(&buf, rm)

	got := buf.String()
	if !strings.Contains(got, "defined in") {
		t.Errorf("Print() missing 'defined in' section: %q", got)
	}
	if !strings.Contains(got, "a.cpp :10") {
		t.Errorf("Print() missing location line: %q", got)
	}
	if !strings.Contains(got, "void foo()") {
		t.Errorf("Print() missing signature: %q", got)
	}
	// defined in is the only section -> must use the "last" connector.
	if !strings.Contains(got, "└── defined in") {
		t.Errorf("Print() expected last-connector for sole section: %q", got)
	}
}

func TestPrint_CalledInAndCalls(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{
		ThreadName: "foo",
		Kind:       "method",
		Ply:        2,
		CalledIn: []compositor.CalledInGroup{
			{File: "a.cpp", Lines: []int{1, 2}},
			{File: "b.cpp", Lines: []int{5}},
		},
		Calls: []string{"bar", "baz"},
	}
	Print(&buf, rm)
	got := buf.String()

	if !strings.Contains(got, "called in  (3)") {
		t.Errorf("Print() expected total call count of 3, got: %q", got)
	}
	if !strings.Contains(got, "a.cpp") || !strings.Contains(got, "b.cpp") {
		t.Errorf("Print() missing caller files: %q", got)
	}
	// b.cpp has a single call-site line: it must fold onto the file's own
	// row ("b.cpp :5") instead of leaving a lone ":5" dangling on its own
	// nested line with nothing else in that branch.
	if !strings.Contains(got, "b.cpp :5") {
		t.Errorf("Print() expected single call-site line folded onto its file row, got: %q", got)
	}
	if !strings.Contains(got, "calls") {
		t.Errorf("Print() missing 'calls' section: %q", got)
	}
	if !strings.Contains(got, "bar") || !strings.Contains(got, "baz") {
		t.Errorf("Print() missing callee names: %q", got)
	}
	// calls is the last section printed -> last connector.
	if !strings.Contains(got, "└── calls") {
		t.Errorf("Print() expected last-connector for final section: %q", got)
	}
	// called in is not last -> non-last connector.
	if !strings.Contains(got, "├── called in") {
		t.Errorf("Print() expected non-last connector for called-in section: %q", got)
	}
}

// TestPrint_SingleCallSiteFoldsLineOntoFile is the regression test for a
// reported bug: a file group with exactly one call-site line rendered as
// a dangling ":21" nested under its own file line, with nothing else in
// that branch. It must instead fold onto the file's own row.
func TestPrint_SingleCallSiteFoldsLineOntoFile(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{
		ThreadName: "foo",
		Kind:       "method",
		CalledIn: []compositor.CalledInGroup{
			{File: "main.cpp", Lines: []int{21}},
		},
	}
	Print(&buf, rm)
	got := buf.String()

	if !strings.Contains(got, "main.cpp :21") {
		t.Errorf("Print() = %q, want the single call site folded onto its file row (\"main.cpp :21\")", got)
	}
	if strings.Contains(got, "main.cpp\n") {
		t.Errorf("Print() = %q, want no separate file-only line when there's a single call site", got)
	}
}

// TestPrint_MultipleCallSitesKeepFileGrouping confirms the fold above
// doesn't regress the existing multi-line behaviour: a file group with
// more than one call-site line keeps the file-header + per-line-children
// structure.
func TestPrint_MultipleCallSitesKeepFileGrouping(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{
		ThreadName: "foo",
		Kind:       "method",
		CalledIn: []compositor.CalledInGroup{
			{File: "main.cpp", Lines: []int{5, 9}},
		},
	}
	Print(&buf, rm)
	got := buf.String()

	if !strings.Contains(got, "main.cpp\n") {
		t.Errorf("Print() = %q, want a standalone file line for a multi-line group", got)
	}
	if !strings.Contains(got, ":5") || !strings.Contains(got, ":9") {
		t.Errorf("Print() = %q, want both call-site lines rendered as children", got)
	}
	if strings.Contains(got, "main.cpp :5") || strings.Contains(got, "main.cpp :9") {
		t.Errorf("Print() = %q, want lines NOT folded onto the file row when there's more than one", got)
	}
}

func TestPrint_AllSectionsOrder(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{
		ThreadName: "foo",
		Kind:       "function",
		DefinedAt:  compositor.Location{Path: "a.cpp", Line: 1},
		CalledIn:   []compositor.CalledInGroup{{File: "b.cpp", Lines: []int{2}}},
		Calls:      []string{"bar"},
	}
	Print(&buf, rm)
	got := buf.String()

	definedIdx := strings.Index(got, "defined in")
	calledIdx := strings.Index(got, "called in")
	callsIdx := strings.Index(got, "calls\n")
	if definedIdx >= calledIdx || calledIdx >= callsIdx {
		t.Errorf("expected section order defined-in < called-in < calls, got:\n%s", got)
	}
}

func TestBranch(t *testing.T) {
	connector, cont := branch(true)
	if connector != "└── " || cont != "    " {
		t.Errorf("branch(true) = (%q, %q), want (\"└── \", \"    \")", connector, cont)
	}
	connector, cont = branch(false)
	if connector != "├── " || cont != "│   " {
		t.Errorf("branch(false) = (%q, %q), want (\"├── \", \"│   \")", connector, cont)
	}
}
