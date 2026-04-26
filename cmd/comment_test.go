package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	tktlog "github.com/zalshy/tkt/internal/log"
)

// runCommentInDir sets rootDir, invokes runComment, returns captured stdout and error.
func runCommentInDir(t *testing.T, dir string, args []string, setupFlags ...func()) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	savedBody := commentBody
	savedBodyFile := commentBodyFile
	savedBodyStdin := commentBodyStdin
	defer func() {
		rootDir = savedRootDir
		commentBody = savedBody
		commentBodyFile = savedBodyFile
		commentBodyStdin = savedBodyStdin
		commentCmd.SetOut(nil)
		commentCmd.SetIn(nil)
		commentCmd.SilenceErrors = false
	}()

	rootDir = dir
	commentBody = ""
	commentBodyFile = ""
	commentBodyStdin = false

	for _, setup := range setupFlags {
		if setup != nil {
			setup()
		}
	}

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

func TestComment_BodyFilePreservesMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-comment-file")
	id := seedTicketWithStatus(t, dir, "Comment target", "TODO")
	body := "Markdown `code` and $(not executed)"
	path := filepath.Join(dir, "comment.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runCommentInDir(t, dir, []string{id}, func() {
		commentBodyFile = path
	})
	if err != nil {
		t.Fatalf("runComment: %v", err)
	}
	if !strings.Contains(out, fmt.Sprintf("#%s", id)) {
		t.Fatalf("expected ticket id in output, got %q", out)
	}

	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	entries, err := tktlog.GetAll(context.Background(), id, database)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Body != body {
		t.Fatalf("log entries = %+v, want body %q", entries, body)
	}
}

func TestComment_ConflictingBodySources(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-comment-conflict")
	id := seedTicketWithStatus(t, dir, "Comment target", "TODO")

	_, err := runCommentInDir(t, dir, []string{id, "positional"}, func() {
		commentBody = "inline"
	})
	if err == nil || !strings.Contains(err.Error(), "provide only one comment body source") {
		t.Fatalf("expected conflict error, got %v", err)
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
