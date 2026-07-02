package compositor

import "fmt"

// TruncationWarning is a human-readable note about what the strand limit
// hid, suitable for a stderr line (draw mode) or a TUI status-bar message
// (docs/SPEC.md section 6). Empty means nothing was truncated.
//
// The limit is applied per relationship section (called-in / calls /
// members / symbols) rather than as one shared budget across the whole
// composed map: a single high-traffic "called in" list (hundreds of call
// sites is common in real codebases) shouldn't be allowed to starve the
// "calls" section's visibility, and vice versa. Each section gets its own
// full budget.
type TruncationWarning string

// TruncateCalledIn caps the total number of call-site lines across all
// CalledIn groups to limit, dropping the excess from the tail (later
// files/lines first), and returns a warning describing what was hidden —
// empty if nothing was truncated. limit <= 0 disables truncation.
func (m *RelationMap) TruncateCalledIn(limit int) TruncationWarning {
	if limit <= 0 {
		return ""
	}
	total := m.CalledInTotal()
	if total <= limit {
		return ""
	}

	remaining := limit
	kept := make([]CalledInGroup, 0, len(m.CalledIn))
	for _, g := range m.CalledIn {
		if remaining <= 0 {
			break
		}
		if len(g.Lines) <= remaining {
			kept = append(kept, g)
			remaining -= len(g.Lines)
			continue
		}
		kept = append(kept, CalledInGroup{File: g.File, Lines: g.Lines[:remaining]})
		remaining = 0
	}
	m.CalledIn = kept
	hidden := total - limit
	return TruncationWarning(fmt.Sprintf("called-in truncated: showing %d of %d call sites (%d hidden)", limit, total, hidden))
}

// TruncateCalls caps the Calls list to limit entries, dropping the tail.
// limit <= 0 disables truncation.
func (m *RelationMap) TruncateCalls(limit int) TruncationWarning {
	if limit <= 0 || len(m.Calls) <= limit {
		return ""
	}
	total := len(m.Calls)
	hidden := total - limit
	m.Calls = m.Calls[:limit]
	return TruncationWarning(fmt.Sprintf("calls truncated: showing %d of %d (%d hidden)", limit, total, hidden))
}

// TruncateMembers caps a class's top-level Members list to limit entries.
// Nested Member.Children aren't individually counted or trimmed — a
// deliberate v1 simplification, since a top-level cap already handles the
// common case (a class with an excessive flat member list) and recursing
// into every nested container to maintain one shared budget would add
// complexity disproportionate to how rarely deeply-nested member trees
// exceed the default limit on their own. limit <= 0 disables truncation.
func (m *ClassMap) TruncateMembers(limit int) TruncationWarning {
	if limit <= 0 || len(m.Members) <= limit {
		return ""
	}
	total := len(m.Members)
	hidden := total - limit
	m.Members = m.Members[:limit]
	return TruncationWarning(fmt.Sprintf("members truncated: showing %d of %d (%d hidden)", limit, total, hidden))
}

// TruncateSymbols caps a file's top-level Symbols list to limit entries.
// Same nested-Children simplification as TruncateMembers applies here.
// limit <= 0 disables truncation.
func (m *FileMap) TruncateSymbols(limit int) TruncationWarning {
	if limit <= 0 || len(m.Symbols) <= limit {
		return ""
	}
	total := len(m.Symbols)
	hidden := total - limit
	m.Symbols = m.Symbols[:limit]
	return TruncationWarning(fmt.Sprintf("symbols truncated: showing %d of %d (%d hidden)", limit, total, hidden))
}
