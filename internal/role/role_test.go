package role

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/zalshy/tkt/internal/models"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// setupTestDB opens a fresh in-memory SQLite DB, creates the roles and sessions
// tables, and seeds the two built-in roles. It is inlined here to keep the
// test hermetic — no dependency on internal/db or migration logic.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ddl := []string{
		`CREATE TABLE sessions (
			id          TEXT PRIMARY KEY,
			role        TEXT NOT NULL,
			name        TEXT NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			last_active DATETIME NOT NULL DEFAULT (datetime('now')),
			expired_at  DATETIME NULL
		)`,
		`CREATE TABLE roles (
			name       TEXT PRIMARY KEY,
			base_role  TEXT NOT NULL,
			is_builtin INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			CHECK (base_role IN ('architect', 'implementer'))
		)`,
		`INSERT INTO roles (name, base_role, is_builtin) VALUES ('architect',   'architect',   1)`,
		`INSERT INTO roles (name, base_role, is_builtin) VALUES ('implementer', 'implementer', 1)`,
	}

	for _, stmt := range ddl {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setupTestDB: %v\nStatement: %s", err, stmt)
		}
	}

	return db
}

// TestCreate_Valid verifies that a valid role is inserted correctly.
func TestCreate_Valid(t *testing.T) {
	db := setupTestDB(t)

	if err := Create("ops", "architect", db); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	var baseRole string
	var isBuiltin int
	err := db.QueryRow(`SELECT base_role, is_builtin FROM roles WHERE name = 'ops'`).Scan(&baseRole, &isBuiltin)
	if err != nil {
		t.Fatalf("query after Create: %v", err)
	}
	if baseRole != "architect" {
		t.Errorf("base_role = %q, want %q", baseRole, "architect")
	}
	if isBuiltin != 0 {
		t.Errorf("is_builtin = %d, want 0", isBuiltin)
	}
}

// TestCreate_Duplicate verifies that a second Create with the same name returns ErrAlreadyExists.
func TestCreate_Duplicate(t *testing.T) {
	db := setupTestDB(t)

	if err := Create("ops", "architect", db); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	err := Create("ops", "architect", db)
	if err == nil {
		t.Fatal("second Create: expected error, got nil")
	}
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("second Create: got %v, want to wrap ErrAlreadyExists", err)
	}
}

// TestCreate_BuiltInGuard verifies that attempting to create a role named "architect" returns ErrBuiltIn.
func TestCreate_BuiltInGuard(t *testing.T) {
	db := setupTestDB(t)

	err := Create("architect", "architect", db)
	if err == nil {
		t.Fatal("Create built-in: expected error, got nil")
	}
	if !errors.Is(err, ErrBuiltIn) {
		t.Errorf("Create built-in: got %v, want to wrap ErrBuiltIn", err)
	}
}

// TestDelete_NotFound verifies that deleting a non-existent role returns ErrNotFound.
func TestDelete_NotFound(t *testing.T) {
	db := setupTestDB(t)

	err := Delete("nonexistent", db)
	if err == nil {
		t.Fatal("Delete non-existent: expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete non-existent: got %v, want to wrap ErrNotFound", err)
	}
}

// TestDelete_BuiltIn verifies that deleting a built-in role returns ErrBuiltIn.
func TestDelete_BuiltIn(t *testing.T) {
	db := setupTestDB(t)

	err := Delete("architect", db)
	if err == nil {
		t.Fatal("Delete built-in: expected error, got nil")
	}
	if !errors.Is(err, ErrBuiltIn) {
		t.Errorf("Delete built-in: got %v, want to wrap ErrBuiltIn", err)
	}
}

// TestDelete_InUse verifies that deleting a role with an active session returns ErrInUse.
func TestDelete_InUse(t *testing.T) {
	db := setupTestDB(t)

	// Insert a user-defined role.
	if _, err := db.Exec(`INSERT INTO roles (name, base_role) VALUES ('ops', 'architect')`); err != nil {
		t.Fatalf("insert ops role: %v", err)
	}
	// Insert an active session holding that role (expired_at IS NULL).
	if _, err := db.Exec(`INSERT INTO sessions (id, role, name) VALUES ('sess-001', 'ops', 'tester')`); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	err := Delete("ops", db)
	if err == nil {
		t.Fatal("Delete in-use: expected error, got nil")
	}
	if !errors.Is(err, ErrInUse) {
		t.Errorf("Delete in-use: got %v, want to wrap ErrInUse", err)
	}
}

// TestList_Ordering verifies that List returns roles in strict ascending alphabetical order.
func TestList_Ordering(t *testing.T) {
	db := setupTestDB(t)

	// Insert two additional roles; "alpha" sorts before "architect", "zed" sorts after.
	for _, name := range []string{"zed", "alpha"} {
		if _, err := db.Exec(`INSERT INTO roles (name, base_role) VALUES (?, 'implementer')`, name); err != nil {
			t.Fatalf("insert role %q: %v", name, err)
		}
	}

	roles, err := List(db)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(roles) < 2 {
		t.Fatalf("List: expected at least 2 roles, got %d", len(roles))
	}
	for i := 1; i < len(roles); i++ {
		if roles[i].Name < roles[i-1].Name {
			t.Errorf("List: out of order at index %d: %q < %q", i, roles[i].Name, roles[i-1].Name)
		}
	}
}

// TestResolveBase covers both the not-found error and the happy path.
func TestResolveBase(t *testing.T) {
	db := setupTestDB(t)

	t.Run("not found", func(t *testing.T) {
		_, err := ResolveBase("ghost", db)
		if err == nil {
			t.Fatal("ResolveBase ghost: expected error, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("ResolveBase ghost: got %v, want to wrap ErrNotFound", err)
		}
	})

	t.Run("built-in architect", func(t *testing.T) {
		got, err := ResolveBase("architect", db)
		if err != nil {
			t.Fatalf("ResolveBase architect: unexpected error: %v", err)
		}
		if got != models.RoleArchitect {
			t.Errorf("ResolveBase architect: got %q, want %q", got, models.RoleArchitect)
		}
	})
}
