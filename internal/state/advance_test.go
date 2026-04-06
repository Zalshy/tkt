package state_test

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	ilog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/state"
)

// mustOpenDB creates a temp directory with a .tkt subdirectory, opens a fresh
// SQLite database with schema applied, and registers cleanup.
func mustOpenDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	tktDir := filepath.Join(dir, ".tkt")
	if err := os.MkdirAll(tktDir, 0o755); err != nil {
		t.Fatalf("mkdir .tkt: %v", err)
	}
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// insertSession inserts a session row and returns a *models.Session.
func insertSession(t *testing.T, database *sql.DB, id string, role models.Role) *models.Session {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO sessions (id, role, name) VALUES (?, ?, ?)`,
		id, string(role), id,
	)
	if err != nil {
		t.Fatalf("insert session %q: %v", id, err)
	}
	return &models.Session{ID: id, Role: role, EffectiveRole: role}
}

// insertTicket inserts a ticket row with the given status and returns its string ID.
func insertTicket(t *testing.T, database *sql.DB, status models.Status, createdBy string) string {
	t.Helper()
	result, err := database.Exec(
		`INSERT INTO tickets (title, description, status, created_by) VALUES ('test', '', ?, ?)`,
		string(status), createdBy,
	)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return fmt.Sprintf("%d", id)
}

// ticketStatus queries the current status of a ticket directly from the DB.
func ticketStatus(t *testing.T, database *sql.DB, id string) models.Status {
	t.Helper()
	var s string
	if err := database.QueryRow(`SELECT status FROM tickets WHERE id = ?`, id).Scan(&s); err != nil {
		t.Fatalf("query ticket status: %v", err)
	}
	return models.Status(s)
}

// logEntryCount returns the count of ticket_log rows for the given ticket ID.
func logEntryCount(t *testing.T, database *sql.DB, id string) int {
	t.Helper()
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM ticket_log WHERE ticket_id = ?`, id).Scan(&n); err != nil {
		t.Fatalf("count log entries: %v", err)
	}
	return n
}

// TestExecute_HappyPath advances a TODO ticket to PLANNING and verifies the DB
// reflects the transition and that exactly one log entry exists with correct fields.
func TestExecute_HappyPath(t *testing.T) {
	database := mustOpenDB(t)
	actor := insertSession(t, database, "impl-alice", models.RoleImplementer)
	id := insertTicket(t, database, models.StatusTodo, actor.ID)

	err := state.Execute(id, models.StatusPlanning, "picking up", actor, database, false)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	// Status must be PLANNING.
	if got := ticketStatus(t, database, id); got != models.StatusPlanning {
		t.Errorf("ticket status = %q, want %q", got, models.StatusPlanning)
	}

	// Exactly one log entry.
	entries, err := ilog.GetAll(id, database)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("log entry count = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Kind != "transition" {
		t.Errorf("Kind = %q, want %q", e.Kind, "transition")
	}
	if e.SessionID != actor.ID {
		t.Errorf("SessionID = %q, want %q", e.SessionID, actor.ID)
	}
	if e.Body != "picking up" {
		t.Errorf("Body = %q, want %q", e.Body, "picking up")
	}
	if e.FromState == nil || *e.FromState != models.StatusTodo {
		t.Errorf("FromState = %v, want %q", e.FromState, models.StatusTodo)
	}
	if e.ToState == nil || *e.ToState != models.StatusPlanning {
		t.Errorf("ToState = %v, want %q", e.ToState, models.StatusPlanning)
	}
}

// TestExecute_Atomicity installs a BEFORE INSERT trigger on ticket_log that raises an
// error, then verifies that after Execute fails, the ticket status is unchanged (rollback).
func TestExecute_Atomicity(t *testing.T) {
	database := mustOpenDB(t)
	actor := insertSession(t, database, "impl-bob", models.RoleImplementer)
	id := insertTicket(t, database, models.StatusTodo, actor.ID)

	// Install a trigger that aborts any INSERT into ticket_log.
	_, err := database.Exec(`
		CREATE TRIGGER block_log_insert
		BEFORE INSERT ON ticket_log
		BEGIN
			SELECT RAISE(ABORT, 'blocked by test trigger');
		END`)
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	err = state.Execute(id, models.StatusPlanning, "should fail", actor, database, false)
	if err == nil {
		t.Fatal("Execute succeeded, want error from blocked log insert")
	}

	// Ticket status must be unchanged — the UPDATE must have been rolled back.
	if got := ticketStatus(t, database, id); got != models.StatusTodo {
		t.Errorf("ticket status = %q after rollback, want %q", got, models.StatusTodo)
	}
}

// TestExecute_PlanGuard verifies that PLANNING → IN_PROGRESS fails when no plan
// log entry exists, and that neither the ticket nor the log are modified.
func TestExecute_PlanGuard(t *testing.T) {
	database := mustOpenDB(t)
	impl := insertSession(t, database, "impl-carol", models.RoleImplementer)
	arch := insertSession(t, database, "arch-dave", models.RoleArchitect)
	id := insertTicket(t, database, models.StatusPlanning, impl.ID)

	err := state.Execute(id, models.StatusInProgress, "approve", arch, database, false)
	if err == nil {
		t.Fatal("Execute succeeded, want plan-required error")
	}
	if !strings.Contains(err.Error(), "plan required") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "plan required")
	}

	// Ticket still PLANNING.
	if got := ticketStatus(t, database, id); got != models.StatusPlanning {
		t.Errorf("ticket status = %q, want %q", got, models.StatusPlanning)
	}

	// No log entries.
	if n := logEntryCount(t, database, id); n != 0 {
		t.Errorf("log entry count = %d, want 0", n)
	}
}

// TestExecute_AutoNextState verifies that passing to="" causes NextState to pick the
// target automatically (TODO → PLANNING).
func TestExecute_AutoNextState(t *testing.T) {
	database := mustOpenDB(t)
	actor := insertSession(t, database, "impl-eve", models.RoleImplementer)
	id := insertTicket(t, database, models.StatusTodo, actor.ID)

	err := state.Execute(id, "", "auto-advance", actor, database, false)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	if got := ticketStatus(t, database, id); got != models.StatusPlanning {
		t.Errorf("ticket status = %q, want %q", got, models.StatusPlanning)
	}
}

// TestExecute_AutoNextState_CanceledFails verifies that auto-next on a CANCELED ticket
// returns an error (architect correction: NextState(CANCELED) is an error).
func TestExecute_AutoNextState_CanceledFails(t *testing.T) {
	database := mustOpenDB(t)
	actor := insertSession(t, database, "impl-frank", models.RoleImplementer)
	id := insertTicket(t, database, models.StatusCanceled, actor.ID)

	err := state.Execute(id, "", "should fail", actor, database, false)
	if err == nil {
		t.Fatal("Execute succeeded on CANCELED ticket with auto-next, want error")
	}

	// Status must remain CANCELED.
	if got := ticketStatus(t, database, id); got != models.StatusCanceled {
		t.Errorf("ticket status = %q, want %q", got, models.StatusCanceled)
	}
}

// TestExecute_ForceWarning verifies that a force=true transition with an isolation
// violation succeeds and emits a non-empty warning to stderr.
func TestExecute_ForceWarning(t *testing.T) {
	database := mustOpenDB(t)
	// Session A plays both implementer (submitter) and architect (approver) roles via
	// force=true. We need a session that is the architect so the role check passes, but
	// shares the same ID as the submitter to trigger the isolation warning.
	arch := insertSession(t, database, "arch-grace", models.RoleArchitect)

	// Insert ticket in DONE status, with arch-grace as creator (the "submitter").
	id := insertTicket(t, database, models.StatusDone, arch.ID)

	// Insert a transition log entry that records arch-grace as the last submitter.
	fromStr := string(models.StatusInProgress)
	toStr := string(models.StatusDone)
	if err := ilog.Append(mustParseID(t, id), "transition", "done", &fromStr, &toStr, arch, database); err != nil {
		t.Fatalf("seed log entry: %v", err)
	}

	// Redirect stderr to a pipe to capture the warning.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	execErr := state.Execute(id, models.StatusVerified, "force verify", arch, database, true)

	w.Close()
	os.Stderr = origStderr

	stderrBytes, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	if execErr != nil {
		t.Fatalf("Execute returned unexpected error: %v", execErr)
	}

	// Status must be VERIFIED.
	if got := ticketStatus(t, database, id); got != models.StatusVerified {
		t.Errorf("ticket status = %q, want %q", got, models.StatusVerified)
	}

	// Stderr must be non-empty (the ForceWarning was printed).
	if strings.TrimSpace(string(stderrBytes)) == "" {
		t.Error("expected non-empty warning on stderr, got empty")
	}
}

// TestExecute_SubmitterResolution verifies that Execute correctly resolves the
// submitter from the last transition log entry.
// - Advancing a DONE ticket to VERIFIED with the same session that submitted (session A)
//   must fail (isolation rule).
// - A different architect session (session B) must succeed.
func TestExecute_SubmitterResolution(t *testing.T) {
	database := mustOpenDB(t)
	impl := insertSession(t, database, "impl-henry", models.RoleImplementer)
	archA := insertSession(t, database, "arch-irene", models.RoleArchitect)
	archB := insertSession(t, database, "arch-jack", models.RoleArchitect)

	// Insert ticket in DONE status; impl-henry is the creator.
	id := insertTicket(t, database, models.StatusDone, impl.ID)

	// Seed a transition log entry recording arch-irene as the last submitter.
	// (Simulates: arch-irene advanced the ticket to DONE.)
	fromStr := string(models.StatusInProgress)
	toStr := string(models.StatusDone)
	if err := ilog.Append(mustParseID(t, id), "transition", "submitted", &fromStr, &toStr, archA, database); err != nil {
		t.Fatalf("seed log entry: %v", err)
	}

	// arch-irene trying to verify their own submission must fail.
	err := state.Execute(id, models.StatusVerified, "self verify", archA, database, false)
	if err == nil {
		t.Fatal("Execute succeeded for same-session verification, want isolation error")
	}

	// Status must still be DONE.
	if got := ticketStatus(t, database, id); got != models.StatusDone {
		t.Errorf("ticket status = %q after rejection, want %q", got, models.StatusDone)
	}

	// arch-jack (different session) must succeed.
	err = state.Execute(id, models.StatusVerified, "verified by different arch", archB, database, false)
	if err != nil {
		t.Fatalf("Execute with different architect returned unexpected error: %v", err)
	}

	if got := ticketStatus(t, database, id); got != models.StatusVerified {
		t.Errorf("ticket status = %q, want %q", got, models.StatusVerified)
	}
}

// mustParseID parses a string ticket ID into int64, failing the test on error.
func mustParseID(t *testing.T, id string) int64 {
	t.Helper()
	n, err := parseInt64(id)
	if err != nil {
		t.Fatalf("mustParseID(%q): %v", id, err)
	}
	return n
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
