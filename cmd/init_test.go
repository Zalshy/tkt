package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/config"
)

// runInitInDir sets rootDir to dir, resets it after the test, then invokes runInit.
func runInitInDir(t *testing.T, dir string) error {
	t.Helper()
	old := rootDir
	rootDir = dir
	t.Cleanup(func() { rootDir = old })
	return runInit(initCmd, nil)
}

// TestInit_Fresh verifies a clean initialisation creates all expected artifacts.
func TestInit_Fresh(t *testing.T) {
	dir := t.TempDir()

	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// .tkt/ directory must exist.
	tktDir := filepath.Join(dir, ".tkt")
	if _, err := os.Stat(tktDir); err != nil {
		t.Errorf(".tkt/ not created: %v", err)
	}

	// db.sqlite must exist.
	dbPath := filepath.Join(tktDir, "db.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("db.sqlite not created: %v", err)
	}

	// config.json must exist and be valid.
	cfgPath := filepath.Join(tktDir, "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config.json not created: %v", err)
	}
	var cfg config.ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config.json invalid JSON: %v", err)
	}
	if cfg.Name == "" {
		t.Error("config.json: name is empty")
	}
	if cfg.CreatedAt == "" {
		t.Error("config.json: created_at is empty")
	}
}

// TestInit_AlreadyExists verifies double-init is rejected.
func TestInit_AlreadyExists(t *testing.T) {
	dir := t.TempDir()

	// First init — must succeed.
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	// Second init — must return an error, not call os.Exit.
	err := runInitInDir(t, dir)
	if err == nil {
		t.Fatal("expected error on second init, got nil")
	}
	if !strings.Contains(err.Error(), ".tkt/ already exists") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestInit_GitignoreCreated verifies a .gitignore is created when absent.
func TestInit_GitignoreCreated(t *testing.T) {
	dir := t.TempDir()

	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore not created: %v", err)
	}
	if !strings.Contains(string(data), ".tkt/") {
		t.Errorf(".gitignore does not contain .tkt/: %q", string(data))
	}
}

// TestInit_GitignoreAppended verifies .tkt/ is appended to an existing .gitignore.
func TestInit_GitignoreAppended(t *testing.T) {
	dir := t.TempDir()

	existing := "node_modules/\ndist/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error("original content removed from .gitignore")
	}
	if !strings.Contains(content, ".tkt/") {
		t.Errorf(".tkt/ not appended to .gitignore: %q", content)
	}
}

// TestInit_GitignoreSkipped verifies no duplicate is appended when .tkt/ is present.
func TestInit_GitignoreSkipped(t *testing.T) {
	dir := t.TempDir()

	existing := "node_modules/\n.tkt/\n"
	giPath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(giPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(giPath)
	if err != nil {
		t.Fatal(err)
	}
	// Count occurrences — must be exactly one.
	count := strings.Count(string(data), ".tkt/")
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of .tkt/ in .gitignore, got %d: %q", count, string(data))
	}
}

// TestInit_NameFlag verifies --name sets the project name in config.json.
func TestInit_NameFlag(t *testing.T) {
	dir := t.TempDir()

	old := initName
	initName = "myproject"
	t.Cleanup(func() { initName = old })

	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".tkt", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "myproject" {
		t.Errorf("expected name 'myproject', got %q", cfg.Name)
	}
}

// TestInit_DefaultName verifies the directory basename is used when --name is omitted.
func TestInit_DefaultName(t *testing.T) {
	// Use a temp dir with a known basename.
	parent := t.TempDir()
	dir := filepath.Join(parent, "myrepo")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".tkt", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "myrepo" {
		t.Errorf("expected default name 'myrepo', got %q", cfg.Name)
	}
}
