package config

import (
	"os"
	"path/filepath"
	"testing"
)

// --- LoadGlobal tests ---

// TestLoadGlobal_Missing verifies that LoadGlobal returns defaults (GitignoreAuto=true,
// DefaultRole="") when no config file exists in the config directory.
func TestLoadGlobal_Missing(t *testing.T) {
	// Point XDG_CONFIG_HOME at an empty temp dir so no file is found.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal() returned unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadGlobal() returned nil config")
	}
	if !cfg.GitignoreAuto {
		t.Errorf("GitignoreAuto = false; want true (default)")
	}
	if cfg.DefaultRole != "" {
		t.Errorf("DefaultRole = %q; want empty string (default)", cfg.DefaultRole)
	}
}

// TestLoadGlobal_Explicit verifies that explicit values in the config file override
// the defaults, including GitignoreAuto=false overriding the true default.
func TestLoadGlobal_Explicit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Write a config that explicitly disables GitignoreAuto and sets a role.
	tktDir := filepath.Join(dir, "tkt")
	if err := os.MkdirAll(tktDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `{"gitignore_auto": false, "default_role": "architect"}`
	if err := os.WriteFile(filepath.Join(tktDir, "config.json"), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal() returned unexpected error: %v", err)
	}
	if cfg.GitignoreAuto {
		t.Errorf("GitignoreAuto = true; want false (explicitly set)")
	}
	if cfg.DefaultRole != "architect" {
		t.Errorf("DefaultRole = %q; want %q", cfg.DefaultRole, "architect")
	}
}

// --- LoadProject tests ---

// TestLoadProject_Missing verifies that LoadProject returns a non-nil error when
// .tkt/config.json does not exist — indicating an uninitialised project.
func TestLoadProject_Missing(t *testing.T) {
	root := t.TempDir()
	// No .tkt/ directory, no config file.

	cfg, err := LoadProject(root)
	if err == nil {
		t.Fatalf("LoadProject() returned nil error; want error for missing file")
	}
	if cfg != nil {
		t.Errorf("LoadProject() returned non-nil config alongside error")
	}
}

// --- WriteProject + LoadProject round-trip tests ---

// TestWriteAndLoadProject_RoundTrip writes a ProjectConfig and reads it back,
// asserting that all fields survive the serialise/deserialise cycle unchanged.
func TestWriteAndLoadProject_RoundTrip(t *testing.T) {
	root := t.TempDir()
	tktDir := filepath.Join(root, ".tkt")
	if err := os.Mkdir(tktDir, 0755); err != nil {
		t.Fatalf("mkdir .tkt: %v", err)
	}

	want := &ProjectConfig{
		Name:            "myproject",
		CreatedAt:       "2026-03-29T10:00:00Z",
		MonitorInterval: 5,
	}

	if err := WriteProject(root, want); err != nil {
		t.Fatalf("WriteProject() error: %v", err)
	}

	got, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject() error: %v", err)
	}

	if got.Name != want.Name {
		t.Errorf("Name = %q; want %q", got.Name, want.Name)
	}
	if got.CreatedAt != want.CreatedAt {
		t.Errorf("CreatedAt = %q; want %q", got.CreatedAt, want.CreatedAt)
	}
	if got.MonitorInterval != want.MonitorInterval {
		t.Errorf("MonitorInterval = %d; want %d", got.MonitorInterval, want.MonitorInterval)
	}
}

// TestWriteProject_Overwrite verifies that calling WriteProject twice with different
// values results in the second value being persisted (overwrite semantics).
func TestWriteProject_Overwrite(t *testing.T) {
	root := t.TempDir()
	tktDir := filepath.Join(root, ".tkt")
	if err := os.Mkdir(tktDir, 0755); err != nil {
		t.Fatalf("mkdir .tkt: %v", err)
	}

	first := &ProjectConfig{Name: "first", CreatedAt: "2026-01-01T00:00:00Z", MonitorInterval: 2}
	second := &ProjectConfig{Name: "second", CreatedAt: "2026-06-01T00:00:00Z", MonitorInterval: 10}

	if err := WriteProject(root, first); err != nil {
		t.Fatalf("first WriteProject() error: %v", err)
	}
	if err := WriteProject(root, second); err != nil {
		t.Fatalf("second WriteProject() error: %v", err)
	}

	got, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject() error: %v", err)
	}
	if got.Name != second.Name {
		t.Errorf("Name = %q; want %q (second write should win)", got.Name, second.Name)
	}
	if got.MonitorInterval != second.MonitorInterval {
		t.Errorf("MonitorInterval = %d; want %d", got.MonitorInterval, second.MonitorInterval)
	}
}
