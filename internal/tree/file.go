package tree

import (
	"fmt"
	"io"

	"github.com/fmbfs/skein/internal/compositor"
)

// PrintFile renders a FileMap as a tree(1)-style Unicode tree.
func PrintFile(w io.Writer, fm *compositor.FileMap) {
	fmt.Fprintf(w, "%s  [%s]\n", fm.ThreadName, fm.Kind)
	if len(fm.Symbols) == 0 {
		return
	}
	printMembersSection(w, "symbols", fm.Symbols, true)
}
