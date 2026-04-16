package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupDBWithRoot creates a temp .tkt directory, opens the DB, and registers
// cleanup. It returns both the root directory and the open database handle so
// callers that need to re-open the same file can do so.
func setupDBWithRoot(t *testing.T) (string, *sql.DB) {
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
	return root, database
}

// TestMigration_FreshDB verifies that a brand-new DB ends up at schema_version=12
// and that the ticket_dependencies table exists.
func TestMigration_FreshDB(t *testing.T) {
	_, database := setupDBWithRoot(t)

	// Assert ticket_dependencies table exists.
	var name string
	err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_dependencies'`,
	).Scan(&name)
	if err != nil {
		t.Errorf("ticket_dependencies table not found in sqlite_master: %v", err)
	}

	// Assert schema_version = 12.
	var version int
	if err := database.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}
}

// TestMigration_V2ToV3 verifies that a second Open on an already-migrated DB is
// a no-op: schema_version stays 12 and pre-seeded data survives.
func TestMigration_V2ToV3(t *testing.T) {
	root, database := setupDBWithRoot(t)

	// Seed a ticket row after the first Open (which ran all migrations).
	_, err := database.Exec(
		`INSERT INTO tickets (title, created_by) VALUES ('test ticket', 'tester')`,
	)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	database.Close()

	// Second Open — must be a no-op (version guard skips all migrations).
	db2, err := Open(root)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer db2.Close()

	// Assert schema_version = 12.
	var version int
	if err := db2.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	// Assert the seeded ticket row survived.
	var title string
	if err := db2.QueryRow(`SELECT title FROM tickets WHERE title='test ticket'`).Scan(&title); err != nil {
		t.Errorf("seeded ticket row not found after second Open: %v", err)
	}

	// Assert ticket_dependencies table still exists.
	var name string
	if err := db2.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_dependencies'`,
	).Scan(&name); err != nil {
		t.Errorf("ticket_dependencies table missing after second Open: %v", err)
	}
}

// TestMigration_Idempotency explicitly tests the version-guard path: a second
// Open must see version=12 and skip all migrations without error.
func TestMigration_Idempotency(t *testing.T) {
	root, db1 := setupDBWithRoot(t)

	var v1 int
	if err := db1.QueryRow(`SELECT version FROM schema_version`).Scan(&v1); err != nil {
		t.Fatalf("schema_version after first Open: %v", err)
	}
	if v1 != 12 {
		t.Errorf("schema_version after first Open = %d, want 12", v1)
	}
	db1.Close()

	db2, err := Open(root)
	if err != nil {
		t.Fatalf("second Open error: %v", err)
	}
	defer db2.Close()

	var v2 int
	if err := db2.QueryRow(`SELECT version FROM schema_version`).Scan(&v2); err != nil {
		t.Fatalf("schema_version after second Open: %v", err)
	}
	if v2 != 12 {
		t.Errorf("schema_version after second Open = %d, want 12", v2)
	}
}

// TestMigration_V4_RolesTableSeeded verifies that after migration V4 the roles
// table exists and contains exactly the two built-in rows.
func TestMigration_V4_RolesTableSeeded(t *testing.T) {
	_, database := setupDBWithRoot(t)

	// Assert schema_version = 12.
	var version int
	if err := database.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	// Assert exactly 2 rows in roles.
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM roles`).Scan(&count); err != nil {
		t.Fatalf("COUNT roles: %v", err)
	}
	if count != 2 {
		t.Errorf("roles COUNT = %d, want 2", count)
	}

	// Assert architect row.
	var baseRole string
	var isBuiltin int
	if err := database.QueryRow(
		`SELECT base_role, is_builtin FROM roles WHERE name='architect'`,
	).Scan(&baseRole, &isBuiltin); err != nil {
		t.Fatalf("SELECT architect row: %v", err)
	}
	if baseRole != "architect" {
		t.Errorf("architect base_role = %q, want %q", baseRole, "architect")
	}
	if isBuiltin != 1 {
		t.Errorf("architect is_builtin = %d, want 1", isBuiltin)
	}

	// Assert implementer row.
	if err := database.QueryRow(
		`SELECT base_role, is_builtin FROM roles WHERE name='implementer'`,
	).Scan(&baseRole, &isBuiltin); err != nil {
		t.Fatalf("SELECT implementer row: %v", err)
	}
	if baseRole != "implementer" {
		t.Errorf("implementer base_role = %q, want %q", baseRole, "implementer")
	}
	if isBuiltin != 1 {
		t.Errorf("implementer is_builtin = %d, want 1", isBuiltin)
	}
}

// TestMigration_V4_BaseRoleConstraint verifies that the CHECK constraint on
// base_role rejects values outside ('architect', 'implementer').
func TestMigration_V4_BaseRoleConstraint(t *testing.T) {
	_, database := setupDBWithRoot(t)

	_, insertErr := database.Exec(
		`INSERT INTO roles (name, base_role, is_builtin) VALUES ('badactor', 'orchestrator', 0)`,
	)
	if insertErr == nil {
		t.Fatal("expected CHECK constraint violation for invalid base_role, got nil error")
	}
	msg := strings.ToLower(insertErr.Error())
	if !strings.Contains(msg, "constraint") && !strings.Contains(msg, "check") {
		t.Errorf("unexpected error text %q — expected constraint/check violation", insertErr.Error())
	}
}

// TestMigration_SelfReferenceConstraint verifies that the CHECK constraint
// (ticket_id != depends_on) prevents a ticket from depending on itself.
func TestMigration_SelfReferenceConstraint(t *testing.T) {
	_, database := setupDBWithRoot(t)

	// Insert a valid ticket to reference.
	res, err := database.Exec(
		`INSERT INTO tickets (title, created_by) VALUES ('dep test ticket', 'tester')`,
	)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	// Attempt a self-referencing dependency — must fail due to CHECK constraint.
	_, insertErr := database.Exec(
		`INSERT INTO ticket_dependencies (ticket_id, depends_on) VALUES (?, ?)`,
		ticketID, ticketID,
	)
	if insertErr == nil {
		t.Fatal("expected CHECK constraint violation for self-referencing dependency, got nil error")
	}
	// Confirm the error mentions a constraint violation (SQLite wording varies slightly).
	msg := strings.ToLower(insertErr.Error())
	if !strings.Contains(msg, "constraint") && !strings.Contains(msg, "check") {
		t.Errorf("unexpected error text %q — expected constraint/check violation", insertErr.Error())
	}
}

// TestMigration_V7_FreshDB verifies that a fresh DB lands at schema_version=12
// and that the ticket_usage table and its index exist.
func TestMigration_V7_FreshDB(t *testing.T) {
	_, database := setupDBWithRoot(t)

	var version int
	if err := database.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	var name string
	if err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_usage'`,
	).Scan(&name); err != nil {
		t.Errorf("ticket_usage table not found in sqlite_master: %v", err)
	}

	var idxName string
	if err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_ticket_usage_ticket_id_deleted_at'`,
	).Scan(&idxName); err != nil {
		t.Errorf("idx_ticket_usage_ticket_id_deleted_at index not found in sqlite_master: %v", err)
	}
}

// TestMigration_V7_Backfill seeds a V6 DB with ticket_log usage rows, re-opens
// to trigger V7, then asserts ticket_usage row count and field values match.
func TestMigration_V7_Backfill(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Open once to land at V11 (current schema), then downgrade to V6 to re-run
	// V7, V8, V9, V10, and V11 on the next Open.
	db1, err := Open(root)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}

	// Insert a session and a ticket to satisfy FKs (only metadata — no log rows yet,
	// because ticket_log is now constrained and 'usage' kind is not allowed).
	if _, err := db1.Exec(
		`INSERT INTO sessions (id, role, name) VALUES ('sess-x', 'implementer', 'tester')`,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	res, err := db1.Exec(`INSERT INTO tickets (title, created_by) VALUES ('backfill test', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, _ := res.LastInsertId()

	// Downgrade to V6 state: restore the unconstrained ticket_log (as it existed
	// before V9 added constraints), drop ticket_usage, and clear V10/V9 artifacts.
	// ticket_log_new does not exist after V10 (it was renamed), so no need to drop it.
	// The canonical indexes must be recreated so V10's unconditional DROP INDEX succeeds.
	for _, stmt := range []string{
		// Drop the canonical indexes that now live on the V10-renamed ticket_log.
		`DROP INDEX IF EXISTS idx_ticket_log_ticket_id_kind`,
		`DROP INDEX IF EXISTS idx_ticket_log_deleted_at`,
		`DROP INDEX IF EXISTS idx_ticket_log_kind`,
		`DROP INDEX IF EXISTS idx_ticket_log_ticket_id`,
		// Drop the constrained ticket_log (post-V10 rename).
		`DROP TABLE IF EXISTS ticket_log`,
		// Recreate the original unconstrained ticket_log (V2–V8 schema).
		`CREATE TABLE ticket_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id  INTEGER NOT NULL REFERENCES tickets(id),
    session_id TEXT NOT NULL,
    kind       TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    from_state TEXT NOT NULL DEFAULT '',
    to_state   TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at DATETIME NULL
)`,
		// Recreate canonical indexes (V2 + V6) so V10's unconditional DROP INDEX works.
		`CREATE INDEX idx_ticket_log_ticket_id ON ticket_log(ticket_id)`,
		`CREATE INDEX idx_ticket_log_kind ON ticket_log(kind)`,
		`CREATE INDEX idx_ticket_log_deleted_at ON ticket_log(deleted_at)`,
		`CREATE INDEX idx_ticket_log_ticket_id_kind ON ticket_log(ticket_id, kind)`,
		// Drop ticket_usage so V7 re-runs cleanly.
		`DROP INDEX IF EXISTS idx_ticket_usage_ticket_id_deleted_at`,
		`DROP TABLE IF EXISTS ticket_usage`,
		// Drop V11 artifacts so V11 re-runs cleanly.
		`DROP INDEX IF EXISTS idx_ticket_dependencies_new_depends_on`,
		`DROP TABLE IF EXISTS ticket_dependencies_new`,
		// Restore pre-V12 state: old ticket_dependencies (TEXT schema) so V12's unconditional DROP works.
		`DROP INDEX IF EXISTS idx_ticket_dependencies_depends_on`,
		`DROP TABLE IF EXISTS ticket_dependencies`,
		`CREATE TABLE ticket_dependencies (
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    depends_on  INTEGER NOT NULL REFERENCES tickets(id),
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (ticket_id, depends_on),
    CHECK (ticket_id != depends_on)
)`,
		`CREATE INDEX idx_ticket_dependencies_depends_on ON ticket_dependencies(depends_on)`,
	} {
		if _, err := db1.Exec(stmt); err != nil {
			t.Fatalf("downgrade stmt %q: %v", stmt, err)
		}
	}

	// Now that ticket_log is unconstrained, insert the usage rows (simulating pre-V7 data).
	// The schema_version downgrade below causes the next Open to re-run V7 (backfill),
	// V8 (delete usage from ticket_log), V9 (create ticket_log_new), V10 (rename), V11 (deps_new), and V12 (drop/rename).
	_, err = db1.Exec(
		`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'sess-x', 'usage', '{"tokens":100,"tools":2,"duration_ms":5000,"agent":"impl","label":"lbl1"}')`,
		ticketID,
	)
	if err != nil {
		t.Fatalf("insert ticket_log usage row 1: %v", err)
	}
	_, err = db1.Exec(
		`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'sess-x', 'usage', '{"tokens":200,"tools":0,"duration_ms":0,"agent":"","label":""}')`,
		ticketID,
	)
	if err != nil {
		t.Fatalf("insert ticket_log usage row 2: %v", err)
	}

	if _, err := db1.Exec(`UPDATE schema_version SET version = 6`); err != nil {
		t.Fatalf("downgrade schema_version: %v", err)
	}
	db1.Close()

	// Re-open — triggers V7 (backfill), V8 (delete usage), V9 (create _new), V10 (rename), V11 (deps_new), V12 (drop/rename).
	db2, err := Open(root)
	if err != nil {
		t.Fatalf("second Open (V7 migration): %v", err)
	}
	defer db2.Close()

	var version int
	if err := db2.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	var count int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM ticket_usage WHERE ticket_id = ?`, ticketID).Scan(&count); err != nil {
		t.Fatalf("COUNT ticket_usage: %v", err)
	}
	if count != 2 {
		t.Errorf("ticket_usage count = %d, want 2", count)
	}

	// V8 should have deleted the usage rows from ticket_log (which is now the
	// renamed constrained table after V10).
	var usageInLog int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM ticket_log WHERE kind='usage'`).Scan(&usageInLog); err != nil {
		t.Fatalf("count usage in ticket_log: %v", err)
	}
	if usageInLog != 0 {
		t.Errorf("expected 0 usage rows in ticket_log after V8, got %d", usageInLog)
	}

	// Verify first row field values.
	var tokens, tools, durationMs int
	var agent, label string
	if err := db2.QueryRow(
		`SELECT tokens, tools, duration_ms, agent, label FROM ticket_usage WHERE ticket_id = ? ORDER BY id ASC LIMIT 1`,
		ticketID,
	).Scan(&tokens, &tools, &durationMs, &agent, &label); err != nil {
		t.Fatalf("SELECT ticket_usage row: %v", err)
	}
	if tokens != 100 {
		t.Errorf("tokens = %d, want 100", tokens)
	}
	if tools != 2 {
		t.Errorf("tools = %d, want 2", tools)
	}
	if durationMs != 5000 {
		t.Errorf("duration_ms = %d, want 5000", durationMs)
	}
	if agent != "impl" {
		t.Errorf("agent = %q, want %q", agent, "impl")
	}
	if label != "lbl1" {
		t.Errorf("label = %q, want %q", label, "lbl1")
	}
}

// TestMigration_V8_DeletesUsageRows verifies that V8 removes all kind='usage'
// rows from ticket_log while leaving non-usage rows intact.
func TestMigration_V8_DeletesUsageRows(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Open once — lands at V11 (current schema).
	db1, err := Open(root)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}

	// Insert a session and a ticket to satisfy FKs.
	if _, err := db1.Exec(
		`INSERT INTO sessions (id, role, name) VALUES ('sess-v8', 'implementer', 'v8tester')`,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	res, err := db1.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v8 test', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, _ := res.LastInsertId()

	// Downgrade to V7 state: restore the unconstrained ticket_log and remove V9/V10
	// artifacts so that the next Open re-runs V8 (DELETE usage), V9 (create _new),
	// and V10 (drop old, rename _new to canonical).
	// The canonical indexes must be recreated so V10's unconditional DROP INDEX succeeds.
	for _, stmt := range []string{
		// Drop canonical indexes on the V10-renamed ticket_log.
		`DROP INDEX IF EXISTS idx_ticket_log_ticket_id_kind`,
		`DROP INDEX IF EXISTS idx_ticket_log_deleted_at`,
		`DROP INDEX IF EXISTS idx_ticket_log_kind`,
		`DROP INDEX IF EXISTS idx_ticket_log_ticket_id`,
		// Drop the constrained ticket_log (post-V10 rename).
		`DROP TABLE IF EXISTS ticket_log`,
		// Recreate the original unconstrained ticket_log (V2–V8 schema).
		`CREATE TABLE ticket_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id  INTEGER NOT NULL REFERENCES tickets(id),
    session_id TEXT NOT NULL,
    kind       TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    from_state TEXT NOT NULL DEFAULT '',
    to_state   TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at DATETIME NULL
)`,
		// Recreate canonical indexes (V2 + V6) so V10's unconditional DROP INDEX works.
		`CREATE INDEX idx_ticket_log_ticket_id ON ticket_log(ticket_id)`,
		`CREATE INDEX idx_ticket_log_kind ON ticket_log(kind)`,
		`CREATE INDEX idx_ticket_log_deleted_at ON ticket_log(deleted_at)`,
		`CREATE INDEX idx_ticket_log_ticket_id_kind ON ticket_log(ticket_id, kind)`,
		// Drop V11 artifacts so V11 re-runs cleanly.
		`DROP INDEX IF EXISTS idx_ticket_dependencies_new_depends_on`,
		`DROP TABLE IF EXISTS ticket_dependencies_new`,
		// Restore pre-V12 state: old ticket_dependencies (TEXT schema) so V12's unconditional DROP works.
		`DROP INDEX IF EXISTS idx_ticket_dependencies_depends_on`,
		`DROP TABLE IF EXISTS ticket_dependencies`,
		`CREATE TABLE ticket_dependencies (
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    depends_on  INTEGER NOT NULL REFERENCES tickets(id),
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (ticket_id, depends_on),
    CHECK (ticket_id != depends_on)
)`,
		`CREATE INDEX idx_ticket_dependencies_depends_on ON ticket_dependencies(depends_on)`,
	} {
		if _, err := db1.Exec(stmt); err != nil {
			t.Fatalf("downgrade stmt %q: %v", stmt, err)
		}
	}
	if _, err := db1.Exec(`UPDATE schema_version SET version = 7`); err != nil {
		t.Fatalf("downgrade schema_version: %v", err)
	}

	// Now that ticket_log is unconstrained, insert 2 usage rows and 1 message row.
	for i := 0; i < 2; i++ {
		if _, err := db1.Exec(
			`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'sess-v8', 'usage', '{}')`,
			ticketID,
		); err != nil {
			t.Fatalf("insert usage row %d: %v", i+1, err)
		}
	}
	if _, err := db1.Exec(
		`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'sess-v8', 'message', 'hello')`,
		ticketID,
	); err != nil {
		t.Fatalf("insert message row: %v", err)
	}
	db1.Close()

	// Re-open — triggers V8 (DELETE usage), V9 (create ticket_log_new), V10 (rename), V11 (deps_new), V12 (drop/rename).
	db2, err := Open(root)
	if err != nil {
		t.Fatalf("second Open (V8 migration): %v", err)
	}
	defer db2.Close()

	// Assert schema_version == 12.
	var version int
	if err := db2.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	// Assert all usage rows were deleted (V8) and are absent from the final ticket_log.
	var usageCount int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM ticket_log WHERE kind='usage'`).Scan(&usageCount); err != nil {
		t.Fatalf("COUNT usage in ticket_log: %v", err)
	}
	if usageCount != 0 {
		t.Errorf("expected 0 usage rows in ticket_log after V8, got %d", usageCount)
	}

	// Assert non-usage rows were not deleted.
	var messageCount int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM ticket_log WHERE kind='message'`).Scan(&messageCount); err != nil {
		t.Fatalf("COUNT message in ticket_log: %v", err)
	}
	if messageCount != 1 {
		t.Errorf("expected 1 message row in ticket_log after V8, got %d", messageCount)
	}
}

// TestMigration_V7_VerifyFunction calls verifyV7Backfill directly with a tx
// where counts are mismatched and asserts the error contains "count mismatch".
func TestMigration_V7_VerifyFunction(t *testing.T) {
	_, database := setupDBWithRoot(t)

	tx, err := database.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// After V10, ticket_log has CHECK(kind IN ('transition','plan','message')), so
	// 'usage' rows can no longer be inserted into it. Instead, manufacture the
	// mismatch by inserting a row into ticket_usage (backfilled=1) while ticket_log
	// has no usage rows (source=0). verifyV7Backfill checks source != backfilled and
	// must return an error regardless of which side is larger.
	if _, err := tx.Exec(
		`INSERT INTO sessions (id, role, name) VALUES ('sess-v7', 'implementer', 'v7tester')`,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	res, err := tx.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v7 test', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, _ := res.LastInsertId()

	// Insert directly into ticket_usage without any corresponding ticket_log usage row.
	// This creates mismatch: source=0 (no usage rows in ticket_log), backfilled=1.
	if _, err := tx.Exec(
		`INSERT INTO ticket_usage (ticket_id, session_id, tokens, tools, duration_ms, agent, label) VALUES (?, 'sess-v7', 1, 0, 0, '', '')`,
		ticketID,
	); err != nil {
		t.Fatalf("insert ticket_usage: %v", err)
	}

	verifyErr := verifyV7Backfill(tx)
	if verifyErr == nil {
		t.Fatal("expected error from verifyV7Backfill, got nil")
	}
	if !strings.Contains(verifyErr.Error(), "count mismatch") {
		t.Errorf("expected error to contain 'count mismatch', got: %v", verifyErr)
	}
}

// TestMigration_V9_FreshDB verifies that a fresh DB lands at schema_version=12
// (V9 ran and V10 completed the rename), that ticket_log_new no longer exists in
// sqlite_master, and that the four _new indexes are absent.
func TestMigration_V9_FreshDB(t *testing.T) {
	_, database := setupDBWithRoot(t)

	var version int
	if err := database.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	// V10 renamed ticket_log_new to ticket_log — the _new table must not exist.
	var name string
	err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_log_new'`,
	).Scan(&name)
	if err == nil {
		t.Errorf("ticket_log_new table should not exist after V10, but was found in sqlite_master")
	}

	// The four _new indexes must also be absent.
	absentIndexes := []string{
		"idx_ticket_log_new_ticket_id",
		"idx_ticket_log_new_kind",
		"idx_ticket_log_new_deleted_at",
		"idx_ticket_log_new_ticket_id_kind",
	}
	for _, idx := range absentIndexes {
		var idxName string
		err := database.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&idxName)
		if err == nil {
			t.Errorf("index %q should not exist after V10, but was found in sqlite_master", idx)
		}
	}
}

// TestMigration_V9_Backfill seeds a V8 DB with ticket_log rows, downgrades to V8,
// re-opens to trigger V9 and V10, then asserts count parity and spot-checks a row kind.
// After V10 completes, ticket_log_new is gone (renamed to ticket_log) so all assertions
// are against the canonical ticket_log table.
func TestMigration_V9_Backfill(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Open once — lands at V11.
	db1, err := Open(root)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}

	// Insert a session and a ticket to satisfy FKs (no ticket_log rows yet).
	if _, err := db1.Exec(
		`INSERT INTO sessions (id, role, name) VALUES ('sess-v9', 'implementer', 'v9tester')`,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	res, err := db1.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v9 backfill test', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, _ := res.LastInsertId()

	// Downgrade to V8 state: restore the unconstrained ticket_log so that V9 can
	// copy rows from it into a fresh ticket_log_new, and V10 can then rename.
	// After V10, ticket_log is the constrained table; we must replace it with the
	// old unconstrained version before inserting test rows.
	// The canonical indexes must be recreated so V10's unconditional DROP INDEX succeeds.
	for _, stmt := range []string{
		// Drop canonical indexes on the V10-renamed ticket_log.
		`DROP INDEX IF EXISTS idx_ticket_log_ticket_id_kind`,
		`DROP INDEX IF EXISTS idx_ticket_log_deleted_at`,
		`DROP INDEX IF EXISTS idx_ticket_log_kind`,
		`DROP INDEX IF EXISTS idx_ticket_log_ticket_id`,
		// Drop the constrained ticket_log (post-V10 rename).
		`DROP TABLE IF EXISTS ticket_log`,
		// Recreate the original unconstrained ticket_log (V2–V8 schema).
		`CREATE TABLE ticket_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id  INTEGER NOT NULL REFERENCES tickets(id),
    session_id TEXT NOT NULL,
    kind       TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    from_state TEXT NOT NULL DEFAULT '',
    to_state   TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at DATETIME NULL
)`,
		// Recreate canonical indexes (V2 + V6) so V10's unconditional DROP INDEX works.
		`CREATE INDEX idx_ticket_log_ticket_id ON ticket_log(ticket_id)`,
		`CREATE INDEX idx_ticket_log_kind ON ticket_log(kind)`,
		`CREATE INDEX idx_ticket_log_deleted_at ON ticket_log(deleted_at)`,
		`CREATE INDEX idx_ticket_log_ticket_id_kind ON ticket_log(ticket_id, kind)`,
		// Drop V11 artifacts so V11 re-runs cleanly.
		`DROP INDEX IF EXISTS idx_ticket_dependencies_new_depends_on`,
		`DROP TABLE IF EXISTS ticket_dependencies_new`,
		// Restore pre-V12 state: old ticket_dependencies (TEXT schema) so V12's unconditional DROP works.
		`DROP INDEX IF EXISTS idx_ticket_dependencies_depends_on`,
		`DROP TABLE IF EXISTS ticket_dependencies`,
		`CREATE TABLE ticket_dependencies (
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    depends_on  INTEGER NOT NULL REFERENCES tickets(id),
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (ticket_id, depends_on),
    CHECK (ticket_id != depends_on)
)`,
		`CREATE INDEX idx_ticket_dependencies_depends_on ON ticket_dependencies(depends_on)`,
	} {
		if _, err := db1.Exec(stmt); err != nil {
			t.Fatalf("downgrade stmt %q: %v", stmt, err)
		}
	}
	if _, err := db1.Exec(`UPDATE schema_version SET version = 8`); err != nil {
		t.Fatalf("downgrade schema_version: %v", err)
	}

	// Insert 3 ticket_log rows (one per valid kind) into the unconstrained table.
	kinds := []string{"transition", "plan", "message"}
	var firstID int64
	for i, kind := range kinds {
		r, err := db1.Exec(
			`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'sess-v9', ?, 'body')`,
			ticketID, kind,
		)
		if err != nil {
			t.Fatalf("insert ticket_log row %d: %v", i+1, err)
		}
		if i == 0 {
			firstID, _ = r.LastInsertId()
		}
	}
	db1.Close()

	// Re-open — triggers V9 (create ticket_log_new, copy rows), V10 (rename), V11 (deps_new), and V12 (drop/rename).
	db2, err := Open(root)
	if err != nil {
		t.Fatalf("second Open (V9+V10 migration): %v", err)
	}
	defer db2.Close()

	var version int
	if err := db2.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	// After V10, ticket_log_new must not exist.
	var newName string
	err = db2.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_log_new'`).Scan(&newName)
	if err == nil {
		t.Errorf("ticket_log_new should not exist after V10, but was found in sqlite_master")
	}

	// The canonical ticket_log must have the 3 rows (V9 copied them, V10 renamed the table).
	var logCount int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM ticket_log`).Scan(&logCount); err != nil {
		t.Fatalf("COUNT ticket_log: %v", err)
	}
	if logCount != 3 {
		t.Errorf("ticket_log count = %d, want 3", logCount)
	}

	// The four canonical indexes must exist.
	canonicalIndexes := []string{
		"idx_ticket_log_ticket_id",
		"idx_ticket_log_kind",
		"idx_ticket_log_deleted_at",
		"idx_ticket_log_ticket_id_kind",
	}
	for _, idx := range canonicalIndexes {
		var idxName string
		if err := db2.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&idxName); err != nil {
			t.Errorf("canonical index %q not found after V10: %v", idx, err)
		}
	}

	// The four _new indexes must be absent.
	absentIndexes := []string{
		"idx_ticket_log_new_ticket_id",
		"idx_ticket_log_new_kind",
		"idx_ticket_log_new_deleted_at",
		"idx_ticket_log_new_ticket_id_kind",
	}
	for _, idx := range absentIndexes {
		var idxName string
		err := db2.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&idxName)
		if err == nil {
			t.Errorf("index %q should not exist after V10, but was found in sqlite_master", idx)
		}
	}

	// Spot-check: the first inserted row should have kind='transition'.
	var kind string
	if err := db2.QueryRow(
		`SELECT kind FROM ticket_log WHERE id = ?`, firstID,
	).Scan(&kind); err != nil {
		t.Fatalf("SELECT kind FROM ticket_log: %v", err)
	}
	if kind != "transition" {
		t.Errorf("ticket_log first row kind = %q, want %q", kind, "transition")
	}
}

// TestMigration_V9_KindConstraint verifies that the CHECK(kind) constraint
// rejects values outside ('transition', 'plan', 'message').
// After V10, the constraint lives on ticket_log (renamed from ticket_log_new).
func TestMigration_V9_KindConstraint(t *testing.T) {
	_, database := setupDBWithRoot(t)

	// Insert a session and ticket to satisfy FKs.
	if _, err := database.Exec(
		`INSERT INTO sessions (id, role, name) VALUES ('sess-kind', 'implementer', 'kindtester')`,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	res, err := database.Exec(`INSERT INTO tickets (title, created_by) VALUES ('kind test', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, _ := res.LastInsertId()

	_, insertErr := database.Exec(
		`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'sess-kind', 'usage', 'body')`,
		ticketID,
	)
	if insertErr == nil {
		t.Fatal("expected CHECK constraint violation for kind='usage', got nil error")
	}
	msg := strings.ToLower(insertErr.Error())
	if !strings.Contains(msg, "constraint") && !strings.Contains(msg, "check") {
		t.Errorf("unexpected error text %q — expected constraint/check violation", insertErr.Error())
	}
}

// TestMigration_V9_FKConstraint verifies that the FK on session_id rejects
// inserts referencing a non-existent session (PRAGMA foreign_keys = ON is set
// by db.Open before migrations run).
// After V10, the FK lives on ticket_log (renamed from ticket_log_new).
func TestMigration_V9_FKConstraint(t *testing.T) {
	_, database := setupDBWithRoot(t)

	res, err := database.Exec(`INSERT INTO tickets (title, created_by) VALUES ('fk test', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, _ := res.LastInsertId()

	_, insertErr := database.Exec(
		`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'ghost-sess', 'message', 'body')`,
		ticketID,
	)
	if insertErr == nil {
		t.Fatal("expected FK constraint violation for non-existent session_id, got nil error")
	}
	msg := strings.ToLower(insertErr.Error())
	if !strings.Contains(msg, "constraint") && !strings.Contains(msg, "foreign") {
		t.Errorf("unexpected error text %q — expected foreign key/constraint violation", insertErr.Error())
	}
}

// TestMigration_V9_VerifyFunction calls verifyV9Backfill directly with a tx
// where ticket_log has rows but ticket_log_new is empty and asserts the error
// contains "count mismatch".
// After V10, ticket_log_new no longer exists in the schema. This test recreates
// it within a transaction (which is rolled back) so verifyV9Backfill can run.
func TestMigration_V9_VerifyFunction(t *testing.T) {
	_, database := setupDBWithRoot(t)

	tx, err := database.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Recreate ticket_log_new within the transaction so verifyV9Backfill can query it.
	// This table is dropped after V10 (renamed to ticket_log), but the verify function
	// still needs it during the V9 migration step itself. Here we simulate that context.
	if _, err := tx.Exec(createTableTicketLogNew); err != nil {
		t.Fatalf("recreate ticket_log_new: %v", err)
	}

	// Insert a session and ticket to satisfy FKs.
	if _, err := tx.Exec(
		`INSERT INTO sessions (id, role, name) VALUES ('sess-v9vf', 'implementer', 'v9vftester')`,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	res, err := tx.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v9vf test', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, _ := res.LastInsertId()

	// Insert a row into ticket_log (source count = 1).
	if _, err := tx.Exec(
		`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'sess-v9vf', 'message', 'hello')`,
		ticketID,
	); err != nil {
		t.Fatalf("insert ticket_log: %v", err)
	}
	// ticket_log_new is empty (count = 0), so counts diverge: source=1, new=0.

	verifyErr := verifyV9Backfill(tx)
	if verifyErr == nil {
		t.Fatal("expected error from verifyV9Backfill, got nil")
	}
	if !strings.Contains(verifyErr.Error(), "count mismatch") {
		t.Errorf("expected error to contain 'count mismatch', got: %v", verifyErr)
	}
}

// TestMigration_V10_DropRenameRebuild verifies that V10 correctly drops the old
// ticket_log table, renames ticket_log_new to ticket_log, and recreates canonical
// indexes. It also asserts that the constraints from V9 survived the rename.
func TestMigration_V10_DropRenameRebuild(t *testing.T) {
	_, database := setupDBWithRoot(t)

	// Assert schema_version = 12.
	var version int
	if err := database.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	// Assert ticket_log_new does NOT exist (V10 renamed it to ticket_log).
	var newName string
	err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_log_new'`,
	).Scan(&newName)
	if err == nil {
		t.Errorf("ticket_log_new should not exist after V10, but was found in sqlite_master")
	}

	// Assert ticket_log DOES exist (the renamed table).
	var logName string
	if err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_log'`,
	).Scan(&logName); err != nil {
		t.Errorf("ticket_log not found in sqlite_master after V10: %v", err)
	}

	// Assert the four canonical indexes exist.
	canonicalIndexes := []string{
		"idx_ticket_log_ticket_id",
		"idx_ticket_log_kind",
		"idx_ticket_log_deleted_at",
		"idx_ticket_log_ticket_id_kind",
	}
	for _, idx := range canonicalIndexes {
		var idxName string
		if err := database.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&idxName); err != nil {
			t.Errorf("canonical index %q not found after V10: %v", idx, err)
		}
	}

	// Assert the four _new indexes do NOT exist.
	absentIndexes := []string{
		"idx_ticket_log_new_ticket_id",
		"idx_ticket_log_new_kind",
		"idx_ticket_log_new_deleted_at",
		"idx_ticket_log_new_ticket_id_kind",
	}
	for _, idx := range absentIndexes {
		var idxName string
		err := database.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&idxName)
		if err == nil {
			t.Errorf("index %q should not exist after V10, but was found in sqlite_master", idx)
		}
	}

	// Assert that the CHECK(kind) constraint survived the rename:
	// inserting kind='usage' into ticket_log must fail.
	if _, err := database.Exec(
		`INSERT INTO sessions (id, role, name) VALUES ('sess-v10', 'implementer', 'v10tester')`,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	res, err := database.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v10 test', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, _ := res.LastInsertId()

	_, checkErr := database.Exec(
		`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'sess-v10', 'usage', 'body')`,
		ticketID,
	)
	if checkErr == nil {
		t.Fatal("expected CHECK constraint violation for kind='usage', got nil error")
	}
	checkMsg := strings.ToLower(checkErr.Error())
	if !strings.Contains(checkMsg, "constraint") && !strings.Contains(checkMsg, "check") {
		t.Errorf("unexpected error %q — expected CHECK constraint violation", checkErr.Error())
	}

	// Assert that the FK(session_id) constraint survived the rename:
	// inserting a non-existent session_id must fail.
	_, fkErr := database.Exec(
		`INSERT INTO ticket_log (ticket_id, session_id, kind, body) VALUES (?, 'ghost-sess', 'message', 'body')`,
		ticketID,
	)
	if fkErr == nil {
		t.Fatal("expected FK constraint violation for non-existent session_id, got nil error")
	}
	fkMsg := strings.ToLower(fkErr.Error())
	if !strings.Contains(fkMsg, "constraint") && !strings.Contains(fkMsg, "foreign") {
		t.Errorf("unexpected error %q — expected FK constraint violation", fkErr.Error())
	}
}

// TestMigration_V11_Backfill seeds a V10 DB with a ticket_dependencies row,
// downgrades to V10, re-opens to trigger V11, then asserts count parity,
// table existence, and index existence.
func TestMigration_V11_Backfill(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Open once — lands at V11 (current schema).
	db1, err := Open(root)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}

	// Insert two tickets and a dependency row to satisfy FKs.
	res1, err := db1.Exec(`INSERT INTO tickets (title, created_by) VALUES ('dep src', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket 1: %v", err)
	}
	ticketA, _ := res1.LastInsertId()

	res2, err := db1.Exec(`INSERT INTO tickets (title, created_by) VALUES ('dep dst', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket 2: %v", err)
	}
	ticketB, _ := res2.LastInsertId()

	if _, err := db1.Exec(
		`INSERT INTO ticket_dependencies (ticket_id, depends_on) VALUES (?, ?)`,
		ticketA, ticketB,
	); err != nil {
		t.Fatalf("insert dependency: %v", err)
	}

	// Downgrade to V10: drop ticket_dependencies_new (if present) and reset version.
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS idx_ticket_dependencies_new_depends_on`,
		`DROP TABLE IF EXISTS ticket_dependencies_new`,
	} {
		if _, err := db1.Exec(stmt); err != nil {
			t.Fatalf("downgrade stmt %q: %v", stmt, err)
		}
	}
	if _, err := db1.Exec(`UPDATE schema_version SET version = 10`); err != nil {
		t.Fatalf("downgrade schema_version: %v", err)
	}
	db1.Close()

	// Re-open — triggers V11 and V12.
	db2, err := Open(root)
	if err != nil {
		t.Fatalf("second Open (V11 migration): %v", err)
	}
	defer db2.Close()

	var version int
	if err := db2.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	// After V12, ticket_dependencies_new is gone (renamed to ticket_dependencies).
	// Check count in the canonical ticket_dependencies instead.
	var count int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM ticket_dependencies`).Scan(&count); err != nil {
		t.Fatalf("COUNT ticket_dependencies: %v", err)
	}
	if count != 1 {
		t.Errorf("ticket_dependencies count = %d, want 1", count)
	}

	// Spot-check row values.
	var gotA, gotB int64
	if err := db2.QueryRow(
		`SELECT ticket_id, depends_on FROM ticket_dependencies WHERE ticket_id = ?`,
		ticketA,
	).Scan(&gotA, &gotB); err != nil {
		t.Fatalf("SELECT from ticket_dependencies: %v", err)
	}
	if gotA != ticketA || gotB != ticketB {
		t.Errorf("row mismatch: got (%d,%d), want (%d,%d)", gotA, gotB, ticketA, ticketB)
	}

	// idx_ticket_dependencies_depends_on must exist (canonical, after V12 rename).
	var idxName string
	if err := db2.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_ticket_dependencies_depends_on'`,
	).Scan(&idxName); err != nil {
		t.Errorf("idx_ticket_dependencies_depends_on not found: %v", err)
	}
}

func TestMigration_V11_VerifyFunction(t *testing.T) {
	_, database := setupDBWithRoot(t)

	tx, err := database.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// After V12 ticket_dependencies_new no longer exists in the schema.
	// Recreate it within the transaction so verifyV11Backfill can query it.
	if _, err := tx.Exec(createTableTicketDependenciesNew); err != nil {
		t.Fatalf("recreate ticket_dependencies_new: %v", err)
	}

	// Insert session and two tickets, then a dependency row into ticket_dependencies.
	if _, err := tx.Exec(
		`INSERT INTO sessions (id, role, name) VALUES ('sess-v11vf', 'implementer', 'v11vftester')`,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	res1, err := tx.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v11vf src', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket 1: %v", err)
	}
	srcID, _ := res1.LastInsertId()

	res2, err := tx.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v11vf dst', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket 2: %v", err)
	}
	dstID, _ := res2.LastInsertId()

	if _, err := tx.Exec(
		`INSERT INTO ticket_dependencies (ticket_id, depends_on) VALUES (?, ?)`,
		srcID, dstID,
	); err != nil {
		t.Fatalf("insert dependency: %v", err)
	}
	// ticket_dependencies_new is empty (0 rows). Source=1. Mismatch guaranteed.

	verifyErr := verifyV11Backfill(tx)
	if verifyErr == nil {
		t.Fatal("expected error from verifyV11Backfill, got nil")
	}
	if !strings.Contains(verifyErr.Error(), "count mismatch") {
		t.Errorf("expected error to contain 'count mismatch', got: %v", verifyErr)
	}
}

func TestMigration_V12_DropRenameRebuild(t *testing.T) {
	_, database := setupDBWithRoot(t)

	var version int
	if err := database.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	// ticket_dependencies_new must NOT exist after V12 (renamed to ticket_dependencies).
	var newName string
	err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_dependencies_new'`,
	).Scan(&newName)
	if err == nil {
		t.Errorf("ticket_dependencies_new should not exist after V12, but was found in sqlite_master")
	}

	// ticket_dependencies DOES exist.
	var depName string
	if err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_dependencies'`,
	).Scan(&depName); err != nil {
		t.Errorf("ticket_dependencies not found in sqlite_master after V12: %v", err)
	}

	// Canonical index exists.
	var idxName string
	if err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_ticket_dependencies_depends_on'`,
	).Scan(&idxName); err != nil {
		t.Errorf("idx_ticket_dependencies_depends_on not found after V12: %v", err)
	}

	// _new index does NOT exist.
	var newIdxName string
	err = database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_ticket_dependencies_new_depends_on'`,
	).Scan(&newIdxName)
	if err == nil {
		t.Errorf("idx_ticket_dependencies_new_depends_on should not exist after V12, but was found in sqlite_master")
	}

	// CHECK constraint survived rename: self-referencing insert must fail.
	res, err := database.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v12 test', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	ticketID, _ := res.LastInsertId()

	_, checkErr := database.Exec(
		`INSERT INTO ticket_dependencies (ticket_id, depends_on) VALUES (?, ?)`,
		ticketID, ticketID,
	)
	if checkErr == nil {
		t.Fatal("expected CHECK constraint violation for self-referencing dependency, got nil error")
	}
	msg := strings.ToLower(checkErr.Error())
	if !strings.Contains(msg, "constraint") && !strings.Contains(msg, "check") {
		t.Errorf("unexpected error %q — expected CHECK constraint violation", checkErr.Error())
	}
}

func TestMigration_V12_Backfill(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Open once — lands at V12.
	db1, err := Open(root)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}

	// Insert two tickets and a dependency row.
	res1, err := db1.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v12 src', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket 1: %v", err)
	}
	ticketA, _ := res1.LastInsertId()

	res2, err := db1.Exec(`INSERT INTO tickets (title, created_by) VALUES ('v12 dst', 'tester')`)
	if err != nil {
		t.Fatalf("insert ticket 2: %v", err)
	}
	ticketB, _ := res2.LastInsertId()

	if _, err := db1.Exec(
		`INSERT INTO ticket_dependencies (ticket_id, depends_on) VALUES (?, ?)`,
		ticketA, ticketB,
	); err != nil {
		t.Fatalf("insert dependency: %v", err)
	}

	// Downgrade to V11: restore ticket_dependencies (old TEXT schema) + ticket_dependencies_new
	// with the row, so V11's verifyV11Backfill passes and V12 can run cleanly.
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS idx_ticket_dependencies_depends_on`,
		`DROP TABLE IF EXISTS ticket_dependencies`,
		// Recreate old ticket_dependencies (TEXT schema) for V12's unconditional DROP to work.
		`CREATE TABLE ticket_dependencies (
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    depends_on  INTEGER NOT NULL REFERENCES tickets(id),
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (ticket_id, depends_on),
    CHECK (ticket_id != depends_on)
)`,
		`CREATE INDEX idx_ticket_dependencies_depends_on ON ticket_dependencies(depends_on)`,
		`DROP INDEX IF EXISTS idx_ticket_dependencies_new_depends_on`,
		`DROP TABLE IF EXISTS ticket_dependencies_new`,
		// Recreate ticket_dependencies_new (DATETIME schema) with the row for V11 to copy from.
		`CREATE TABLE ticket_dependencies_new (
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    depends_on  INTEGER NOT NULL REFERENCES tickets(id),
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (ticket_id, depends_on),
    CHECK (ticket_id != depends_on)
)`,
		`CREATE INDEX idx_ticket_dependencies_new_depends_on ON ticket_dependencies_new(depends_on)`,
	} {
		if _, err := db1.Exec(stmt); err != nil {
			t.Fatalf("downgrade stmt %q: %v", stmt, err)
		}
	}
	// Insert row into both tables for count parity.
	if _, err := db1.Exec(
		`INSERT INTO ticket_dependencies (ticket_id, depends_on) VALUES (?, ?)`,
		ticketA, ticketB,
	); err != nil {
		t.Fatalf("insert dep into ticket_dependencies: %v", err)
	}
	if _, err := db1.Exec(
		`INSERT INTO ticket_dependencies_new (ticket_id, depends_on) VALUES (?, ?)`,
		ticketA, ticketB,
	); err != nil {
		t.Fatalf("insert dep into ticket_dependencies_new: %v", err)
	}
	if _, err := db1.Exec(`UPDATE schema_version SET version = 11`); err != nil {
		t.Fatalf("downgrade schema_version: %v", err)
	}
	db1.Close()

	// Re-open — triggers V12.
	db2, err := Open(root)
	if err != nil {
		t.Fatalf("second Open (V12 migration): %v", err)
	}
	defer db2.Close()

	var version int
	if err := db2.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 12 {
		t.Errorf("schema_version = %d, want 12", version)
	}

	// ticket_dependencies_new must NOT exist.
	var newName string
	err = db2.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='ticket_dependencies_new'`).Scan(&newName)
	if err == nil {
		t.Errorf("ticket_dependencies_new should not exist after V12")
	}

	// Row must be accessible from ticket_dependencies.
	var count int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM ticket_dependencies WHERE ticket_id = ?`, ticketA).Scan(&count); err != nil {
		t.Fatalf("COUNT ticket_dependencies: %v", err)
	}
	if count != 1 {
		t.Errorf("ticket_dependencies count = %d, want 1", count)
	}
}
