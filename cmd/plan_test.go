package cmd

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/log"
)

// runPlanInDir sets rootDir to dir, resets state, then calls runPlan.
// Returns captured stdout and any error.
func runPlanInDir(t *testing.T, dir string, args []string) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	defer func() {
		rootDir = savedRootDir
		planCmd.SetOut(nil)
	}()

	rootDir = dir

	var buf bytes.Buffer
	planCmd.SetOut(&buf)

	err := runPlan(planCmd, args)
	return buf.String(), err
}

// TestResolveEditor_FallbackChain verifies editor resolution behaviour.
func TestResolveEditor_FallbackChain(t *testing.T) {
	t.Run("no_editor_found", func(t *testing.T) {
		// PATH set to a temp dir that contains no editors.
		emptyDir := t.TempDir()
		t.Setenv("PATH", emptyDir)

		_, _, err := resolveEditor("")
		if err == nil {
			t.Fatal("expected error when no editor is available, got nil")
		}
		if !strings.Contains(err.Error(), "no editor found") {
			t.Errorf("unexpected error text: %v", err)
		}
	})

	t.Run("multi_word_env_splits", func(t *testing.T) {
		// Resolve with a multi-word $EDITOR that names a real binary.
		// We use "env --" as a stand-in: both "env" and "--" are valid fields;
		// what matters is that resolveEditor splits on whitespace and uses fields[0].
		// Because we only care about splitting, just test that fields[0] is returned as bin.
		// We can observe this by passing an editor string whose first word IS a real binary.
		bin, extraArgs, err := resolveEditor("env --arg1")
		if err != nil {
			// env may not exist on all platforms; skip gracefully.
			t.Skipf("env not found: %v", err)
		}
		if !strings.HasSuffix(bin, "env") {
			t.Errorf("expected bin to end in 'env', got %q", bin)
		}
		if len(extraArgs) != 1 || extraArgs[0] != "--arg1" {
			t.Errorf("expected extraArgs [--arg1], got %v", extraArgs)
		}
	})
}

// TestPlan_StatusGuard verifies that running plan on a non-PLANNING ticket returns an error.
func TestPlan_StatusGuard(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-plan-0001")

	// Seed a ticket in TODO state (not PLANNING).
	id := seedTicketWithStatus(t, dir, "Guard test ticket", "TODO")

	_, err := runPlanInDir(t, dir, []string{id})
	if err == nil {
		t.Fatal("expected error for non-PLANNING ticket, got nil")
	}
	if !strings.Contains(err.Error(), "plan is frozen") {
		t.Errorf("expected 'plan is frozen' in error, got: %v", err)
	}
}

// TestPlan_NoSession verifies that running plan without an active session returns an error.
func TestPlan_NoSession(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	// No session seeded — LoadActive should return ErrNoSession.
	_, err := runPlanInDir(t, dir, []string{"1"})
	if err == nil {
		t.Fatal("expected error for no session, got nil")
	}
	if !strings.Contains(err.Error(), "active session") {
		t.Errorf("expected 'active session' in error, got: %v", err)
	}
}

// TestPlan_BodyFlag verifies that --body writes the plan directly without invoking $EDITOR.
func TestPlan_BodyFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-body-flag-0001")

	id := seedTicketWithStatus(t, dir, "Body flag test ticket", "PLANNING")

	// Set --body flag and reset it after the test to prevent leakage.
	if err := planCmd.Flags().Set("body", "my plan content"); err != nil {
		t.Fatalf("set --body flag: %v", err)
	}
	defer planCmd.Flags().Set("body", "") //nolint:errcheck

	out, err := runPlanInDir(t, dir, []string{id})
	if err != nil {
		t.Fatalf("runPlan with --body: %v", err)
	}

	if !strings.Contains(out, "Plan updated") {
		t.Errorf("expected output to contain 'Plan updated', got: %q", out)
	}
	if !strings.Contains(out, id) {
		t.Errorf("expected output to contain ticket id %q, got: %q", id, out)
	}

	// Verify the stored body via log.LatestPlan.
	savedRootDir := rootDir
	rootDir = dir
	defer func() { rootDir = savedRootDir }()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	idInt, _ := strconv.ParseInt(id, 10, 64)
	entry, err := log.LatestPlan(strconv.FormatInt(idInt, 10), database)
	if err != nil {
		t.Fatalf("LatestPlan: %v", err)
	}
	if entry == nil {
		t.Fatal("expected a plan log entry, got nil")
	}
	if entry.Body != "my plan content" {
		t.Errorf("expected body %q, got %q", "my plan content", entry.Body)
	}
}

// TestPlan_StdinFlag verifies that --stdin reads piped input, trims whitespace, and saves.
func TestPlan_StdinFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-stdin-0001")

	id := seedTicketWithStatus(t, dir, "Stdin flag test ticket", "PLANNING")

	if err := planCmd.Flags().Set("stdin", "true"); err != nil {
		t.Fatalf("set --stdin flag: %v", err)
	}
	defer planCmd.Flags().Set("stdin", "false") //nolint:errcheck

	// Provide input via cmd.InOrStdin().
	planCmd.SetIn(strings.NewReader("  ## Plan from stdin\n  content  \n"))
	defer planCmd.SetIn(nil)

	out, err := runPlanInDir(t, dir, []string{id})
	if err != nil {
		t.Fatalf("runPlan with --stdin: %v", err)
	}

	if !strings.Contains(out, "Plan updated") {
		t.Errorf("expected 'Plan updated' in output, got: %q", out)
	}

	// Verify stored body is trimmed.
	savedRootDir := rootDir
	rootDir = dir
	defer func() { rootDir = savedRootDir }()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	idInt, _ := strconv.ParseInt(id, 10, 64)
	entry, err := log.LatestPlan(strconv.FormatInt(idInt, 10), database)
	if err != nil {
		t.Fatalf("LatestPlan: %v", err)
	}
	if entry == nil {
		t.Fatal("expected a plan log entry, got nil")
	}
	want := "## Plan from stdin\n  content"
	if entry.Body != want {
		t.Errorf("expected body %q, got %q", want, entry.Body)
	}
}

// TestPlan_FileFlag verifies that --file reads from a temp file, trims whitespace, and saves.
func TestPlan_FileFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-file-0001")

	id := seedTicketWithStatus(t, dir, "File flag test ticket", "PLANNING")

	// Write plan content to a temp file.
	planFile, err := os.CreateTemp("", "plan-test-*.md")
	if err != nil {
		t.Fatalf("create temp plan file: %v", err)
	}
	defer os.Remove(planFile.Name())
	if _, err := planFile.WriteString("  ## Plan from file\n  content  \n"); err != nil {
		t.Fatalf("write temp plan file: %v", err)
	}
	planFile.Close()

	if err := planCmd.Flags().Set("file", planFile.Name()); err != nil {
		t.Fatalf("set --file flag: %v", err)
	}
	defer planCmd.Flags().Set("file", "") //nolint:errcheck

	out, err := runPlanInDir(t, dir, []string{id})
	if err != nil {
		t.Fatalf("runPlan with --file: %v", err)
	}

	if !strings.Contains(out, "Plan updated") {
		t.Errorf("expected 'Plan updated' in output, got: %q", out)
	}

	// Verify stored body is trimmed.
	savedRootDir := rootDir
	rootDir = dir
	defer func() { rootDir = savedRootDir }()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	idInt, _ := strconv.ParseInt(id, 10, 64)
	entry, err := log.LatestPlan(strconv.FormatInt(idInt, 10), database)
	if err != nil {
		t.Fatalf("LatestPlan: %v", err)
	}
	if entry == nil {
		t.Fatal("expected a plan log entry, got nil")
	}
	want := "## Plan from file\n  content"
	if entry.Body != want {
		t.Errorf("expected body %q, got %q", want, entry.Body)
	}
}

// TestPlan_FileFlag_Nonexistent verifies that --file with a bad path returns a wrapped error.
func TestPlan_FileFlag_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-file-nonexist-0001")

	id := seedTicketWithStatus(t, dir, "File nonexist test ticket", "PLANNING")

	if err := planCmd.Flags().Set("file", "/no/such/file/plan.md"); err != nil {
		t.Fatalf("set --file flag: %v", err)
	}
	defer planCmd.Flags().Set("file", "") //nolint:errcheck

	_, err := runPlanInDir(t, dir, []string{id})
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "plan: read file") {
		t.Errorf("expected 'plan: read file' in error, got: %v", err)
	}
}

// TestPlan_StdinFlag_Empty verifies that --stdin with empty input returns an error.
func TestPlan_StdinFlag_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-stdin-empty-0001")

	id := seedTicketWithStatus(t, dir, "Stdin empty test ticket", "PLANNING")

	if err := planCmd.Flags().Set("stdin", "true"); err != nil {
		t.Fatalf("set --stdin flag: %v", err)
	}
	defer planCmd.Flags().Set("stdin", "false") //nolint:errcheck

	// Provide empty input.
	planCmd.SetIn(strings.NewReader("   \n   "))
	defer planCmd.SetIn(nil)

	_, err := runPlanInDir(t, dir, []string{id})
	if err == nil {
		t.Fatal("expected error for empty stdin input, got nil")
	}
	if !strings.Contains(err.Error(), "plan: body is empty") {
		t.Errorf("expected 'plan: body is empty' in error, got: %v", err)
	}
}

// resetPlanFlags resets all three mutually-exclusive plan flags to their zero values.
// Call at the start of any test that sets these flags to avoid cross-test leakage on the
// shared planCmd global.
func resetPlanFlags(t *testing.T) {
	t.Helper()
	planCmd.Flags().Set("body", "")    //nolint:errcheck
	planCmd.Flags().Set("stdin", "false") //nolint:errcheck
	planCmd.Flags().Set("file", "")    //nolint:errcheck
}

// TestPlan_MutualExclusion_BodyStdin verifies cobra rejects --body and --stdin together.
func TestPlan_MutualExclusion_BodyStdin(t *testing.T) {
	resetPlanFlags(t)
	t.Cleanup(func() { resetPlanFlags(t) })

	if err := planCmd.Flags().Set("body", "inline"); err != nil {
		t.Fatalf("set --body: %v", err)
	}
	if err := planCmd.Flags().Set("stdin", "true"); err != nil {
		t.Fatalf("set --stdin: %v", err)
	}

	err := planCmd.ValidateFlagGroups()
	if err == nil {
		t.Fatal("expected mutual exclusion error for --body + --stdin, got nil")
	}
	if !strings.Contains(err.Error(), "none of the others can be") {
		t.Errorf("expected mutex error for --body + --stdin, got: %v", err)
	}
}

// TestPlan_MutualExclusion_BodyFile verifies cobra rejects --body and --file together.
func TestPlan_MutualExclusion_BodyFile(t *testing.T) {
	resetPlanFlags(t)
	t.Cleanup(func() { resetPlanFlags(t) })

	if err := planCmd.Flags().Set("body", "inline"); err != nil {
		t.Fatalf("set --body: %v", err)
	}
	if err := planCmd.Flags().Set("file", "/some/file.md"); err != nil {
		t.Fatalf("set --file: %v", err)
	}

	err := planCmd.ValidateFlagGroups()
	if err == nil {
		t.Fatal("expected mutual exclusion error for --body + --file, got nil")
	}
	if !strings.Contains(err.Error(), "none of the others can be") {
		t.Errorf("expected mutex error for --body + --file, got: %v", err)
	}
}

// TestPlan_MutualExclusion_StdinFile verifies cobra rejects --stdin and --file together.
func TestPlan_MutualExclusion_StdinFile(t *testing.T) {
	resetPlanFlags(t)
	t.Cleanup(func() { resetPlanFlags(t) })

	if err := planCmd.Flags().Set("stdin", "true"); err != nil {
		t.Fatalf("set --stdin: %v", err)
	}
	if err := planCmd.Flags().Set("file", "/some/file.md"); err != nil {
		t.Fatalf("set --file: %v", err)
	}

	err := planCmd.ValidateFlagGroups()
	if err == nil {
		t.Fatal("expected mutual exclusion error for --stdin + --file, got nil")
	}
	if !strings.Contains(err.Error(), "none of the others can be") {
		t.Errorf("expected mutex error for --stdin + --file, got: %v", err)
	}
}
