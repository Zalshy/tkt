package kanban

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/models"
)

// TestBoard_SetTickets_distributes verifies that SetTickets correctly distributes
// tickets into the 3 visual columns:
//   - TODO        → TODO column (index 0)
//   - PLANNING    → PLANNING column (index 1)
//   - IN_PROGRESS → PLANNING column (index 1, rendered with IN PROGRESS tag)
//   - DONE        → DONE column (index 2)
//   - VERIFIED    → DONE column (index 2, rendered with VERIFIED tag)
func TestBoard_SetTickets_distributes(t *testing.T) {
	b := New(200, 40)
	tickets := []models.Ticket{
		{ID: 1, Title: "Todo 1", Status: models.StatusTodo},
		{ID: 2, Title: "Planning 1", Status: models.StatusPlanning},
		{ID: 3, Title: "In Progress 1", Status: models.StatusInProgress},
		{ID: 4, Title: "Done 1", Status: models.StatusDone},
		{ID: 5, Title: "Verified 1", Status: models.StatusVerified},
	}
	b = b.SetTickets(tickets)

	// TODO column (index 0) has 1 StatusTodo ticket.
	b.columns[0] = b.columns[0].SetFocus(true)
	todoTicket := b.columns[0].SelectedTicket()
	if todoTicket == nil {
		t.Fatal("TODO column: expected a selected ticket, got nil")
	}
	if todoTicket.Status != models.StatusTodo {
		t.Errorf("TODO column: ticket status = %q, want %q", todoTicket.Status, models.StatusTodo)
	}

	// PLANNING column (index 1) has 2 tickets: StatusPlanning + StatusInProgress.
	if got := len(b.columns[1].tickets); got != 2 {
		t.Fatalf("PLANNING column: expected 2 tickets (planning+in_progress), got %d", got)
	}
	hasPlanning, hasInProgress := false, false
	for _, tk := range b.columns[1].tickets {
		switch tk.Status {
		case models.StatusPlanning:
			hasPlanning = true
		case models.StatusInProgress:
			hasInProgress = true
		}
	}
	if !hasPlanning {
		t.Error("PLANNING column: missing StatusPlanning ticket")
	}
	if !hasInProgress {
		t.Error("PLANNING column: missing StatusInProgress ticket (should be bucketed here)")
	}

	// DONE column (index 2) has 2 tickets: StatusDone + StatusVerified.
	if got := len(b.columns[2].tickets); got != 2 {
		t.Fatalf("DONE column: expected 2 tickets (done+verified), got %d", got)
	}
	hasDone, hasVerified := false, false
	for _, tk := range b.columns[2].tickets {
		switch tk.Status {
		case models.StatusDone:
			hasDone = true
		case models.StatusVerified:
			hasVerified = true
		}
	}
	if !hasDone {
		t.Error("DONE column: missing StatusDone ticket")
	}
	if !hasVerified {
		t.Error("DONE column: missing StatusVerified ticket (should be bucketed here)")
	}
}

// TestColumn_CursorMovement verifies that CursorDown and CursorUp move the cursor.
func TestColumn_CursorMovement(t *testing.T) {
	col := newColumn(models.StatusTodo, "TODO", 40, 20)
	col = col.SetFocus(true)
	col = col.SetTickets([]models.Ticket{
		{ID: 1, Title: "First", Status: models.StatusTodo},
		{ID: 2, Title: "Second", Status: models.StatusTodo},
		{ID: 3, Title: "Third", Status: models.StatusTodo},
	})

	// Initial cursor at 0.
	if got := col.SelectedTicket(); got == nil || got.ID != 1 {
		t.Fatalf("initial cursor: want ID 1, got %v", got)
	}

	// CursorDown moves to index 1.
	col = col.CursorDown()
	if got := col.SelectedTicket(); got == nil || got.ID != 2 {
		t.Errorf("after CursorDown: want ID 2, got %v", got)
	}

	// CursorDown again moves to index 2.
	col = col.CursorDown()
	if got := col.SelectedTicket(); got == nil || got.ID != 3 {
		t.Errorf("after second CursorDown: want ID 3, got %v", got)
	}

	// CursorDown at last item stays at last item (clamped).
	col = col.CursorDown()
	if got := col.SelectedTicket(); got == nil || got.ID != 3 {
		t.Errorf("after CursorDown at last: want ID 3 (clamped), got %v", got)
	}

	// CursorUp moves to index 1.
	col = col.CursorUp()
	if got := col.SelectedTicket(); got == nil || got.ID != 2 {
		t.Errorf("after CursorUp: want ID 2, got %v", got)
	}

	// CursorUp again moves to index 0.
	col = col.CursorUp()
	if got := col.SelectedTicket(); got == nil || got.ID != 1 {
		t.Errorf("after second CursorUp: want ID 1, got %v", got)
	}

	// CursorUp at first item stays at first item (clamped).
	col = col.CursorUp()
	if got := col.SelectedTicket(); got == nil || got.ID != 1 {
		t.Errorf("after CursorUp at first: want ID 1 (clamped), got %v", got)
	}
}

// TestCard_TierColor verifies that tierColor returns correct colors for each tier.
func TestCard_TierColor(t *testing.T) {
	tests := []struct {
		tier string
		want lipgloss.Color
	}{
		{"critical", lipgloss.Color("#EF4444")},
		{"low", lipgloss.Color("#10B981")},
		{"standard", lipgloss.Color("#6366F1")},
		{"", lipgloss.Color("#6366F1")},      // unknown falls back to standard
		{"other", lipgloss.Color("#6366F1")}, // unknown falls back to standard
	}

	for _, tc := range tests {
		got := tierColor(tc.tier)
		if got != tc.want {
			t.Errorf("tierColor(%q) = %v, want %v", tc.tier, got, tc.want)
		}
	}
}

// TestBoard_ActiveCol_Navigation verifies that left/right keys navigate between
// the 3 visual columns (indices 0–2) and are clamped at the boundaries.
func TestBoard_ActiveCol_Navigation(t *testing.T) {
	b := New(200, 40)

	if got := b.ActiveCol(); got != 0 {
		t.Fatalf("initial active col: want 0, got %d", got)
	}

	right := tea.KeyMsg{Type: tea.KeyRight}
	left := tea.KeyMsg{Type: tea.KeyLeft}

	// Right from col 0 → 1.
	b, _ = b.Update(right)
	if got := b.ActiveCol(); got != 1 {
		t.Errorf("after right: want 1, got %d", got)
	}

	// Right again → 2 (last column).
	b, _ = b.Update(right)
	if got := b.ActiveCol(); got != 2 {
		t.Errorf("after second right: want 2, got %d", got)
	}

	// Right at last col stays at 2 (clamped).
	b, _ = b.Update(right)
	if got := b.ActiveCol(); got != 2 {
		t.Errorf("after right at col 2: want 2 (clamped), got %d", got)
	}

	// Left → 1.
	b, _ = b.Update(left)
	if got := b.ActiveCol(); got != 1 {
		t.Errorf("after left: want 1, got %d", got)
	}

	// Left → 0.
	b, _ = b.Update(left)
	if got := b.ActiveCol(); got != 0 {
		t.Errorf("after second left: want 0, got %d", got)
	}

	// Left at col 0 stays at 0 (clamped).
	b, _ = b.Update(left)
	if got := b.ActiveCol(); got != 0 {
		t.Errorf("after left at col 0: want 0 (clamped), got %d", got)
	}
}
