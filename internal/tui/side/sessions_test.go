package side

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/zalshy/tkt/internal/db"
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
		{name: "arch-1", role: "architect", startedAt: time.Now().Add(-6 * time.Minute)},
		{name: "arch-2", role: "architect", startedAt: time.Now().Add(-5 * time.Minute)},
		{name: "arch-3", role: "architect", startedAt: time.Now().Add(-4 * time.Minute)},
		{name: "impl-1", role: "implementer", startedAt: time.Now().Add(-3 * time.Minute)},
		{name: "impl-2", role: "implementer", startedAt: time.Now().Add(-2 * time.Minute)},
		{name: "impl-3", role: "implementer", startedAt: time.Now().Add(-1 * time.Minute)},
	}
	// maxVisible=1 means only 1 row shown, but counts must still be 3 arch + 3 impl.
	out := renderSessions(events, 80, 1)
	if !strings.Contains(out, "architect  3") {
		t.Errorf("expected architect count '3' in output even with maxVisible=1, got: %q", out)
	}
	if !strings.Contains(out, "implementer  3") {
		t.Errorf("expected implementer count '3' in output even with maxVisible=1, got: %q", out)
	}
	if strings.Contains(out, "arch-2") || strings.Contains(out, "impl-3") {
		t.Errorf("expected maxVisible=1 to hide rows after first event, got: %q", out)
	}
}

func TestLoadSessionsReturnsAllActiveNonMonitorSessions(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir .tkt: %v", err)
	}
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	for i := 0; i < 7; i++ {
		role := "implementer"
		if i%2 == 0 {
			role = "architect"
		}
		_, err := database.Exec(
			`INSERT INTO sessions (id, role, name, created_at, last_active)
			 VALUES (?, ?, ?, datetime('now', ?), datetime('now'))`,
			fmt.Sprintf("sess-%d", i), role, fmt.Sprintf("session-%d", i), fmt.Sprintf("-%d minutes", i),
		)
		if err != nil {
			t.Fatalf("insert session %d: %v", i, err)
		}
	}

	_, err = database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active)
		 VALUES ('monitor-sess', 'monitor', 'monitor-session', datetime('now'), datetime('now'))`,
	)
	if err != nil {
		t.Fatalf("insert monitor session: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active, expired_at)
		 VALUES ('expired-sess', 'implementer', 'expired-session', datetime('now'), datetime('now'), datetime('now'))`,
	)
	if err != nil {
		t.Fatalf("insert expired session: %v", err)
	}

	events, err := loadSessions(database)
	if err != nil {
		t.Fatalf("loadSessions: %v", err)
	}
	if len(events) != 7 {
		t.Fatalf("loadSessions returned %d events, want 7", len(events))
	}
	for _, e := range events {
		if e.role == "monitor" || e.name == "monitor-session" || e.name == "expired-session" {
			t.Fatalf("loadSessions returned excluded session: %+v", e)
		}
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

// TestRenderSessionsHighlightFade verifies that an event highlight lingers for
// about four seconds, fades in one-second steps, then returns to normal output.
// Forces TrueColor profile so lipgloss emits ANSI codes even outside a TTY.
func TestRenderSessionsHighlightFade(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	now := time.Now()
	base := sessionEvent{
		name:      "arch-session",
		role:      "architect",
		startedAt: now.Add(-2 * time.Minute),
	}

	oldEvent := base
	outOld := renderSessions([]sessionEvent{oldEvent}, 80, 10)

	var previous string
	for _, age := range []time.Duration{
		500 * time.Millisecond,
		1500 * time.Millisecond,
		2500 * time.Millisecond,
		3500 * time.Millisecond,
	} {
		event := base
		event.arrivedAt = time.Now().Add(-age)
		out := renderSessions([]sessionEvent{event}, 80, 10)
		if out == outOld {
			t.Fatalf("expected session highlight at age %s to differ from normal output", age)
		}
		if previous != "" && out == previous {
			t.Fatalf("expected session highlight at age %s to fade to a different style", age)
		}
		previous = out
	}

	expiredEvent := base
	expiredEvent.arrivedAt = time.Now().Add(-4500 * time.Millisecond)
	outExpired := renderSessions([]sessionEvent{expiredEvent}, 80, 10)
	if outExpired != outOld {
		t.Errorf("expected session highlight after expiry to match normal output")
	}
}
