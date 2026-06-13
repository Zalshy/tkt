package kanban

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/styles"
)

func stripANSIKanban(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

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

func TestCardAttentionHelpers(t *testing.T) {
	colorTests := []struct {
		level int
		want  lipgloss.Color
	}{
		{0, styles.Muted}, {1, styles.AttentionA}, {21, styles.AttentionB},
		{41, styles.AttentionC}, {61, styles.AttentionD}, {81, styles.AttentionE},
	}
	for _, tt := range colorTests {
		if got := attentionColor(tt.level); got != tt.want {
			t.Fatalf("attentionColor(%d) = %v, want %v", tt.level, got, tt.want)
		}
	}

	labelTests := []struct {
		level int
		tier  string
		want  string
	}{
		{0, "critical", "[critical]"}, {20, "", "[low]"}, {33, "", "[low]"},
		{66, "", "[standard]"}, {80, "", "[critical]"}, {81, "", "[critical]"},
	}
	for _, tt := range labelTests {
		got, _ := attentionTierLabel(tt.level, tt.tier)
		if got != tt.want {
			t.Fatalf("attentionTierLabel(%d, %q) = %q, want %q", tt.level, tt.tier, got, tt.want)
		}
	}

	if got := attentionDisplay(12); got != "👁 12" {
		t.Fatalf("attentionDisplay(12) = %q", got)
	}
	if got := attentionDisplay(0); got != "👁 --" {
		t.Fatalf("attentionDisplay(0) = %q", got)
	}
}

func TestStatusLabelCoversStatuses(t *testing.T) {
	statuses := map[models.Status]string{
		models.StatusInProgress: "IN PROGRESS",
		models.StatusVerified:   "VERIFIED",
		models.StatusDone:       "DONE",
		models.StatusTodo:       "TODO",
		models.StatusPlanning:   "PLANNING",
		models.StatusCanceled:   "CANCELED",
		models.StatusArchived:   "ARCHIVED",
		models.Status("bogus"):  "UNKNOWN",
	}
	for status, want := range statuses {
		got, _ := statusLabel(status)
		if got != want {
			t.Fatalf("statusLabel(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestRenderCardContent(t *testing.T) {
	ticket := models.Ticket{ID: 42, Title: "A very long title for truncation", Status: models.StatusInProgress, Tier: "critical", MainType: "feature", AttentionLevel: 85}
	result := stripANSIKanban(renderCard(ticket, 24, true, true, 2))
	for _, want := range []string{"#42", "[critical]", "feat…", "A very long title", "IN PROGRESS", "👁 85", "█"} {
		if !strings.Contains(result, want) {
			t.Fatalf("renderCard missing %q in:\n%s", want, result)
		}
	}
	if lines := strings.Count(result, "\n") + 1; lines != 4 {
		t.Fatalf("in-progress card line count = %d, want 4", lines)
	}
}

// TestColumn_ScrollDown verifies that scrollOffset increments when the cursor
// moves past the last visible normal card.
//
// Column height 11 → innerH = 11-4 = 7.
// Normal card height = 3. Two cards fit: card0=3, card1=3+1spacer=4, total=7.
// Moving cursor to index 2 must set scrollOffset=1.
func TestColumn_ScrollDown(t *testing.T) {
	// height=11 → innerH=7 → fits exactly 2 normal cards
	col := newColumn(models.StatusTodo, "TODO", 40, 11)
	col = col.SetFocus(true)
	col = col.SetTickets([]models.Ticket{
		{ID: 1, Title: "T1", Status: models.StatusTodo},
		{ID: 2, Title: "T2", Status: models.StatusTodo},
		{ID: 3, Title: "T3", Status: models.StatusTodo},
		{ID: 4, Title: "T4", Status: models.StatusTodo},
	})

	// cursor=0, scrollOffset should be 0
	if col.scrollOffset != 0 {
		t.Fatalf("initial scrollOffset = %d, want 0", col.scrollOffset)
	}

	// move to index 1 — still visible, no scroll
	col = col.CursorDown()
	if col.scrollOffset != 0 {
		t.Errorf("after CursorDown to 1: scrollOffset = %d, want 0", col.scrollOffset)
	}

	// move to index 2 — exceeds visible window, must scroll
	col = col.CursorDown()
	if col.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", col.cursor)
	}
	if col.scrollOffset != 1 {
		t.Errorf("after CursorDown to 2: scrollOffset = %d, want 1", col.scrollOffset)
	}

	// move to index 3 — must scroll again
	col = col.CursorDown()
	if col.scrollOffset != 2 {
		t.Errorf("after CursorDown to 3: scrollOffset = %d, want 2", col.scrollOffset)
	}
}

// TestColumn_ScrollUp verifies that scrollOffset decrements when the cursor
// moves back above the current window.
func TestColumn_ScrollUp(t *testing.T) {
	col := newColumn(models.StatusTodo, "TODO", 40, 11)
	col = col.SetFocus(true)
	col = col.SetTickets([]models.Ticket{
		{ID: 1, Title: "T1", Status: models.StatusTodo},
		{ID: 2, Title: "T2", Status: models.StatusTodo},
		{ID: 3, Title: "T3", Status: models.StatusTodo},
		{ID: 4, Title: "T4", Status: models.StatusTodo},
	})

	// scroll down to index 3
	col = col.CursorDown()
	col = col.CursorDown()
	col = col.CursorDown()
	if col.scrollOffset != 2 {
		t.Fatalf("pre-condition: scrollOffset = %d, want 2", col.scrollOffset)
	}

	// scroll back to index 2 — cursor == scrollOffset, neither condition fires
	col = col.CursorUp()
	if col.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", col.cursor)
	}
	if col.scrollOffset != 2 {
		t.Errorf("after CursorUp to 2: scrollOffset = %d, want 2", col.scrollOffset)
	}

	// scroll back to index 1 — cursor < scrollOffset, fires: scrollOffset = 1
	col = col.CursorUp()
	if col.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", col.cursor)
	}
	if col.scrollOffset != 1 {
		t.Errorf("after CursorUp to 1: scrollOffset = %d, want 1", col.scrollOffset)
	}

	// scroll back to index 0 — cursor < scrollOffset, fires: scrollOffset = 0
	col = col.CursorUp()
	if col.scrollOffset != 0 {
		t.Errorf("after CursorUp to 0: scrollOffset = %d, want 0", col.scrollOffset)
	}
}

// TestColumn_ScrollInProgress verifies scroll with IN_PROGRESS cards (height=4).
//
// Column height 13 → innerH = 13-4 = 9.
// IN_PROGRESS card height = 4. Two cards fit: card0=4, card1=4+1spacer=5, total=9.
// Moving cursor to index 2 must set scrollOffset=1.
func TestColumn_ScrollInProgress(t *testing.T) {
	col := newColumn(models.StatusPlanning, "PLANNING", 40, 13)
	col = col.SetFocus(true)
	col = col.SetTickets([]models.Ticket{
		{ID: 1, Title: "T1", Status: models.StatusInProgress},
		{ID: 2, Title: "T2", Status: models.StatusInProgress},
		{ID: 3, Title: "T3", Status: models.StatusInProgress},
	})

	// index 0 and 1 visible, no scroll yet
	col = col.CursorDown()
	if col.scrollOffset != 0 {
		t.Errorf("after CursorDown to 1: scrollOffset = %d, want 0", col.scrollOffset)
	}

	// index 2 exceeds window → must scroll
	col = col.CursorDown()
	if col.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", col.cursor)
	}
	if col.scrollOffset != 1 {
		t.Errorf("after CursorDown to 2: scrollOffset = %d, want 1", col.scrollOffset)
	}
}

// TestColumn_RestoreCursor verifies that RestoreCursor positions the cursor on
// the ticket with the given ID and adjusts scroll accordingly.
func TestColumn_RestoreCursor(t *testing.T) {
	// height=11 → innerH=7 → fits exactly 2 normal cards
	col := newColumn(models.StatusTodo, "TODO", 40, 11)
	col = col.SetFocus(true)
	tickets := []models.Ticket{
		{ID: 10, Title: "T1", Status: models.StatusTodo},
		{ID: 20, Title: "T2", Status: models.StatusTodo},
		{ID: 30, Title: "T3", Status: models.StatusTodo},
		{ID: 40, Title: "T4", Status: models.StatusTodo},
		{ID: 50, Title: "T5", Status: models.StatusTodo},
	}
	col = col.SetTickets(tickets)

	// RestoreCursor to index 3 (ID=40)
	col = col.RestoreCursor(40)
	if col.cursor != 3 {
		t.Fatalf("cursor = %d, want 3", col.cursor)
	}
	// scrollOffset must have adjusted: index 3 exceeds window of 2 from offset 0
	if col.scrollOffset == 0 {
		t.Errorf("scrollOffset = 0, expected > 0 after restoring cursor to index 3")
	}
	if got := col.SelectedTicket(); got == nil || got.ID != 40 {
		t.Fatalf("SelectedTicket = %v, want ID 40", got)
	}
}

// TestColumn_RestoreCursor_NotFound verifies that RestoreCursor returns c
// unchanged when the ID does not exist in the column.
func TestColumn_RestoreCursor_NotFound(t *testing.T) {
	col := newColumn(models.StatusTodo, "TODO", 40, 20)
	col = col.SetFocus(true)
	col = col.SetTickets([]models.Ticket{
		{ID: 1, Title: "T1", Status: models.StatusTodo},
		{ID: 2, Title: "T2", Status: models.StatusTodo},
	})
	col = col.CursorDown() // cursor at 1
	col = col.RestoreCursor(999)
	// cursor should be unchanged at 1
	if col.cursor != 1 {
		t.Fatalf("cursor = %d after RestoreCursor(999), want 1 (unchanged)", col.cursor)
	}
}

// TestBoard_SetTickets_PreservesCursor verifies that SetTickets preserves the
// cursor position in each column when the same tickets are reloaded.
func TestBoard_SetTickets_PreservesCursor(t *testing.T) {
	b := New(200, 40)
	tickets := []models.Ticket{
		{ID: 1, Title: "T1", Status: models.StatusTodo},
		{ID: 2, Title: "T2", Status: models.StatusTodo},
		{ID: 3, Title: "T3", Status: models.StatusTodo},
	}
	b = b.SetTickets(tickets)

	// Move cursor to index 2 in TODO column (activeCol=0)
	b.columns[0] = b.columns[0].CursorDown()
	b.columns[0] = b.columns[0].CursorDown()
	if got := b.columns[0].SelectedTicket(); got == nil || got.ID != 3 {
		t.Fatalf("pre-condition: cursor at ID %v, want 3", got)
	}

	// Reload same tickets
	b = b.SetTickets(tickets)

	// Cursor should still be at index 2 (ID=3)
	if got := b.columns[0].SelectedTicket(); got == nil || got.ID != 3 {
		t.Fatalf("after SetTickets: cursor at ID %v, want 3 (preserved)", got)
	}
	if b.columns[0].cursor != 2 {
		t.Errorf("cursor index = %d, want 2", b.columns[0].cursor)
	}
}

// TestBoard_SetTickets_RemovedTicket verifies that when the cursored ticket
// disappears from the next load, cursor resets to 0.
func TestBoard_SetTickets_RemovedTicket(t *testing.T) {
	b := New(200, 40)
	tickets := []models.Ticket{
		{ID: 1, Title: "T1", Status: models.StatusTodo},
		{ID: 2, Title: "T2", Status: models.StatusTodo},
		{ID: 3, Title: "T3", Status: models.StatusTodo},
	}
	b = b.SetTickets(tickets)

	// Move cursor to index 2 (ID=3)
	b.columns[0] = b.columns[0].CursorDown()
	b.columns[0] = b.columns[0].CursorDown()
	if got := b.columns[0].SelectedTicket(); got == nil || got.ID != 3 {
		t.Fatalf("pre-condition: cursor at ID %v, want 3", got)
	}

	// Reload without ID=3
	ticketsReduced := []models.Ticket{
		{ID: 1, Title: "T1", Status: models.StatusTodo},
		{ID: 2, Title: "T2", Status: models.StatusTodo},
	}
	b = b.SetTickets(ticketsReduced)

	// Cursor should reset to 0 (ID=1)
	if b.columns[0].cursor != 0 {
		t.Errorf("cursor = %d after removed ticket, want 0", b.columns[0].cursor)
	}
	if got := b.columns[0].SelectedTicket(); got == nil || got.ID != 1 {
		t.Fatalf("SelectedTicket = %v, want ID 1", got)
	}
}

func TestCardStringHelpers(t *testing.T) {
	if got := truncate("abcdef", 4); got != "abc…" {
		t.Fatalf("truncate = %q", got)
	}
	if got := truncate("abcdef", 1); got != "a" {
		t.Fatalf("truncate width 1 = %q", got)
	}
	if got := truncate("abcdef", 0); got != "" {
		t.Fatalf("truncate width 0 = %q", got)
	}
	if got := truncate("abc", 5); got != "abc" {
		t.Fatalf("truncate short = %q", got)
	}
	if got := padRight("ab", 4); got != "ab  " {
		t.Fatalf("padRight = %q", got)
	}
	if got := padRight("abcd", 2); got != "ab" {
		t.Fatalf("padRight clipped = %q", got)
	}
	if got := max(1, 2); got != 2 {
		t.Fatalf("max(1,2) = %d", got)
	}
}
