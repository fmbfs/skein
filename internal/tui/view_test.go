package tui

import (
	"strings"
	"testing"
)

func TestFilterByDirectionHidesIncoming(t *testing.T) {
	nodes := []Node{
		{Label: "in", Direction: directionIncoming},
		{Label: "out", Direction: directionOutgoing},
		{Label: "neutral"},
	}
	got := filterByDirection(nodes, false, true)
	var labels []string
	for _, n := range got {
		labels = append(labels, n.Label)
	}
	if len(got) != 2 || labels[0] != "out" || labels[1] != "neutral" {
		t.Errorf("filterByDirection(showIn=false) = %+v, want [out neutral]", labels)
	}
}

func TestFilterByDirectionHidesOutgoing(t *testing.T) {
	nodes := []Node{
		{Label: "in", Direction: directionIncoming},
		{Label: "out", Direction: directionOutgoing},
	}
	got := filterByDirection(nodes, true, false)
	if len(got) != 1 || got[0].Label != "in" {
		t.Errorf("filterByDirection(showOut=false) = %+v, want [in]", got)
	}
}

func TestFilterByDirectionRecursesIntoChildren(t *testing.T) {
	nodes := []Node{
		{Label: "parent", Children: []Node{
			{Label: "in child", Direction: directionIncoming},
			{Label: "neutral child"},
		}},
	}
	got := filterByDirection(nodes, false, true)
	if len(got) != 1 || len(got[0].Children) != 1 || got[0].Children[0].Label != "neutral child" {
		t.Errorf("filterByDirection did not recurse correctly: %+v", got)
	}
}

func TestFilterByDirectionShowsAllByDefault(t *testing.T) {
	nodes := []Node{
		{Label: "in", Direction: directionIncoming},
		{Label: "out", Direction: directionOutgoing},
		{Label: "neutral"},
	}
	got := filterByDirection(nodes, true, true)
	if len(got) != 3 {
		t.Errorf("filterByDirection(true,true) = %+v, want all 3 nodes", got)
	}
}

func TestModelViewSmoke(t *testing.T) {
	m := newTestModel()
	m.bundles[0].thread = threadState{
		name: "Foo", kind: "method",
		nodes: []Node{{Label: "child", Follow: followMethod, Target: "Bar"}},
	}
	out := m.View()
	if out == "" {
		t.Error("View() returned empty output")
	}
	if !strings.Contains(out, "Foo") {
		t.Errorf("View() output missing thread name:\n%s", out)
	}
}

func TestModelViewQuittingReturnsEmpty(t *testing.T) {
	m := newTestModel()
	m.quitting = true
	if out := m.View(); out != "" {
		t.Errorf("View() while quitting = %q, want empty", out)
	}
}

func TestModelViewBeforeWindowSize(t *testing.T) {
	m := New(nil, "/root", "", "")
	out := m.View()
	if !strings.Contains(out, "loading") {
		t.Errorf("View() before a WindowSizeMsg = %q, want a loading placeholder", out)
	}
}

func TestModelViewHelpOverlay(t *testing.T) {
	m := newTestModel()
	m.help = true
	out := m.View()
	if !strings.Contains(out, "Keybindings") {
		t.Errorf("View() with help=true = %q, want the keybindings overlay", out)
	}
}

func TestModelViewShowsErrorInStatusBar(t *testing.T) {
	m := newTestModel()
	m.err = errBoom
	out := m.View()
	if !strings.Contains(out, "boom") {
		t.Errorf("View() with an error set = %q, want it to surface the error", out)
	}
}
