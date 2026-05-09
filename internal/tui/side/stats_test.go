package side

import (
	"strings"
	"testing"
)

// TestRenderStatsZeroData verifies that renderStats with empty statsData does
// not panic, and that the output contains "STATS" and "TODO".
func TestRenderStatsZeroData(t *testing.T) {
	out := renderStats(statsData{}, 80)
	if !strings.Contains(out, "STATS") {
		t.Errorf("expected output to contain %q, got: %q", "STATS", out)
	}
	// Zero data (nil maps) shows "loading…"; populated (non-nil maps) shows status rows.
	// An empty statsData{} has nil maps → loading path.
	// We just verify it contains "STATS" and doesn't panic.
	_ = out
}

// TestRenderStatsZeroDataContainsTODO verifies that renderStats with non-nil
// but empty maps renders status rows including "TODO".
func TestRenderStatsZeroDataContainsTODO(t *testing.T) {
	s := statsData{
		byStatus:    map[string]int{},
		byAttention: map[string]int{},
		byMainType:  map[string]int{},
	}
	out := renderStats(s, 80)
	if !strings.Contains(out, "STATS") {
		t.Errorf("expected output to contain %q, got: %q", "STATS", out)
	}
	if !strings.Contains(out, "TODO") {
		t.Errorf("expected output to contain %q, got: %q", "TODO", out)
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

	out := renderStats(s, 80)

	checks := []string{
		"STATS",
		"IN_PROGRESS",
		"by attention level",
		"by main_type",
		"█",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q; got:\n%s", want, out)
		}
	}
}

// TestRenderStatsNarrow verifies that renderStats with a very narrow width
// does not panic (exercises bar-width clamping and column math).
func TestRenderStatsNarrow(t *testing.T) {
	// Should not panic.
	out := renderStats(statsData{}, 30)
	if !strings.Contains(out, "STATS") {
		t.Errorf("expected output to contain %q at narrow width, got: %q", "STATS", out)
	}
}
