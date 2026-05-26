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
	if !strings.Contains(out, "in_progress") {
		t.Errorf("expected 'in_progress' in output, got: %q", out)
	}
}

// TestRenderFeedHighlightFade verifies that an entry highlight lingers for
// about four seconds, fades in one-second steps, then returns to normal output.
// Forces TrueColor profile so lipgloss emits ANSI codes even outside a TTY.
func TestActivityHighlightPaletteStaysMutedPink(t *testing.T) {
	want := []string{"#B85F86", "#965270", "#74445B", "#553743"}
	if len(activityHighlightBackgrounds) != len(want) {
		t.Fatalf("activity highlight color count = %d, want %d", len(activityHighlightBackgrounds), len(want))
	}
	for i, wantColor := range want {
		if got := string(activityHighlightBackgrounds[i]); got != wantColor {
			t.Fatalf("activity highlight color[%d] = %s, want muted pink %s", i, got, wantColor)
		}
	}
}

func TestRenderFeedHighlightFade(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	now := time.Now()
	base := feedEntry{
		ticketID:    7,
		sessionName: "arch-session",
		toState:     "DONE",
		createdAt:   now.Add(-5 * time.Second),
	}

	oldEntry := base
	outOld := renderFeed([]feedEntry{oldEntry}, 80, 10)

	var previous string
	for _, age := range []time.Duration{
		500 * time.Millisecond,
		1500 * time.Millisecond,
		2500 * time.Millisecond,
		3500 * time.Millisecond,
	} {
		entry := base
		entry.arrivedAt = time.Now().Add(-age)
		out := renderFeed([]feedEntry{entry}, 80, 10)
		if out == outOld {
			t.Fatalf("expected feed highlight at age %s to differ from normal output", age)
		}
		if previous != "" && out == previous {
			t.Fatalf("expected feed highlight at age %s to fade to a different style", age)
		}
		previous = out
	}

	expiredEntry := base
	expiredEntry.arrivedAt = time.Now().Add(-4500 * time.Millisecond)
	outExpired := renderFeed([]feedEntry{expiredEntry}, 80, 10)
	if outExpired != outOld {
		t.Errorf("expected feed highlight after expiry to match normal output")
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
