package tree

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fmbfs/skein/internal/compositor"
)

func TestPrintClass_HeaderOnly(t *testing.T) {
	var buf bytes.Buffer
	cm := &compositor.ClassMap{ThreadName: "Foo", Kind: "class"}
	PrintClass(&buf, cm)

	want := "Foo  [class]\n"
	if buf.String() != want {
		t.Errorf("PrintClass() = %q, want %q", buf.String(), want)
	}
}

func TestPrintClass_AllSections(t *testing.T) {
	var buf bytes.Buffer
	cm := &compositor.ClassMap{
		ThreadName:  "Foo",
		Kind:        "class",
		DefinedAt:   compositor.Location{Path: "foo.h", Line: 3},
		Inherits:    []string{"Base"},
		InheritedBy: []string{"Derived1", "Derived2"},
		Members: []compositor.Member{
			{Name: "bar", Kind: "method"},
			{Name: "field", Kind: "field"},
		},
	}
	PrintClass(&buf, cm)
	got := buf.String()

	for _, want := range []string{
		"defined in", "foo.h :3",
		"inherits", "Base",
		"inherited by", "Derived1", "Derived2",
		"members", "bar()", "[method]", "field", "[field]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PrintClass() missing %q in output:\n%s", want, got)
		}
	}
	// members is the last section -> last connector.
	if !strings.Contains(got, "└── members") {
		t.Errorf("PrintClass() expected members to use last connector:\n%s", got)
	}
}

func TestPrintClass_NestedMembers(t *testing.T) {
	var buf bytes.Buffer
	cm := &compositor.ClassMap{
		ThreadName: "Foo",
		Kind:       "class",
		Members: []compositor.Member{
			{
				Name: "Inner",
				Kind: "class",
				Children: []compositor.Member{
					{Name: "innerMethod", Kind: "method"},
				},
			},
		},
	}
	PrintClass(&buf, cm)
	got := buf.String()
	if !strings.Contains(got, "Inner") {
		t.Errorf("expected outer member 'Inner' in output:\n%s", got)
	}
	if !strings.Contains(got, "innerMethod()") {
		t.Errorf("expected nested child 'innerMethod()' in output:\n%s", got)
	}
}

func TestMemberLabel(t *testing.T) {
	cases := []struct {
		m    compositor.Member
		want string
	}{
		{compositor.Member{Name: "foo", Kind: "method"}, "foo() [method]"},
		{compositor.Member{Name: "foo", Kind: "function"}, "foo() [function]"},
		{compositor.Member{Name: "x", Kind: "field"}, "x [field]"},
		{compositor.Member{Name: "Inner", Kind: "class"}, "Inner [class]"},
	}
	for _, c := range cases {
		if got := memberLabel(c.m); got != c.want {
			t.Errorf("memberLabel(%+v) = %q, want %q", c.m, got, c.want)
		}
	}
}
