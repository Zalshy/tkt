package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// runDocAddInDir sets rootDir to dir, resets state, then calls runDocAdd.
// Returns captured stdout and any error.
func runDocAddInDir(t *testing.T, dir string, args []string) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	defer func() {
		rootDir = savedRootDir
		docAddCmd.SetOut(nil)
		// Reset flags.
		docAddCmd.Flags().Set("body", "")   //nolint:errcheck
		docAddCmd.Flags().Set("stdin", "false") //nolint:errcheck
		docAddCmd.Flags().Set("file", "")   //nolint:errcheck
		docAddCmd.SetIn(nil)
	}()

	rootDir = dir

	var buf bytes.Buffer
	docAddCmd.SetOut(&buf)

	err := runDocAdd(docAddCmd, args)
	return buf.String(), err
}

// resetDocAddFlags resets all three mutually-exclusive doc add flags to zero values.
func resetDocAddFlags(t *testing.T) {
	t.Helper()
	docAddCmd.Flags().Set("body", "")    //nolint:errcheck
	docAddCmd.Flags().Set("stdin", "false") //nolint:errcheck
	docAddCmd.Flags().Set("file", "")    //nolint:errcheck
}

// TestDocAdd_BodyFlag verifies --body creates the file and prints "created docs/".
func TestDocAdd_BodyFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-docadd-body-001")

	if err := docAddCmd.Flags().Set("body", "# My doc\n\ncontent here"); err != nil {
		t.Fatalf("set --body: %v", err)
	}

	out, err := runDocAddInDir(t, dir, []string{"my-analysis"})
	if err != nil {
		t.Fatalf("runDocAdd --body: %v", err)
	}

	if !strings.Contains(out, "created docs/") {
		t.Errorf("expected 'created docs/' in output, got: %q", out)
	}
}

// TestDocAdd_FileFlag verifies --file reads content from a file and creates the doc.
func TestDocAdd_FileFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-docadd-file-001")

	// Write content to a temp file.
	f, err := os.CreateTemp("", "doc-content-*.md")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("# Doc from file\n\nbody text\n"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()

	if err := docAddCmd.Flags().Set("file", f.Name()); err != nil {
		t.Fatalf("set --file: %v", err)
	}

	out, err := runDocAddInDir(t, dir, []string{"file-doc"})
	if err != nil {
		t.Fatalf("runDocAdd --file: %v", err)
	}

	if !strings.Contains(out, "created docs/") {
		t.Errorf("expected 'created docs/' in output, got: %q", out)
	}
}

// TestDocAdd_StdinFlag verifies --stdin reads content via cmd.SetIn and creates the doc.
func TestDocAdd_StdinFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-docadd-stdin-001")

	if err := docAddCmd.Flags().Set("stdin", "true"); err != nil {
		t.Fatalf("set --stdin: %v", err)
	}
	docAddCmd.SetIn(strings.NewReader("# Doc from stdin\n\ncontent\n"))

	out, err := runDocAddInDir(t, dir, []string{"stdin-doc"})
	if err != nil {
		t.Fatalf("runDocAdd --stdin: %v", err)
	}

	if !strings.Contains(out, "created docs/") {
		t.Errorf("expected 'created docs/' in output, got: %q", out)
	}
}

// TestDocAdd_MutualExclusion_BodyStdin verifies cobra rejects --body and --stdin together.
func TestDocAdd_MutualExclusion_BodyStdin(t *testing.T) {
	resetDocAddFlags(t)
	t.Cleanup(func() { resetDocAddFlags(t) })

	if err := docAddCmd.Flags().Set("body", "inline"); err != nil {
		t.Fatalf("set --body: %v", err)
	}
	if err := docAddCmd.Flags().Set("stdin", "true"); err != nil {
		t.Fatalf("set --stdin: %v", err)
	}

	err := docAddCmd.ValidateFlagGroups()
	if err == nil {
		t.Fatal("expected mutual exclusion error for --body + --stdin, got nil")
	}
	if !strings.Contains(err.Error(), "none of the others can be") {
		t.Errorf("expected mutex error text, got: %v", err)
	}
}

// TestDocAdd_NoFlags_NoEditor verifies that omitting all flags with no $EDITOR returns an error.
func TestDocAdd_NoFlags_NoEditor(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-docadd-noeditor-001")

	// Ensure no editor is found.
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	t.Setenv("EDITOR", "")

	_, err := runDocAddInDir(t, dir, []string{"no-editor-doc"})
	if err == nil {
		t.Fatal("expected error when no editor available, got nil")
	}
	if !strings.Contains(err.Error(), "no editor found") {
		t.Errorf("expected 'no editor found' in error, got: %v", err)
	}
}
