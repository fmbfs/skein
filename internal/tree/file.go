package tree

import (
	"fmt"
	"io"

	"github.com/fmbfs/skein/internal/compositor"
)

// PrintFile renders a FileMap as a tree(1)-style Unicode tree. FileMap has no
// inherent call direction (it's a flat symbol listing), so this accepts
// Option for API symmetry with Print/PrintClass but currently renders
// monochrome regardless of WithColour.
func PrintFile(w io.Writer, fm *compositor.FileMap, opts ...Option) {
	_ = resolveOpts(opts)
	fmt.Fprintf(w, "%s  [%s]\n", fm.ThreadName, fm.Kind)
	if len(fm.Symbols) == 0 {
		return
	}
	printMembersSection(w, "symbols", fm.Symbols, true)
}
