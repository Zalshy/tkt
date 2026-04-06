package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
)

// runNewInDir sets rootDir to dir, resets flag vars, applies setupFlags, then calls runNew.
// Returns captured stdout and any error.
func runNewInDir(t *testing.T, dir string, args []string, setupFlags func()) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	savedDesc := newDescription
	savedAfter := newAfter
	defer func() {
		rootDir = savedRootDir
		newDescription = savedDesc
		newAfter = savedAfter
		newCmd.SetOut(nil)
	}()

	rootDir = dir
	newDescription = ""
	newAfter = ""

	if setupFlags != nil {
		setupFlags()
	}

	var buf bytes.Buffer
	newCmd.SetOut(&buf)

	err := runNew(newCmd, args)
	return buf.String(), err
}

// seedSession inserts a session row and writes the session file so LoadActive succeeds.
func seedSession(t *testing.T, dir string, id string) {
	t.Helper()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("seedSession: open db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active)
		 VALUES (?, 'implementer', ?, datetime('now'), datetime('now'))`,
		id, id,
	); err != nil {
		t.Fatalf("seedSession: insert: %v", err)
	}

	sessionFile := filepath.Join(dir, ".tkt", "session")
	if err := os.WriteFile(sessionFile, []byte(id), 0o644); err != nil {
		t.Fatalf("seedSession: write session file: %v", err)
	}
}

// TestNew_NoSession verifies that running tkt new without an active session
// returns an error mentioning "no active session".
func TestNew_NoSession(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runNewInDir(t, dir, []string{"Fix login"}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no active session") {
		t.Errorf("expected 'no active session' in error, got: %v", err)
	}
}

// TestNew_WithSession verifies that tkt new with an active session creates a ticket
// and outputs "Created #1  \"Fix login\"".
func TestNew_WithSession(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-test-0001")

	out, err := runNewInDir(t, dir, []string{"Fix login"}, nil)
	if err != nil {
		t.Fatalf("runNew: %v", err)
	}

	want := `Created #1  "Fix login"`
	if !strings.Contains(out, want) {
		t.Errorf("expected output %q, got: %q", want, out)
	}
}

// TestNew_MissingTitleArg verifies that cobra's ExactArgs(1) returns an error
// when no title is given. We call runNew directly with an empty args slice so the
// arg validation check fires without going through Execute().
func TestNew_MissingTitleArg(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	// ExactArgs(1) is enforced by cobra before RunE is called when using Execute(),
	// but when calling RunE directly we must call ValidateArgs ourselves.
	err := newCmd.ValidateArgs([]string{})
	if err == nil {
		t.Fatal("expected ExactArgs error for empty args, got nil")
	}
}

// TestNew_AfterNotProvided verifies that when --after is not given the output is
// exactly one line: `Created #N  "title"` with no "Depends on:" second line.
func TestNew_AfterNotProvided(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-after-0001")

	out, err := runNewInDir(t, dir, []string{"No Deps"}, nil)
	if err != nil {
		t.Fatalf("runNew: %v", err)
	}
	if !strings.Contains(out, `Created #1  "No Deps"`) {
		t.Errorf("expected created line, got: %q", out)
	}
	if strings.Contains(out, "Depends on:") {
		t.Errorf("expected no 'Depends on:' line when --after not provided, got: %q", out)
	}
}

// TestNew_AfterSingle verifies that --after with a single existing ticket ID creates
// the ticket and prints "Depends on: #5".
func TestNew_AfterSingle(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-after-0002")
	seedTickets(t, dir, 5) // tickets #1–#5

	out, err := runNewInDir(t, dir, []string{"After Single"}, func() {
		newAfter = "5"
	})
	if err != nil {
		t.Fatalf("runNew: %v", err)
	}
	if !strings.Contains(out, "Created #6") {
		t.Errorf("expected 'Created #6' in output, got: %q", out)
	}
	if !strings.Contains(out, "Depends on: #5") {
		t.Errorf("expected 'Depends on: #5' in output, got: %q", out)
	}
}

// TestNew_AfterMultiple verifies that --after with two existing ticket IDs creates
// the ticket and prints "Depends on: #5, #7".
func TestNew_AfterMultiple(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-after-0003")
	seedTickets(t, dir, 7) // tickets #1–#7

	out, err := runNewInDir(t, dir, []string{"After Multiple"}, func() {
		newAfter = "5,7"
	})
	if err != nil {
		t.Fatalf("runNew: %v", err)
	}
	if !strings.Contains(out, "Created #8") {
		t.Errorf("expected 'Created #8' in output, got: %q", out)
	}
	if !strings.Contains(out, "Depends on: #5, #7") {
		t.Errorf("expected 'Depends on: #5, #7' in output, got: %q", out)
	}
}

// TestNew_AfterNonExistent verifies that --after with a non-existent ticket ID
// returns an error containing "ticket not found" and the orphan cleanup hint.
func TestNew_AfterNonExistent(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-after-0004")
	// No tickets seeded — #99 does not exist.

	_, err := runNewInDir(t, dir, []string{"Orphan"}, func() {
		newAfter = "99"
	})
	if err == nil {
		t.Fatal("expected error for non-existent dep, got nil")
	}
	if !strings.Contains(err.Error(), "ticket not found") {
		t.Errorf("expected 'ticket not found' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "tkt cancel") {
		t.Errorf("expected orphan cleanup hint in error, got: %v", err)
	}
}

// TestNew_AfterInvalidID verifies that --after with a non-integer value returns
// an error containing "not a valid ticket ID", not a raw strconv error.
func TestNew_AfterInvalidID(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-after-0005")

	_, err := runNewInDir(t, dir, []string{"Bad ID"}, func() {
		newAfter = "abc"
	})
	if err == nil {
		t.Fatal("expected error for non-integer --after value, got nil")
	}
	if !strings.Contains(err.Error(), "not a valid ticket ID") {
		t.Errorf("expected 'not a valid ticket ID' in error, got: %v", err)
	}
}

// TestNew_AfterWithSpaces verifies that --after tolerates spaces around commas,
// e.g. "5, 7" parses identically to "5,7".
func TestNew_AfterWithSpaces(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-after-0006")
	seedTickets(t, dir, 7) // tickets #1–#7

	out, err := runNewInDir(t, dir, []string{"Spaced After"}, func() {
		newAfter = "5, 7"
	})
	if err != nil {
		t.Fatalf("runNew: %v", err)
	}
	if !strings.Contains(out, "Depends on: #5, #7") {
		t.Errorf("expected 'Depends on: #5, #7' in output, got: %q", out)
	}
}
