package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	database, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestCleanupStaleSessions_NoStale(t *testing.T) {
	database := setupDB(t)

	// Insert a session with last_active within 48h (just now).
	_, err := database.Exec(
		`INSERT INTO sessions (id, role, name, last_active) VALUES (?, ?, ?, datetime('now'))`,
		"sess-fresh", "implementer", "Fresh Session",
	)
	if err != nil {
		t.Fatalf("insert fresh session: %v", err)
	}

	n, err := CleanupStaleSessions(database, false)
	if err != nil {
		t.Fatalf("CleanupStaleSessions: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 sessions expired, got %d", n)
	}

	// Verify expired_at is still NULL.
	var expiredAt sql.NullString
	if err := database.QueryRow(`SELECT expired_at FROM sessions WHERE id = ?`, "sess-fresh").Scan(&expiredAt); err != nil {
		t.Fatalf("select expired_at: %v", err)
	}
	if expiredAt.Valid {
		t.Errorf("expected expired_at to be NULL, got %q", expiredAt.String)
	}
}

func TestCleanupStaleSessions_StaleExpired(t *testing.T) {
	database := setupDB(t)

	// Insert two stale sessions (last_active 72h ago).
	for _, id := range []string{"sess-stale-1", "sess-stale-2"} {
		_, err := database.Exec(
			`INSERT INTO sessions (id, role, name, last_active) VALUES (?, ?, ?, datetime('now', '-72 hours'))`,
			id, "implementer", "Stale Session",
		)
		if err != nil {
			t.Fatalf("insert stale session %s: %v", id, err)
		}
	}

	n, err := CleanupStaleSessions(database, false)
	if err != nil {
		t.Fatalf("CleanupStaleSessions: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 sessions expired, got %d", n)
	}

	// Verify both have non-NULL expired_at.
	for _, id := range []string{"sess-stale-1", "sess-stale-2"} {
		var expiredAt sql.NullString
		if err := database.QueryRow(`SELECT expired_at FROM sessions WHERE id = ?`, id).Scan(&expiredAt); err != nil {
			t.Fatalf("select expired_at for %s: %v", id, err)
		}
		if !expiredAt.Valid {
			t.Errorf("expected expired_at to be set for %s, got NULL", id)
		}
	}
}

func TestCleanupStaleSessions_AlreadyExpiredIgnored(t *testing.T) {
	database := setupDB(t)

	// Insert a stale session that already has expired_at set.
	_, err := database.Exec(
		`INSERT INTO sessions (id, role, name, last_active, expired_at) VALUES (?, ?, ?, datetime('now', '-72 hours'), datetime('now', '-24 hours'))`,
		"sess-already-expired", "implementer", "Already Expired",
	)
	if err != nil {
		t.Fatalf("insert already-expired session: %v", err)
	}

	n, err := CleanupStaleSessions(database, false)
	if err != nil {
		t.Fatalf("CleanupStaleSessions: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 sessions expired, got %d", n)
	}
}

func TestCleanupStaleSessions_DryRunNoWrite(t *testing.T) {
	database := setupDB(t)

	// Insert one stale session.
	_, err := database.Exec(
		`INSERT INTO sessions (id, role, name, last_active) VALUES (?, ?, ?, datetime('now', '-72 hours'))`,
		"sess-stale-dry", "implementer", "Stale Dry Run",
	)
	if err != nil {
		t.Fatalf("insert stale session: %v", err)
	}

	n, err := CleanupStaleSessions(database, true)
	if err != nil {
		t.Fatalf("CleanupStaleSessions dry-run: %v", err)
	}
	if n != 1 {
		t.Errorf("expected count 1, got %d", n)
	}

	// Verify expired_at is still NULL (dry-run must not write).
	var expiredAt sql.NullString
	if err := database.QueryRow(`SELECT expired_at FROM sessions WHERE id = ?`, "sess-stale-dry").Scan(&expiredAt); err != nil {
		t.Fatalf("select expired_at: %v", err)
	}
	if expiredAt.Valid {
		t.Errorf("dry-run should not set expired_at, got %q", expiredAt.String)
	}
}

func TestCleanupStaleSessions_Mixed(t *testing.T) {
	database := setupDB(t)

	// One stale, unexpired — should be updated.
	_, err := database.Exec(
		`INSERT INTO sessions (id, role, name, last_active) VALUES (?, ?, ?, datetime('now', '-72 hours'))`,
		"sess-stale", "implementer", "Stale",
	)
	if err != nil {
		t.Fatalf("insert stale session: %v", err)
	}

	// One fresh — should be untouched.
	_, err = database.Exec(
		`INSERT INTO sessions (id, role, name, last_active) VALUES (?, ?, ?, datetime('now'))`,
		"sess-fresh", "implementer", "Fresh",
	)
	if err != nil {
		t.Fatalf("insert fresh session: %v", err)
	}

	// One stale but already expired — should be ignored.
	_, err = database.Exec(
		`INSERT INTO sessions (id, role, name, last_active, expired_at) VALUES (?, ?, ?, datetime('now', '-72 hours'), datetime('now', '-24 hours'))`,
		"sess-pre-expired", "implementer", "Pre-Expired",
	)
	if err != nil {
		t.Fatalf("insert pre-expired session: %v", err)
	}

	n, err := CleanupStaleSessions(database, false)
	if err != nil {
		t.Fatalf("CleanupStaleSessions: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 session expired, got %d", n)
	}

	// Stale session must now have expired_at set.
	var staleExpired sql.NullString
	if err := database.QueryRow(`SELECT expired_at FROM sessions WHERE id = ?`, "sess-stale").Scan(&staleExpired); err != nil {
		t.Fatalf("select expired_at for stale: %v", err)
	}
	if !staleExpired.Valid {
		t.Error("expected expired_at to be set for stale session, got NULL")
	}

	// Fresh session must still have NULL expired_at.
	var freshExpired sql.NullString
	if err := database.QueryRow(`SELECT expired_at FROM sessions WHERE id = ?`, "sess-fresh").Scan(&freshExpired); err != nil {
		t.Fatalf("select expired_at for fresh: %v", err)
	}
	if freshExpired.Valid {
		t.Errorf("expected fresh session expired_at to be NULL, got %q", freshExpired.String)
	}
}
