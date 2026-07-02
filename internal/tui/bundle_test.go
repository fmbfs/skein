package tui

import (
	"strings"
	"testing"
)

func TestJumpToBundle(t *testing.T) {
	tests := []struct {
		name          string
		active, count int
		digit         int
		want          int
	}{
		{"jump to 1 -> index 0", 5, 3, 1, 0},
		{"jump to 3 -> index 2", 5, 3, 3, 2},
		{"digit beyond bundle count is no-op", 1, 3, 5, 1},
		{"digit 0 is no-op", 1, 3, 0, 1},
		{"digit out of 1-9 range is no-op", 1, 3, 10, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jumpToBundle(tt.active, tt.count, tt.digit)
			if got != tt.want {
				t.Errorf("jumpToBundle(%d,%d,%d) = %d, want %d", tt.active, tt.count, tt.digit, got, tt.want)
			}
		})
	}
}

func TestPinBundle(t *testing.T) {
	bundles := []bundle{{name: "tangle"}}
	bundles, active := pinBundle(bundles, "Foo", threadState{name: "Foo"})
	if len(bundles) != 2 {
		t.Fatalf("expected 2 bundles, got %d", len(bundles))
	}
	if active != 1 {
		t.Errorf("active = %d, want 1", active)
	}
	if bundles[1].name != "Foo" || bundles[1].thread.name != "Foo" || bundles[1].origin.name != "Foo" {
		t.Errorf("pinned bundle = %+v, want name/thread/origin = Foo", bundles[1])
	}
}

func TestCloseBundleRefusesLastOne(t *testing.T) {
	bundles := []bundle{{name: "only"}}
	got, active := closeBundle(bundles, 0, 0)
	if len(got) != 1 {
		t.Errorf("closeBundle should refuse to close the last bundle, got %d bundles", len(got))
	}
	if active != 0 {
		t.Errorf("active = %d, want 0", active)
	}
}

func TestCloseBundleRemovesAndClampsActive(t *testing.T) {
	bundles := []bundle{{name: "a"}, {name: "b"}, {name: "c"}}
	got, active := closeBundle(bundles, 2, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 bundles remaining, got %d", len(got))
	}
	if active != 1 {
		t.Errorf("active = %d, want clamped to 1", active)
	}

	got2, active2 := closeBundle(bundles, 0, 0)
	if len(got2) != 2 || got2[0].name != "b" {
		t.Errorf("expected [b c] remaining, got %+v", got2)
	}
	if active2 != 0 {
		t.Errorf("active2 = %d, want 0", active2)
	}
}

func TestCloseBundleOutOfBounds(t *testing.T) {
	bundles := []bundle{{name: "a"}, {name: "b"}}
	got, active := closeBundle(bundles, 5, 0)
	if len(got) != 2 || active != 0 {
		t.Errorf("out-of-bounds close should be a no-op, got bundles=%+v active=%d", got, active)
	}
}

func TestCycleBundle(t *testing.T) {
	tests := []struct {
		name                       string
		active, count, delta, want int
	}{
		{"next wraps", 2, 3, 1, 0},
		{"prev wraps", 0, 3, -1, 2},
		{"next no wrap", 0, 3, 1, 1},
		{"empty", 0, 0, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cycleBundle(tt.active, tt.count, tt.delta)
			if got != tt.want {
				t.Errorf("cycleBundle(%d,%d,%d) = %d, want %d", tt.active, tt.count, tt.delta, got, tt.want)
			}
		})
	}
}

func TestRenderBundleTabs(t *testing.T) {
	bundles := []bundle{{name: "a"}, {name: "b"}}
	got := renderBundleTabs(bundles, 1)
	if got == "" {
		t.Error("renderBundleTabs returned empty string")
	}
	// Smoke test only — exact ANSI styling isn't asserted, just that both
	// tab names and the pin affordance show up in the rendered output.
	for _, want := range []string{"a", "b", "+"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderBundleTabs output %q missing %q", got, want)
		}
	}
}
