package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// editorCommand builds the *exec.Cmd used by the g ("goto") key to open
// path at line in the user's default editor (docs/SPEC.md section 7). It
// reads $VISUAL then $EDITOR, falling back to "vi" if neither is set, and
// adapts the arguments to whichever convention that editor's binary name
// expects: GUI editors like VS Code/Sublime/Atom take "--goto path:line"
// or "path:line", while terminal editors (vim/nvim/vi/nano/emacs, and
// anything else not special-cased) use the universal "+N path"
// convention. line == 0 means "no specific line" — just open the file.
func editorCommand(path string, line int) *exec.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	// editor may itself be a command line with arguments (e.g. "code -w");
	// only the basename of the first field decides the argument style.
	fields := strings.Fields(editor)
	bin := fields[0]
	args := append([]string{}, fields[1:]...)
	base := filepath.Base(bin)

	switch base {
	case "code", "code-insiders", "codium":
		if line > 0 {
			args = append(args, "--goto", fmt.Sprintf("%s:%d", path, line))
		} else {
			args = append(args, path)
		}
	case "subl", "sublime_text", "atom":
		if line > 0 {
			args = append(args, fmt.Sprintf("%s:%d", path, line))
		} else {
			args = append(args, path)
		}
	default:
		// vim, nvim, vi, nano, emacs, and any unrecognised editor: the
		// universal "+N path" convention.
		if line > 0 {
			args = append(args, fmt.Sprintf("+%d", line), path)
		} else {
			args = append(args, path)
		}
	}

	cmd := exec.CommandContext(context.Background(), bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
