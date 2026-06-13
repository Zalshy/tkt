package session

import (
	"database/sql"
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

// setupDB is the real helper used by integration tests.
func setupDB(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir .tkt: %v", err)
	}
	return root
}

// ---------------------------------------------------------------------------
// GenerateName / GenerateULID tests
// ---------------------------------------------------------------------------

func TestGenerateName_RandomWord(t *testing.T) {
	id := GenerateName("")
	found := false
	for _, w := range wordlist {
		if id == w {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GenerateName(\"\") = %q, not found in wordlist", id)
	}
}

func TestGenerateName_RandomUnique(t *testing.T) {
	seen := make(map[string]bool)
	const calls = 30
	for i := 0; i < calls; i++ {
		seen[GenerateName("")] = true
	}
	if len(seen) < 2 {
		t.Errorf("GenerateName produced only 1 distinct value in %d calls — wordlist or RNG broken", calls)
	}
}

func TestGenerateName_NamePassthrough(t *testing.T) {
	for _, name := range []string{"oak", "my-session", "foo123", "ab"} {
		got := GenerateName(name)
		if got != name {
			t.Errorf("GenerateName(%q) = %q, want %q", name, got, name)
		}
	}
}

func TestGenerateULID_NonEmpty(t *testing.T) {
	if got := GenerateULID(); len(got) != 26 {
		t.Fatalf("GenerateULID() len = %d, want 26", len(got))
	}
}

func TestCandidateNames_AlwaysSingleHexSuffix(t *testing.T) {
	candidates, err := candidateNames("cedar")
	if err != nil {
		t.Fatalf("candidateNames: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("len(candidateNames) = %d, want 2", len(candidates))
	}
	pattern := regexp.MustCompile(`^cedar-[0-9a-f]{4}$`)
	for _, candidate := range candidates {
		if !pattern.MatchString(candidate) {
			t.Fatalf("candidate %q does not match single-suffix format", candidate)
		}
		if strings.Count(candidate, "-") != 1 {
			t.Fatalf("candidate %q has bare or double-suffix shape", candidate)
		}
	}
}

// ---------------------------------------------------------------------------
// ValidateName tests
// ---------------------------------------------------------------------------

func TestValidateName_Valid(t *testing.T) {
	cases := []string{
		"oak",
		"my-session",
		"foo123",
		"ab",
		"a",
		"cedar-oak",
		strings.Repeat("a", 32),
	}
	for _, name := range cases {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) returned unexpected error: %v", name, err)
		}
	}
}

func TestValidateName_Invalid(t *testing.T) {
	cases := []struct {
		name string
		desc string
	}{
		{"", "empty"},
		{strings.Repeat("a", 33), "too long (33 chars)"},
		{"-foo", "leading hyphen"},
		{"foo-", "trailing hyphen"},
		{"foo--bar", "consecutive hyphens"},
		{"FOO", "uppercase"},
		{"foo bar", "space"},
		{"foo_bar", "underscore"},
	}
	for _, tc := range cases {
		if err := ValidateName(tc.name); err == nil {
			t.Errorf("ValidateName(%q) returned nil, want error (%s)", tc.name, tc.desc)
		}
	}
}

// ---------------------------------------------------------------------------
// Create tests (updated: Create now takes an extra name string param)
// ---------------------------------------------------------------------------

func TestCreate_CustomArchitectRole_IDIsULID(t *testing.T) {
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

	s, err := Create(models.Role("security_expert"), "", sqlDB, root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// New IDs are ULIDs; session names stay human-readable.
	if strings.HasPrefix(s.ID, "arch-") || strings.HasPrefix(s.ID, "impl-") {
		t.Errorf("Create with security_expert: ID = %q still uses old role-prefix format", s.ID)
	}
	if len(s.ID) != 26 {
		t.Errorf("Create with security_expert: ID len = %d, want 26", len(s.ID))
	}
}

// TestCreate_WithName verifies that passing a name uses it as the session name.
func TestCreate_WithName(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	s, err := Create(models.RoleImplementer, "my-session", sqlDB, root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.ID == "my-session" {
		t.Errorf("Create with name: ID should be ULID, got %q", s.ID)
	}
	if s.Name != "my-session" {
		t.Errorf("Create with name: Name = %q, want %q", s.Name, "my-session")
	}
}

// TestCreate_NameDiffersFromID verifies that generated names are separate from ULID ids.
func TestCreate_NameDiffersFromID(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	s, err := Create(models.RoleArchitect, "", sqlDB, root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.Name == s.ID {
		t.Errorf("s.Name = %q, s.ID = %q — should differ after ULID split", s.Name, s.ID)
	}
}

func TestCreate_AutoNameHasSingleHexSuffix(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	s, err := Create(models.RoleArchitect, "", sqlDB, root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertGeneratedSessionName(t, s.Name)
}

func TestCreate_Roundtrip(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	s, err := Create(models.RoleImplementer, "", sqlDB, root)
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

func TestCreate_ExpiresStaleSessions(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	_, err = sqlDB.Exec(
		`INSERT INTO sessions (id, role, name, last_active) VALUES (?, ?, ?, datetime('now', '-5 hours'))`,
		"stale-session", "implementer", "stale-session",
	)
	if err != nil {
		t.Fatalf("insert stale session: %v", err)
	}
	_, err = sqlDB.Exec(
		`INSERT INTO sessions (id, role, name, last_active) VALUES (?, ?, ?, datetime('now', '-3 hours'))`,
		"fresh-session", "implementer", "fresh-session",
	)
	if err != nil {
		t.Fatalf("insert fresh session: %v", err)
	}

	if _, err := Create(models.RoleArchitect, "new-session", sqlDB, root); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var staleExpired, freshExpired sql.NullString
	if err := sqlDB.QueryRow(`SELECT expired_at FROM sessions WHERE id = ?`, "stale-session").Scan(&staleExpired); err != nil {
		t.Fatalf("select stale expired_at: %v", err)
	}
	if err := sqlDB.QueryRow(`SELECT expired_at FROM sessions WHERE id = ?`, "fresh-session").Scan(&freshExpired); err != nil {
		t.Fatalf("select fresh expired_at: %v", err)
	}
	if !staleExpired.Valid {
		t.Error("expected stale session to be expired during Create")
	}
	if freshExpired.Valid {
		t.Errorf("expected fresh session to remain active, got expired_at=%q", freshExpired.String)
	}
}

func TestLoadActive_ReturnsSession(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	created, err := Create(models.RoleArchitect, "", sqlDB, root)
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
	expiredID := "expired-test"
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

	s, err := Create(models.Role("security_expert"), "", sqlDB, root)
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
		`INSERT INTO sessions (id, role, name) VALUES ('ghost-session', 'ghost_role', 'ghost')`,
	); err != nil {
		t.Fatalf("insert ghost session: %v", err)
	}
	if err := os.WriteFile(project.SessionFile(root), []byte("ghost-session"), 0o644); err != nil {
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

	_, err = Create(models.Role("phantom"), "", sqlDB, root)
	if err == nil {
		t.Fatal("Create succeeded with unregistered role, want error")
	}
	if !strings.Contains(err.Error(), "phantom") && !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want it to mention %q or %q", err.Error(), "phantom", "not registered")
	}
}

// ---------------------------------------------------------------------------
// CreateSystem tests
// ---------------------------------------------------------------------------

// TestCreateSystem_InsertsRowNoFile verifies that CreateSystem inserts a DB row,
// does NOT write a session file, and populates EffectiveRole correctly.
func TestCreateSystem_InsertsRowNoFile(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	s, err := CreateSystem(models.RoleMonitor, sqlDB)
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}

	// Row must exist in sessions table.
	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, s.ID).Scan(&count); err != nil {
		t.Fatalf("query session row: %v", err)
	}
	if count != 1 {
		t.Errorf("sessions row count = %d, want 1", count)
	}

	// Session file must NOT exist.
	sessionFilePath := project.SessionFile(root)
	if _, statErr := os.Stat(sessionFilePath); statErr == nil {
		t.Error("session file was written — CreateSystem must not write session file")
	}

	// EffectiveRole must be RoleMonitor (resolved via ResolveBase).
	if s.EffectiveRole != models.RoleMonitor {
		t.Errorf("EffectiveRole = %q, want %q", s.EffectiveRole, models.RoleMonitor)
	}
	assertGeneratedSessionName(t, s.Name)
}

func TestInsertWithNameFallback_RetriesOneCollision(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	if _, err := sqlDB.Exec(`INSERT INTO sessions (id, role, name) VALUES ('existing-id', 'architect', 'cedar-1111')`); err != nil {
		t.Fatalf("insert existing session: %v", err)
	}

	s := models.Session{ID: "new-id", Role: models.RoleArchitect, EffectiveRole: models.RoleArchitect}
	if err := insertWithNameFallback(&s, []string{"cedar-1111", "cedar-2222"}, sqlDB, "test"); err != nil {
		t.Fatalf("insertWithNameFallback: %v", err)
	}
	if s.Name != "cedar-2222" {
		t.Fatalf("s.Name = %q, want second candidate", s.Name)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ? AND name = ?`, "new-id", "cedar-2222").Scan(&count); err != nil {
		t.Fatalf("query inserted session: %v", err)
	}
	if count != 1 {
		t.Fatalf("inserted row count = %d, want 1", count)
	}
}

func TestInsertWithNameFallback_TwoCollisionsReturnsError(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	for _, row := range []struct{ id, name string }{{"existing-a", "cedar-1111"}, {"existing-b", "cedar-2222"}} {
		if _, err := sqlDB.Exec(`INSERT INTO sessions (id, role, name) VALUES (?, 'architect', ?)`, row.id, row.name); err != nil {
			t.Fatalf("insert existing session %s: %v", row.id, err)
		}
	}

	s := models.Session{ID: "new-id", Role: models.RoleArchitect, EffectiveRole: models.RoleArchitect}
	err = insertWithNameFallback(&s, []string{"cedar-1111", "cedar-2222"}, sqlDB, "test")
	if err == nil {
		t.Fatal("insertWithNameFallback succeeded, want constraint error")
	}
	if !isPKConstraintError(err) && !strings.Contains(err.Error(), "constraint") && !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatalf("error = %v, want constraint error", err)
	}
}

func assertGeneratedSessionName(t *testing.T, name string) {
	t.Helper()
	pattern := regexp.MustCompile(`^[a-z]+-[0-9a-f]{4}$`)
	if !pattern.MatchString(name) {
		t.Fatalf("generated session name %q does not match word-xxxx format", name)
	}
	if strings.Count(name, "-") != 1 {
		t.Fatalf("generated session name %q has bare or double-suffix shape", name)
	}
}

// ---------------------------------------------------------------------------
// ExpireByID tests
// ---------------------------------------------------------------------------

// TestExpireByID_SetsExpiredAt verifies that ExpireByID sets expired_at on the row.
func TestExpireByID_SetsExpiredAt(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	// Insert a session row directly.
	const sessID = "expire-test-session"
	if _, err := sqlDB.Exec(
		`INSERT INTO sessions (id, role, name) VALUES (?, 'monitor', 'test')`, sessID,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	if err := ExpireByID(sessID, sqlDB); err != nil {
		t.Fatalf("ExpireByID: %v", err)
	}

	var expiredAt *string
	if err := sqlDB.QueryRow(`SELECT expired_at FROM sessions WHERE id = ?`, sessID).Scan(&expiredAt); err != nil {
		t.Fatalf("query expired_at: %v", err)
	}
	if expiredAt == nil {
		t.Error("expired_at is NULL after ExpireByID — want it set")
	}
}

// TestExpireByID_NoRowIdempotent verifies that ExpireByID with an unknown ID returns no error.
func TestExpireByID_NoRowIdempotent(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	if err := ExpireByID("nonexistent-id", sqlDB); err != nil {
		t.Errorf("ExpireByID with unknown ID: got error %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// LoadByName tests
// ---------------------------------------------------------------------------

// TestLoadByName_ReturnsActiveSession verifies that LoadByName finds an active session by name.
func TestLoadByName_ReturnsActiveSession(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	s, err := Create(models.RoleArchitect, "alice-arch", sqlDB, root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	loaded, err := LoadByName("alice-arch", sqlDB)
	if err != nil {
		t.Fatalf("LoadByName: %v", err)
	}
	if loaded.ID != s.ID {
		t.Errorf("loaded ID = %q, want %q", loaded.ID, s.ID)
	}
	if loaded.Name != "alice-arch" {
		t.Errorf("loaded Name = %q, want %q", loaded.Name, "alice-arch")
	}
	if loaded.EffectiveRole != models.RoleArchitect {
		t.Errorf("loaded EffectiveRole = %q, want %q", loaded.EffectiveRole, models.RoleArchitect)
	}
}

// TestLoadByName_ErrNoSession verifies that LoadByName returns ErrNoSession for unknown name.
func TestLoadByName_ErrNoSession(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	_, err = LoadByName("no-such-session", sqlDB)
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("expected ErrNoSession, got %v", err)
	}
}

// TestLoadByName_ExpiredSession verifies that LoadByName returns ErrNoSession for expired sessions.
func TestLoadByName_ExpiredSession(t *testing.T) {
	root := setupDB(t)
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	// Insert an expired session with a known name.
	if _, err := sqlDB.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active, expired_at)
		 VALUES ('expired-id', 'architect', 'expired-name', datetime('now'), datetime('now'), datetime('now'))`,
	); err != nil {
		t.Fatalf("insert expired session: %v", err)
	}

	_, err = LoadByName("expired-name", sqlDB)
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("expected ErrNoSession for expired session, got %v", err)
	}
}
