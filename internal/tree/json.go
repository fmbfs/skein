package tree

import (
	"encoding/json"
	"io"

	"github.com/fmbfs/skein/internal/compositor"
)

// PrintJSON renders a RelationMap as JSON for scripting/CI consumption
// (the `--json` flag in draw mode).
func PrintJSON(w io.Writer, rm *compositor.RelationMap) error {
	return encodeJSON(w, rm)
}

// PrintClassJSON renders a ClassMap as JSON.
func PrintClassJSON(w io.Writer, cm *compositor.ClassMap) error {
	return encodeJSON(w, cm)
}

// PrintFileJSON renders a FileMap as JSON.
func PrintFileJSON(w io.Writer, fm *compositor.FileMap) error {
	return encodeJSON(w, fm)
}

func encodeJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
