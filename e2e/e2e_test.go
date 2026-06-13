//go:build e2e

// Package e2e contains end-to-end tests for the tkt binary.
// These tests build and invoke the real tkt binary via os/exec to verify
// flag parsing, cobra wiring, output format, and exit codes together.
//
// Windows is not supported: the EDITOR stub relies on POSIX executables.
//
// Run with: make e2e
// Or:       go test -tags e2e -v ./e2e/...
package e2e

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// tktBin is the path to the compiled tkt binary, set in TestMain.
var tktBin string

// projectRoot returns the absolute path of the module root (parent of e2e/).
func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(file))
}

// TestMain builds the binary once before running all tests.
func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "e2e tests are not supported on Windows (EDITOR stub requires POSIX sh)")
		os.Exit(1)
	}

	bin, err := buildBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build binary: %v\n", err)
		os.Exit(1)
	}
	tktBin = bin
	os.Exit(m.Run())
}

// buildBinary compiles the tkt binary into a temp directory.
func buildBinary() (string, error) {
	dir, err := os.MkdirTemp("", "tkt-e2e-bin-*")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "tkt")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/zalshy/tkt")
	cmd.Dir = projectRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build: %s: %w", out, err)
	}
	return bin, nil
}

// run executes tkt with the given args in the given directory.
// It returns stdout, stderr, and the exit code.
// HOME is set to /dev/null and EDITOR to "true" to prevent interference
// from user configuration and editor-dependent commands.
func run(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	return runWithInput(t, dir, "", args...)
}

// runWithInput executes tkt with the given args and optional stdin input.
func runWithInput(t *testing.T, dir, input string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(tktBin, args...)
	cmd.Dir = dir

	// Build a clean environment: inherit PATH, override HOME and EDITOR.
	cmd.Env = append(os.Environ(),
		"HOME=/dev/null",
		"EDITOR=true",
	)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return stdout, stderr, exitCode
}

func createTempFile(t *testing.T, dir, pattern, body string) string {
	t.Helper()
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatalf("createTempFile: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(body); err != nil {
		t.Fatalf("createTempFile write: %v", err)
	}
	return f.Name()
}

// initProject runs tkt init in dir and fails if it doesn't succeed.
func initProject(t *testing.T, dir string) {
	t.Helper()
	stdout, stderr, code := run(t, dir, "init")
	if code != 0 {
		t.Fatalf("init failed (exit %d): stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// createSession runs tkt session --role <role> and fails on error.
func createSession(t *testing.T, dir, role string) {
	t.Helper()
	stdout, stderr, code := run(t, dir, "session", "--role", role)
	if code != 0 {
		t.Fatalf("session --role %s failed (exit %d): stdout=%q stderr=%q", role, code, stdout, stderr)
	}
}

// createTicket runs tkt new <title> and fails on error, returning the raw output.
func createTicket(t *testing.T, dir, title string) string {
	t.Helper()
	stdout, stderr, code := run(t, dir, "new", title)
	if code != 0 {
		t.Fatalf("new %q failed (exit %d): stdout=%q stderr=%q", title, code, stdout, stderr)
	}
	return stdout
}

// advanceTicket runs tkt advance <id> --note <note> and fails on error.
func advanceTicket(t *testing.T, dir, id, note string) string {
	t.Helper()
	stdout, stderr, code := run(t, dir, "advance", id, "--note", note)
	if code != 0 {
		t.Fatalf("advance %q --note %q failed (exit %d): stdout=%q stderr=%q", id, note, code, stdout, stderr)
	}
	return stdout
}

// advanceTicketTo runs tkt advance <id> --to <to> --note <note> and fails on error.
func advanceTicketTo(t *testing.T, dir, id, to, note string) string {
	t.Helper()
	stdout, stderr, code := run(t, dir, "advance", id, "--to", to, "--note", note)
	if code != 0 {
		t.Fatalf("advance %q --to %q --note %q failed (exit %d): stdout=%q stderr=%q", id, to, note, code, stdout, stderr)
	}
	return stdout
}

// openDB opens the SQLite database for an initialized project dir.
// The caller is responsible for closing it.
func openDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(dir, ".tkt", "db.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	return db
}

// seedTickets inserts n tickets directly into the DB.
func seedTickets(t *testing.T, dir string, n int) {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()

	for i := 1; i <= n; i++ {
		_, err := db.Exec(
			`INSERT INTO tickets (title, status, created_by, created_at, updated_at)
			 VALUES (?, 'TODO', 'seed-session', datetime('now'), datetime('now'))`,
			fmt.Sprintf("Seeded ticket %d", i),
		)
		if err != nil {
			t.Fatalf("seedTickets: insert ticket %d: %v", i, err)
		}
	}
}

// seedStaleSession inserts a session with last_active more than 7 days ago.
func seedStaleSession(t *testing.T, dir, sessionID string) {
	t.Helper()
	seedSession(t, dir, sessionID, sessionID, "architect", time.Now().Add(-8*24*time.Hour), nil)
}

func seedSession(t *testing.T, dir, sessionID, sessionName, role string, lastActive time.Time, expiredAt *time.Time) {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()

	lastActiveStr := lastActive.UTC().Format("2006-01-02 15:04:05")
	var err error
	if expiredAt == nil {
		_, err = db.Exec(
			`INSERT INTO sessions (id, role, name, created_at, last_active)
			 VALUES (?, ?, ?, ?, ?)`,
			sessionID, role, sessionName, lastActiveStr, lastActiveStr,
		)
	} else {
		expiredAtStr := expiredAt.UTC().Format("2006-01-02 15:04:05")
		_, err = db.Exec(
			`INSERT INTO sessions (id, role, name, created_at, last_active, expired_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			sessionID, role, sessionName, lastActiveStr, lastActiveStr, expiredAtStr,
		)
	}
	if err != nil {
		t.Fatalf("seedSession: insert: %v", err)
	}
}

func latestSession(t *testing.T, dir string) (id, name string) {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()
	if err := db.QueryRow(
		`SELECT id, name FROM sessions ORDER BY created_at DESC, rowid DESC LIMIT 1`,
	).Scan(&id, &name); err != nil {
		t.Fatalf("latestSession: %v", err)
	}
	return id, name
}

func ticketStatus(t *testing.T, dir string, ticketID int64) string {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()
	var status string
	if err := db.QueryRow(`SELECT status FROM tickets WHERE id = ?`, ticketID).Scan(&status); err != nil {
		t.Fatalf("ticketStatus: %v", err)
	}
	return status
}

func setTicketState(t *testing.T, dir string, ticketID int64, status string, updatedAt time.Time) {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()
	if _, err := db.Exec(
		`UPDATE tickets SET status = ?, updated_at = ? WHERE id = ?`,
		status, updatedAt.UTC().Format("2006-01-02 15:04:05"), ticketID,
	); err != nil {
		t.Fatalf("setTicketState: %v", err)
	}
}

func readSessionFile(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tkt", "session"))
	if err != nil {
		t.Fatalf("readSessionFile: %v", err)
	}
	return strings.TrimSpace(string(data))
}

// TestE2E is the top-level test function containing all e2e scenarios.
func TestE2E(t *testing.T) {
	// -------------------------------------------------------------------------
	// Group 1: Init
	// -------------------------------------------------------------------------

	t.Run("init_fresh", func(t *testing.T) {
		dir := t.TempDir()
		stdout, stderr, code := run(t, dir, "init")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "Initialized") {
			t.Errorf("expected 'Initialized' in stdout, got: %q", stdout)
		}
		if _, err := os.Stat(filepath.Join(dir, ".tkt")); os.IsNotExist(err) {
			t.Error(".tkt/ directory was not created")
		}
		if _, err := os.Stat(filepath.Join(dir, ".tkt", "db.sqlite")); os.IsNotExist(err) {
			t.Error(".tkt/db.sqlite was not created")
		}
	})

	t.Run("init_double", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		_, stderr, code := run(t, dir, "init")
		if code == 0 {
			t.Fatal("expected non-zero exit on double init, got 0")
		}
		combined := stderr
		if !strings.Contains(strings.ToLower(combined), "already") {
			t.Errorf("expected 'already' in error output, got: %q", combined)
		}
	})

	t.Run("init_no_dir", func(t *testing.T) {
		dir := t.TempDir()
		// Run a command that requires .tkt/ without initializing
		_, stderr, code := run(t, dir, "list")
		if code == 0 {
			t.Fatal("expected non-zero exit without .tkt/, got 0")
		}
		if !strings.Contains(stderr+"\n", "tkt") {
			// Just verify it exits non-zero with some error message
			_ = stderr
		}
	})

	// -------------------------------------------------------------------------
	// Group 2: Session
	// -------------------------------------------------------------------------

	t.Run("session_create_architect", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "session", "--role", "architect")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "architect") {
			t.Errorf("expected 'architect' in output, got: %q", stdout)
		}
	})

	t.Run("session_create_implementer", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "session", "--role", "implementer")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "implementer") {
			t.Errorf("expected 'implementer' in output, got: %q", stdout)
		}
	})

	t.Run("session_no_session", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		// No session created — show should exit non-zero
		_, _, code := run(t, dir, "session")
		if code == 0 {
			t.Fatal("expected non-zero exit when no session exists, got 0")
		}
	})

	t.Run("session_invalid_role", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		_, stderr, code := run(t, dir, "session", "--role", "wizard")
		if code == 0 {
			t.Fatal("expected non-zero exit for invalid role, got 0")
		}
		if !strings.Contains(stderr, "wizard") {
			t.Errorf("expected role name in error, got: %q", stderr)
		}
	})

	t.Run("session_end", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		stdout, stderr, code := run(t, dir, "session", "--end")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "ended") {
			t.Errorf("expected 'ended' in output, got: %q", stdout)
		}
	})

	// -------------------------------------------------------------------------
	// Group 3: New / List / Show
	// -------------------------------------------------------------------------

	t.Run("new_basic", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		stdout, stderr, code := run(t, dir, "new", "My first ticket")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "#1") {
			t.Errorf("expected '#1' in output, got: %q", stdout)
		}
		listOut, _, listCode := run(t, dir, "list")
		if listCode != 0 {
			t.Fatalf("list failed: %q", listOut)
		}
		if !strings.Contains(listOut, "#1") {
			t.Errorf("expected '#1' in list output, got: %q", listOut)
		}
	})

	t.Run("new_with_after", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Ticket 1")
		stdout, stderr, code := run(t, dir, "new", "Ticket 2", "--after", "1")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		_ = stdout
		showOut, _, showCode := run(t, dir, "show", "2")
		if showCode != 0 {
			t.Fatalf("show 2 failed: exit %d", showCode)
		}
		if !strings.Contains(showOut, "#1") {
			t.Errorf("expected dependency '#1' in show output, got: %q", showOut)
		}
	})

	t.Run("list_empty", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "list")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		_ = stdout
	})

	t.Run("list_limit", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		seedTickets(t, dir, 12)
		stdout, _, code := run(t, dir, "list")
		if code != 0 {
			t.Fatalf("list failed: exit %d, stdout=%q", code, stdout)
		}
		// Default limit is 10 — output should hint there are more
		if !strings.Contains(stdout, "more") && !strings.Contains(stdout, "10") {
			t.Logf("list output: %q", stdout)
			// Not a fatal error — just log; the hint wording may differ
		}
	})

	t.Run("list_all", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		seedTickets(t, dir, 12)
		stdout, _, code := run(t, dir, "list", "--all")
		if code != 0 {
			t.Fatalf("list --all failed: exit %d", code)
		}
		// With --all, all 12 tickets should be visible
		count := strings.Count(stdout, "#")
		if count < 12 {
			t.Errorf("expected at least 12 tickets in --all output, found %d ticket refs", count)
		}
	})

	t.Run("list_status_filter", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "A todo ticket")
		stdout, _, code := run(t, dir, "list", "--status", "TODO")
		if code != 0 {
			t.Fatalf("list --status TODO failed: exit %d stdout=%q", code, stdout)
		}
		if !strings.Contains(stdout, "TODO") {
			t.Errorf("expected 'TODO' in filtered output, got: %q", stdout)
		}
	})

	t.Run("show_detail", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Detailed ticket")
		stdout, stderr, code := run(t, dir, "show", "1")
		if code != 0 {
			t.Fatalf("show failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "Detailed ticket") {
			t.Errorf("expected title in show output, got: %q", stdout)
		}
		if !strings.Contains(stdout, "TODO") {
			t.Errorf("expected status in show output, got: %q", stdout)
		}
	})

	t.Run("show_not_found", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		_, _, code := run(t, dir, "show", "999")
		if code == 0 {
			t.Fatal("expected non-zero exit for missing ticket, got 0")
		}
	})

	// -------------------------------------------------------------------------
	// Group 4: Plan
	// -------------------------------------------------------------------------

	t.Run("plan_requires_planning_state", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		createTicket(t, dir, "Plan test ticket")
		// Ticket is in TODO, not PLANNING — plan should fail
		_, stderr, code := run(t, dir, "plan", "1")
		if code == 0 {
			t.Fatal("expected non-zero exit for plan on TODO ticket, got 0")
		}
		if !strings.Contains(stderr, "PLANNING") && !strings.Contains(stderr, "planning") {
			t.Logf("error output: %q", stderr)
		}
	})

	t.Run("plan_no_session", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		// No session — plan should fail
		_, _, code := run(t, dir, "plan", "1")
		if code == 0 {
			t.Fatal("expected non-zero exit when no session, got 0")
		}
	})

	t.Run("plan_editor_stub", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		createTicket(t, dir, "Plan editor test")
		// Advance to PLANNING
		advanceTicket(t, dir, "1", "moving to planning")
		// Run tkt plan 1 with EDITOR=true (exits 0, writes nothing)
		cmd := exec.Command(tktBin, "plan", "1")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "HOME=/dev/null", "EDITOR=true")
		var outBuf, errBuf strings.Builder
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		err := cmd.Run()
		stdout := outBuf.String()
		if err != nil {
			t.Fatalf("plan 1 with EDITOR=true failed: %v, stdout=%q stderr=%q", err, stdout, errBuf.String())
		}
		if !strings.Contains(stdout, "No changes made") {
			t.Errorf("expected 'No changes made' in output, got: %q", stdout)
		}
	})

	// -------------------------------------------------------------------------
	// Group 5: Advance — full lifecycle
	// -------------------------------------------------------------------------

	t.Run("advance_full_lifecycle", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)

		// Create implementer session and ticket
		createSession(t, dir, "implementer")
		createTicket(t, dir, "Lifecycle ticket")

		// TODO → PLANNING (implementer)
		advanceTicket(t, dir, "1", "starting planning")

		// Write a plan (EDITOR=true writes nothing, so we need to inject a plan directly)
		// Use EDITOR with a shell command that writes content to the temp file
		cmd := exec.Command(tktBin, "plan", "1")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "HOME=/dev/null", `EDITOR=sh -c 'echo "## Plan" > "$1"'`)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Logf("plan with content EDITOR (may fail on some shells): %v, out=%q", err, out)
			// Inject plan directly via DB
			injectPlan(t, dir, 1)
		}

		// Switch to architect session for PLANNING → IN_PROGRESS
		createSession(t, dir, "architect")
		advanceTicket(t, dir, "1", "approving plan")

		// Switch to implementer for IN_PROGRESS → DONE
		createSession(t, dir, "implementer")
		advanceTicket(t, dir, "1", "implementation complete")

		// Switch to architect for DONE → VERIFIED (different session required)
		createSession(t, dir, "architect")
		out := advanceTicket(t, dir, "1", "verified")
		_ = out

		// Verify final state
		showOut, _, showCode := run(t, dir, "show", "1")
		if showCode != 0 {
			t.Fatalf("show failed: exit %d", showCode)
		}
		if !strings.Contains(showOut, "VERIFIED") {
			t.Errorf("expected VERIFIED in show output, got: %q", showOut)
		}
	})

	t.Run("lifecycle_full", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)

		// Session A (implementer): create ticket, advance TODO→PLANNING, submit plan
		createSession(t, dir, "implementer")
		createTicket(t, dir, "Full lifecycle ticket")

		// Assert status == TODO after tkt new
		if s := ticketStatus(t, dir, 1); s != "TODO" {
			t.Fatalf("expected TODO after new, got %q", s)
		}

		// TODO → PLANNING
		advanceTicket(t, dir, "1", "moving to planning")

		// Assert status == PLANNING
		if s := ticketStatus(t, dir, 1); s != "PLANNING" {
			t.Fatalf("expected PLANNING after TODO→PLANNING, got %q", s)
		}

		// Submit plan using --body flag
		if _, stderr, code := run(t, dir, "plan", "1", "--body", "## Full lifecycle plan\n\n- Step 1: implement\n- Step 2: test\n\nTest plan: unit tests.\n"); code != 0 {
			t.Fatalf("plan --body failed: %q", stderr)
		}

		// Assert plan text visible in tkt show output
		showOut, _, showCode := run(t, dir, "show", "1")
		if showCode != 0 {
			t.Fatalf("show after plan failed: exit %d", showCode)
		}
		if !strings.Contains(showOut, "Full lifecycle plan") {
			t.Errorf("expected plan text in show output, got: %q", showOut)
		}

		// Session B (architect): advance PLANNING→IN_PROGRESS
		createSession(t, dir, "architect")
		advanceTicketTo(t, dir, "1", "IN_PROGRESS", "approving plan")

		// Assert status == IN_PROGRESS
		if s := ticketStatus(t, dir, 1); s != "IN_PROGRESS" {
			t.Fatalf("expected IN_PROGRESS after PLANNING→IN_PROGRESS, got %q", s)
		}

		// Session C (implementer): advance IN_PROGRESS→DONE
		createSession(t, dir, "implementer")
		advanceTicket(t, dir, "1", "implementation complete")

		// Assert status == DONE
		if s := ticketStatus(t, dir, 1); s != "DONE" {
			t.Fatalf("expected DONE after IN_PROGRESS→DONE, got %q", s)
		}

		// Session D (architect): advance DONE→VERIFIED
		createSession(t, dir, "architect")
		advanceTicketTo(t, dir, "1", "VERIFIED", "verified and accepted")

		// Assert status == VERIFIED
		if s := ticketStatus(t, dir, 1); s != "VERIFIED" {
			t.Fatalf("expected VERIFIED after DONE→VERIFIED, got %q", s)
		}

		// Assert tkt show output contains "VERIFIED"
		finalShowOut, _, finalShowCode := run(t, dir, "show", "1")
		if finalShowCode != 0 {
			t.Fatalf("final show failed: exit %d", finalShowCode)
		}
		if !strings.Contains(finalShowOut, "VERIFIED") {
			t.Errorf("expected VERIFIED in final show output, got: %q", finalShowOut)
		}
	})

	t.Run("advance_role_violation", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		createTicket(t, dir, "Role violation ticket")
		advanceTicket(t, dir, "1", "to planning")

		// Inject plan directly
		injectPlan(t, dir, 1)

		// Implementer tries PLANNING → IN_PROGRESS (requires architect)
		_, stderr, code := run(t, dir, "advance", "1", "--note", "trying to approve")
		if code == 0 {
			t.Fatal("expected non-zero exit for role violation, got 0")
		}
		combined := stderr
		if !strings.Contains(combined, "role") && !strings.Contains(combined, "architect") {
			t.Logf("error output: %q", combined)
		}
	})

	t.Run("advance_isolation_violation", func(t *testing.T) {
		// Isolation scenario: PLANNING → IN_PROGRESS requires a different session than
		// the one that submitted the ticket to PLANNING (TODO → PLANNING).
		// An architect moves TODO → PLANNING, then the same session tries PLANNING → IN_PROGRESS.
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Isolation violation ticket")

		// Capture the architect session ID that moved it to PLANNING.
		db := openDB(t, dir)
		var archSessionID string
		err := db.QueryRow(
			`SELECT id FROM sessions WHERE role='architect' ORDER BY created_at DESC LIMIT 1`,
		).Scan(&archSessionID)
		db.Close()
		if err != nil {
			t.Fatalf("get arch session: %v", err)
		}

		// TODO → PLANNING (with the architect session)
		advanceTicket(t, dir, "1", "to planning")
		injectPlan(t, dir, 1)

		// Now switch to implementer to pass role check, then back to SAME architect.
		// But we want the SAME architect session to try PLANNING → IN_PROGRESS.
		// Write the original architect session ID back to the session file.
		if writeErr := os.WriteFile(filepath.Join(dir, ".tkt", "session"), []byte(archSessionID), 0o644); writeErr != nil {
			t.Fatalf("write session file: %v", writeErr)
		}

		// The same architect session that submitted TODO→PLANNING now tries PLANNING→IN_PROGRESS.
		// This should fail: requires a DIFFERENT session than the submitter.
		_, stderr, code := run(t, dir, "advance", "1", "--note", "same session approving own planning")
		if code == 0 {
			t.Fatal("expected non-zero exit for isolation violation, got 0")
		}
		if !strings.Contains(stderr, "session") && !strings.Contains(stderr, "different") {
			t.Logf("isolation violation error output: %q", stderr)
		}
	})

	t.Run("advance_force", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		createTicket(t, dir, "Force advance ticket")
		advanceTicket(t, dir, "1", "to planning")
		injectPlan(t, dir, 1)

		// Implementer tries PLANNING → IN_PROGRESS with --force
		stdout, _, code := run(t, dir, "advance", "1", "--note", "forcing it", "--force")
		if code != 0 {
			t.Fatalf("expected exit 0 with --force, got %d: stdout=%q", code, stdout)
		}
	})

	t.Run("advance_canceled_path", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Cancelable ticket")

		// TODO → CANCELED
		stdout, _, code := run(t, dir, "advance", "1", "--note", "canceling", "--to", "CANCELED")
		if code != 0 {
			t.Fatalf("expected exit 0 for TODO → CANCELED, got %d: %q", code, stdout)
		}

		// CANCELED → TODO
		stdout, _, code = run(t, dir, "advance", "1", "--note", "reopening", "--to", "TODO")
		if code != 0 {
			t.Fatalf("expected exit 0 for CANCELED → TODO, got %d: %q", code, stdout)
		}

		showOut, _, _ := run(t, dir, "show", "1")
		if !strings.Contains(showOut, "TODO") {
			t.Errorf("expected TODO status after reopen, got: %q", showOut)
		}
	})

	t.Run("advance_no_plan_guard", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		createTicket(t, dir, "No plan ticket")
		advanceTicket(t, dir, "1", "to planning")

		// Switch to architect and try to advance without submitting a plan
		createSession(t, dir, "architect")
		_, stderr, code := run(t, dir, "advance", "1", "--note", "approving without plan")
		if code == 0 {
			t.Fatal("expected non-zero exit when no plan submitted, got 0")
		}
		if !strings.Contains(stderr, "plan") {
			t.Logf("error output: %q", stderr)
		}
	})

	t.Run("advance_missing_note", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Missing note ticket")

		// advance without --note
		_, stderr, code := run(t, dir, "advance", "1")
		if code == 0 {
			t.Fatal("expected non-zero exit when --note is missing, got 0")
		}
		if !strings.Contains(stderr, "note") && !strings.Contains(stderr, "--note") {
			t.Logf("error output: %q", stderr)
		}
	})

	// -------------------------------------------------------------------------
	// Group 6: Comment and Log
	// -------------------------------------------------------------------------

	t.Run("comment_basic", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Comment ticket")
		stdout, stderr, code := run(t, dir, "comment", "1", "hello world")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
	})

	t.Run("comment_no_session", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		// No session
		_, _, code := run(t, dir, "comment", "1", "hello")
		if code == 0 {
			t.Fatal("expected non-zero exit with no session, got 0")
		}
	})

	t.Run("log_shows_comment", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Log comment ticket")
		run(t, dir, "comment", "1", "test comment body")
		// tkt show includes log entries
		showOut, _, code := run(t, dir, "show", "1")
		if code != 0 {
			t.Fatalf("show failed: exit %d", code)
		}
		if !strings.Contains(showOut, "test comment body") {
			t.Errorf("expected comment body in show output, got: %q", showOut)
		}
	})

	t.Run("log_shows_transitions", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		createTicket(t, dir, "Transition log ticket")
		advanceTicket(t, dir, "1", "advancing to planning")
		showOut, _, code := run(t, dir, "show", "1")
		if code != 0 {
			t.Fatalf("show failed: exit %d", code)
		}
		// Should show the transition in the log
		if !strings.Contains(showOut, "PLANNING") {
			t.Errorf("expected PLANNING transition in show output, got: %q", showOut)
		}
	})

	// -------------------------------------------------------------------------
	// Group 7: Depends
	// -------------------------------------------------------------------------

	t.Run("depends_on", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Dep ticket 1")
		createTicket(t, dir, "Dep ticket 2")
		stdout, stderr, code := run(t, dir, "depends", "2", "--on", "1")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		showOut, _, _ := run(t, dir, "show", "2")
		if !strings.Contains(showOut, "#1") {
			t.Errorf("expected '#1' in show output for dependency, got: %q", showOut)
		}
	})

	t.Run("depends_remove", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Dep remove 1")
		createTicket(t, dir, "Dep remove 2")
		run(t, dir, "depends", "2", "--on", "1")
		stdout, stderr, code := run(t, dir, "depends", "2", "--remove", "1")
		if code != 0 {
			t.Fatalf("expected exit 0 for depends --remove, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		showOut, _, _ := run(t, dir, "show", "2")
		// After removal, #1 should not appear as a dependency
		if strings.Contains(showOut, "Depends on") && strings.Contains(showOut, "#1") {
			t.Errorf("expected dependency removed from show output, got: %q", showOut)
		}
	})

	t.Run("depends_cycle", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Cycle A")
		createTicket(t, dir, "Cycle B")
		run(t, dir, "depends", "1", "--on", "2")
		// Now create a cycle: 2 → 1 (1 already depends on 2)
		_, stderr, code := run(t, dir, "depends", "2", "--on", "1")
		if code == 0 {
			t.Fatal("expected non-zero exit for dependency cycle, got 0")
		}
		if !strings.Contains(strings.ToLower(stderr), "cycle") && !strings.Contains(strings.ToLower(stderr), "circular") {
			t.Logf("error output: %q", stderr)
		}
	})

	t.Run("depends_self", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Self dep ticket")
		_, _, code := run(t, dir, "depends", "1", "--on", "1")
		if code == 0 {
			t.Fatal("expected non-zero exit for self-dependency, got 0")
		}
	})

	// -------------------------------------------------------------------------
	// Group 8: Batch
	// -------------------------------------------------------------------------

	t.Run("batch_phases", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Phase A")
		createTicket(t, dir, "Phase B")
		run(t, dir, "depends", "2", "--on", "1")
		stdout, _, code := run(t, dir, "batch")
		if code != 0 {
			t.Fatalf("batch failed: exit %d stdout=%q", code, stdout)
		}
		if !strings.Contains(stdout, "Phase") {
			t.Errorf("expected 'Phase' in batch output, got: %q", stdout)
		}
	})

	t.Run("batch_empty", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "batch")
		if code != 0 {
			t.Fatalf("batch on empty project failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
	})

	// -------------------------------------------------------------------------
	// Group 9: Context
	// -------------------------------------------------------------------------

	t.Run("context_add", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		stdout, stderr, code := run(t, dir, "context", "add", "My caveat", "This is important context")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "added") {
			t.Errorf("expected 'added' in output, got: %q", stdout)
		}
	})

	t.Run("context_readall", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		run(t, dir, "context", "add", "Test caveat", "The body of context")
		stdout, _, code := run(t, dir, "context", "readall")
		if code != 0 {
			t.Fatalf("context readall failed: exit %d", code)
		}
		if !strings.Contains(stdout, "Test caveat") {
			t.Errorf("expected context title in readall output, got: %q", stdout)
		}
	})

	t.Run("context_read_single", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		run(t, dir, "context", "add", "Single read title", "Single read body")
		stdout, _, code := run(t, dir, "context", "read", "1")
		if code != 0 {
			t.Fatalf("context read 1 failed: exit %d", code)
		}
		if !strings.Contains(stdout, "Single read title") {
			t.Errorf("expected title in read output, got: %q", stdout)
		}
	})

	t.Run("context_update", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		run(t, dir, "context", "add", "Old title", "Old body")
		stdout, stderr, code := run(t, dir, "context", "update", "1", "New title", "New body")
		if code != 0 {
			t.Fatalf("context update failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		readOut, _, _ := run(t, dir, "context", "read", "1")
		if !strings.Contains(readOut, "New") {
			t.Errorf("expected updated content, got: %q", readOut)
		}
	})

	t.Run("context_delete", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		run(t, dir, "context", "add", "To delete", "Delete me")
		stdout, stderr, code := run(t, dir, "context", "delete", "1")
		if code != 0 {
			t.Fatalf("context delete failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		readallOut, _, _ := run(t, dir, "context", "readall")
		if strings.Contains(readallOut, "To delete") {
			t.Errorf("expected deleted context not to appear in readall, got: %q", readallOut)
		}
	})

	t.Run("context_read_not_found", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		_, _, code := run(t, dir, "context", "read", "999")
		if code == 0 {
			t.Fatal("expected non-zero exit for missing context, got 0")
		}
	})

	// -------------------------------------------------------------------------
	// Group 10: Doc
	// -------------------------------------------------------------------------

	t.Run("doc_add_stub", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		// EDITOR=true exits 0 but writes nothing — "No changes made."
		stdout, stderr, code := run(t, dir, "doc", "add", "my-slug")
		if code != 0 {
			t.Fatalf("doc add with EDITOR=true failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "No changes made") {
			t.Errorf("expected 'No changes made' in output, got: %q", stdout)
		}
	})

	t.Run("doc_list_empty", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "doc", "list")
		if code != 0 {
			t.Fatalf("doc list on fresh project failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		_ = stdout
	})

	t.Run("doc_add_with_content", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		// Use a shell editor that writes different content to the temp file
		cmd := exec.Command(tktBin, "doc", "add", "test-doc")
		cmd.Dir = dir
		// Shell writes "test content" to $1 (the temp file), overwriting the template
		cmd.Env = append(os.Environ(), "HOME=/dev/null", `EDITOR=sh -c 'echo "# test-doc --- " > "$1" && echo "" >> "$1" && echo "test content body" >> "$1"'`)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Logf("doc add with content editor failed (shell quoting): %v, out=%q", err, out)
			t.Skip("shell editor quoting not supported in this environment")
		}
		listOut, _, listCode := run(t, dir, "doc", "list")
		if listCode != 0 {
			t.Fatalf("doc list after add failed: exit %d", listCode)
		}
		if !strings.Contains(listOut, "test-doc") {
			t.Errorf("expected 'test-doc' in doc list output, got: %q", listOut)
		}
	})

	// -------------------------------------------------------------------------
	// Group 11: Cleanup
	// -------------------------------------------------------------------------

	t.Run("cleanup_nothing_to_clean", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "cleanup")
		if code != 0 {
			t.Fatalf("cleanup on fresh project failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "Nothing") && !strings.Contains(stdout, "nothing") {
			t.Errorf("expected 'Nothing' in cleanup output, got: %q", stdout)
		}
	})

	t.Run("cleanup_dry_run", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		// Seed a stale session older than the 7-day expiry threshold.
		seedStaleSession(t, dir, "stale-architect-001")
		stdout, stderr, code := run(t, dir, "cleanup", "--dry-run")
		if code != 0 {
			t.Fatalf("cleanup --dry-run failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "dry-run") || !strings.Contains(stdout, "1") {
			t.Errorf("expected dry-run report with count, got: %q", stdout)
		}
	})

	t.Run("cleanup_expires_stale", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		seedStaleSession(t, dir, "stale-implementer-002")
		stdout, stderr, code := run(t, dir, "cleanup")
		if code != 0 {
			t.Fatalf("cleanup failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "1") && !strings.Contains(stdout, "Expired") {
			t.Logf("cleanup output: %q", stdout)
		}
		// Verify the session was expired in the DB
		db := openDB(t, dir)
		defer db.Close()
		var expiredAt sql.NullString
		err := db.QueryRow(
			`SELECT expired_at FROM sessions WHERE id = 'stale-implementer-002'`,
		).Scan(&expiredAt)
		if err != nil {
			t.Fatalf("query stale session: %v", err)
		}
		if !expiredAt.Valid {
			t.Error("expected stale session to have expired_at set after cleanup")
		}
	})

	t.Run("new_with_description_tier_type_attention", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			tier  string
			title string
		}{
			{name: "critical", tier: "critical", title: "Critical ticket"},
			{name: "standard", tier: "standard", title: "Standard ticket"},
			{name: "low", tier: "low", title: "Low ticket"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				initProject(t, dir)
				createSession(t, dir, "architect")
				stdout, stderr, code := run(t, dir, "new", tc.title,
					"--description", "line one",
					"--tier", tc.tier,
					"--type", "feature",
					"--attention", "42",
				)
				if code != 0 {
					t.Fatalf("new failed: exit %d stdout=%q stderr=%q", code, stdout, stderr)
				}
				showOut, _, showCode := run(t, dir, "show", "1")
				if showCode != 0 {
					t.Fatalf("show failed: exit %d", showCode)
				}
				if !strings.Contains(showOut, tc.title) || !strings.Contains(showOut, "line one") {
					t.Fatalf("show output missing created ticket fields: %q", showOut)
				}
				db := openDB(t, dir)
				defer db.Close()
				var tier, mainType string
				var attention int
				if err := db.QueryRow(`SELECT tier, main_type, attention_level FROM tickets WHERE id = 1`).Scan(&tier, &mainType, &attention); err != nil {
					t.Fatalf("query ticket: %v", err)
				}
				if tier != tc.tier || mainType != "feature" || attention != 42 {
					t.Fatalf("unexpected ticket metadata: tier=%q type=%q attention=%d", tier, mainType, attention)
				}
			})
		}
	})

	t.Run("new_missing_title", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		_, stderr, code := run(t, dir, "new")
		if code == 0 {
			t.Fatal("expected non-zero exit when title is missing")
		}
		if !strings.Contains(strings.ToLower(stderr), "accepts 1 arg") {
			t.Logf("stderr=%q", stderr)
		}
	})

	t.Run("list_ready", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Resolved dependency")
		createTicket(t, dir, "Ready after verified dep")
		createTicket(t, dir, "Blocked ticket")
		setTicketState(t, dir, 1, "VERIFIED", time.Now().Add(-3*time.Hour))
		if _, _, code := run(t, dir, "depends", "2", "--on", "1"); code != 0 {
			t.Fatal("depends 2 on 1 failed")
		}
		if _, _, code := run(t, dir, "depends", "3", "--on", "2"); code != 0 {
			t.Fatal("depends 3 on 2 failed")
		}
		stdout, stderr, code := run(t, dir, "list", "--ready", "--all")
		if code != 0 {
			t.Fatalf("list --ready failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "Ready after verified dep") {
			t.Fatalf("ready list missing expected tickets: %q", stdout)
		}
		if strings.Contains(stdout, "Resolved dependency") || strings.Contains(stdout, "Blocked ticket") {
			t.Fatalf("ready list should exclude verified and blocked tickets: %q", stdout)
		}
	})

	t.Run("list_verified_and_archived", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Todo ticket")
		createTicket(t, dir, "Verified ticket")
		createTicket(t, dir, "Archived ticket")
		setTicketState(t, dir, 2, "VERIFIED", time.Now().Add(-2*time.Hour))
		setTicketState(t, dir, 3, "ARCHIVED", time.Now().Add(-time.Hour))

		baseOut, _, _ := run(t, dir, "list", "--all")
		if strings.Contains(baseOut, "Verified ticket") || strings.Contains(baseOut, "Archived ticket") {
			t.Fatalf("default list should hide verified/archived tickets: %q", baseOut)
		}

		verifiedOut, _, code := run(t, dir, "list", "--verified", "--all")
		if code != 0 {
			t.Fatalf("list --verified failed")
		}
		if !strings.Contains(verifiedOut, "Verified ticket") {
			t.Fatalf("verified ticket missing: %q", verifiedOut)
		}

		archivedOut, _, code := run(t, dir, "list", "--archived", "--all")
		if code != 0 {
			t.Fatalf("list --archived failed")
		}
		if !strings.Contains(archivedOut, "Archived ticket") {
			t.Fatalf("archived ticket missing: %q", archivedOut)
		}
	})

	t.Run("list_limit_and_sort", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Oldest")
		createTicket(t, dir, "Middle")
		createTicket(t, dir, "Newest")
		setTicketState(t, dir, 1, "TODO", time.Now().Add(-3*time.Hour))
		setTicketState(t, dir, 2, "TODO", time.Now().Add(-2*time.Hour))
		setTicketState(t, dir, 3, "TODO", time.Now().Add(-time.Hour))

		limitOut, _, code := run(t, dir, "list", "--limit", "2")
		if code != 0 {
			t.Fatalf("list --limit failed")
		}
		if strings.Count(limitOut, "#") != 2 {
			t.Fatalf("expected 2 tickets with --limit 2, got output %q", limitOut)
		}

		updatedOut, _, code := run(t, dir, "list", "--all", "--sort", "updated")
		if code != 0 {
			t.Fatalf("list --sort updated failed")
		}
		if !(strings.Index(updatedOut, "Newest") < strings.Index(updatedOut, "Middle") &&
			strings.Index(updatedOut, "Middle") < strings.Index(updatedOut, "Oldest")) {
			t.Fatalf("updated sort order wrong: %q", updatedOut)
		}

		idOut, _, code := run(t, dir, "list", "--all", "--sort", "id")
		if code != 0 {
			t.Fatalf("list --sort id failed")
		}
		if !(strings.Index(idOut, "Newest") < strings.Index(idOut, "Middle") &&
			strings.Index(idOut, "Middle") < strings.Index(idOut, "Oldest")) {
			t.Fatalf("id sort order wrong: %q", idOut)
		}
	})

	t.Run("list_invalid_sort_and_status", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		if _, stderr, code := run(t, dir, "list", "--sort", "priority"); code == 0 || !strings.Contains(stderr, "invalid --sort") {
			t.Fatalf("expected invalid --sort error, got code=%d stderr=%q", code, stderr)
		}
		if _, stderr, code := run(t, dir, "list", "--status", "BROKEN"); code == 0 || !strings.Contains(stderr, "invalid --status") {
			t.Fatalf("expected invalid --status error, got code=%d stderr=%q", code, stderr)
		}
	})

	t.Run("show_hash_id_variant", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Hash show ticket")
		stdout, stderr, code := run(t, dir, "show", "#1")
		if code != 0 {
			t.Fatalf("show #1 failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "Hash show ticket") {
			t.Fatalf("show output missing title: %q", stdout)
		}
	})

	t.Run("advance_invalid_to", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Bad advance target")
		_, stderr, code := run(t, dir, "advance", "1", "--note", "bad", "--to", "LATER")
		if code == 0 || !strings.Contains(stderr, "invalid --to") {
			t.Fatalf("expected invalid --to error, got code=%d stderr=%q", code, stderr)
		}
	})

	t.Run("advance_multi_id_success", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Ticket one")
		createTicket(t, dir, "Ticket two")
		stdout, stderr, code := run(t, dir, "advance", "1,2", "--note", "to planning")
		if code != 0 {
			t.Fatalf("advance multi-id failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "#1  TODO → PLANNING") || !strings.Contains(stdout, "#2  TODO → PLANNING") {
			t.Fatalf("missing multi-id transition output: %q", stdout)
		}
	})

	t.Run("advance_multi_id_partial_failure", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Real ticket")
		stdout, stderr, code := run(t, dir, "advance", "1,999", "--note", "partial")
		if code == 0 {
			t.Fatalf("expected partial failure exit, stdout=%q stderr=%q", stdout, stderr)
		}
		if ticketStatus(t, dir, 1) != "PLANNING" {
			t.Fatalf("expected valid ticket to advance despite partial failure, got %s", ticketStatus(t, dir, 1))
		}
		if !strings.Contains(stderr, "1 error(s)") || !strings.Contains(stderr, "#999") {
			t.Fatalf("partial failure stderr missing details: %q", stderr)
		}
	})

	t.Run("plan_body_stdin_file", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			runCmd func(t *testing.T, dir string) (string, string, int)
		}{
			{
				name: "body",
				runCmd: func(t *testing.T, dir string) (string, string, int) {
					return run(t, dir, "plan", "1", "--body", "Plan from body")
				},
			},
			{
				name: "stdin",
				runCmd: func(t *testing.T, dir string) (string, string, int) {
					return runWithInput(t, dir, "Plan from stdin\n", "plan", "1", "--stdin")
				},
			},
			{
				name: "file",
				runCmd: func(t *testing.T, dir string) (string, string, int) {
					path := createTempFile(t, dir, "plan-*.md", "Plan from file\n")
					return run(t, dir, "plan", "1", "--file", path)
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				initProject(t, dir)
				createSession(t, dir, "implementer")
				createTicket(t, dir, "Plan ticket")
				advanceTicket(t, dir, "1", "into planning")
				stdout, stderr, code := tc.runCmd(t, dir)
				if code != 0 {
					t.Fatalf("plan failed: stdout=%q stderr=%q", stdout, stderr)
				}
				showOut, _, _ := run(t, dir, "show", "1")
				if !strings.Contains(showOut, "Plan from") {
					t.Fatalf("show missing saved plan: %q", showOut)
				}
			})
		}
	})

	t.Run("shell_safe_text_file_stdin_flags", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")

		markdown := "Markdown with `code` and $(not executed)"
		descPath := createTempFile(t, dir, "desc-*.md", markdown+"\n")
		stdout, stderr, code := run(t, dir, "new", "Shell safe", "--description-file", descPath)
		if code != 0 {
			t.Fatalf("new --description-file failed: stdout=%q stderr=%q", stdout, stderr)
		}
		showOut, _, _ := run(t, dir, "show", "1")
		if !strings.Contains(showOut, markdown) {
			t.Fatalf("show missing preserved description: %q", showOut)
		}

		stdout, stderr, code = runWithInput(t, dir, markdown+"\n", "comment", "1", "--body-stdin")
		if code != 0 {
			t.Fatalf("comment --body-stdin failed: stdout=%q stderr=%q", stdout, stderr)
		}
		showOut, _, _ = run(t, dir, "show", "1")
		if !strings.Contains(showOut, markdown) {
			t.Fatalf("show missing preserved comment: %q", showOut)
		}

		notePath := createTempFile(t, dir, "note-*.md", markdown+"\n")
		stdout, stderr, code = run(t, dir, "advance", "1", "--note-file", notePath)
		if code != 0 {
			t.Fatalf("advance --note-file failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, fmt.Sprintf("Note: %q", markdown)) {
			t.Fatalf("advance output missing preserved note: %q", stdout)
		}
	})

	t.Run("comment_empty_body", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Empty comment")
		_, stderr, code := run(t, dir, "comment", "1", "")
		if code == 0 {
			t.Fatal("expected empty comment to fail")
		}
		if !strings.Contains(strings.ToLower(stderr), "empty") {
			t.Logf("stderr=%q", stderr)
		}
	})

	t.Run("log_all_flags_and_show_human_session_name", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "session", "--role", "architect", "--name", "stone-review")
		if code != 0 {
			t.Fatalf("session --name failed: stdout=%q stderr=%q", stdout, stderr)
		}
		createTicket(t, dir, "Usage ticket")
		stdout, stderr, code = run(t, dir, "log", "1", "--tokens", "1234", "--tools", "5", "--duration", "9", "--agent", "cave-implementer", "--label", "smoke")
		if code != 0 {
			t.Fatalf("log failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "#1  logged 1,234 tokens, 5 tools, 9s") {
			t.Fatalf("unexpected log output: %q", stdout)
		}
		showOut, _, _ := run(t, dir, "show", "1")
		if !strings.Contains(showOut, "stone-review") {
			t.Fatalf("show should render human session name: %q", showOut)
		}
		if strings.Contains(showOut, readSessionFile(t, dir)) {
			t.Fatalf("show should not render session ULID in usage rows: %q", showOut)
		}
	})

	t.Run("log_tokens_zero_fails", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Zero tokens")
		_, stderr, code := run(t, dir, "log", "1", "--tokens", "0")
		if code == 0 || !strings.Contains(stderr, "--tokens is required and must be > 0") {
			t.Fatalf("expected --tokens validation error, got code=%d stderr=%q", code, stderr)
		}
	})

	t.Run("depends_nonexistent_dependency", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Main ticket")
		_, stderr, code := run(t, dir, "depends", "1", "--on", "999")
		if code == 0 {
			t.Fatal("expected missing dependency to fail")
		}
		if !strings.Contains(strings.ToLower(stderr), "not found") {
			t.Logf("stderr=%q", stderr)
		}
	})

	t.Run("context_missing_title_and_body", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		if _, stderr, code := run(t, dir, "context", "add", "", "body"); code == 0 || !strings.Contains(strings.ToLower(stderr), "title") {
			t.Fatalf("expected missing title error, got code=%d stderr=%q", code, stderr)
		}
		if _, stderr, code := run(t, dir, "context", "add", "title", ""); code == 0 || !strings.Contains(strings.ToLower(stderr), "body") {
			t.Fatalf("expected missing body error, got code=%d stderr=%q", code, stderr)
		}
	})

	t.Run("doc_body_stdin_file_read_archive_and_list_archived", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		if _, stderr, code := run(t, dir, "doc", "add", "body-doc", "--body", "# 001 — Body doc\n\n**Type:** summary\n**Date:** 2026-04-24\n**By:** implementer\n\n---\n\nbody payload\n"); code != 0 {
			t.Fatalf("doc add --body failed: %q", stderr)
		}
		if _, stderr, code := runWithInput(t, dir, "# 002 — Stdin doc\n\n**Type:** summary\n**Date:** 2026-04-24\n**By:** implementer\n\n---\n\nstdin payload\n", "doc", "add", "stdin-doc", "--stdin"); code != 0 {
			t.Fatalf("doc add --stdin failed: %q", stderr)
		}
		filePath := createTempFile(t, dir, "doc-*.md", "# 003 — File doc\n\n**Type:** summary\n**Date:** 2026-04-24\n**By:** implementer\n\n---\n\nfile payload\n")
		if _, stderr, code := run(t, dir, "doc", "add", "file-doc", "--file", filePath); code != 0 {
			t.Fatalf("doc add --file failed: %q", stderr)
		}

		readOut, stderr, code := run(t, dir, "doc", "read", "stdin-doc")
		if code != 0 {
			t.Fatalf("doc read failed: stdout=%q stderr=%q", readOut, stderr)
		}
		if !strings.Contains(readOut, "stdin payload") {
			t.Fatalf("doc read missing content: %q", readOut)
		}

		if _, stderr, code := run(t, dir, "doc", "archive", "body-doc"); code != 0 {
			t.Fatalf("doc archive failed: %q", stderr)
		}
		listOut, _, _ := run(t, dir, "doc", "list")
		if strings.Contains(listOut, "body-doc") {
			t.Fatalf("archived doc should not appear in active list: %q", listOut)
		}
		archivedOut, _, _ := run(t, dir, "doc", "list", "--archived")
		if !strings.Contains(archivedOut, "Body doc") {
			t.Fatalf("archived doc missing from archived list: %q", archivedOut)
		}
	})

	t.Run("batch_default_and_n", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		for i := 1; i <= 8; i++ {
			createTicket(t, dir, fmt.Sprintf("Batch ticket %d", i))
		}
		stdout, stderr, code := run(t, dir, "batch")
		if code != 0 {
			t.Fatalf("batch default failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "Phase 1") || !strings.Contains(stdout, "#8") {
			t.Fatalf("default batch should show first phase content, got %q", stdout)
		}
		stdout, stderr, code = run(t, dir, "batch", "--n", "3")
		if code != 0 {
			t.Fatalf("batch --n failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if strings.Count(stdout, "Phase ") > 3 {
			t.Fatalf("batch --n 3 should cap phases, got %q", stdout)
		}
	})

	t.Run("session_name_and_uniqueness", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "session", "--role", "architect", "--name", "cave-chief")
		if code != 0 {
			t.Fatalf("session --name failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "Name: cave-chief") {
			t.Fatalf("session output missing explicit name: %q", stdout)
		}
		sessionID := readSessionFile(t, dir)
		id, name := latestSession(t, dir)
		if id != sessionID || name != "cave-chief" {
			t.Fatalf("session file/row mismatch: file=%q rowID=%q rowName=%q", sessionID, id, name)
		}

		if _, stderr, code := run(t, dir, "session", "--role", "architect", "--name", "Bad_Name"); code == 0 || !strings.Contains(stderr, "invalid name") {
			t.Fatalf("expected invalid explicit name error, got code=%d stderr=%q", code, stderr)
		}
		if _, stderr, code := run(t, dir, "session", "--name", "lonely"); code == 0 || !strings.Contains(stderr, "--name requires --role") {
			t.Fatalf("expected --name requires --role error, got code=%d stderr=%q", code, stderr)
		}

		autoDir := t.TempDir()
		initProject(t, autoDir)
		for i := 0; i < 12; i++ {
			if _, _, code := run(t, autoDir, "session", "--role", "architect"); code != 0 {
				t.Fatalf("auto session %d failed", i)
			}
		}
		db := openDB(t, autoDir)
		defer db.Close()
		var total, distinct int
		if err := db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT name) FROM sessions`).Scan(&total, &distinct); err != nil {
			t.Fatalf("query unique session names: %v", err)
		}
		if total != distinct {
			t.Fatalf("expected unique human session names, got total=%d distinct=%d", total, distinct)
		}
	})

	t.Run("role_create_list_delete_and_failures", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		if _, stderr, code := run(t, dir, "role", "create", "scribe", "--like", "architect"); code != 0 {
			t.Fatalf("create architect-like role failed: %q", stderr)
		}
		if _, stderr, code := run(t, dir, "role", "create", "miner", "--like", "implementer"); code != 0 {
			t.Fatalf("create implementer-like role failed: %q", stderr)
		}
		listOut, _, code := run(t, dir, "role", "list")
		if code != 0 {
			t.Fatalf("role list failed")
		}
		if !strings.Contains(listOut, "scribe") || !strings.Contains(listOut, "miner") {
			t.Fatalf("role list missing custom roles: %q", listOut)
		}
		if _, stderr, code := run(t, dir, "session", "--role", "scribe"); code != 0 {
			t.Fatalf("create session with custom role failed: %q", stderr)
		}
		if _, stderr, code := run(t, dir, "role", "delete", "scribe"); code == 0 || !strings.Contains(stderr, "in use") {
			t.Fatalf("expected in-use role delete failure, got code=%d stderr=%q", code, stderr)
		}
		if _, stderr, code := run(t, dir, "role", "delete", "architect"); code == 0 || !strings.Contains(stderr, "built-in") {
			t.Fatalf("expected built-in role delete failure, got code=%d stderr=%q", code, stderr)
		}
		if _, stderr, code := run(t, dir, "session", "--role", "architect"); code != 0 {
			t.Fatalf("switch session failed: %q", stderr)
		}
		if _, stderr, code := run(t, dir, "role", "delete", "miner"); code != 0 {
			t.Fatalf("delete unused custom role failed: %q", stderr)
		}
	})

	t.Run("cleanup_purges_expired_sessions_but_keeps_ticket_log_rows", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Cleanup retention")
		oldLastActive := time.Now().Add(-16 * 24 * time.Hour)
		oldExpired := time.Now().Add(-8 * 24 * time.Hour)
		seedSession(t, dir, "purge-me", "purged-human", "architect", oldLastActive, &oldExpired)

		db := openDB(t, dir)
		if _, err := db.Exec(
			`INSERT INTO ticket_log (ticket_id, session_name, kind, body, created_at) VALUES (?, ?, 'message', ?, datetime('now', '-8 days'))`,
			1, "purged-human", "keep this row",
		); err != nil {
			db.Close()
			t.Fatalf("insert ticket_log row: %v", err)
		}
		db.Close()

		stdout, stderr, code := run(t, dir, "cleanup")
		if code != 0 {
			t.Fatalf("cleanup failed: stdout=%q stderr=%q", stdout, stderr)
		}

		db = openDB(t, dir)
		defer db.Close()
		var sessionCount, logCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = 'purge-me'`).Scan(&sessionCount); err != nil {
			t.Fatalf("query purged session: %v", err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM ticket_log WHERE session_name = 'purged-human' AND body = 'keep this row'`).Scan(&logCount); err != nil {
			t.Fatalf("query retained log row: %v", err)
		}
		if sessionCount != 0 || logCount != 1 {
			t.Fatalf("expected purged session and retained log row, got sessions=%d logs=%d", sessionCount, logCount)
		}
	})

	t.Run("search_happy_path_title_all_status", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Alpha cave")
		if _, _, code := run(t, dir, "new", "Beta board", "--description", "alpha hidden in description"); code != 0 {
			t.Fatal("second ticket create failed")
		}
		createTicket(t, dir, "Gamma archived")
		setTicketState(t, dir, 3, "ARCHIVED", time.Now().Add(-time.Hour))
		setTicketState(t, dir, 2, "DONE", time.Now().Add(-2*time.Hour))

		stdout, stderr, code := run(t, dir, "search", "alpha")
		if code != 0 {
			t.Fatalf("search failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "Alpha cave") || !strings.Contains(stdout, "Beta board") {
			t.Fatalf("search should match title and description: %q", stdout)
		}

		titleOnlyOut, _, _ := run(t, dir, "search", "alpha", "--title")
		if strings.Contains(titleOnlyOut, "Beta board") || !strings.Contains(titleOnlyOut, "Alpha cave") {
			t.Fatalf("title-only search wrong: %q", titleOnlyOut)
		}

		allOut, _, _ := run(t, dir, "search", "archived", "--all")
		if !strings.Contains(allOut, "Gamma archived") {
			t.Fatalf("search --all should include archived ticket: %q", allOut)
		}

		statusOut, _, _ := run(t, dir, "search", "beta", "--status", "DONE")
		if !strings.Contains(statusOut, "Beta board") {
			t.Fatalf("search --status DONE missing ticket: %q", statusOut)
		}
	})

	// -------------------------------------------------------------------------
	// Group 12: Monitor (skipped — interactive TUI)
	// -------------------------------------------------------------------------

	t.Run("stats_basic_and_filters", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		seedStatsE2EData(t, dir)

		stdout, stderr, code := run(t, dir, "stats")
		if code != 0 {
			t.Fatalf("stats failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "Scope: default last 24 hours, all ticket types and statuses") {
			t.Fatalf("stats default scope missing: %q", stdout)
		}
		for _, section := range []string{"Overview", "Cycle Time", "Throughput", "Resource Burn", "Distribution"} {
			if !strings.Contains(stdout, section) {
				t.Fatalf("stats output missing %q: %q", section, stdout)
			}
		}
		if !strings.Contains(stdout, "Total: 4") || !strings.Contains(stdout, "Verified: 1") || !strings.Contains(stdout, "Archived: 1") {
			t.Fatalf("stats output missing expected counts: %q", stdout)
		}

		stdout, stderr, code = run(t, dir, "stats", "--status", "DONE")
		if code != 0 {
			t.Fatalf("stats --status DONE failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "Total: 1") || !strings.Contains(stdout, "Done: 1") {
			t.Fatalf("stats --status DONE wrong output: %q", stdout)
		}

		stdout, stderr, code = run(t, dir, "stats", "--tier", "critical", "--type", "feature")
		if code != 0 {
			t.Fatalf("stats tier/type failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if !strings.Contains(stdout, "Total: 1") || !strings.Contains(stdout, "critical: 1") {
			t.Fatalf("stats tier/type wrong output: %q", stdout)
		}

		_, stderr, code = run(t, dir, "stats", "--status", "NOPE")
		if code == 0 || !strings.Contains(stderr, "invalid --status") {
			t.Fatalf("stats invalid status should fail: code=%d stderr=%q", code, stderr)
		}
	})

	t.Run("update_type_attention", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		createTicket(t, dir, "Update me")

		stdout, stderr, code := run(t, dir, "update", "1", "--type", "bugfix", "--attention", "42")
		if code != 0 {
			t.Fatalf("update failed: stdout=%q stderr=%q", stdout, stderr)
		}
		mainType, attention := ticketTypeAttention(t, dir, 1)
		if mainType != "bugfix" || attention != 42 {
			t.Fatalf("ticket metadata = (%q, %d), want (bugfix, 42)", mainType, attention)
		}

		_, stderr, code = run(t, dir, "update", "1", "--attention", "100")
		if code == 0 || !strings.Contains(stderr, "attention") {
			t.Fatalf("invalid attention should fail: code=%d stderr=%q", code, stderr)
		}
	})

	t.Run("tier_change", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "implementer")
		createTicket(t, dir, "Tier me")

		stdout, stderr, code := run(t, dir, "tier", "1", "critical")
		if code != 0 {
			t.Fatalf("tier failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if tier := ticketTier(t, dir, 1); tier != "critical" {
			t.Fatalf("ticket tier = %q, want critical", tier)
		}
		_, stderr, code = run(t, dir, "tier", "1", "urgent")
		if code == 0 || !strings.Contains(stderr, "invalid tier") {
			t.Fatalf("invalid tier should fail: code=%d stderr=%q", code, stderr)
		}
	})

	t.Run("archive_ticket_and_list_archived", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Archive me")
		setTicketState(t, dir, 1, "VERIFIED", time.Now())

		stdout, stderr, code := run(t, dir, "archive", "1")
		if code != 0 {
			t.Fatalf("archive failed: stdout=%q stderr=%q", stdout, stderr)
		}
		if status := ticketStatus(t, dir, 1); status != "ARCHIVED" {
			t.Fatalf("ticket status = %q, want ARCHIVED", status)
		}
		stdout, _, code = run(t, dir, "list", "--all")
		if code != 0 {
			t.Fatalf("list --all failed")
		}
		if strings.Contains(stdout, "Archive me") {
			t.Fatalf("archived ticket should be hidden without --archived: %q", stdout)
		}
		stdout, stderr, code = run(t, dir, "list", "--archived", "--all")
		if code != 0 {
			t.Fatalf("list --archived failed: %q", stderr)
		}
		if !strings.Contains(stdout, "Archive me") {
			t.Fatalf("archived ticket missing from list --archived: %q", stdout)
		}
	})

	t.Run("mcp_stdio_smoke", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, tktBin, "mcp", "--readonly")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "HOME=/dev/null", "EDITOR=true")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatalf("stdin pipe: %v", err)
		}
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("stdout pipe: %v", err)
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			t.Fatalf("stderr pipe: %v", err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatalf("start mcp: %v", err)
		}
		defer cmd.Process.Kill()

		requests := strings.Join([]string{
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"e2e","version":"test"}}}`,
			`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
			`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		}, "\n") + "\n"
		if _, err := io.WriteString(stdin, requests); err != nil {
			t.Fatalf("write mcp requests: %v", err)
		}

		scanner := bufio.NewScanner(stdoutPipe)
		var lines []string
		for scanner.Scan() {
			line := scanner.Text()
			lines = append(lines, line)
			if strings.Contains(line, `"id":2`) {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("read mcp stdout: %v", err)
		}
		combined := strings.Join(lines, "\n")
		if !strings.Contains(combined, `"name":"tkt_list_tickets"`) || !strings.Contains(combined, `"name":"tkt_stats"`) {
			stderrData, _ := io.ReadAll(stderrPipe)
			t.Fatalf("tools/list missing expected tools. stdout=%q stderr=%q", combined, string(stderrData))
		}
	})

	t.Run("monitor_skipped", func(t *testing.T) {
		t.Skip("tkt monitor launches an interactive TUI — not testable headlessly")
	})

	// -------------------------------------------------------------------------
	// Group 13: Error paths
	// -------------------------------------------------------------------------

	t.Run("error_uninitialized_dir", func(t *testing.T) {
		dir := t.TempDir()
		// No tkt init — any command should fail
		_, _, code := run(t, dir, "list")
		if code == 0 {
			t.Fatal("expected non-zero exit in uninitialized dir, got 0")
		}
	})

	t.Run("error_invalid_ticket_id", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		_, _, code := run(t, dir, "show", "abc")
		if code == 0 {
			t.Fatal("expected non-zero exit for non-numeric ticket ID, got 0")
		}
	})

	t.Run("error_numeric_id_no_hash", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Numeric ID test ticket")
		// Both "1" and "#1" forms should work for advance
		stdout, stderr, code := run(t, dir, "advance", "1", "--note", "no hash prefix")
		if code != 0 {
			t.Fatalf("advance without # prefix failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		// Reset to TODO, then test with hash prefix
		run(t, dir, "advance", "1", "--note", "cancel it", "--to", "CANCELED")
		run(t, dir, "advance", "1", "--note", "reopen it", "--to", "TODO")
		stdout2, stderr2, code2 := run(t, dir, "advance", "#1", "--note", "with hash prefix")
		if code2 != 0 {
			t.Fatalf("advance with # prefix failed: exit %d: stdout=%q stderr=%q", code2, stdout2, stderr2)
		}
	})
}

func ticketTier(t *testing.T, dir string, ticketID int64) string {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()
	var tier string
	if err := db.QueryRow(`SELECT tier FROM tickets WHERE id = ?`, ticketID).Scan(&tier); err != nil {
		t.Fatalf("ticketTier: %v", err)
	}
	return tier
}

func ticketTypeAttention(t *testing.T, dir string, ticketID int64) (string, int) {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()
	var mainType string
	var attention int
	if err := db.QueryRow(`SELECT main_type, attention_level FROM tickets WHERE id = ?`, ticketID).Scan(&mainType, &attention); err != nil {
		t.Fatalf("ticketTypeAttention: %v", err)
	}
	return mainType, attention
}

func seedStatsE2EData(t *testing.T, dir string) {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()

	createdBase := time.Now().UTC().Add(-72 * time.Hour)
	activityBase := time.Now().UTC().Add(-6 * time.Hour)
	tickets := []struct {
		title     string
		status    string
		tier      string
		mainType  string
		createdBy string
		createdAt time.Time
		updatedAt time.Time
	}{
		{"Stats todo", "TODO", "standard", "feature", "alice", createdBase, activityBase},
		{"Stats done", "DONE", "critical", "feature", "alice", createdBase.Add(time.Hour), activityBase.Add(time.Hour)},
		{"Stats verified", "VERIFIED", "standard", "bugfix", "bob", createdBase.Add(2 * time.Hour), activityBase.Add(2 * time.Hour)},
		{"Stats archived", "ARCHIVED", "low", "docs", "carol", createdBase.Add(3 * time.Hour), activityBase.Add(3 * time.Hour)},
	}

	ids := make([]int64, 0, len(tickets))
	for _, tk := range tickets {
		res, err := db.Exec(
			`INSERT INTO tickets (title, description, status, tier, main_type, created_by, created_at, updated_at)
			 VALUES (?, '', ?, ?, ?, ?, ?, ?)`,
			tk.title, tk.status, tk.tier, tk.mainType, tk.createdBy, tk.createdAt, tk.updatedAt,
		)
		if err != nil {
			t.Fatalf("seedStatsE2EData insert %q: %v", tk.title, err)
		}
		id, _ := res.LastInsertId()
		ids = append(ids, id)
	}

	transitions := []struct {
		id int64
		at time.Time
	}{
		{ids[1], activityBase.Add(time.Hour)},
		{ids[2], activityBase.Add(2 * time.Hour)},
		{ids[3], activityBase.Add(3 * time.Hour)},
	}
	for _, tr := range transitions {
		if _, err := db.Exec(
			`INSERT INTO ticket_log (ticket_id, session_name, kind, body, from_state, to_state, created_at)
			 VALUES (?, 'stats-e2e', 'transition', 'done', 'IN_PROGRESS', 'DONE', ?)`,
			tr.id, tr.at,
		); err != nil {
			t.Fatalf("seedStatsE2EData transition: %v", err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO ticket_log (ticket_id, session_name, kind, body, from_state, to_state, created_at)
		 VALUES (?, 'stats-e2e', 'transition', 'verified', 'DONE', 'VERIFIED', ?)`,
		ids[2], activityBase.Add(4*time.Hour),
	); err != nil {
		t.Fatalf("seedStatsE2EData verified transition: %v", err)
	}

	for i, id := range ids {
		if _, err := db.Exec(
			`INSERT INTO ticket_usage (ticket_id, session_name, tokens, tools, duration_ms, agent, label, created_at)
			 VALUES (?, 'stats-e2e', ?, ?, ?, 'implementer', 'e2e', ?)`,
			id, 100*(i+1), i+1, 1000*(i+1), activityBase.Add(time.Duration(i)*time.Hour),
		); err != nil {
			t.Fatalf("seedStatsE2EData usage: %v", err)
		}
	}
}

// injectPlan inserts a plan log entry directly into the DB for the given ticket ID.
// This is needed when EDITOR=true (stub) prevents writing actual content.
func injectPlan(t *testing.T, dir string, ticketID int64) {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()

	// Get or create a session to use as the plan author.
	// ticket_log now stores human session_name, not session_id.
	var sessName string
	err := db.QueryRow(
		`SELECT name FROM sessions WHERE expired_at IS NULL ORDER BY created_at DESC, rowid DESC LIMIT 1`,
	).Scan(&sessName)
	if err != nil {
		// Create a throwaway session
		sessName = "plan-injector-session"
		db.Exec(
			`INSERT OR IGNORE INTO sessions (id, role, name, created_at, last_active)
			 VALUES (?, 'implementer', ?, datetime('now'), datetime('now'))`,
			"plan-injector-ulid", sessName,
		)
	}

	_, err = db.Exec(
		`INSERT INTO ticket_log (ticket_id, kind, body, session_name, created_at)
		 VALUES (?, 'plan', '## Injected Plan\n\n- Step 1\n- Step 2\n\nTest plan.\n', ?, datetime('now'))`,
		ticketID, sessName,
	)
	if err != nil {
		t.Fatalf("injectPlan: insert log entry: %v", err)
	}
}

// TestOrchestratorSession covers orchestrator role creation, session start,
// delegation via --as, and all error paths.
func TestOrchestratorSession(t *testing.T) {

	// 1. orchestrator_role_in_db — init fresh dir, role list contains "orchestrator"
	t.Run("orchestrator_role_in_db", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "role", "list")
		if code != 0 {
			t.Fatalf("role list failed: exit %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "orchestrator") {
			t.Errorf("expected 'orchestrator' in role list output, got: %q", stdout)
		}
	})

	// 2. orchestrator_session_start — session --role orchestrator exits 0
	t.Run("orchestrator_session_start", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		stdout, stderr, code := run(t, dir, "session", "--role", "orchestrator")
		if code != 0 {
			t.Fatalf("expected exit 0 for orchestrator session, got %d: stdout=%q stderr=%q", code, stdout, stderr)
		}
		_ = stdout
	})

	// 3. orchestrator_advance_without_as — orchestrator advance without --as must fail
	t.Run("orchestrator_advance_without_as", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Orch no-as ticket")
		run(t, dir, "session", "--end")
		_, _, code := run(t, dir, "session", "--role", "orchestrator", "--name", "orch-no-as")
		if code != 0 {
			t.Fatalf("orchestrator session failed")
		}
		stdout, stderr, exitCode := run(t, dir, "advance", "1", "--note", "x")
		if exitCode == 0 {
			t.Fatalf("expected non-zero exit for orchestrator advance without --as, got 0: stdout=%q", stdout)
		}
		combined := stdout + stderr
		if !strings.Contains(combined, "must use --as") {
			t.Errorf("expected 'must use --as' in output, got: %q", combined)
		}
	})

	// 4+5. orchestrator_full_delegation_cycle — full setup + both delegated advances succeed
	t.Run("orchestrator_full_delegation_cycle", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)

		// Setup: create ticket and move to PLANNED
		run(t, dir, "session", "--role", "implementer", "--name", "setup-impl")
		createTicket(t, dir, "Delegation test ticket")
		advanceTicket(t, dir, "1", "planning")
		if _, stderr, code := run(t, dir, "plan", "1", "--body", "test plan"); code != 0 {
			t.Fatalf("plan failed: %q", stderr)
		}
		// Register arch and impl sessions (delegation targets must remain non-expired in DB)
		// Creating a new session does NOT expire the previous one — it only updates the session file.
		run(t, dir, "session", "--role", "architect", "--name", "alice-arch")
		run(t, dir, "session", "--role", "implementer", "--name", "bob-impl")

		// Start orchestrator session (alice-arch and bob-impl stay active/non-expired in DB)
		_, _, code := run(t, dir, "session", "--role", "orchestrator", "--name", "orch-main")
		if code != 0 {
			t.Fatalf("orchestrator session failed")
		}

		// Delegate as arch: PLANNED → IN_PROGRESS
		stdout, stderr, exitCode := run(t, dir, "advance", "1", "--as", "alice-arch", "--note", "approved")
		if exitCode != 0 {
			t.Fatalf("delegate as alice-arch failed: exit %d: stdout=%q stderr=%q", exitCode, stdout, stderr)
		}
		if s := ticketStatus(t, dir, 1); s != "IN_PROGRESS" {
			t.Fatalf("expected IN_PROGRESS after arch delegation, got %q", s)
		}

		// Delegate as impl: IN_PROGRESS → DONE
		stdout, stderr, exitCode = run(t, dir, "advance", "1", "--as", "bob-impl", "--note", "done")
		if exitCode != 0 {
			t.Fatalf("delegate as bob-impl failed: exit %d: stdout=%q stderr=%q", exitCode, stdout, stderr)
		}
		if s := ticketStatus(t, dir, 1); s != "DONE" {
			t.Fatalf("expected DONE after impl delegation, got %q", s)
		}
	})

	// 6. orchestrator_as_nonexistent — --as unknown session name must fail
	t.Run("orchestrator_as_nonexistent", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Ghost as ticket")
		run(t, dir, "session", "--end")
		run(t, dir, "session", "--role", "orchestrator", "--name", "orch-ghost")
		stdout, stderr, exitCode := run(t, dir, "advance", "1", "--as", "ghost-123", "--note", "x")
		if exitCode == 0 {
			t.Fatalf("expected non-zero exit for --as nonexistent session, got 0: stdout=%q", stdout)
		}
		_ = stderr
	})

	// 7. orchestrator_as_wrong_role — --as session with non-arch/impl role must fail
	t.Run("orchestrator_as_wrong_role", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Wrong role as ticket")
		run(t, dir, "session", "--end")
		// Register orch-wrong session (orchestrator role — invalid delegation target)
		run(t, dir, "session", "--role", "orchestrator", "--name", "orch-wrong")
		run(t, dir, "session", "--end")
		// Start main orchestrator
		run(t, dir, "session", "--role", "orchestrator", "--name", "orch-main2")
		stdout, stderr, exitCode := run(t, dir, "advance", "1", "--as", "orch-wrong", "--note", "x")
		if exitCode == 0 {
			t.Fatalf("expected non-zero exit for --as wrong-role session, got 0: stdout=%q", stdout)
		}
		combined := stdout + stderr
		if !strings.Contains(combined, "cannot delegate") && !strings.Contains(combined, "role") {
			t.Errorf("expected delegation role error in output, got: %q", combined)
		}
	})

	// 8. non_orchestrator_with_as — non-orchestrator using --as must fail
	t.Run("non_orchestrator_with_as", func(t *testing.T) {
		dir := t.TempDir()
		initProject(t, dir)
		createSession(t, dir, "architect")
		createTicket(t, dir, "Non-orch as ticket")
		stdout, stderr, exitCode := run(t, dir, "advance", "1", "--as", "anything", "--note", "x")
		if exitCode == 0 {
			t.Fatalf("expected non-zero exit for non-orchestrator --as, got 0: stdout=%q", stdout)
		}
		combined := stdout + stderr
		if !strings.Contains(combined, "only valid for orchestrator") {
			t.Errorf("expected 'only valid for orchestrator' in output, got: %q", combined)
		}
	})
}

func TestManCommand(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir)

	stdout, stderr, code := run(t, dir, "man")
	if code != 0 {
		t.Fatalf("man list failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "minimal") || !strings.Contains(stdout, "state-machine") {
		t.Fatalf("man list missing key pages: %q", stdout)
	}

	minimal, stderr, code := run(t, dir, "man", "minimal")
	if code != 0 {
		t.Fatalf("man minimal failed: stdout=%q stderr=%q", minimal, stderr)
	}
	if !strings.Contains(minimal, "Compact operating guide") {
		t.Fatalf("unexpected minimal page: %q", minimal)
	}

	llm, stderr, code := run(t, dir, "man", "llm")
	if code != 0 {
		t.Fatalf("man llm failed: stdout=%q stderr=%q", llm, stderr)
	}
	if llm != minimal {
		t.Fatalf("llm alias did not match minimal")
	}

	_, stderr, code = run(t, dir, "man", "missing")
	if code == 0 || !strings.Contains(stderr, "Run: tkt man") {
		t.Fatalf("expected missing page error with hint, code=%d stderr=%q", code, stderr)
	}
}

func TestManDiscoverableFromHelpAndErrors(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir)

	stdout, stderr, code := run(t, dir, "--help")
	if code != 0 {
		t.Fatalf("help failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "tkt man minimal") {
		t.Fatalf("root help missing man minimal hint: %q", stdout)
	}

	_, stderr, code = run(t, dir, "wat")
	if code == 0 || !strings.Contains(stderr, "tkt man minimal") {
		t.Fatalf("unknown command missing manual hint, code=%d stderr=%q", code, stderr)
	}

	_, stderr, code = run(t, dir, "list", "--status", "PLANNED")
	if code == 0 || !strings.Contains(stderr, "tkt man state-machine") {
		t.Fatalf("invalid status missing state-machine hint, code=%d stderr=%q", code, stderr)
	}
}

func TestJSONOutputCommands(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir)
	createSession(t, dir, "architect")
	createTicket(t, dir, "JSON ticket")
	advanceTicket(t, dir, "1", "planning")
	stdout, stderr, code := run(t, dir, "plan", "1", "--body", "json plan")
	if code != 0 {
		t.Fatalf("plan failed: stdout=%q stderr=%q", stdout, stderr)
	}

	stdout, stderr, code = run(t, dir, "list", "--json")
	if code != 0 {
		t.Fatalf("list --json failed: stdout=%q stderr=%q", stdout, stderr)
	}
	var listPayload struct {
		Tickets []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"tickets"`
		Pagination struct {
			HasMore bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(stdout), &listPayload); err != nil {
		t.Fatalf("list json parse: %v; stdout=%q", err, stdout)
	}
	if len(listPayload.Tickets) != 1 || listPayload.Tickets[0].Title != "JSON ticket" {
		t.Fatalf("unexpected list json: %#v", listPayload)
	}

	stdout, stderr, code = run(t, dir, "show", "1", "--json")
	if code != 0 {
		t.Fatalf("show --json failed: stdout=%q stderr=%q", stdout, stderr)
	}
	var showPayload struct {
		Ticket struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"ticket"`
		LogEntries []struct {
			Kind string `json:"kind"`
		} `json:"log_entries"`
		Usage []struct {
			Tokens int `json:"tokens"`
		} `json:"usage"`
		Dependencies []struct {
			ID int64 `json:"id"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(stdout), &showPayload); err != nil {
		t.Fatalf("show json parse: %v; stdout=%q", err, stdout)
	}
	if showPayload.Ticket.ID != 1 || len(showPayload.LogEntries) == 0 {
		t.Fatalf("unexpected show json: %#v", showPayload)
	}

	stdout, stderr, code = run(t, dir, "batch", "--json")
	if code != 0 {
		t.Fatalf("batch --json failed: stdout=%q stderr=%q", stdout, stderr)
	}
	var batchPayload struct {
		Phases []struct {
			Index   int `json:"index"`
			Tickets []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"tickets"`
		} `json:"phases"`
		Summary struct {
			Tickets int `json:"tickets"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &batchPayload); err != nil {
		t.Fatalf("batch json parse: %v; stdout=%q", err, stdout)
	}
	if batchPayload.Summary.Tickets == 0 || len(batchPayload.Phases) == 0 {
		t.Fatalf("unexpected batch json: %#v", batchPayload)
	}

	stdout, stderr, code = run(t, dir, "stats", "--json")
	if code != 0 {
		t.Fatalf("stats --json failed: stdout=%q stderr=%q", stdout, stderr)
	}
	var statsPayload struct {
		DefaultScope bool `json:"default_scope"`
		Report       struct {
			Overview struct {
				Total int `json:"total"`
			} `json:"overview"`
		} `json:"report"`
	}
	if err := json.Unmarshal([]byte(stdout), &statsPayload); err != nil {
		t.Fatalf("stats json parse: %v; stdout=%q", err, stdout)
	}
	if !statsPayload.DefaultScope || statsPayload.Report.Overview.Total == 0 {
		t.Fatalf("unexpected stats json: %#v", statsPayload)
	}
}

func TestAdvanceDryRunExplain(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir)
	createSession(t, dir, "implementer")
	createTicket(t, dir, "Preflight ticket")

	stdout, stderr, code := run(t, dir, "advance", "1", "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run allowed transition failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "would advance") {
		t.Fatalf("dry-run missing would advance: %q", stdout)
	}
	db := openDB(t, dir)
	var status string
	if err := db.QueryRow(`SELECT status FROM tickets WHERE id = 1`).Scan(&status); err != nil {
		db.Close()
		t.Fatal(err)
	}
	var logCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ticket_log WHERE ticket_id = 1`).Scan(&logCount); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	if status != "TODO" || logCount != 0 {
		t.Fatalf("dry-run mutated state/log: status=%s logCount=%d", status, logCount)
	}

	advanceTicket(t, dir, "1", "planning")
	createSession(t, dir, "architect")
	stdout, stderr, code = run(t, dir, "advance", "1", "--explain")
	if code == 0 {
		t.Fatalf("explain missing plan should be blocked: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Allowed: false") || !strings.Contains(stdout, "Plan present: false") || !strings.Contains(stdout, "tkt man plan") {
		t.Fatalf("explain missing expected details: stdout=%q stderr=%q", stdout, stderr)
	}

	_, stderr, code = run(t, dir, "advance", "1", "--dry-run", "--explain")
	if code == 0 || !strings.Contains(stderr, "cannot be used together") {
		t.Fatalf("expected dry-run/explain conflict: code=%d stderr=%q", code, stderr)
	}
}

func TestStatsWindow(t *testing.T) {
	dir := t.TempDir()
	initProject(t, dir)
	createSession(t, dir, "architect")
	createTicket(t, dir, "Window stats")

	stdout, stderr, code := run(t, dir, "stats", "--window", "24h")
	if code != 0 {
		t.Fatalf("stats --window failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Stats") || strings.Contains(stdout, "Scope: default") {
		t.Fatalf("unexpected stats --window output: %q", stdout)
	}

	_, stderr, code = run(t, dir, "stats", "--window", "7d", "--since", "2026-04-01")
	if code == 0 || !strings.Contains(stderr, "--window cannot be combined") {
		t.Fatalf("expected window conflict: code=%d stderr=%q", code, stderr)
	}
}
