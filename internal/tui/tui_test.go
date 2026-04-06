package tui

import (
	"testing"

	"github.com/zalshy/tkt/internal/models"
)

// TestBucketTickets verifies that tickets of every status land in the correct column.
func TestBucketTickets(t *testing.T) {
	tickets := []models.Ticket{
		{ID: 1, Status: models.StatusTodo},
		{ID: 2, Status: models.StatusPlanning},
		{ID: 3, Status: models.StatusInProgress},
		{ID: 4, Status: models.StatusDone},
		{ID: 5, Status: models.StatusVerified},
		{ID: 6, Status: models.StatusCanceled},
	}

	cols := bucketTickets(tickets)

	cases := []struct {
		idx      int
		wantID   int64
		wantName string
	}{
		{0, 1, "TODO"},
		{1, 2, "PLANNING"},
		{2, 3, "IN_PROGRESS"},
		{3, 4, "DONE"},
		{4, 5, "VERIFIED"},
		{5, 6, "CANCELED"},
	}

	for _, c := range cases {
		if len(cols[c.idx]) != 1 {
			t.Errorf("column %d (%s): want 1 ticket, got %d", c.idx, c.wantName, len(cols[c.idx]))
			continue
		}
		if cols[c.idx][0].ID != c.wantID {
			t.Errorf("column %d (%s): want ticket ID %d, got %d", c.idx, c.wantName, c.wantID, cols[c.idx][0].ID)
		}
	}

	// CANCELED goes to index 5, not anywhere else.
	if len(cols[5]) != 1 || cols[5][0].ID != 6 {
		t.Error("CANCELED ticket did not land in column index 5")
	}
}

// TestClampCursor_EmptyColumn verifies that the cursor stays at row 0 when the
// current column is empty (no panic, valid output).
func TestClampCursor_EmptyColumn(t *testing.T) {
	var cols [6][]models.Ticket
	// Only column 1 (PLANNING) has tickets.
	cols[1] = []models.Ticket{{ID: 10}}

	// Cursor on column 0 (empty) with rowIdx 5 — should clamp to (0, 0).
	colIdx, rowIdx := clampCursor(0, 5, cols, false)
	if colIdx != 0 {
		t.Errorf("colIdx: want 0, got %d", colIdx)
	}
	if rowIdx != 0 {
		t.Errorf("rowIdx: want 0, got %d (empty column should return 0)", rowIdx)
	}
}

// TestClampCursor_ToggleCanceled verifies that a colIdx pointing at the CANCELED
// column (5) clamps to 4 when showCanceled is false.
func TestClampCursor_ToggleCanceled(t *testing.T) {
	var cols [6][]models.Ticket
	cols[4] = []models.Ticket{{ID: 20}} // VERIFIED
	cols[5] = []models.Ticket{{ID: 21}} // CANCELED

	// Cursor was at colIdx=5 (CANCELED), toggle off.
	colIdx, rowIdx := clampCursor(5, 0, cols, false)
	if colIdx != 4 {
		t.Errorf("colIdx: want 4 (last visible), got %d", colIdx)
	}
	if rowIdx != 0 {
		t.Errorf("rowIdx: want 0, got %d", rowIdx)
	}
}

// TestTruncateTitle_Short verifies that a title shorter than maxRunes is returned unchanged.
func TestTruncateTitle_Short(t *testing.T) {
	title := "Fix bug"
	got := truncateTitle(title, 20)
	if got != title {
		t.Errorf("want %q, got %q", title, got)
	}
}

// TestTruncateTitle_Long verifies that a title longer than maxRunes is truncated.
func TestTruncateTitle_Long(t *testing.T) {
	title := "This is a very long ticket title that should be truncated"
	got := truncateTitle(title, 10)
	runes := []rune(got)
	if len(runes) != 10 {
		t.Errorf("want 10 runes, got %d: %q", len(runes), got)
	}
	want := "This is a "
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

// TestTruncateTitle_Multibyte verifies that truncation counts runes, not bytes,
// so multibyte UTF-8 characters are handled correctly.
func TestTruncateTitle_Multibyte(t *testing.T) {
	// "日本語" = 3 runes but 9 bytes. maxRunes=2 should give "日本".
	title := "日本語テスト"
	got := truncateTitle(title, 2)
	runes := []rune(got)
	if len(runes) != 2 {
		t.Errorf("want 2 runes, got %d: %q", len(runes), got)
	}
	if got != "日本" {
		t.Errorf("want %q, got %q", "日本", got)
	}
}
