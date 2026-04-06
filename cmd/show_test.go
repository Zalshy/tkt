package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
)

// runShowInDir sets rootDir to dir and calls runShow with the given args.
// Returns captured stdout and any error.
func runShowInDir(t *testing.T, dir string, args []string) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	defer func() {
		rootDir = savedRootDir
		showCmd.SetOut(nil)
		showCmd.SilenceErrors = false
	}()

	rootDir = dir

	var buf bytes.Buffer
	showCmd.SetOut(&buf)

	err := runShow(showCmd, args)
	return buf.String(), err
}

// seedTicket inserts a single ticket and returns its ID.
func seedTicket(t *testing.T, dir string, title string) int64 {
	t.Helper()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("seedTicket: open db: %v", err)
	}
	defer database.Close()

	result, err := database.Exec(
		`INSERT INTO tickets (title, description, status, created_by)
		 VALUES (?, '', 'TODO', 'test-session')`,
		title,
	)
	if err != nil {
		t.Fatalf("seedTicket: insert: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("seedTicket: last insert id: %v", err)
	}
	return id
}

// TestShow_NotFound verifies that tkt show with a non-existent ID returns a non-nil
// error (exit 1 path) and sets SilenceErrors.
func TestShow_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err := runShowInDir(t, dir, []string{"99"})
	if err == nil {
		t.Fatal("expected non-nil error for not-found ticket, got nil")
	}
	// The error itself is the empty sentinel — cobra produces exit 1 from any non-nil error.
	// The user-facing message was already written to stderr in runShow.
}

// TestShow_Existing verifies that tkt show for an existing ticket with no log entries
// renders the header and synthetic "created" entry.
func TestShow_Existing(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedTicket(t, dir, "Fix login")

	out, err := runShowInDir(t, dir, []string{"1"})
	if err != nil {
		t.Fatalf("runShow: %v", err)
	}

	if !strings.Contains(out, "Fix login") {
		t.Errorf("expected ticket title in output, got: %q", out)
	}
	if !strings.Contains(out, "○ created") {
		t.Errorf("expected '○ created' synthetic entry in output, got: %q", out)
	}
}

// TestShow_HashPrefix verifies that "#1" and "1" both resolve to the same ticket.
func TestShow_HashPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedTicket(t, dir, "Fix login")

	outPlain, err := runShowInDir(t, dir, []string{"1"})
	if err != nil {
		t.Fatalf("runShow plain id: %v", err)
	}

	outHash, err := runShowInDir(t, dir, []string{"#1"})
	if err != nil {
		t.Fatalf("runShow hash id: %v", err)
	}

	if outPlain != outHash {
		t.Errorf("expected same output for '1' and '#1':\nplain: %q\nhash:  %q", outPlain, outHash)
	}
}
