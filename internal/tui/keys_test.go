package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestHintsForFocus(t *testing.T) {
	search := hints(focusSearch, true)
	if !strings.Contains(search, "select result") {
		t.Errorf("search hints = %q, want it to mention selecting a result", search)
	}

	searchNoResults := hints(focusSearch, false)
	if !strings.Contains(searchNoResults, "recall search") {
		t.Errorf("search hints with no results = %q, want it to mention recalling a search", searchNoResults)
	}

	mapHints := hints(focusMap, false)
	if !strings.Contains(mapHints, "follow") {
		t.Errorf("map hints = %q, want it to mention follow", mapHints)
	}
}

func TestKeyBindingsMatchExpectedRunes(t *testing.T) {
	tests := []struct {
		name    string
		keyStr  string
		matches func(tea.KeyMsg) bool
	}{
		{"quit q", "q", func(m tea.KeyMsg) bool { return key.Matches(m, keys.Quit) }},
		{"up k", "k", func(m tea.KeyMsg) bool { return key.Matches(m, keys.Up) }},
		{"down j", "j", func(m tea.KeyMsg) bool { return key.Matches(m, keys.Down) }},
		{"follow enter", "enter", func(m tea.KeyMsg) bool { return key.Matches(m, keys.Follow) }},
		{"follow l", "l", func(m tea.KeyMsg) bool { return key.Matches(m, keys.Follow) }},
		{"back u", "u", func(m tea.KeyMsg) bool { return key.Matches(m, keys.Back) }},
		{"back h", "h", func(m tea.KeyMsg) bool { return key.Matches(m, keys.Back) }},
		{"toggle in i", "i", func(m tea.KeyMsg) bool { return key.Matches(m, keys.ToggleIn) }},
		{"toggle out o", "o", func(m tea.KeyMsg) bool { return key.Matches(m, keys.ToggleOut) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := keyMsgFor(tt.keyStr)
			if !tt.matches(msg) {
				t.Errorf("expected key %q to match %s", tt.keyStr, tt.name)
			}
		})
	}
}

func TestSelectResultDoesNotMatchTypedLetterL(t *testing.T) {
	// SelectResult must be enter-only so typing "l" in the search box still
	// inserts the letter instead of confirming the highlighted result.
	msg := keyMsgFor("l")
	if key.Matches(msg, keys.SelectResult) {
		t.Error("SelectResult should not match the letter 'l'")
	}
}

func TestResultNavDoesNotMatchTypedLetters(t *testing.T) {
	// ResultUp/ResultDown must be arrow-only so typing "j"/"k" in the search
	// box still inserts those letters instead of moving the result cursor.
	for _, r := range []string{"j", "k"} {
		msg := keyMsgFor(r)
		if key.Matches(msg, keys.ResultUp) || key.Matches(msg, keys.ResultDown) {
			t.Errorf("ResultUp/ResultDown should not match typed letter %q", r)
		}
	}
}

func keyMsgFor(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}
