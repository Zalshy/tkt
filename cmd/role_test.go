package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/role"
)

// runRoleCreateInDir sets rootDir and roleLike, calls runRoleCreate, captures stdout.
func runRoleCreateInDir(t *testing.T, dir, name, like string) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	savedLike := roleLike
	defer func() {
		rootDir = savedRootDir
		roleLike = savedLike
		roleCreateCmd.SetOut(nil)
	}()

	rootDir = dir
	roleLike = like

	var buf bytes.Buffer
	roleCreateCmd.SetOut(&buf)

	err := runRoleCreate(roleCreateCmd, []string{name})
	return buf.String(), err
}

// runRoleListInDir sets rootDir, calls runRoleList, captures stdout.
func runRoleListInDir(t *testing.T, dir string) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	defer func() {
		rootDir = savedRootDir
		roleListCmd.SetOut(nil)
	}()

	rootDir = dir

	var buf bytes.Buffer
	roleListCmd.SetOut(&buf)

	err := runRoleList(roleListCmd, nil)
	return buf.String(), err
}

// runRoleDeleteInDir sets rootDir, calls runRoleDelete, captures stdout.
func runRoleDeleteInDir(t *testing.T, dir, name string) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	defer func() {
		rootDir = savedRootDir
		roleDeleteCmd.SetOut(nil)
	}()

	rootDir = dir

	var buf bytes.Buffer
	roleDeleteCmd.SetOut(&buf)

	err := runRoleDelete(roleDeleteCmd, []string{name})
	return buf.String(), err
}

// TestRoleCreate_Valid verifies that creating a new role succeeds with the expected output.
func TestRoleCreate_Valid(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, err := runRoleCreateInDir(t, dir, "ops", "architect")
	if err != nil {
		t.Fatalf("runRoleCreate: %v", err)
	}

	want := "Role 'ops' created (behaves like architect)."
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in output, got: %q", want, out)
	}
}

// TestRoleCreate_Duplicate verifies that creating a role that already exists returns ErrAlreadyExists.
func TestRoleCreate_Duplicate(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, err := runRoleCreateInDir(t, dir, "ops", "architect"); err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err := runRoleCreateInDir(t, dir, "ops", "architect")
	if err == nil {
		t.Fatal("expected error for duplicate role, got nil")
	}
	if !strings.Contains(err.Error(), role.ErrAlreadyExists.Error()) {
		t.Errorf("expected ErrAlreadyExists in error, got: %v", err)
	}
}

// TestRoleCreate_BuiltIn verifies that creating a role named "architect" returns ErrBuiltIn.
func TestRoleCreate_BuiltIn(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runRoleCreateInDir(t, dir, "architect", "architect")
	if err == nil {
		t.Fatal("expected error for built-in role name, got nil")
	}
	if !strings.Contains(err.Error(), role.ErrBuiltIn.Error()) {
		t.Errorf("expected ErrBuiltIn in error, got: %v", err)
	}
}

// TestRoleList_ContainsBuiltIns verifies that list output contains architect and (built-in).
func TestRoleList_ContainsBuiltIns(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, err := runRoleListInDir(t, dir)
	if err != nil {
		t.Fatalf("runRoleList: %v", err)
	}

	if !strings.Contains(out, "architect") {
		t.Errorf("expected 'architect' in output, got: %q", out)
	}
	if !strings.Contains(out, "(built-in)") {
		t.Errorf("expected '(built-in)' in output, got: %q", out)
	}
}

// TestRoleList_CustomRoleAppears verifies a custom role appears without (built-in).
func TestRoleList_CustomRoleAppears(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, err := runRoleCreateInDir(t, dir, "ops", "architect"); err != nil {
		t.Fatalf("create role: %v", err)
	}

	out, err := runRoleListInDir(t, dir)
	if err != nil {
		t.Fatalf("runRoleList: %v", err)
	}

	if !strings.Contains(out, "ops") {
		t.Errorf("expected 'ops' in output, got: %q", out)
	}
	// The ops row must not have (built-in).
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "ops") && strings.Contains(line, "(built-in)") {
			t.Errorf("custom role 'ops' should not have (built-in) suffix, got line: %q", line)
		}
	}
}

// TestRoleDelete_Valid verifies that deleting a custom role succeeds with the expected output.
func TestRoleDelete_Valid(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, err := runRoleCreateInDir(t, dir, "ops", "architect"); err != nil {
		t.Fatalf("create role: %v", err)
	}

	out, err := runRoleDeleteInDir(t, dir, "ops")
	if err != nil {
		t.Fatalf("runRoleDelete: %v", err)
	}

	want := "Role 'ops' deleted."
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in output, got: %q", want, out)
	}
}

// TestRoleDelete_NotFound verifies that deleting a non-existent role returns "not found".
func TestRoleDelete_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runRoleDeleteInDir(t, dir, "ghost")
	if err == nil {
		t.Fatal("expected error for non-existent role, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// TestRoleDelete_BuiltIn verifies that deleting a built-in role returns "built-in".
func TestRoleDelete_BuiltIn(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runRoleDeleteInDir(t, dir, "architect")
	if err == nil {
		t.Fatal("expected error for built-in role deletion, got nil")
	}
	if !strings.Contains(err.Error(), "built-in") {
		t.Errorf("expected 'built-in' in error, got: %v", err)
	}
}

// TestRoleDelete_InUse verifies that deleting a role with an active session succeeds
// by expiring those sessions first (no longer blocked by ErrInUse).
func TestRoleDelete_InUse(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Create a custom role.
	if _, err := runRoleCreateInDir(t, dir, "ops", "architect"); err != nil {
		t.Fatalf("create role: %v", err)
	}

	// Insert an active session using the custom role directly into the DB.
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active) VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		"ops-test-0001", "ops", "test",
	)
	database.Close()
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Delete should now succeed — active sessions are expired automatically.
	_, err = runRoleDeleteInDir(t, dir, "ops")
	if err != nil {
		t.Fatalf("expected delete to succeed, got: %v", err)
	}
}

// TestSession_CreateCustomRole verifies that a session can be created with a custom role.
func TestSession_CreateCustomRole(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Create the custom role directly via the internal package.
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := role.Create("ops", "architect", database); err != nil {
		database.Close()
		t.Fatalf("role.Create: %v", err)
	}
	database.Close()

	// Now start a session with the custom role.
	out, err := runSessionInDir(t, dir, func() { sessionRole = "ops" })
	if err != nil {
		t.Fatalf("runSession with custom role: %v", err)
	}

	if !strings.Contains(out, "Session created") {
		t.Errorf("expected 'Session created' in output, got: %q", out)
	}
}
