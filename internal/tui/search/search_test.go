package search

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/testutil"
)

// TestNew verifies that a freshly constructed Model is inactive with an empty query.
func TestNew(t *testing.T) {
	m := New(80)
	if m.IsActive() {
		t.Error("expected IsActive() == false for new model")
	}
	if m.Query() != "" {
		t.Errorf("expected empty query, got %q", m.Query())
	}
}

// TestOpen verifies that Open() makes the model active with an empty query.
func TestOpen(t *testing.T) {
	m := New(80).Open()
	if !m.IsActive() {
		t.Error("expected IsActive() == true after Open()")
	}
	if m.Query() != "" {
		t.Errorf("expected empty query after Open(), got %q", m.Query())
	}
}

// TestOpenClearsQuery verifies that Open() clears any existing query.
func TestOpenClearsQuery(t *testing.T) {
	m := New(80).Open()
	m, _ = m.Update(testutil.KeyMsg('a'))
	m = m.Open()
	if m.Query() != "" {
		t.Errorf("expected query cleared after Open(), got %q", m.Query())
	}
}

// TestClose verifies that Close() makes the model inactive.
func TestClose(t *testing.T) {
	m := New(80).Open().Close()
	if m.IsActive() {
		t.Error("expected IsActive() == false after Close()")
	}
}

// TestRuneInput verifies that typing a rune appends it to the query.
func TestRuneInput(t *testing.T) {
	m := New(80).Open()
	m, _ = m.Update(testutil.KeyMsg('a'))
	if m.Query() != "a" {
		t.Errorf("expected query == \"a\", got %q", m.Query())
	}
}

// TestMultiRuneInput verifies that multiple runes accumulate in the query.
func TestMultiRuneInput(t *testing.T) {
	m := New(80).Open()
	for _, r := range []rune{'f', 'o', 'o'} {
		m, _ = m.Update(testutil.KeyMsg(r))
	}
	if m.Query() != "foo" {
		t.Errorf("expected query == \"foo\", got %q", m.Query())
	}
}

// TestBackspace verifies that backspace removes the last rune.
func TestBackspace(t *testing.T) {
	m := New(80).Open()
	m, _ = m.Update(testutil.KeyMsg('a'))
	m, _ = m.Update(testutil.KeyMsg('b'))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.Query() != "a" {
		t.Errorf("expected query == \"a\" after backspace, got %q", m.Query())
	}
}

// TestBackspaceMultibyte verifies that backspace is rune-safe for multi-byte characters.
func TestBackspaceMultibyte(t *testing.T) {
	m := New(80).Open()
	// Send 'é' (U+00E9, two UTF-8 bytes)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'é'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.Query() != "" {
		t.Errorf("expected empty query after backspace over 'é', got %q", m.Query())
	}
}

// TestBackspaceEmpty verifies that backspace on an empty query does not panic.
func TestBackspaceEmpty(t *testing.T) {
	m := New(80).Open()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.Query() != "" {
		t.Errorf("expected empty query, got %q", m.Query())
	}
}

// TestEscCloses verifies that pressing Escape while active closes the overlay.
func TestEscCloses(t *testing.T) {
	m := New(80).Open()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.IsActive() {
		t.Error("expected IsActive() == false after Escape")
	}
}

// TestUpdateInactiveNoop verifies that Update is a no-op when the model is inactive.
func TestUpdateInactiveNoop(t *testing.T) {
	m := New(80) // inactive
	m, _ = m.Update(testutil.KeyMsg('x'))
	if m.Query() != "" {
		t.Errorf("expected query unchanged when inactive, got %q", m.Query())
	}
}

// TestFilterEmptyQuery verifies that Filter with an empty query returns the input slice unchanged.
func TestFilterEmptyQuery(t *testing.T) {
	m := New(80)
	tickets := []models.Ticket{
		{ID: 1, Title: "fix bug"},
		{ID: 2, Title: "add feature"},
	}
	result := m.Filter(tickets)
	if len(result) != len(tickets) {
		t.Errorf("expected %d tickets, got %d", len(tickets), len(result))
	}
}

// TestFilterMatchingQuery verifies that a matching query returns the correct ticket.
func TestFilterMatchingQuery(t *testing.T) {
	m := New(80).Open()
	m, _ = m.Update(testutil.KeyMsg('b'))
	m, _ = m.Update(testutil.KeyMsg('u'))
	m, _ = m.Update(testutil.KeyMsg('g'))

	tickets := []models.Ticket{
		{ID: 1, Title: "fix bug"},
		{ID: 2, Title: "add feature"},
	}
	result := m.Filter(tickets)
	if len(result) == 0 {
		t.Fatal("expected at least one matching ticket, got none")
	}
	found := false
	for _, r := range result {
		if r.ID == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ticket #1 (fix bug) in results, got %+v", result)
	}
}

// TestFilterNoMatch verifies that a non-matching query returns a non-nil empty slice.
func TestFilterNoMatch(t *testing.T) {
	m := New(80).Open()
	m, _ = m.Update(testutil.KeyMsg('z'))
	m, _ = m.Update(testutil.KeyMsg('z'))
	m, _ = m.Update(testutil.KeyMsg('z'))
	m, _ = m.Update(testutil.KeyMsg('z'))

	tickets := []models.Ticket{
		{ID: 1, Title: "fix bug"},
		{ID: 2, Title: "add feature"},
	}
	result := m.Filter(tickets)
	if result == nil {
		t.Error("expected non-nil slice for no-match result")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

// TestViewInactive verifies that View() returns an empty string when inactive.
func TestViewInactive(t *testing.T) {
	m := New(80)
	if m.View() != "" {
		t.Errorf("expected empty string from View() when inactive, got %q", m.View())
	}
}

// TestViewActive verifies that View() contains the prompt prefix and cursor when active.
func TestViewActive(t *testing.T) {
	m := New(80).Open()
	view := testutil.StripANSI(m.View())
	if view == "" {
		t.Error("expected non-empty View() when active")
	}
	if len(view) < 2 {
		t.Fatalf("View() too short: %q", view)
	}
	// Should contain the "/ " prompt prefix
	found := false
	for i := 0; i < len(view)-1; i++ {
		if view[i] == '/' && view[i+1] == ' ' {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected \"/ \" in View(), got %q", view)
	}
	// Should contain the block cursor
	foundCursor := false
	for _, r := range view {
		if r == '█' {
			foundCursor = true
			break
		}
	}
	if !foundCursor {
		t.Errorf("expected '█' cursor in View(), got %q", view)
	}
}
