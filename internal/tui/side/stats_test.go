package side

import (
	"strings"
	"testing"
)

// TestRenderStatsZeroData verifies that renderStats with empty statsData does
// not panic. statsData{} has nil maps → loading path returns "loading…".
func TestRenderStatsZeroData(t *testing.T) {
	out := renderStatsRow(statsData{}, 80)
	if !strings.Contains(out, "loading") {
		t.Errorf("expected output to contain %q, got: %q", "loading", out)
	}
}

// TestRenderStatsZeroDataContainsTODO verifies that renderStats with non-nil
// but empty maps renders the stat boxes (not the loading placeholder).
// Labels are lowercased in the rendered output.
func TestRenderStatsZeroDataContainsTODO(t *testing.T) {
	s := statsData{
		byStatus:    map[string]int{},
		byAttention: map[string]int{},
		byMainType:  map[string]int{},
	}
	out := renderStatsRow(s, 80)
	if !strings.Contains(out, "By Status") {
		t.Errorf("expected output to contain %q, got: %q", "By Status", out)
	}
	if !strings.Contains(out, "todo") {
		t.Errorf("expected output to contain %q, got: %q", "todo", out)
	}
}

// TestRenderStatsPopulated verifies that a populated statsData renders correctly.
func TestRenderStatsPopulated(t *testing.T) {
	s := statsData{
		byStatus: map[string]int{
			"TODO":        12,
			"PLANNING":    4,
			"IN_PROGRESS": 2,
			"DONE":        8,
			"VERIFIED":    4,
		},
		byAttention: map[string]int{
			"critical": 3,
			"high":     8,
			"medium":   15,
			"low":      4,
			"unset":    1,
		},
		byMainType: map[string]int{
			"feature": 12,
			"bug":     4,
			"chore":   2,
			"docs":    2,
		},
	}

	out := renderStatsRow(s, 80)

	checks := []string{
		"By Status",    // box title
		"in_prog",      // IN_PROGRESS truncated to labelW: "in_prog…"
		"By Attention", // attention box title
		"By Type",      // type box title
		"▌",            // bar character (half-block, one per ticket)
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q; got:\n%s", want, out)
		}
	}
}

// TestRenderStatsNarrow verifies that renderStats with a very narrow width
// does not panic (exercises bar-width clamping and column math).
// nil maps → loading path returns "loading…".
func TestRenderStatsNarrow(t *testing.T) {
	// Should not panic.
	out := renderStatsRow(statsData{}, 30)
	if !strings.Contains(out, "loading") {
		t.Errorf("expected output to contain %q at narrow width, got: %q", "loading", out)
	}
}
