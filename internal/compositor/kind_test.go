package compositor

import (
	"testing"

	"github.com/fmbfs/skein/internal/lsp"
)

// TestKindName_PropertyAndConstant is the regression test for the real bug
// found against a real-world C++ class: a static class constant member
// has SymbolKind Property(7), which kindName didn't handle,
// falling back to the generic "symbol" label.
func TestKindName_PropertyAndConstant(t *testing.T) {
	cases := []struct {
		kind lsp.SymbolKind
		want string
	}{
		{lsp.SymbolKindProperty, "field"},
		{lsp.SymbolKindVariable, "field"},
		{lsp.SymbolKindConstant, "field"},
		{lsp.SymbolKindField, "field"},
	}
	for _, c := range cases {
		if got := kindName(c.kind); got != c.want {
			t.Errorf("kindName(%d) = %q, want %q", c.kind, got, c.want)
		}
	}
}
