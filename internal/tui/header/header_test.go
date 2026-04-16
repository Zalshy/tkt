package header

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/testutil"
)

// TestView_ContainsTitleText verifies that the title "tkt" appears in the output.
func TestView_ContainsTitleText(t *testing.T) {
	got := testutil.StripANSI(New(80, 0).View())
	if !strings.Contains(got, "tkt") {
		t.Errorf("expected stripped output to contain %q, got:\n%s", "tkt", got)
	}
}

// TestView_TwoLines verifies that View() returns exactly two lines (title + separator).
func TestView_TwoLines(t *testing.T) {
	view := New(80, 0).View()
	if count := strings.Count(view, "\n"); count != 1 {
		t.Errorf("expected 1 newline in View() (two lines), got %d\noutput: %q", count, view)
	}
}

// TestView_WidthRespected verifies that no line's visual width exceeds the specified width.
func TestView_WidthRespected(t *testing.T) {
	view := New(80, 0).View()
	for i, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line %d has visual width %d, want <= 80: %q", i, w, testutil.StripANSI(line))
		}
	}
}

// TestView_EmptyWidth verifies that a zero-width model returns a non-panicking string.
func TestView_EmptyWidth(t *testing.T) {
	got := New(0, 0).View()
	// Returns "\n\n" for zero width — just verify no panic.
	_ = got
}

// TestAnimation_CompletesEventually verifies that sending enough tickMsgs causes
// animDone to become true within 50 ticks.
func TestAnimation_CompletesEventually(t *testing.T) {
	m := New(80, 0)
	const maxTicks = 50
	for i := 0; i < maxTicks; i++ {
		var cmd interface{}
		m, cmd = m.Update(TickMsg{})
		_ = cmd
		if m.animDone {
			return // passed
		}
	}
	t.Errorf("animation did not complete within %d ticks", maxTicks)
}
