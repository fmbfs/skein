package tree

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fmbfs/skein/internal/compositor"
)

func TestPrintFile_NoSymbols(t *testing.T) {
	var buf bytes.Buffer
	fm := &compositor.FileMap{ThreadName: "a.cpp", Kind: "file"}
	PrintFile(&buf, fm)

	want := "a.cpp  [file]\n"
	if buf.String() != want {
		t.Errorf("PrintFile() = %q, want %q", buf.String(), want)
	}
}

func TestPrintFile_WithSymbols(t *testing.T) {
	var buf bytes.Buffer
	fm := &compositor.FileMap{
		ThreadName: "a.cpp",
		Kind:       "file",
		Symbols: []compositor.Member{
			{Name: "foo", Kind: "function"},
			{Name: "Bar", Kind: "class", Children: []compositor.Member{
				{Name: "method", Kind: "method"},
			}},
		},
	}
	PrintFile(&buf, fm)
	got := buf.String()

	if !strings.Contains(got, "symbols") {
		t.Errorf("PrintFile() missing 'symbols' section: %q", got)
	}
	if !strings.Contains(got, "foo()") {
		t.Errorf("PrintFile() missing function symbol: %q", got)
	}
	if !strings.Contains(got, "Bar") {
		t.Errorf("PrintFile() missing class symbol: %q", got)
	}
	if !strings.Contains(got, "method()") {
		t.Errorf("PrintFile() missing nested method: %q", got)
	}
}
