package compositor

import "testing"

func TestTruncateCalledInNoLimitNeeded(t *testing.T) {
	rm := &RelationMap{CalledIn: []CalledInGroup{{File: "a.cpp", Lines: []int{1, 2}}}}
	warn := rm.TruncateCalledIn(50)
	if warn != "" {
		t.Errorf("expected no warning when under the limit, got %q", warn)
	}
	if rm.CalledInTotal() != 2 {
		t.Errorf("CalledIn should be untouched, total = %d", rm.CalledInTotal())
	}
}

func TestTruncateCalledInDisabledAtZeroOrNegative(t *testing.T) {
	rm := &RelationMap{CalledIn: []CalledInGroup{{File: "a.cpp", Lines: []int{1, 2, 3}}}}
	if warn := rm.TruncateCalledIn(0); warn != "" {
		t.Errorf("limit<=0 should disable truncation, got warning %q", warn)
	}
	if rm.CalledInTotal() != 3 {
		t.Error("CalledIn should be untouched when truncation is disabled")
	}
}

func TestTruncateCalledInAcrossGroups(t *testing.T) {
	rm := &RelationMap{CalledIn: []CalledInGroup{
		{File: "a.cpp", Lines: []int{1, 2, 3}},
		{File: "b.cpp", Lines: []int{4, 5, 6}},
		{File: "c.cpp", Lines: []int{7, 8}},
	}}
	warn := rm.TruncateCalledIn(4)
	if warn == "" {
		t.Fatal("expected a truncation warning")
	}
	if rm.CalledInTotal() != 4 {
		t.Errorf("CalledInTotal() = %d, want 4", rm.CalledInTotal())
	}
	if len(rm.CalledIn) != 2 || rm.CalledIn[0].File != "a.cpp" || len(rm.CalledIn[0].Lines) != 3 {
		t.Errorf("first group should be kept whole, got %+v", rm.CalledIn[0])
	}
	if rm.CalledIn[1].File != "b.cpp" || len(rm.CalledIn[1].Lines) != 1 {
		t.Errorf("second group should be cut to 1 line, got %+v", rm.CalledIn[1])
	}
}

func TestTruncateCalledInExactLimitIsNoop(t *testing.T) {
	rm := &RelationMap{CalledIn: []CalledInGroup{{File: "a.cpp", Lines: []int{1, 2}}}}
	if warn := rm.TruncateCalledIn(2); warn != "" {
		t.Errorf("exactly at the limit should not warn, got %q", warn)
	}
}

func TestTruncateCallsUnderLimit(t *testing.T) {
	rm := &RelationMap{Calls: []string{"a", "b"}}
	if warn := rm.TruncateCalls(5); warn != "" {
		t.Errorf("expected no warning, got %q", warn)
	}
	if len(rm.Calls) != 2 {
		t.Error("Calls should be untouched")
	}
}

func TestTruncateCallsOverLimit(t *testing.T) {
	rm := &RelationMap{Calls: []string{"a", "b", "c", "d"}}
	warn := rm.TruncateCalls(2)
	if warn == "" {
		t.Fatal("expected a truncation warning")
	}
	if len(rm.Calls) != 2 || rm.Calls[0] != "a" || rm.Calls[1] != "b" {
		t.Errorf("Calls = %+v, want first 2 kept", rm.Calls)
	}
}

func TestTruncateCallsDisabledAtZero(t *testing.T) {
	rm := &RelationMap{Calls: []string{"a", "b"}}
	if warn := rm.TruncateCalls(0); warn != "" {
		t.Errorf("limit<=0 should disable truncation, got %q", warn)
	}
	if len(rm.Calls) != 2 {
		t.Error("Calls should be untouched when disabled")
	}
}

func TestTruncateMembers(t *testing.T) {
	cm := &ClassMap{Members: []Member{{Name: "a"}, {Name: "b"}, {Name: "c"}}}
	warn := cm.TruncateMembers(2)
	if warn == "" {
		t.Fatal("expected a truncation warning")
	}
	if len(cm.Members) != 2 {
		t.Errorf("Members = %+v, want 2 kept", cm.Members)
	}
}

func TestTruncateMembersUnderLimit(t *testing.T) {
	cm := &ClassMap{Members: []Member{{Name: "a"}}}
	if warn := cm.TruncateMembers(50); warn != "" {
		t.Errorf("expected no warning, got %q", warn)
	}
}

func TestTruncateSymbols(t *testing.T) {
	fm := &FileMap{Symbols: []Member{{Name: "a"}, {Name: "b"}, {Name: "c"}}}
	warn := fm.TruncateSymbols(1)
	if warn == "" {
		t.Fatal("expected a truncation warning")
	}
	if len(fm.Symbols) != 1 {
		t.Errorf("Symbols = %+v, want 1 kept", fm.Symbols)
	}
}

func TestTruncateSymbolsUnderLimit(t *testing.T) {
	fm := &FileMap{Symbols: []Member{{Name: "a"}}}
	if warn := fm.TruncateSymbols(50); warn != "" {
		t.Errorf("expected no warning, got %q", warn)
	}
}
