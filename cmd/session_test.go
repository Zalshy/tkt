package cmd

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/project"
	"github.com/zalshy/tkt/internal/session"
)

// runSessionInDir sets rootDir to dir, applies the given flag setup function,
// runs runSession, captures stdout, then immediately restores all state.
// Returns captured stdout and any error returned by runSession.
func runSessionInDir(t *testing.T, dir string, setupFlags func()) (string, error) {
	t.Helper()

	// Save and restore flag vars immediately after each call (not via t.Cleanup,
	// which would run at test end and corrupt subsequent calls in the same test).
	savedRole := sessionRole
	savedEnd := sessionEnd
	savedRootDir := rootDir
	defer func() {
		sessionRole = savedRole
		sessionEnd = savedEnd
		rootDir = savedRootDir
		sessionCmd.SetOut(nil)
	}()

	// Reset flags to zero before caller overrides.
	sessionRole = ""
	sessionEnd = false
	rootDir = dir

	// Caller configures flags.
	if setupFlags != nil {
		setupFlags()
	}

	// Capture stdout by swapping the command's output writer.
	var buf bytes.Buffer
	sessionCmd.SetOut(&buf)

	err := runSession(sessionCmd, nil)
	return buf.String(), err
}

// openProjectDB opens the DB for an initialized project dir.
func openProjectDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("openProjectDB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// TestSession_CreateArchitect verifies --role architect creates a session and
// writes the session file, DB row, and correct stdout.
func TestSession_CreateArchitect(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, err := runSessionInDir(t, dir, func() { sessionRole = "architect" })
	if err != nil {
		t.Fatalf("runSession: %v", err)
	}

	if !strings.Contains(out, "Session created:") {
		t.Errorf("expected 'Session created:' in output, got: %q", out)
	}
	if !strings.Contains(out, "architect") {
		t.Errorf("expected 'architect' in output, got: %q", out)
	}

	// Session file must exist.
	data, err := os.ReadFile(project.SessionFile(dir))
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		t.Fatal("session file is empty")
	}

	// DB row must exist with correct role.
	database := openProjectDB(t, dir)
	var role string
	if err := database.QueryRow(`SELECT role FROM sessions WHERE id = ?`, id).Scan(&role); err != nil {
		t.Fatalf("query session row: %v", err)
	}
	if role != "architect" {
		t.Errorf("DB role = %q, want 'architect'", role)
	}
}

// TestSession_CreateImplementer verifies --role implementer creates a session correctly.
func TestSession_CreateImplementer(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, err := runSessionInDir(t, dir, func() { sessionRole = "implementer" })
	if err != nil {
		t.Fatalf("runSession: %v", err)
	}

	if !strings.Contains(out, "implementer") {
		t.Errorf("expected 'implementer' in output, got: %q", out)
	}

	// Session file must exist.
	data, err := os.ReadFile(project.SessionFile(dir))
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}
	id := strings.TrimSpace(string(data))

	// DB row must have role implementer.
	database := openProjectDB(t, dir)
	var role string
	if err := database.QueryRow(`SELECT role FROM sessions WHERE id = ?`, id).Scan(&role); err != nil {
		t.Fatalf("query session row: %v", err)
	}
	if role != "implementer" {
		t.Errorf("DB role = %q, want 'implementer'", role)
	}
}

// TestSession_CreateInvalidRole verifies that an unrecognised role returns an
// error containing "invalid role".
func TestSession_CreateInvalidRole(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runSessionInDir(t, dir, func() { sessionRole = "wizard" })
	if err == nil {
		t.Fatal("expected error for invalid role, got nil")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Errorf("error should mention 'invalid role', got: %v", err)
	}
}

// TestSession_ShowActive verifies that `tkt session` (no flags) after creating
// a session prints Session:, Role:, Status:, and Active since: fields.
func TestSession_ShowActive(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Create a session first.
	if _, err := runSessionInDir(t, dir, func() { sessionRole = "architect" }); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Now show it.
	out, err := runSessionInDir(t, dir, nil)
	if err != nil {
		t.Fatalf("show session: %v", err)
	}

	for _, want := range []string{"Session:", "Role:", "Status:", "Active since:"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %q", want, out)
		}
	}
	if !strings.Contains(out, "active") {
		t.Errorf("expected 'active' status in output, got: %q", out)
	}
}

// TestSession_ShowNoSession verifies that `tkt session` with no .tkt/session
// file prints the no-session help message.
func TestSession_ShowNoSession(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Do not create a session; just show.
	out, err := runSessionInDir(t, dir, nil)
	// We expect a non-nil sentinel error (empty string) so the process exits 1,
	// but the message should be printed to stdout, not via the error.
	if err == nil {
		t.Fatal("expected non-nil sentinel error for no-session path, got nil")
	}
	// The error string itself should be empty (sentinel).
	if err.Error() != "" {
		t.Errorf("expected empty sentinel error, got: %v", err)
	}
	if !strings.Contains(out, "No active session") {
		t.Errorf("expected 'No active session' in stdout, got: %q", out)
	}
}

// TestSession_ShowExpired verifies that `tkt session` when the session is
// expired prints a message containing "has expired".
func TestSession_ShowExpired(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	database := openProjectDB(t, dir)

	expiredID := "arch-test-aaaa"
	if _, err := database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active, expired_at)
		 VALUES (?, 'architect', 'test', datetime('now'), datetime('now'), datetime('2000-01-01'))`,
		expiredID,
	); err != nil {
		t.Fatalf("insert expired session: %v", err)
	}
	database.Close()

	if err := os.WriteFile(filepath.Join(dir, ".tkt", "session"), []byte(expiredID), 0o644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	_, err := runSessionInDir(t, dir, nil)
	if err == nil {
		t.Fatal("expected error for expired session, got nil")
	}
	if !strings.Contains(err.Error(), "has expired") {
		t.Errorf("expected 'has expired' in error, got: %v", err)
	}
}

// TestSession_End verifies that `tkt session --end` sets expired_at in the DB
// and prints a confirmation containing "ended".
func TestSession_End(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Create a session.
	if _, err := runSessionInDir(t, dir, func() { sessionRole = "architect" }); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Read the session ID from the file before ending.
	data, err := os.ReadFile(project.SessionFile(dir))
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}
	id := strings.TrimSpace(string(data))

	// End the session.
	out, err := runSessionInDir(t, dir, func() { sessionEnd = true })
	if err != nil {
		t.Fatalf("end session: %v", err)
	}
	if !strings.Contains(out, "ended") {
		t.Errorf("expected 'ended' in output, got: %q", out)
	}

	// Verify expired_at is set in DB.
	database := openProjectDB(t, dir)
	var expiredAt sql.NullString
	if err := database.QueryRow(`SELECT expired_at FROM sessions WHERE id = ?`, id).Scan(&expiredAt); err != nil {
		t.Fatalf("query expired_at: %v", err)
	}
	if !expiredAt.Valid || expiredAt.String == "" {
		t.Error("expected expired_at to be set in DB, got NULL or empty")
	}
}

// TestSession_EndNoSession verifies that `tkt session --end` with no session
// file returns an error containing "no active session".
func TestSession_EndNoSession(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runSessionInDir(t, dir, func() { sessionEnd = true })
	if err == nil {
		t.Fatal("expected error for end with no session, got nil")
	}
	if !strings.Contains(err.Error(), "no active session") {
		t.Errorf("expected 'no active session' in error, got: %v", err)
	}
}

// Ensure session package is reachable (compile check used in setup helpers).
var _ = errors.New
var _ = session.ErrNoSession
