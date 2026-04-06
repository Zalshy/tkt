package session

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/project"
	rolepkg "github.com/zalshy/tkt/internal/role"
)

// openTestDB opens a fresh DB in a temp .tkt/ directory and returns both the root
// path and the open *sql.DB. It registers cleanup automatically.
func openTestDB(t *testing.T) (root string, sqlDB interface {
	Exec(query string, args ...any) (interface{ LastInsertId() (int64, error) }, error)
}) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("setup mkdir .tkt: %v", err)
	}
	return root, nil
}

// setupDB is the real helper used by integration tests.
func setupDB(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(project.TicketsDir(root), 0o755); err != nil {
		t.Fatalf("mkdir .tkt: %v", err)
	}
	return root
}

var idPattern = regexp.MustCompile(`^(arch|impl)-[a-z0-9]+-[0-9a-f]{4}$`)

func TestGenerateID_Format(t *testing.T) {
	for _, role := range []models.Role{models.RoleArchitect, models.RoleImplementer} {
		id := GenerateID(role)
		if !idPattern.MatchString(id) {
			t.Errorf("GenerateID(%s) = %q, does not match pattern %s", role, id, idPattern)
		}
	}
}

func TestGenerateID_Unique(t *testing.T) {
	id1 := GenerateID(models.RoleImplementer)
	id2 := GenerateID(models.RoleImplementer)
	if id1 == id2 {
		t.Errorf("GenerateID returned the same ID twice: %q", id1)
	}
}

func TestGenerateID_Prefix(t *testing.T) {
	tests := []struct {
		baseRole models.Role
		wantPfx  string
	}{
		{models.RoleArchitect, "arch"},
		{models.RoleImplementer, "impl"},
	}
	for _, tt := range tests {
		id := GenerateID(tt.baseRole)
		if !strings.HasPrefix(id, tt.wantPfx+"-") {
			t.Errorf("GenerateID(%s) = %q, want prefix %q", tt.baseRole, id, tt.wantPfx+"-")
		}
	}
}

// TestCreate_CustomArchitectRole_IDPrefix verifies that a session created with a custom
// role whose base_role is 'architect' receives an 'arch-' prefixed ID, not 'impl-'.
func TestCreate_CustomArchitectRole_IDPrefix(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	if _, err := sqlDB.Exec(
		`INSERT INTO roles (name, base_role) VALUES ('security_expert', 'architect')`,
	); err != nil {
		t.Fatalf("insert custom role: %v", err)
	}

	s, err := Create(models.Role("security_expert"), sqlDB, root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(s.ID, "arch-") {
		t.Errorf("Create with security_expert (base=architect): ID = %q, want arch- prefix", s.ID)
	}
}

func TestCreate_Roundtrip(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	s, err := Create(models.RoleImplementer, sqlDB, root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if s.Role != models.RoleImplementer {
		t.Errorf("Session.Role = %q, want %q", s.Role, models.RoleImplementer)
	}

	// Session file must exist and contain exactly Session.ID with no trailing whitespace.
	data, err := os.ReadFile(project.SessionFile(root))
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}
	if string(data) != s.ID {
		t.Errorf("session file contains %q, want %q (no trailing bytes)", string(data), s.ID)
	}
	if strings.TrimSpace(string(data)) != string(data) {
		t.Errorf("session file has leading/trailing whitespace: %q", string(data))
	}
	if len(data) != len(s.ID) {
		t.Errorf("session file byte length %d != session ID length %d", len(data), len(s.ID))
	}
}

func TestLoadActive_ReturnsSession(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	created, err := Create(models.RoleArchitect, sqlDB, root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	loaded, err := LoadActive(root, sqlDB)
	if err != nil {
		t.Fatalf("LoadActive: %v", err)
	}
	if loaded.ID != created.ID {
		t.Errorf("loaded ID = %q, want %q", loaded.ID, created.ID)
	}
	// LastActive should be populated (non-zero).
	if loaded.LastActive.IsZero() {
		t.Error("LastActive is zero — expected it to be updated")
	}
}

func TestLoadActive_ErrNoSession(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	_, err = LoadActive(root, sqlDB)
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("expected ErrNoSession, got %v", err)
	}
}

func TestLoadActive_ExpiredSession(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	// Insert an expired session directly.
	expiredID := "arch-test-aaaa"
	if _, err := sqlDB.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active, expired_at)
		 VALUES (?, 'architect', 'test', datetime('now'), datetime('now'), datetime('2000-01-01'))`,
		expiredID,
	); err != nil {
		t.Fatalf("insert expired session: %v", err)
	}

	// Write its ID to the session file.
	if err := os.WriteFile(project.SessionFile(root), []byte(expiredID), 0o644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	_, err = LoadActive(root, sqlDB)
	if !errors.Is(err, ErrExpiredSession) {
		t.Errorf("expected ErrExpiredSession, got %v", err)
	}
}

// TestLoadActive_CustomRole_EffectiveRole verifies that a session created with a custom
// role whose base_role is 'architect' has EffectiveRole == RoleArchitect after LoadActive.
func TestLoadActive_CustomRole_EffectiveRole(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	// Insert a custom role with base_role = 'architect'.
	if _, err := sqlDB.Exec(
		`INSERT INTO roles (name, base_role) VALUES ('security_expert', 'architect')`,
	); err != nil {
		t.Fatalf("insert custom role: %v", err)
	}

	s, err := Create(models.Role("security_expert"), sqlDB, root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.Role != models.Role("security_expert") {
		t.Errorf("Create: Role = %q, want %q", s.Role, "security_expert")
	}
	if s.EffectiveRole != models.RoleArchitect {
		t.Errorf("Create: EffectiveRole = %q, want %q", s.EffectiveRole, models.RoleArchitect)
	}

	loaded, err := LoadActive(root, sqlDB)
	if err != nil {
		t.Fatalf("LoadActive: %v", err)
	}
	if loaded.Role != models.Role("security_expert") {
		t.Errorf("LoadActive: Role = %q, want %q", loaded.Role, "security_expert")
	}
	if loaded.EffectiveRole != models.RoleArchitect {
		t.Errorf("LoadActive: EffectiveRole = %q, want %q", loaded.EffectiveRole, models.RoleArchitect)
	}
}

// TestLoadActive_UnknownRole_Error verifies that LoadActive fails with a wrapped
// ErrNotFound when the session's role no longer exists in the roles table.
func TestLoadActive_UnknownRole_Error(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	// Insert a session row directly using a role not present in the roles table.
	if _, err := sqlDB.Exec(
		`INSERT INTO sessions (id, role, name) VALUES ('arch-ghost-0000', 'ghost_role', 'ghost')`,
	); err != nil {
		t.Fatalf("insert ghost session: %v", err)
	}
	if err := os.WriteFile(project.SessionFile(root), []byte("arch-ghost-0000"), 0o644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	_, err = LoadActive(root, sqlDB)
	if err == nil {
		t.Fatal("LoadActive succeeded with unknown role, want error")
	}
	if !errors.Is(err, rolepkg.ErrNotFound) {
		t.Errorf("expected errors.Is(err, role.ErrNotFound), got %v", err)
	}
}

// TestCreate_UnregisteredRole_Error verifies that Create fails when the requested role
// is not present in the roles table.
func TestCreate_UnregisteredRole_Error(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	_, err = Create(models.Role("phantom"), sqlDB, root)
	if err == nil {
		t.Fatal("Create succeeded with unregistered role, want error")
	}
	if !strings.Contains(err.Error(), "phantom") && !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want it to mention %q or %q", err.Error(), "phantom", "not registered")
	}
}
