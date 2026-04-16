package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/kanban"
	"github.com/zalshy/tkt/internal/tui/modal"
	"github.com/zalshy/tkt/internal/tui/testutil"
	"github.com/zalshy/tkt/internal/tui/toast"
)

// TestRootModel_WindowSize verifies that a WindowSizeMsg updates width and height.
func TestRootModel_WindowSize(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("Update returned %T, want RootModel", m2)
	}
	if root.width != 120 {
		t.Errorf("width: want 120, got %d", root.width)
	}
	if root.height != 40 {
		t.Errorf("height: want 40, got %d", root.height)
	}
}

// TestRootModel_SizeGuard_Triggers verifies that a terminal smaller than 60×20
// renders the size-guard error message.
func TestRootModel_SizeGuard_Triggers(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m.width = 59
	m.height = 20
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "too small") && !strings.Contains(view, "Terminal") {
		t.Errorf("size guard did not trigger: got %q", view)
	}
}

// TestRootModel_SizeGuard_ExactThreshold verifies that a terminal of exactly
// 60×20 renders normal output (not the size-guard error).
func TestRootModel_SizeGuard_ExactThreshold(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m.width = 60
	m.height = 20
	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "too small") {
		t.Errorf("size guard triggered at 60×20 (exact threshold), should not: got %q", view)
	}
}

// TestBoard_ColumnSwitch verifies that left/right keys move the active column.
func TestBoard_ColumnSwitch(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("WindowSizeMsg: Update returned %T, want RootModel", m2)
	}

	// Initial active column is 0 (TODO).
	if got := root.board.ActiveCol(); got != 0 {
		t.Fatalf("initial active column: want 0, got %d", got)
	}

	// Right arrow moves to column 1.
	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	m3, _ := testutil.Update(root, rightMsg)
	root2, ok := m3.(RootModel)
	if !ok {
		t.Fatalf("after right: Update returned %T, want RootModel", m3)
	}
	if got := root2.board.ActiveCol(); got != 1 {
		t.Errorf("after right: want activeCol==1, got %d", got)
	}

	// Left arrow moves back to column 0.
	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	m4, _ := testutil.Update(root2, leftMsg)
	root3, ok := m4.(RootModel)
	if !ok {
		t.Fatalf("after left: Update returned %T, want RootModel", m4)
	}
	if got := root3.board.ActiveCol(); got != 0 {
		t.Errorf("after left: want activeCol==0, got %d", got)
	}
}

// TestRootModel_WindowSizeUpdatesBoard verifies that a WindowSizeMsg propagates
// to the board component and the board produces non-empty output.
func TestRootModel_WindowSizeUpdatesBoard(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("Update returned %T, want RootModel", m2)
	}

	view := testutil.StripANSI(root.board.View())
	if view == "" {
		t.Error("board View() returned empty after WindowSizeMsg")
	}
}

// TestRootModel_SearchOpenClose verifies that '/' opens the search overlay and
// Esc closes it.
func TestRootModel_SearchOpenClose(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("WindowSizeMsg: Update returned %T, want RootModel", m2)
	}

	// Send '/' to open search.
	m3, _ := testutil.Update(root, testutil.KeyMsg('/'))
	root2, ok := m3.(RootModel)
	if !ok {
		t.Fatalf("after '/': Update returned %T, want RootModel", m3)
	}
	if !root2.search.IsActive() {
		t.Error("after '/': search should be active, got inactive")
	}

	// Send Esc to close search.
	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	m4, _ := testutil.Update(root2, escMsg)
	root3, ok := m4.(RootModel)
	if !ok {
		t.Fatalf("after Esc: Update returned %T, want RootModel", m4)
	}
	if root3.search.IsActive() {
		t.Error("after Esc: search should be inactive, got active")
	}
}

// TestRootModel_EnterFiresDetailLoad verifies that pressing Enter when a ticket
// is selected in the active board column causes a non-nil cmd to be returned.
// The cmd is NOT invoked — it would panic with a nil db.
func TestRootModel_EnterFiresDetailLoad(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("WindowSizeMsg: Update returned %T, want RootModel", m2)
	}

	// Inject a ticket into the board via BoardLoadedMsg so the first column has a selection.
	m3, _ := testutil.Update(root, kanban.BoardLoadedMsg{
		Tickets: []models.Ticket{
			{ID: 5, Title: "T", Status: models.StatusTodo},
		},
		Epoch: 0,
	})
	root2, ok := m3.(RootModel)
	if !ok {
		t.Fatalf("BoardLoadedMsg: Update returned %T, want RootModel", m3)
	}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmds := testutil.Update(root2, enterMsg)
	if len(cmds) == 0 {
		t.Error("after Enter on selected ticket: expected at least one cmd (DetailLoadCmd), got none")
	}
}

// TestRootModel_HelpModalOpenClose verifies that '?' opens the help modal and
// Esc closes it.
func TestRootModel_HelpModalOpenClose(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("WindowSizeMsg: Update returned %T, want RootModel", m2)
	}

	// Send '?' to open the help modal.
	m3, _ := testutil.Update(root, testutil.KeyMsg('?'))
	root2, ok := m3.(RootModel)
	if !ok {
		t.Fatalf("after '?': Update returned %T, want RootModel", m3)
	}
	if !root2.modals.HasActive() {
		t.Error("after '?': expected modal active, got none")
	}
	kind, _ := root2.modals.Active()
	if kind != modal.KindHelp {
		t.Errorf("after '?': expected KindHelp, got %v", kind)
	}

	// Send Esc to close the modal.
	m4, _ := testutil.Update(root2, testutil.EscMsg())
	root3, ok := m4.(RootModel)
	if !ok {
		t.Fatalf("after Esc: Update returned %T, want RootModel", m4)
	}
	if root3.modals.HasActive() {
		t.Error("after Esc: expected no active modal, but one is still active")
	}
}

// TestRootModel_HelpModalEscPriority verifies that when the help modal is open,
// Esc dismisses the modal rather than closing the search overlay.
func TestRootModel_HelpModalEscPriority(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("WindowSizeMsg: Update returned %T, want RootModel", m2)
	}

	// Search must not be active before or after.
	if root.search.IsActive() {
		t.Fatal("search should not be active on initial model")
	}

	// Open the help modal.
	m3, _ := testutil.Update(root, testutil.KeyMsg('?'))
	root2, ok := m3.(RootModel)
	if !ok {
		t.Fatalf("after '?': Update returned %T, want RootModel", m3)
	}
	if root2.search.IsActive() {
		t.Error("search became active after '?' — unexpected")
	}

	// Esc must dismiss the modal without touching search.
	m4, _ := testutil.Update(root2, testutil.EscMsg())
	root3, ok := m4.(RootModel)
	if !ok {
		t.Fatalf("after Esc: Update returned %T, want RootModel", m4)
	}
	if root3.modals.HasActive() {
		t.Error("after Esc: modal should be dismissed")
	}
	if root3.search.IsActive() {
		t.Error("after Esc: search became active — Esc hit wrong branch")
	}
}

// TestRootModel_ToastExpiredDismisses verifies that ToastExpiredMsg causes an
// active toast modal to be dismissed.
func TestRootModel_ToastExpiredDismisses(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("WindowSizeMsg: Update returned %T, want RootModel", m2)
	}

	// Manually show a toast modal.
	root.modals = root.modals.Show(modal.KindToast, "test toast", 120)
	if !root.modals.HasActive() {
		t.Fatal("expected modal to be active after Show, got none")
	}

	// Inject ToastExpiredMsg directly — never call toast.ExpireCmd() in tests.
	m3, _ := testutil.Update(root, toast.ToastExpiredMsg{})
	root2, ok := m3.(RootModel)
	if !ok {
		t.Fatalf("after ToastExpiredMsg: Update returned %T, want RootModel", m3)
	}
	if root2.modals.HasActive() {
		t.Error("after ToastExpiredMsg: expected no active modal, but one remains")
	}
}

// TestRootModel_NavJK_ForwardedToBoard verifies that 'j' and 'k' move the board
// cursor when search is inactive and no modal is open.
func TestRootModel_NavJK_ForwardedToBoard(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("WindowSizeMsg: Update returned %T, want RootModel", m2)
	}

	// Inject two tickets into the TODO column.
	m3, _ := testutil.Update(root, kanban.BoardLoadedMsg{
		Tickets: []models.Ticket{
			{ID: 1, Title: "First", Status: models.StatusTodo},
			{ID: 2, Title: "Second", Status: models.StatusTodo},
		},
		Epoch: 0,
	})
	root2, ok := m3.(RootModel)
	if !ok {
		t.Fatalf("BoardLoadedMsg: Update returned %T, want RootModel", m3)
	}

	// Initial selected ticket should be ID 1.
	if got := root2.board.SelectedTicket(); got == nil || got.ID != 1 {
		t.Fatalf("initial cursor: want ticket ID 1, got %v", got)
	}

	// 'j' should move cursor down.
	m4, _ := testutil.Update(root2, testutil.KeyMsg('j'))
	root3, ok := m4.(RootModel)
	if !ok {
		t.Fatalf("after 'j': Update returned %T, want RootModel", m4)
	}
	if got := root3.board.SelectedTicket(); got == nil || got.ID != 2 {
		t.Errorf("after 'j': want ticket ID 2, got %v", got)
	}

	// 'k' should move cursor back up.
	m5, _ := testutil.Update(root3, testutil.KeyMsg('k'))
	root4, ok := m5.(RootModel)
	if !ok {
		t.Fatalf("after 'k': Update returned %T, want RootModel", m5)
	}
	if got := root4.board.SelectedTicket(); got == nil || got.ID != 1 {
		t.Errorf("after 'k': want ticket ID 1, got %v", got)
	}
}

// TestRootModel_NavJK_NotForwardedWhenSearchActive verifies that 'j' does NOT
// move the board cursor while the search overlay is open.
func TestRootModel_NavJK_NotForwardedWhenSearchActive(t *testing.T) {
	m := NewRootModel(nil, nil, "", nil)
	m2, _ := testutil.Update(m, testutil.WindowSize(120, 40))
	root, ok := m2.(RootModel)
	if !ok {
		t.Fatalf("WindowSizeMsg: Update returned %T, want RootModel", m2)
	}

	// Inject tickets into the TODO column.
	m3, _ := testutil.Update(root, kanban.BoardLoadedMsg{
		Tickets: []models.Ticket{
			{ID: 1, Title: "First", Status: models.StatusTodo},
			{ID: 2, Title: "Second", Status: models.StatusTodo},
		},
		Epoch: 0,
	})
	root2, ok := m3.(RootModel)
	if !ok {
		t.Fatalf("BoardLoadedMsg: Update returned %T, want RootModel", m3)
	}

	// Open search with '/'.
	m4, _ := testutil.Update(root2, testutil.KeyMsg('/'))
	root3, ok := m4.(RootModel)
	if !ok {
		t.Fatalf("after '/': Update returned %T, want RootModel", m4)
	}
	if !root3.search.IsActive() {
		t.Fatal("search should be active after '/'")
	}

	before := root3.board.SelectedTicket()

	// 'j' should go to search, not the board.
	m5, _ := testutil.Update(root3, testutil.KeyMsg('j'))
	root4, ok := m5.(RootModel)
	if !ok {
		t.Fatalf("after 'j': Update returned %T, want RootModel", m5)
	}
	after := root4.board.SelectedTicket()

	// If 'j' was routed to search (as a character to filter on), it would filter
	// the list. The selection may be nil (no match). Either way, the cursor
	// must not have been moved as a navigation key.
	if after != nil && before != nil && after.ID != before.ID {
		t.Errorf("cursor moved while search active: before ID=%d, after ID=%d — 'j' routed to board instead of search", before.ID, after.ID)
	}
}
