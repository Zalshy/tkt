package side

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestRenderSessionsEmpty verifies that renderSessions with nil events does not
// panic and produces output containing the "SESSIONS" section header.
func TestRenderSessionsEmpty(t *testing.T) {
	out := renderSessions(nil, 80)
	if !strings.Contains(out, "SESSIONS") {
		t.Errorf("expected 'SESSIONS' in output, got: %q", out)
	}
}

// TestRenderSessionsEntries verifies that an event with a known name appears
// in the rendered output alongside the "started" label.
func TestRenderSessionsEntries(t *testing.T) {
	events := []sessionEvent{
		{
			name:      "impl-fast-session",
			role:      "implementer",
			startedAt: time.Now().Add(-10 * time.Minute),
		},
	}
	out := renderSessions(events, 80)
	if !strings.Contains(out, "impl-fast-session") {
		t.Errorf("expected 'impl-fast-session' in output, got: %q", out)
	}
	if !strings.Contains(out, "started") {
		t.Errorf("expected 'started' label in output, got: %q", out)
	}
}

// TestRenderSessionsHighlight verifies that an event with a recent arrivedAt
// produces different output than an event with a zero arrivedAt.
// Forces TrueColor profile so lipgloss emits ANSI codes even outside a TTY.
func TestRenderSessionsHighlight(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	base := sessionEvent{
		name:      "arch-session",
		role:      "architect",
		startedAt: time.Now().Add(-2 * time.Minute),
	}

	// Event with recent arrivedAt — should trigger highlight.
	newEvent := base
	newEvent.arrivedAt = time.Now()

	// Event with zero arrivedAt — no highlight.
	oldEvent := base

	outNew := renderSessions([]sessionEvent{newEvent}, 80)
	outOld := renderSessions([]sessionEvent{oldEvent}, 80)

	if outNew == outOld {
		t.Errorf("expected different output for new vs old session, but both rendered identically")
	}
}
