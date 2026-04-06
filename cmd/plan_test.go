package cmd

import (
	"bytes"
	"strings"
	"testing"
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
