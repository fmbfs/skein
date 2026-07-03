package tui

import (
	"testing"
)

func TestEditorCommandDefaultsToViWithPlusLineConvention(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	cmd := editorCommand("foo.cpp", 12)
	if got := cmd.Args; len(got) != 3 || got[0] != "vi" || got[1] != "+12" || got[2] != "foo.cpp" {
		t.Errorf("editorCommand args = %v, want [vi +12 foo.cpp]", got)
	}
}

func TestEditorCommandNoLineJustOpensFile(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nano")
	cmd := editorCommand("foo.cpp", 0)
	if got := cmd.Args; len(got) != 2 || got[0] != "nano" || got[1] != "foo.cpp" {
		t.Errorf("editorCommand args = %v, want [nano foo.cpp]", got)
	}
}

func TestEditorCommandVSCodeUsesGotoFlag(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "code")
	cmd := editorCommand("foo.cpp", 12)
	if got := cmd.Args; len(got) != 3 || got[0] != "code" || got[1] != "--goto" || got[2] != "foo.cpp:12" {
		t.Errorf("editorCommand args = %v, want [code --goto foo.cpp:12]", got)
	}
}

func TestEditorCommandSublimeUsesColonConvention(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "subl")
	cmd := editorCommand("foo.cpp", 12)
	if got := cmd.Args; len(got) != 2 || got[0] != "subl" || got[1] != "foo.cpp:12" {
		t.Errorf("editorCommand args = %v, want [subl foo.cpp:12]", got)
	}
}

func TestEditorCommandVISUALTakesPrecedenceOverEDITOR(t *testing.T) {
	t.Setenv("VISUAL", "nvim")
	t.Setenv("EDITOR", "nano")
	cmd := editorCommand("foo.cpp", 5)
	if got := cmd.Args; len(got) != 3 || got[0] != "nvim" || got[1] != "+5" || got[2] != "foo.cpp" {
		t.Errorf("editorCommand args = %v, want VISUAL (nvim) to win over EDITOR (nano), got %v", got, got)
	}
}

func TestEditorCommandWithArgumentsInEnvVar(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "code -w")
	cmd := editorCommand("foo.cpp", 12)
	if got := cmd.Args; len(got) != 4 || got[0] != "code" || got[1] != "-w" || got[2] != "--goto" || got[3] != "foo.cpp:12" {
		t.Errorf("editorCommand args = %v, want [code -w --goto foo.cpp:12]", got)
	}
}
