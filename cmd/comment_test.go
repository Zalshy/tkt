package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// runCommentInDir sets rootDir, invokes runComment, returns captured stdout and error.
func runCommentInDir(t *testing.T, dir string, args []string) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	defer func() {
		rootDir = savedRootDir
		commentCmd.SetOut(nil)
		commentCmd.SilenceErrors = false
	}()

	rootDir = dir

	var buf bytes.Buffer
	commentCmd.SetOut(&buf)

	err := runComment(commentCmd, args)
	return buf.String(), err
}

// TestComment_SingleID verifies a basic comment on a single ticket.
func TestComment_SingleID(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-comment-001")
	id := seedTicketWithStatus(t, dir, "Comment target", "TODO")

	out, err := runCommentInDir(t, dir, []string{id, "hello world"})
	if err != nil {
		t.Fatalf("runComment: %v", err)
	}
	if !strings.Contains(out, fmt.Sprintf("#%s", id)) {
		t.Errorf("expected '#%s' in output, got: %q", id, out)
	}
	if !strings.Contains(out, `"hello world"`) {
		t.Errorf("expected quoted body in output, got: %q", out)
	}
}

// TestComment_MultiID_AllSuccess verifies all tickets get commented and exit 0.
func TestComment_MultiID_AllSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-comment-002")
	id1 := seedTicketWithStatus(t, dir, "Ticket A", "TODO")
	id2 := seedTicketWithStatus(t, dir, "Ticket B", "TODO")

	raw := id1 + "," + id2
	out, err := runCommentInDir(t, dir, []string{raw, "batch comment"})
	if err != nil {
		t.Fatalf("runComment: %v", err)
	}
	if !strings.Contains(out, fmt.Sprintf("#%s", id1)) {
		t.Errorf("expected '#%s' in output, got: %q", id1, out)
	}
	if !strings.Contains(out, fmt.Sprintf("#%s", id2)) {
		t.Errorf("expected '#%s' in output, got: %q", id2, out)
	}
}

// TestComment_MultiID_PartialFailure verifies successes applied, errors summarised, exit non-zero.
func TestComment_MultiID_PartialFailure(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-comment-003")
	id1 := seedTicketWithStatus(t, dir, "Good ticket", "TODO")
	// 99999 does not exist.
	badID := "99999"

	raw := id1 + "," + badID
	out, err := runCommentInDir(t, dir, []string{raw, "partial"})
	// Must return non-nil sentinel error.
	if err == nil {
		t.Fatal("expected non-nil error for partial failure, got nil")
	}
	if err.Error() != "" {
		t.Errorf("expected empty sentinel error, got: %v", err)
	}
	// Good ticket should still appear in stdout.
	if !strings.Contains(out, fmt.Sprintf("#%s", id1)) {
		t.Errorf("expected '#%s' (success) in output, got: %q", id1, out)
	}
}
