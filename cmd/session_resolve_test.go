package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/session"
)

// seedSessionIDName inserts a session row with distinct id and name (unlike seedSession
// in new_test.go, which uses the same value for both) so id-vs-name resolution paths can
// be tested independently, and optionally writes the .tkt/session file pointer.
func seedSessionIDName(t *testing.T, dir, id, name string, writeFile bool) {
	t.Helper()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("seedSessionIDName: open db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active)
		 VALUES (?, 'implementer', ?, datetime('now'), datetime('now'))`,
		id, name,
	); err != nil {
		t.Fatalf("seedSessionIDName: insert: %v", err)
	}

	if writeFile {
		sessionFile := filepath.Join(dir, ".tkt", "session")
		if err := os.WriteFile(sessionFile, []byte(id), 0o644); err != nil {
			t.Fatalf("seedSessionIDName: write session file: %v", err)
		}
	}
}

// TestResolveSession_ValidULID verifies resolveSession resolves the actor by ULID when
// --session is set, even when no .tkt/session file pointer exists.
func TestResolveSession_ValidULID(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSessionIDName(t, dir, "resolve-ulid-id", "resolve-ulid-name", false)

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	savedOverride := sessionOverride
	sessionOverride = "resolve-ulid-id"
	defer func() { sessionOverride = savedOverride }()

	sess, err := resolveSession(dir, database)
	if err != nil {
		t.Fatalf("resolveSession: %v", err)
	}
	if sess.ID != "resolve-ulid-id" {
		t.Errorf("sess.ID = %q, want %q", sess.ID, "resolve-ulid-id")
	}
	if sess.Name != "resolve-ulid-name" {
		t.Errorf("sess.Name = %q, want %q", sess.Name, "resolve-ulid-name")
	}
}

// TestResolveSession_ValidName verifies resolveSession resolves the actor by name when
// --session is set.
func TestResolveSession_ValidName(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSessionIDName(t, dir, "resolve-name-id", "resolve-name-name", false)

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	savedOverride := sessionOverride
	sessionOverride = "resolve-name-name"
	defer func() { sessionOverride = savedOverride }()

	sess, err := resolveSession(dir, database)
	if err != nil {
		t.Fatalf("resolveSession: %v", err)
	}
	if sess.ID != "resolve-name-id" {
		t.Errorf("sess.ID = %q, want %q", sess.ID, "resolve-name-id")
	}
	if sess.Name != "resolve-name-name" {
		t.Errorf("sess.Name = %q, want %q", sess.Name, "resolve-name-name")
	}
}

// TestResolveSession_InvalidValue_NoSilentFallback verifies an unmatched --session value
// hard-errors with a clear message and does NOT silently fall back to the file pointer,
// even when the file pointer would have resolved successfully.
func TestResolveSession_InvalidValue_NoSilentFallback(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	// File pointer resolves fine on its own.
	seedSessionIDName(t, dir, "fallback-candidate-id", "fallback-candidate-name", true)

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	savedOverride := sessionOverride
	sessionOverride = "totally-bogus-value"
	defer func() { sessionOverride = savedOverride }()

	sess, err := resolveSession(dir, database)
	if err == nil {
		t.Fatalf("resolveSession: expected error for unmatched --session value, got session %+v", sess)
	}
	if !strings.Contains(err.Error(), "totally-bogus-value") || !strings.Contains(err.Error(), "no matching session") {
		t.Errorf("error = %q, want it to mention the value and 'no matching session'", err.Error())
	}
}

// TestResolveSession_ExpiredViaFlag verifies resolveSession's --session branch returns
// the exact same user-facing expired-session message (msgExpiredSession) that every
// call site produces for the file-pointer path via its own
// `errors.Is(err, session.ErrExpiredSession) -> errors.New(msgExpiredSession)` mapping.
//
// resolveSession's flag branch maps ErrExpiredSession to msgExpiredSession internally
// (see session_resolve.go); the no-flag branch deliberately returns the raw
// session.ErrExpiredSession sentinel unchanged, so existing call sites' errors.Is
// checks keep working unmodified (plan requirement). This test applies that same
// call-site-style mapping to the file-pointer result and asserts the two final
// user-facing strings are byte-for-byte identical (architect review note (a)).
func TestResolveSession_ExpiredViaFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active, expired_at)
		 VALUES (?, 'architect', ?, datetime('now'), datetime('now'), datetime('2000-01-01'))`,
		"expired-flag-id", "expired-flag-name",
	); err != nil {
		t.Fatalf("insert expired session: %v", err)
	}

	savedOverride := sessionOverride
	sessionOverride = "expired-flag-id"
	defer func() { sessionOverride = savedOverride }()

	_, errViaFlag := resolveSession(dir, database)
	if errViaFlag == nil {
		t.Fatal("resolveSession via flag: expected expired-session error, got nil")
	}

	// Now resolve the SAME expired session via the file-pointer path. resolveSession's
	// no-flag branch returns the raw sentinel; apply the identical call-site mapping
	// every command uses before comparing user-facing text.
	sessionOverride = ""
	if err := os.WriteFile(filepath.Join(dir, ".tkt", "session"), []byte("expired-flag-id"), 0o644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	_, errViaFileRaw := resolveSession(dir, database)
	if !errors.Is(errViaFileRaw, session.ErrExpiredSession) {
		t.Fatalf("resolveSession via file: expected ErrExpiredSession, got %v", errViaFileRaw)
	}
	errViaFile := errors.New(msgExpiredSession) // mirrors every call site's mapping

	if errViaFlag.Error() != errViaFile.Error() {
		t.Errorf("expired-session message mismatch: via flag = %q, via file (call-site mapped) = %q", errViaFlag.Error(), errViaFile.Error())
	}
	if errViaFlag.Error() != msgExpiredSession {
		t.Errorf("via flag error = %q, want exactly msgExpiredSession %q", errViaFlag.Error(), msgExpiredSession)
	}
}

// TestResolveSession_NoFlagFallsThrough verifies that with sessionOverride empty,
// resolveSession behaves identically to calling session.LoadActive directly — the
// existing file-pointer regression path must remain unchanged.
func TestResolveSession_NoFlagFallsThrough(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSessionIDName(t, dir, "fallthrough-id", "fallthrough-name", true)

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	savedOverride := sessionOverride
	sessionOverride = ""
	defer func() { sessionOverride = savedOverride }()

	gotViaResolve, errResolve := resolveSession(dir, database)
	if errResolve != nil {
		t.Fatalf("resolveSession: %v", errResolve)
	}

	gotViaLoadActive, errLoadActive := session.LoadActive(dir, database)
	if errLoadActive != nil {
		t.Fatalf("session.LoadActive: %v", errLoadActive)
	}

	if gotViaResolve.ID != gotViaLoadActive.ID {
		t.Errorf("resolveSession ID = %q, LoadActive ID = %q, want match", gotViaResolve.ID, gotViaLoadActive.ID)
	}
	if gotViaResolve.ID != "fallthrough-id" {
		t.Errorf("resolveSession ID = %q, want %q", gotViaResolve.ID, "fallthrough-id")
	}
}
