package side

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TestDefaultPollInterval verifies that a nil cfg produces a 3s poll interval.
func TestDefaultPollInterval(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	if m.pollInterval != 3*time.Second {
		t.Errorf("expected 3s poll interval, got %v", m.pollInterval)
	}
}

// TestMinSizeGuard verifies that View() renders a "too small" message when
// dimensions are below the 60×20 threshold.
func TestMinSizeGuard(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	// zero-value model has width=0, height=0 — guard must fire
	out := m.View()
	if !strings.Contains(out, "too small") && !strings.Contains(out, "0x0") {
		t.Errorf("expected min-size guard output, got: %q", out)
	}
}

// TestPlaceholderSections verifies that View() renders all three placeholder
// sections when terminal dimensions are sufficient.
func TestPlaceholderSections(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	// Set dimensions above the guard threshold via Update (value receiver — must
	// capture the returned model).
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(RootModel)

	out := m.View()
	for _, want := range []string{"loading", "TICKET ACTIVITY", "SESSIONS"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in View() output, got: %q", want, out)
		}
	}
}

func TestFeedLoadedPreservesArrivedAtAcrossReorder(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	olderCreated := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	newerCreated := olderCreated.Add(time.Minute)
	preservedArrival := time.Now().Add(-1500 * time.Millisecond)

	m.feedLoaded = true
	m.feed = []feedEntry{
		{ticketID: 10, sessionName: "arch", toState: "DONE", createdAt: olderCreated, arrivedAt: preservedArrival},
	}
	m.feedEpoch = 2

	updated, _ := m.Update(feedLoadedMsg{
		epoch: m.feedEpoch,
		entries: []feedEntry{
			{ticketID: 11, sessionName: "impl", toState: "IN_PROGRESS", createdAt: newerCreated},
			{ticketID: 10, sessionName: "arch", toState: "DONE", createdAt: olderCreated},
		},
	})
	m = updated.(RootModel)

	if len(m.feed) != 2 {
		t.Fatalf("feed entries = %d, want 2", len(m.feed))
	}
	if m.feed[1].arrivedAt != preservedArrival {
		t.Fatalf("reordered existing feed arrival = %v, want %v", m.feed[1].arrivedAt, preservedArrival)
	}
	if m.feed[0].arrivedAt.IsZero() {
		t.Fatalf("new feed entry arrivedAt was not marked")
	}
	if m.feed[0].arrivedAt == preservedArrival {
		t.Fatalf("new feed entry reused existing arrival")
	}
}

func TestSessionsLoadedPreservesArrivedAtAcrossReload(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	preservedArrival := time.Now().Add(-1500 * time.Millisecond)
	startedAt := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)

	m.sessLoaded = true
	m.sessionsData = []sessionEvent{
		{name: "arch-session", role: "architect", startedAt: startedAt, arrivedAt: preservedArrival},
	}
	m.sessEpoch = 2

	updated, _ := m.Update(sessionsLoadedMsg{
		epoch: m.sessEpoch,
		events: []sessionEvent{
			{name: "impl-session", role: "implementer", startedAt: startedAt.Add(time.Minute)},
			{name: "arch-session", role: "architect", startedAt: startedAt},
		},
	})
	m = updated.(RootModel)

	if len(m.sessionsData) != 2 {
		t.Fatalf("session entries = %d, want 2", len(m.sessionsData))
	}
	if m.sessionsData[1].arrivedAt != preservedArrival {
		t.Fatalf("existing session arrival = %v, want %v", m.sessionsData[1].arrivedAt, preservedArrival)
	}
	if m.sessionsData[0].arrivedAt.IsZero() {
		t.Fatalf("new session arrivedAt was not marked")
	}
	if m.sessionsData[0].arrivedAt == preservedArrival {
		t.Fatalf("new session reused existing arrival")
	}
}

func TestView_FitsTerminalWithHighlightedSession(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 101, Height: 28})
	m = updated.(RootModel)

	m.sessionsData = []sessionEvent{
		{name: "grot-cf5e", role: "architect", startedAt: time.Now(), arrivedAt: time.Now()},
		{name: "sedge-bf2c", role: "implementer", startedAt: time.Now()},
	}

	out := m.View()
	if got := lipgloss.Height(out); got > m.height {
		t.Fatalf("View height = %d, want <= %d", got, m.height)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("line %d width = %d, want <= %d: %q", i, got, m.width, line)
		}
	}
}

// TestQuitOnQ verifies that pressing "q" returns tea.Quit.
func TestQuitOnQ(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(RootModel)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected a non-nil cmd for 'q'")
	}
	// tea.Quit returns a tea.QuitMsg when invoked
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestQuitOnCtrlC verifies that pressing ctrl+c returns tea.Quit.
func TestQuitOnCtrlC(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(RootModel)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected a non-nil cmd for ctrl+c")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}
