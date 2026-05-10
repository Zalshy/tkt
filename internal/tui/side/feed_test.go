package side

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestRenderFeedEmpty verifies that renderFeed with nil entries does not panic
// and produces output containing the "TICKET ACTIVITY" section header.
func TestRenderFeedEmpty(t *testing.T) {
	out := renderFeed(nil, 80, 10)
	if !strings.Contains(out, "TICKET ACTIVITY") {
		t.Errorf("expected 'TICKET ACTIVITY' in output, got: %q", out)
	}
}

// TestRenderFeedEntries verifies that an entry with a known session name and
// state appears in the rendered output.
func TestRenderFeedEntries(t *testing.T) {
	entries := []feedEntry{
		{
			ticketID:    42,
			sessionName: "test-session",
			toState:     "IN_PROGRESS",
			createdAt:   time.Now().Add(-30 * time.Second),
		},
	}
	out := renderFeed(entries, 80, 10)
	if !strings.Contains(out, "test-session") {
		t.Errorf("expected 'test-session' in output, got: %q", out)
	}
	if !strings.Contains(out, "IN_PROGRESS") {
		t.Errorf("expected 'IN_PROGRESS' in output, got: %q", out)
	}
}

// TestRenderFeedHighlight verifies that an entry with a recent arrivedAt
// produces different output than an entry with a zero arrivedAt.
// Forces TrueColor profile so lipgloss emits ANSI codes even outside a TTY.
func TestRenderFeedHighlight(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	base := feedEntry{
		ticketID:    7,
		sessionName: "arch-session",
		toState:     "DONE",
		createdAt:   time.Now().Add(-5 * time.Second),
	}

	// Entry with recent arrivedAt — should trigger highlight.
	newEntry := base
	newEntry.arrivedAt = time.Now()

	// Entry with zero arrivedAt — no highlight.
	oldEntry := base

	outNew := renderFeed([]feedEntry{newEntry}, 80, 10)
	outOld := renderFeed([]feedEntry{oldEntry}, 80, 10)

	if outNew == outOld {
		t.Errorf("expected different output for new vs old entry, but both rendered identically")
	}
}

// TestRelAge verifies the relative age formatting for all four buckets.
func TestRelAge(t *testing.T) {
	cases := []struct {
		secs int
		want string
	}{
		{30, "just now"},
		{90, "1m ago"},
		{3700, "1h ago"},
		{90000, "1d ago"},
	}

	for _, tc := range cases {
		t := t
		t.Run(tc.want, func(t *testing.T) {
			got := relAge(time.Now().Add(-time.Duration(tc.secs) * time.Second))
			if got != tc.want {
				t.Errorf("relAge(-%ds) = %q, want %q", tc.secs, got, tc.want)
			}
		})
	}
}
