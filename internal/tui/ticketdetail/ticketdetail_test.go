package ticketdetail

import (
	"strings"
	"testing"
	"time"

	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/testutil"
)

// makeTicket returns a minimal ticket suitable for testing.
func makeTicket(title string) *models.Ticket {
	return &models.Ticket{
		ID:          1,
		Title:       title,
		Description: "A test description",
		Status:      models.StatusTodo,
	}
}

// makeLogs returns n log entries with distinct bodies to ensure rendered content
// is long enough to exceed the visible area in an 80x24 model.
func makeLogs(n int) []models.LogEntry {
	entries := make([]models.LogEntry, n)
	for i := range entries {
		entries[i] = models.LogEntry{
			ID:        int64(i + 1),
			TicketID:  1,
			SessionID: "test-session",
			Kind:      "message",
			Body:      strings.Repeat("log entry content line ", 5) + time.Now().String(),
			CreatedAt: time.Now(),
		}
	}
	return entries
}

// TestSetTicket_ResetsOffset verifies that calling SetTicket resets the scroll
// offset to 0 even when the user has previously scrolled.
func TestSetTicket_ResetsOffset(t *testing.T) {
	m := New(80, 24, true)
	m = m.SetTicket(makeTicket("First Ticket"), 1)
	// Populate with enough log lines to have scrollable content.
	m = m.SetDetail(makeLogs(50), nil, 1)

	// Scroll down several times.
	for i := 0; i < 5; i++ {
		m, _ = m.Update(testutil.KeyMsg('j'))
	}
	if m.offset == 0 {
		t.Skip("offset did not advance — content may not be tall enough; skipping offset-reset check")
	}

	// Replace the ticket — offset must reset to 0.
	m = m.SetTicket(makeTicket("Second Ticket"), 2)
	if m.offset != 0 {
		t.Errorf("SetTicket did not reset offset: got %d, want 0", m.offset)
	}
}

// TestUpdate_ScrollJ verifies that pressing j increments the offset when the
// model is focused and has scrollable content.
func TestUpdate_ScrollJ(t *testing.T) {
	m := New(80, 24, true)
	m = m.SetTicket(makeTicket("Scroll Test"), 1)
	m = m.SetDetail(makeLogs(50), nil, 1)

	before := m.offset
	m, _ = m.Update(testutil.KeyMsg('j'))
	if m.offset != before+1 {
		t.Errorf("j did not increment offset: before=%d after=%d", before, m.offset)
	}
}

// TestUpdate_ScrollK_Clamps verifies that pressing k when offset is already 0
// keeps the offset at 0 (clamps, does not go negative).
func TestUpdate_ScrollK_Clamps(t *testing.T) {
	m := New(80, 24, true)
	m = m.SetTicket(makeTicket("Clamp Test"), 1)
	m = m.SetDetail(makeLogs(5), nil, 1)

	if m.offset != 0 {
		t.Fatalf("expected initial offset 0, got %d", m.offset)
	}

	m, _ = m.Update(testutil.KeyMsg('k'))
	if m.offset != 0 {
		t.Errorf("k below 0 should clamp to 0, got %d", m.offset)
	}
}

// TestView_NoTicket verifies that View() on a freshly constructed model
// (no ticket selected) contains a placeholder string.
func TestView_NoTicket(t *testing.T) {
	m := New(80, 24, false)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Select a ticket") {
		t.Errorf("expected placeholder text in view, got: %q", view)
	}
}

// TestView_WithTicket verifies that after SetTicket the rendered view contains
// the ticket title.
func TestView_WithTicket(t *testing.T) {
	m := New(80, 24, false)
	m = m.SetTicket(makeTicket("My Ticket"), 1)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "My Ticket") {
		t.Errorf("expected ticket title in view, got: %q", view)
	}
}

// TestEpochGuard_Discard verifies that a DetailLoadedMsg with an epoch that
// does not match the model's epoch is discarded — logs are not updated.
func TestEpochGuard_Discard(t *testing.T) {
	m := New(80, 24, false)
	m = m.SetTicket(makeTicket("Epoch Test"), 1) // model epoch = 1

	someData := makeLogs(3)
	// Send a message tagged with epoch 2 — should be discarded.
	m, _ = m.Update(DetailLoadedMsg{Epoch: 2, Logs: someData})

	if len(m.logs) != 0 {
		t.Errorf("stale DetailLoadedMsg was applied: got %d logs, want 0", len(m.logs))
	}
}

// TestUpdate_UnfocusedIgnoresScroll verifies that j/k key presses are ignored
// when the model is not focused.
func TestUpdate_UnfocusedIgnoresScroll(t *testing.T) {
	m := New(80, 24, false) // not focused
	m = m.SetTicket(makeTicket("Unfocused Test"), 1)
	m = m.SetDetail(makeLogs(50), nil, 1)

	m, _ = m.Update(testutil.KeyMsg('j'))
	if m.offset != 0 {
		t.Errorf("unfocused model should ignore j; got offset %d, want 0", m.offset)
	}
}
