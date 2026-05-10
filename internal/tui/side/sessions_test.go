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
	out := renderSessions(nil, 80, 10)
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
	out := renderSessions(events, 80, 10)
	if !strings.Contains(out, "impl-fast-session") {
		t.Errorf("expected 'impl-fast-session' in output, got: %q", out)
	}
	if !strings.Contains(out, "impl") {
		t.Errorf("expected role abbreviation 'impl' in output, got: %q", out)
	}
}

// TestRenderSessionsCompact verifies that compact mode (maxVisible=0) shows
// role counts from ALL events but omits the divider and session name rows.
func TestRenderSessionsCompact(t *testing.T) {
	events := []sessionEvent{
		{name: "arch-1", role: "architect", startedAt: time.Now().Add(-5 * time.Minute)},
		{name: "impl-1", role: "implementer", startedAt: time.Now().Add(-3 * time.Minute)},
		{name: "impl-2", role: "implementer", startedAt: time.Now().Add(-1 * time.Minute)},
	}
	out := renderSessions(events, 80, 0)

	// Must still show section header and counts.
	if !strings.Contains(out, "SESSIONS") {
		t.Errorf("compact: expected 'SESSIONS' header, got: %q", out)
	}
	if !strings.Contains(out, "architect") {
		t.Errorf("compact: expected 'architect' count line, got: %q", out)
	}
	if !strings.Contains(out, "implementer") {
		t.Errorf("compact: expected 'implementer' count line, got: %q", out)
	}

	// Must NOT show session name rows or the divider.
	if strings.Contains(out, "arch-1") {
		t.Errorf("compact: expected session name 'arch-1' to be hidden, got: %q", out)
	}
	if strings.Contains(out, "─") {
		t.Errorf("compact: expected divider to be absent, got: %q", out)
	}
}

// TestRenderSessionsCountsFromAllEvents verifies that role counts reflect all
// events even when maxVisible caps the display rows.
func TestRenderSessionsCountsFromAllEvents(t *testing.T) {
	events := []sessionEvent{
		{name: "arch-1", role: "architect", startedAt: time.Now().Add(-5 * time.Minute)},
		{name: "arch-2", role: "architect", startedAt: time.Now().Add(-4 * time.Minute)},
		{name: "impl-1", role: "implementer", startedAt: time.Now().Add(-3 * time.Minute)},
	}
	// maxVisible=1 means only 1 row shown, but counts must still be 2 arch + 1 impl.
	out := renderSessions(events, 80, 1)
	if !strings.Contains(out, "2") {
		t.Errorf("expected architect count '2' in output even with maxVisible=1, got: %q", out)
	}
}

// TestRenderTokenBurnCompact verifies that compact mode shows only the total
// line and omits the arch/impl breakdown.
func TestRenderTokenBurnCompact(t *testing.T) {
	d := tokenBurnData{total: 1_840_000, arch: 1_210_000, impl: 630_000}

	full := renderTokenBurn(d, 40, false)
	compact := renderTokenBurn(d, 40, true)

	if !strings.Contains(full, "arch") {
		t.Errorf("full mode: expected 'arch' line, got: %q", full)
	}
	if strings.Contains(compact, "arch") {
		t.Errorf("compact mode: expected 'arch' line to be absent, got: %q", compact)
	}
	if !strings.Contains(compact, "total") {
		t.Errorf("compact mode: expected 'total' line, got: %q", compact)
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

	outNew := renderSessions([]sessionEvent{newEvent}, 80, 10)
	outOld := renderSessions([]sessionEvent{oldEvent}, 80, 10)

	if outNew == outOld {
		t.Errorf("expected different output for new vs old session, but both rendered identically")
	}
}
