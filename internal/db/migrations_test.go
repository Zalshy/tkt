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

// TestMigration_FreshDB verifies that a brand-new DB ends up at schema_version=4
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

	// Assert schema_version = 4.
	var version int
	if err := database.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 5 {
		t.Errorf("schema_version = %d, want 5", version)
	}
}

// TestMigration_V2ToV3 verifies that a second Open on an already-migrated DB is
// a no-op: schema_version stays 4 and pre-seeded data survives.
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

	// Assert schema_version = 4.
	var version int
	if err := db2.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 5 {
		t.Errorf("schema_version = %d, want 5", version)
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
// Open must see version=4 and skip all migrations without error.
func TestMigration_Idempotency(t *testing.T) {
	root, db1 := setupDBWithRoot(t)

	var v1 int
	if err := db1.QueryRow(`SELECT version FROM schema_version`).Scan(&v1); err != nil {
		t.Fatalf("schema_version after first Open: %v", err)
	}
	if v1 != 5 {
		t.Errorf("schema_version after first Open = %d, want 5", v1)
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
	if v2 != 5 {
		t.Errorf("schema_version after second Open = %d, want 5", v2)
	}
}

// TestMigration_V4_RolesTableSeeded verifies that after migration V4 the roles
// table exists and contains exactly the two built-in rows.
func TestMigration_V4_RolesTableSeeded(t *testing.T) {
	_, database := setupDBWithRoot(t)

	// Assert schema_version = 4.
	var version int
	if err := database.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 5 {
		t.Errorf("schema_version = %d, want 5", version)
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
