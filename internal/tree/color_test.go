package tree

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/fmbfs/skein/internal/compositor"
)

func TestPrint_MonochromeByDefault(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{
		ThreadName: "foo",
		Kind:       "method",
		CalledIn:   []compositor.CalledInGroup{{File: "a.cpp", Lines: []int{1}}},
		Calls:      []string{"bar"},
	}
	Print(&buf, rm)
	got := buf.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("Print() without WithColour emitted ANSI codes: %q", got)
	}
}

func TestPrint_WithColourEnabled(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{
		ThreadName: "foo",
		Kind:       "method",
		CalledIn:   []compositor.CalledInGroup{{File: "a.cpp", Lines: []int{1}}},
		Calls:      []string{"bar"},
	}
	Print(&buf, rm, WithColour(true))
	got := buf.String()
	if !strings.Contains(got, ansiIncoming) {
		t.Errorf("Print() with WithColour(true) missing incoming colour code: %q", got)
	}
	if !strings.Contains(got, ansiOutgoing) {
		t.Errorf("Print() with WithColour(true) missing outgoing colour code: %q", got)
	}
	if !strings.Contains(got, ansiReset) {
		t.Errorf("Print() with WithColour(true) missing reset code: %q", got)
	}
	// The "a.cpp" file entry (called in) still appears verbatim between codes.
	if !strings.Contains(got, "a.cpp") {
		t.Errorf("Print() with colour lost file name: %q", got)
	}
	if !strings.Contains(got, "bar") {
		t.Errorf("Print() with colour lost callee name: %q", got)
	}
}

func TestPrint_WithColourFalseExplicit(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{
		ThreadName: "foo",
		Kind:       "method",
		Calls:      []string{"bar"},
	}
	Print(&buf, rm, WithColour(false))
	got := buf.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("Print() with WithColour(false) emitted ANSI codes: %q", got)
	}
}

func TestPrintClass_MonochromeByDefault(t *testing.T) {
	var buf bytes.Buffer
	cm := &compositor.ClassMap{
		ThreadName:  "Foo",
		Kind:        "class",
		Inherits:    []string{"Base"},
		InheritedBy: []string{"Derived"},
	}
	PrintClass(&buf, cm)
	got := buf.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("PrintClass() without WithColour emitted ANSI codes: %q", got)
	}
}

func TestPrintClass_WithColourEnabled(t *testing.T) {
	var buf bytes.Buffer
	cm := &compositor.ClassMap{
		ThreadName:  "Foo",
		Kind:        "class",
		Inherits:    []string{"Base"},
		InheritedBy: []string{"Derived"},
	}
	PrintClass(&buf, cm, WithColour(true))
	got := buf.String()
	if !strings.Contains(got, ansiOutgoing+"inherits") && !strings.Contains(got, "inherits") {
		t.Errorf("PrintClass() with colour missing inherits label: %q", got)
	}
	if !strings.Contains(got, ansiIncoming) {
		t.Errorf("PrintClass() with colour missing incoming colour code for 'inherited by': %q", got)
	}
	if !strings.Contains(got, ansiOutgoing) {
		t.Errorf("PrintClass() with colour missing outgoing colour code for 'inherits': %q", got)
	}
	if !strings.Contains(got, "Base") || !strings.Contains(got, "Derived") {
		t.Errorf("PrintClass() with colour lost class names: %q", got)
	}
}

func TestPrintFile_AcceptsOptionsButStaysMonochrome(t *testing.T) {
	var buf bytes.Buffer
	fm := &compositor.FileMap{
		ThreadName: "a.cpp",
		Kind:       "file",
		Symbols:    []compositor.Member{{Name: "foo", Kind: "function"}},
	}
	PrintFile(&buf, fm, WithColour(true))
	got := buf.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("PrintFile() emitted ANSI codes despite having no directional data: %q", got)
	}
	if !strings.Contains(got, "foo()") {
		t.Errorf("PrintFile() missing symbol: %q", got)
	}
}

func TestAutoColour_NoColorFlagForcesOff(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	if got := AutoColour(w, true); got {
		t.Errorf("AutoColour(w, noColour=true) = true, want false")
	}
}

func TestAutoColour_NonTTYDisablesColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	// A pipe is never a TTY, so AutoColour must report false regardless of
	// the noColour flag's value.
	if got := AutoColour(w, false); got {
		t.Errorf("AutoColour(pipe, noColour=false) = true, want false (pipes are never a TTY)")
	}
}

func TestAutoColour_NoColorEnvVarDisablesColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	if got := AutoColour(w, false); got {
		t.Errorf("AutoColour with NO_COLOR set = true, want false")
	}
}
