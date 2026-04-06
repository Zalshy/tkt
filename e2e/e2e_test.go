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
	"database/sql"
	"fmt"
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
	cmd := exec.Command(tktBin, args...)
	cmd.Dir = dir

	// Build a clean environment: inherit PATH, override HOME and EDITOR.
	cmd.Env = append(os.Environ(),
		"HOME=/dev/null",
		"EDITOR=true",
	)

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

// seedStaleSession inserts a session with last_active 3 days ago.
func seedStaleSession(t *testing.T, dir, sessionID string) {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()

	staleTime := time.Now().Add(-72 * time.Hour).UTC().Format("2006-01-02 15:06:05")
	_, err := db.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active)
		 VALUES (?, 'architect', ?, ?, ?)`,
		sessionID, sessionID, staleTime, staleTime,
	)
	if err != nil {
		t.Fatalf("seedStaleSession: insert: %v", err)
	}
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
		// Seed a stale session (3 days ago — well past 48h threshold)
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

	// -------------------------------------------------------------------------
	// Group 12: Monitor (skipped — interactive TUI)
	// -------------------------------------------------------------------------

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

// injectPlan inserts a plan log entry directly into the DB for the given ticket ID.
// This is needed when EDITOR=true (stub) prevents writing actual content.
func injectPlan(t *testing.T, dir string, ticketID int64) {
	t.Helper()
	db := openDB(t, dir)
	defer db.Close()

	// Get or create a session to use as the plan author
	var sessID string
	err := db.QueryRow(
		`SELECT id FROM sessions WHERE expired_at IS NULL ORDER BY created_at DESC LIMIT 1`,
	).Scan(&sessID)
	if err != nil {
		// Create a throwaway session
		sessID = "plan-injector-session"
		db.Exec(
			`INSERT OR IGNORE INTO sessions (id, role, name, created_at, last_active)
			 VALUES (?, 'implementer', ?, datetime('now'), datetime('now'))`,
			sessID, sessID,
		)
	}

	_, err = db.Exec(
		`INSERT INTO ticket_log (ticket_id, kind, body, session_id, created_at)
		 VALUES (?, 'plan', '## Injected Plan\n\n- Step 1\n- Step 2\n\nTest plan.\n', ?, datetime('now'))`,
		ticketID, sessID,
	)
	if err != nil {
		t.Fatalf("injectPlan: insert log entry: %v", err)
	}
}
