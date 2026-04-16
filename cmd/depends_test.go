package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// runDependsInDir sets rootDir to dir, resets flag vars, applies setupFlags, then calls runDepends.
// Returns captured stdout and any error.
func runDependsInDir(t *testing.T, dir string, args []string, setupFlags func()) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	savedOn := dependsOn
	savedRemove := dependsRemove
	defer func() {
		rootDir = savedRootDir
		dependsOn = savedOn
		dependsRemove = savedRemove
		dependsCmd.SetOut(nil)
	}()

	rootDir = dir
	dependsOn = ""
	dependsRemove = ""

	if setupFlags != nil {
		setupFlags()
	}

	var buf bytes.Buffer
	dependsCmd.SetOut(&buf)

	err := runDepends(dependsCmd, args)
	return buf.String(), err
}

// TestDepends_OnSingle verifies --on inserts a single dependency and prints correct output.
func TestDepends_OnSingle(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-0001")
	seedTickets(t, dir, 2) // ticket 1, ticket 2

	out, err := runDependsInDir(t, dir, []string{"2"}, func() {
		dependsOn = "1"
	})
	if err != nil {
		t.Fatalf("runDepends: %v", err)
	}
	if !strings.Contains(out, "#2 now depends on #1") {
		t.Errorf("expected '#2 now depends on #1', got: %q", out)
	}
}

// TestDepends_OnMultiple verifies --on inserts multiple dependencies.
func TestDepends_OnMultiple(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-0002")
	seedTickets(t, dir, 4) // tickets 1-4

	out, err := runDependsInDir(t, dir, []string{"4"}, func() {
		dependsOn = "1,2,3"
	})
	if err != nil {
		t.Fatalf("runDepends: %v", err)
	}
	if !strings.Contains(out, "#4 now depends on #1, #2, #3") {
		t.Errorf("expected '#4 now depends on #1, #2, #3', got: %q", out)
	}
}

// TestDepends_OnDuplicate verifies that inserting a duplicate edge is silently ignored.
func TestDepends_OnDuplicate(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-0003")
	seedTickets(t, dir, 2)

	// Seed an existing dependency.
	insertDependency(t, dir, 2, 1)

	// Insert again — should not error.
	out, err := runDependsInDir(t, dir, []string{"2"}, func() {
		dependsOn = "1"
	})
	if err != nil {
		t.Fatalf("runDepends: %v", err)
	}
	if !strings.Contains(out, "#2 now depends on #1") {
		t.Errorf("expected '#2 now depends on #1', got: %q", out)
	}
}

// TestDepends_OnCycle verifies that a cycle is detected and returns the exact error format.
func TestDepends_OnCycle(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-0004")
	seedTickets(t, dir, 2)

	// #2 depends on #1 — now try to make #1 depend on #2 (cycle).
	insertDependency(t, dir, 2, 1)

	_, err := runDependsInDir(t, dir, []string{"1"}, func() {
		dependsOn = "2"
	})
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	want := "cycle detected — #1 is already downstream of #2"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("expected %q in error, got: %v", want, err)
	}
}

// TestDepends_OnNonExistentDep verifies that a non-existent dep ticket returns "ticket not found".
func TestDepends_OnNonExistentDep(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-0005")
	seedTickets(t, dir, 1) // only ticket 1 exists

	_, err := runDependsInDir(t, dir, []string{"1"}, func() {
		dependsOn = "999"
	})
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
	if !strings.Contains(err.Error(), "ticket not found") {
		t.Errorf("expected 'ticket not found' in error, got: %v", err)
	}
}

// TestDepends_RemoveExisting verifies --remove deletes the edge and prints correct output.
func TestDepends_RemoveExisting(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-0006")
	seedTickets(t, dir, 2)
	insertDependency(t, dir, 2, 1)

	out, err := runDependsInDir(t, dir, []string{"2"}, func() {
		dependsRemove = "1"
	})
	if err != nil {
		t.Fatalf("runDepends: %v", err)
	}
	if !strings.Contains(out, "Removed dependency: #2 no longer depends on #1") {
		t.Errorf("expected removal message, got: %q", out)
	}
}

// TestDepends_RemoveNonExistent verifies --remove on a non-existent edge prints
// a user-friendly message and does not return a fatal error.
func TestDepends_RemoveNonExistent(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-0007")
	seedTickets(t, dir, 2) // tickets exist but no dep row

	_, err := runDependsInDir(t, dir, []string{"2"}, func() {
		dependsRemove = "1"
	})
	if err != nil {
		t.Fatalf("expected no fatal error for removing non-existent edge, got: %v", err)
	}
}

// TestDepends_NeitherFlag verifies that supplying neither --on nor --remove returns a user error.
func TestDepends_NeitherFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-0008")
	seedTickets(t, dir, 1)

	_, err := runDependsInDir(t, dir, []string{"1"}, nil)
	if err == nil {
		t.Fatal("expected error when neither flag given, got nil")
	}
	if !strings.Contains(err.Error(), "--on") && !strings.Contains(err.Error(), "--remove") {
		t.Errorf("expected flag mention in error, got: %v", err)
	}
}

// TestDepends_BothFlags verifies that supplying both --on and --remove returns a user error.
func TestDepends_BothFlags(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-dep-0009")
	seedTickets(t, dir, 2)

	_, err := runDependsInDir(t, dir, []string{"2"}, func() {
		dependsOn = "1"
		dependsRemove = "1"
	})
	if err == nil {
		t.Fatal("expected error when both flags given, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

// TestDepends_NoSession verifies that missing session returns ErrNoSession message.
func TestDepends_NoSession(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	// No seedSession call.
	seedTickets(t, dir, 2)

	_, err := runDependsInDir(t, dir, []string{"2"}, func() {
		dependsOn = "1"
	})
	if err == nil {
		t.Fatal("expected no-session error, got nil")
	}
	if !strings.Contains(err.Error(), "no active session") {
		t.Errorf("expected 'no active session' in error, got: %v", err)
	}
}
