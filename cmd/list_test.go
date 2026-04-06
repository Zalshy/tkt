package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
)

// runListInDir sets rootDir to dir, resets flag vars, applies setupFlags, then calls runList.
// Returns captured stdout and any error.
func runListInDir(t *testing.T, dir string, setupFlags func()) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	savedStatus := listStatus
	savedLimit := listLimit
	savedAll := listAll
	savedVerified := listVerified
	savedSort := listSort
	savedReady := listReady
	defer func() {
		rootDir = savedRootDir
		listStatus = savedStatus
		listLimit = savedLimit
		listAll = savedAll
		listVerified = savedVerified
		listSort = savedSort
		listReady = savedReady
		listCmd.SetOut(nil)
	}()

	rootDir = dir
	listStatus = ""
	listLimit = 10
	listAll = false
	listVerified = false
	listSort = "updated"
	listReady = false

	if setupFlags != nil {
		setupFlags()
	}

	var buf bytes.Buffer
	listCmd.SetOut(&buf)

	err := runList(listCmd, nil)
	return buf.String(), err
}

// seedTickets inserts n tickets directly into the DB.
func seedTickets(t *testing.T, dir string, n int) {
	t.Helper()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("seedTickets: open db: %v", err)
	}
	defer database.Close()

	for i := 1; i <= n; i++ {
		if _, err := database.Exec(
			`INSERT INTO tickets (title, description, status, created_by)
			 VALUES (?, '', 'TODO', 'test-session')`,
			fmt.Sprintf("Ticket %d", i),
		); err != nil {
			t.Fatalf("seedTickets: insert %d: %v", i, err)
		}
	}
}

// TestList_Empty verifies that listing an empty project returns "(no tickets)".
func TestList_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, err := runListInDir(t, dir, nil)
	if err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(out, "(no tickets)") {
		t.Errorf("expected '(no tickets)', got: %q", out)
	}
}

// TestList_TwoTickets verifies that two inserted tickets both appear in output.
func TestList_TwoTickets(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedTickets(t, dir, 2)

	out, err := runListInDir(t, dir, nil)
	if err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(out, "Ticket 1") {
		t.Errorf("expected 'Ticket 1' in output, got: %q", out)
	}
	if !strings.Contains(out, "Ticket 2") {
		t.Errorf("expected 'Ticket 2' in output, got: %q", out)
	}
}

// TestList_HasMore verifies that inserting 11 tickets with default limit 10
// produces the hint line.
func TestList_HasMore(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedTickets(t, dir, 11)

	out, err := runListInDir(t, dir, nil)
	if err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(out, "more tickets") {
		t.Errorf("expected 'more tickets' hint in output, got: %q", out)
	}
}

// TestList_StatusFilter verifies that --status TODO with matching tickets shows
// the status footer hint "N tickets in TODO.".
func TestList_StatusFilter(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedTickets(t, dir, 3)

	out, err := runListInDir(t, dir, func() { listStatus = "TODO" })
	if err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(out, "3 tickets in TODO.") {
		t.Errorf("expected '3 tickets in TODO.' in output, got: %q", out)
	}
}

// TestList_InvalidStatus verifies that an invalid --status value returns an error.
func TestList_InvalidStatus(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runListInDir(t, dir, func() { listStatus = "BOGUS" })
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --status") {
		t.Errorf("expected 'invalid --status' in error, got: %v", err)
	}
}

// TestList_InvalidSort verifies that an invalid --sort value returns an error.
func TestList_InvalidSort(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runListInDir(t, dir, func() { listSort = "name" })
	if err == nil {
		t.Fatal("expected error for invalid sort, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --sort") {
		t.Errorf("expected 'invalid --sort' in error, got: %v", err)
	}
}

// insertDependency inserts a row into ticket_dependencies making ticketID depend on dependsOnID.
func insertDependency(t *testing.T, dir string, ticketID, dependsOnID int64) {
	t.Helper()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("insertDependency: open db: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec(
		`INSERT INTO ticket_dependencies (ticket_id, depends_on) VALUES (?, ?)`,
		ticketID, dependsOnID,
	); err != nil {
		t.Fatalf("insertDependency: insert: %v", err)
	}
}

// TestListReady_FiltersBlockedTickets verifies that --ready hides tickets with unresolved deps.
func TestListReady_FiltersBlockedTickets(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedTickets(t, dir, 2) // Ticket 1 (id=1), Ticket 2 (id=2)

	// Make Ticket 2 depend on Ticket 1 (which is still TODO — not VERIFIED).
	insertDependency(t, dir, 2, 1)

	out, err := runListInDir(t, dir, func() { listReady = true })
	if err != nil {
		t.Fatalf("runList --ready: %v", err)
	}
	if !strings.Contains(out, "Ticket 1") {
		t.Errorf("expected 'Ticket 1' (unblocked) in output, got: %q", out)
	}
	if strings.Contains(out, "Ticket 2") {
		t.Errorf("expected 'Ticket 2' (blocked) to be absent from output, got: %q", out)
	}
}

// TestListReady_EmptyWhenAllBlocked verifies that --ready returns empty list when all tickets are blocked.
func TestListReady_EmptyWhenAllBlocked(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedTickets(t, dir, 2) // Ticket 1 (id=1), Ticket 2 (id=2)

	// Each ticket depends on the other — both blocked.
	insertDependency(t, dir, 1, 2)
	insertDependency(t, dir, 2, 1)

	out, err := runListInDir(t, dir, func() { listReady = true })
	if err != nil {
		t.Fatalf("runList --ready (all blocked): %v", err)
	}
	if strings.Contains(out, "Ticket 1") || strings.Contains(out, "Ticket 2") {
		t.Errorf("expected no tickets in output when all blocked, got: %q", out)
	}
}
