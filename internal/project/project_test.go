package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindRoot_FromSubdirectory(t *testing.T) {
	// Create temp root with .tkt/ and a nested subdir two levels deep.
	tmpRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpRoot, ".tkt"), 0o755); err != nil {
		t.Fatalf("setup: mkdir .tkt: %v", err)
	}
	nested := filepath.Join(tmpRoot, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("setup: mkdir nested: %v", err)
	}

	// Preserve original cwd and restore after test.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, err := FindRoot()
	if err != nil {
		t.Fatalf("FindRoot() unexpected error: %v", err)
	}

	// Use EvalSymlinks for symlink safety (e.g. macOS /var -> /private/var).
	wantEval, err := filepath.EvalSymlinks(tmpRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks(want): %v", err)
	}
	gotEval, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(got): %v", err)
	}

	if gotEval != wantEval {
		t.Errorf("FindRoot() = %q, want %q", gotEval, wantEval)
	}
}

func TestFindRoot_NotFound(t *testing.T) {
	// Plain temp dir with no .tkt/ inside.
	tmpDir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err = FindRoot()
	if err == nil {
		t.Fatal("FindRoot() expected error, got nil")
	}
	const wantMsg = "no .tkt/ directory found. Run: tkt init"
	if !strings.Contains(err.Error(), wantMsg) {
		t.Errorf("FindRoot() error = %q, want it to contain %q", err.Error(), wantMsg)
	}
}

func TestPathHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"TicketsDir", TicketsDir("/foo"), "/foo/.tkt"},
		{"SessionFile", SessionFile("/foo"), "/foo/.tkt/session"},
		{"DBPath", DBPath("/foo"), "/foo/.tkt/db.sqlite"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
