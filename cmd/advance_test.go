package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
)

// runAdvanceInDir sets rootDir to dir, resets flag vars, applies setupFlags, then calls runAdvance.
// Returns captured stdout and any error.
func runAdvanceInDir(t *testing.T, dir string, args []string, setupFlags func()) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	savedNote := advanceNote
	savedTo := advanceTo
	savedForce := advanceForce
	defer func() {
		rootDir = savedRootDir
		advanceNote = savedNote
		advanceTo = savedTo
		advanceForce = savedForce
		advanceCmd.SetOut(nil)
		advanceCmd.SilenceErrors = false
	}()

	rootDir = dir
	advanceNote = ""
	advanceTo = ""
	advanceForce = false

	if setupFlags != nil {
		setupFlags()
	}

	var buf bytes.Buffer
	advanceCmd.SetOut(&buf)

	err := runAdvance(advanceCmd, args)
	return buf.String(), err
}

// seedSessionWithRole inserts a session row with the given role and writes the session file.
func seedSessionWithRole(t *testing.T, dir string, id string, role string) {
	t.Helper()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("seedSessionWithRole: open db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active)
		 VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		id, role, id,
	); err != nil {
		t.Fatalf("seedSessionWithRole: insert: %v", err)
	}

	sessionFile := filepath.Join(dir, ".tkt", "session")
	if err := os.WriteFile(sessionFile, []byte(id), 0o644); err != nil {
		t.Fatalf("seedSessionWithRole: write session file: %v", err)
	}
}

// seedTicketWithStatus inserts a ticket with the given status and returns its string ID.
func seedTicketWithStatus(t *testing.T, dir string, title string, status string) string {
	t.Helper()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("seedTicketWithStatus: open db: %v", err)
	}
	defer database.Close()

	result, err := database.Exec(
		`INSERT INTO tickets (title, description, status, created_by)
		 VALUES (?, '', ?, 'seed-session')`,
		title, status,
	)
	if err != nil {
		t.Fatalf("seedTicketWithStatus: insert: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("seedTicketWithStatus: last insert id: %v", err)
	}
	return fmt.Sprintf("%d", id)
}

// TestAdvance_MissingNote verifies that an empty --note returns a usage error.
func TestAdvance_MissingNote(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runAdvanceInDir(t, dir, []string{"1"}, func() {
		// advanceNote left empty
	})
	if err == nil {
		t.Fatal("expected error for empty note, got nil")
	}
	if !strings.Contains(err.Error(), "--note is required") {
		t.Errorf("expected '--note is required' in error, got: %v", err)
	}
}

// TestAdvance_NoSession verifies that running without an active session returns an error.
func TestAdvance_NoSession(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runAdvanceInDir(t, dir, []string{"1"}, func() {
		advanceNote = "some note"
	})
	if err == nil {
		t.Fatal("expected error for no session, got nil")
	}
	if !strings.Contains(err.Error(), "no active session") {
		t.Errorf("expected 'no active session' in error, got: %v", err)
	}
}

// TestAdvance_InvalidTo verifies that an unrecognized --to value returns an error.
func TestAdvance_InvalidTo(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-test-0001")
	seedTicketWithStatus(t, dir, "Test ticket", "TODO")

	_, err := runAdvanceInDir(t, dir, []string{"1"}, func() {
		advanceNote = "some note"
		advanceTo = "BADSTATE"
	})
	if err == nil {
		t.Fatal("expected error for invalid --to, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --to") {
		t.Errorf("expected 'invalid --to' in error, got: %v", err)
	}
}

// TestAdvance_Success verifies that a valid advance prints the §7 format:
// #<id>  <from> → <to>, Session: ..., Note: "...".
func TestAdvance_Success(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-test-0001")
	id := seedTicketWithStatus(t, dir, "Test ticket", "TODO")

	out, err := runAdvanceInDir(t, dir, []string{id}, func() {
		advanceNote = "starting work"
	})
	if err != nil {
		t.Fatalf("runAdvance: %v", err)
	}

	// Output must contain the transition line.
	wantTransition := fmt.Sprintf("#%s  TODO → PLANNING", id)
	if !strings.Contains(out, wantTransition) {
		t.Errorf("expected %q in output, got: %q", wantTransition, out)
	}
	if !strings.Contains(out, "Session: impl-test-0001") {
		t.Errorf("expected 'Session: impl-test-0001' in output, got: %q", out)
	}
	if !strings.Contains(out, `Note: "starting work"`) {
		t.Errorf(`expected 'Note: "starting work"' in output, got: %q`, out)
	}
}

// TestAdvance_RoleViolation verifies that a role violation without --force prints
// the §7 error format to stderr and returns an empty sentinel error (exit 1).
func TestAdvance_RoleViolation(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Seed a ticket in PLANNING state. Advancing PLANNING → IN_PROGRESS requires architect.
	// Use an implementer session to trigger the role violation.
	seedSessionWithRole(t, dir, "impl-test-0002", "implementer")
	id := seedTicketWithStatus(t, dir, "Test ticket", "PLANNING")

	// We also need a log entry so Execute can find the submitter; without one it falls
	// back to created_by. The ticket was created with "seed-session" so the isolation
	// check uses that, but the role check fires first.
	_ = id

	_, err := runAdvanceInDir(t, dir, []string{id}, func() {
		advanceNote = "approve"
	})
	// We expect a non-nil sentinel error (empty string) — same pattern as session no-session.
	if err == nil {
		t.Fatal("expected sentinel error for role violation, got nil")
	}
	if err.Error() != "" {
		t.Errorf("expected empty sentinel error, got: %v", err)
	}

	// SilenceErrors was set; the real message went to stderr (we cannot easily capture
	// os.Stderr in this test harness). We verify the command returned the sentinel.
	// A more thorough test could redirect os.Stderr before the call.
}

// TestAdvance_DependencyWarning verifies that after a successful transition, an advisory
// warning is printed when the ticket has unresolved dependencies.
func TestAdvance_DependencyWarning(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-warn-001")

	// Ticket A is the dependency (TODO — unresolved).
	aID := seedTicketWithStatus(t, dir, "Dependency ticket", "TODO")
	// Ticket B is the ticket we will advance.
	bID := seedTicketWithStatus(t, dir, "Main ticket", "TODO")

	// Parse IDs.
	var aIDInt, bIDInt int64
	fmt.Sscanf(aID, "%d", &aIDInt)
	fmt.Sscanf(bID, "%d", &bIDInt)

	// Make B depend on A.
	insertDependency(t, dir, bIDInt, aIDInt)

	out, err := runAdvanceInDir(t, dir, []string{bID}, func() {
		advanceNote = "advancing with unresolved dep"
	})
	if err != nil {
		t.Fatalf("runAdvance: %v", err)
	}

	wantWarning := fmt.Sprintf("Warning: #%s has 1 unresolved dependency", bID)
	if !strings.Contains(out, wantWarning) {
		t.Errorf("expected %q in output, got: %q", wantWarning, out)
	}
	if !strings.Contains(out, "Transition recorded.") {
		t.Errorf("expected 'Transition recorded.' in output, got: %q", out)
	}
}

// TestAdvance_NoDependencyWarning verifies that no warning is printed when a ticket has
// no dependencies.
func TestAdvance_NoDependencyWarning(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-no-dep-warn-001")
	id := seedTicketWithStatus(t, dir, "No dep ticket", "TODO")

	out, err := runAdvanceInDir(t, dir, []string{id}, func() {
		advanceNote = "advancing with no deps"
	})
	if err != nil {
		t.Fatalf("runAdvance: %v", err)
	}

	if strings.Contains(out, "Warning") {
		t.Errorf("expected no 'Warning' in output for ticket with no deps, got: %q", out)
	}
}

// TestAdvance_WithHashPrefix verifies that a ticket ID with a "#" prefix is accepted.
func TestAdvance_WithHashPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-test-0003")
	id := seedTicketWithStatus(t, dir, "Hash prefix ticket", "TODO")

	out, err := runAdvanceInDir(t, dir, []string{"#" + id}, func() {
		advanceNote = "hash prefix test"
	})
	if err != nil {
		t.Fatalf("runAdvance with # prefix: %v", err)
	}

	// Output should still show the numeric id without the hash.
	wantTransition := fmt.Sprintf("#%s  TODO → PLANNING", id)
	if !strings.Contains(out, wantTransition) {
		t.Errorf("expected %q in output, got: %q", wantTransition, out)
	}
}
