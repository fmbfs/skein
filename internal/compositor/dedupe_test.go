package compositor

import (
	"reflect"
	"testing"

	"github.com/fmbfs/skein/internal/lsp"
)

// TestDedupeTypeNames_SameLocationDifferentSymbolID is the regression test
// for the real bug found against a real-world PIMPL class: typeHierarchy/
// subtypes returned "EnableMakeUnique" twice under two different
// symbolIDs, but with identical name, URI, and range — a CRTP indexing
// artefact, not two real distinct base classes.
func TestDedupeTypeNames_SameLocationDifferentSymbolID(t *testing.T) {
	sameRange := lsp.Range{Start: lsp.Position{Line: 135, Character: 15}, End: lsp.Position{Line: 135, Character: 31}}
	items := []lsp.TypeHierarchyItem{
		{Name: "EnableMakeUnique", URI: "file:///Dtc.h", Range: sameRange, Data: []byte(`"A"`)},
		{Name: "EnableMakeUnique", URI: "file:///Dtc.h", Range: sameRange, Data: []byte(`"B"`)},
		{Name: "OtherBase", URI: "file:///Dtc.h", Range: lsp.Range{Start: lsp.Position{Line: 10}}},
	}

	got := dedupeTypeNames(items)
	want := []string{"EnableMakeUnique", "OtherBase"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupeTypeNames = %v, want %v", got, want)
	}
}

func TestDedupeTypeNames_SameNameDifferentLocationKept(t *testing.T) {
	items := []lsp.TypeHierarchyItem{
		{Name: "Rule", URI: "file:///a.h", Range: lsp.Range{Start: lsp.Position{Line: 1}}},
		{Name: "Rule", URI: "file:///b.h", Range: lsp.Range{Start: lsp.Position{Line: 1}}},
	}
	got := dedupeTypeNames(items)
	want := []string{"Rule", "Rule"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupeTypeNames = %v, want %v (genuinely distinct classes with the same name should both be kept)", got, want)
	}
}
