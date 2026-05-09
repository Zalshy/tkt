package side

import (
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// TestClockFormat verifies that renderHeader output contains a HH:MM time string.
func TestClockFormat(t *testing.T) {
	c := newClockModel()
	result := renderHeader(c, 80)
	pattern := regexp.MustCompile(`\d{2}:\d{2}`)
	if !pattern.MatchString(result) {
		t.Errorf("renderHeader output does not contain HH:MM pattern; got: %q", result)
	}
}

// TestClockTickAdvances verifies that sending a clockTickMsg to clockModel.update
// stores a non-zero time in the returned model.
func TestClockTickAdvances(t *testing.T) {
	c := clockModel{now: time.Time{}} // zero time
	updated, cmd := c.update(clockTickMsg{})
	if updated.now.IsZero() {
		t.Error("expected non-zero time after clockTickMsg, got zero")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd after clockTickMsg (should reschedule next tick)")
	}
}

// TestHeaderWidth verifies that renderHeader at width 80 produces output whose
// lipgloss-measured width does not exceed 80 columns.
func TestHeaderWidth(t *testing.T) {
	c := newClockModel()
	result := renderHeader(c, 80)
	w := lipgloss.Width(result)
	if w > 80 {
		t.Errorf("renderHeader width %d exceeds 80", w)
	}
}
