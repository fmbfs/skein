package tui

import "strings"

// bundle is a named, persistent pinned thread — docs/SPEC.md's "bundle"
// vocabulary entry. The TUI keeps one bundle per tab; v1 layout is tabs
// only (split-view/overlay are v2/v3, explicitly deferred in SPEC.md).
//
// Each bundle carries its own spool (docs/SPEC.md's navigation-history
// vocabulary entry): back/fwd are snapshotted threadStates, so back/forward
// navigation (u / ctrl+r) doesn't re-query clangd, and origin remembers the
// bundle's first thread for the r (reset) key.
type bundle struct {
	name   string
	thread threadState
	origin threadState
	back   []threadState
	fwd    []threadState
}

// renderBundleTabs renders the tab bar: one tab per bundle plus a trailing
// "[+]" affordance hinting that pinning (docs/SPEC.md's `p` key) opens a
// new one.
func renderBundleTabs(bundles []bundle, active int) string {
	var b strings.Builder
	for i, bd := range bundles {
		label := "[" + bd.name + "]"
		if i == active {
			b.WriteString(bundleTabActiveStyle.Render(label))
		} else {
			b.WriteString(bundleTabStyle.Render(label))
		}
		b.WriteByte(' ')
	}
	b.WriteString(bundleTabStyle.Render("[+]"))
	return b.String()
}

// pinBundle appends a new bundle snapshotting the given thread as its own
// tab and returns the updated slice plus the new active index. Pinning the
// same thread name twice is allowed — docs/SPEC.md doesn't forbid it, and
// two pins of the same symbol taken at different ply/filter settings are a
// legitimate workflow (compare before/after a filter change).
func pinBundle(bundles []bundle, name string, t threadState) ([]bundle, int) {
	bundles = append(bundles, bundle{name: name, thread: t, origin: t})
	return bundles, len(bundles) - 1
}

// closeBundle removes the bundle at index i, clamping the new active
// index into bounds. Refuses to close the last remaining bundle — a TUI
// with zero tabs has nothing to render in the map/detail panels.
func closeBundle(bundles []bundle, i, active int) ([]bundle, int) {
	if len(bundles) <= 1 || i < 0 || i >= len(bundles) {
		return bundles, active
	}
	bundles = append(bundles[:i], bundles[i+1:]...)
	if active >= len(bundles) {
		active = len(bundles) - 1
	}
	return bundles, active
}

// jumpToBundle maps digit key "1".."9" to bundle index 0..8 (docs/SPEC.md
// section 7: "1-9: jump to bundle N"), clamped to the current bundle count.
// Digits beyond the number of open bundles, or "0", are no-ops (return the
// current active index unchanged).
func jumpToBundle(active, count int, digit int) int {
	idx := digit - 1
	if digit < 1 || digit > 9 || idx >= count {
		return active
	}
	return idx
}

// cycleBundle moves the active index by delta, wrapping around.
func cycleBundle(active, count, delta int) int {
	if count == 0 {
		return 0
	}
	next := (active + delta) % count
	if next < 0 {
		next += count
	}
	return next
}
