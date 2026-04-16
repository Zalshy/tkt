package log_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	ilog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/models"
)

func mustOpenDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	tktDir := filepath.Join(dir, ".tkt")
	if err := os.MkdirAll(tktDir, 0o755); err != nil {
		t.Fatalf("mkdir .tkt: %v", err)
	}
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func mustSeedDB(t *testing.T, database *sql.DB) *models.Session {
	t.Helper()
	_, err := database.Exec(`INSERT INTO sessions (id, role, name) VALUES ('test-sess', 'implementer', 'test')`)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	_, err = database.Exec(`INSERT INTO tickets (id, title, created_by) VALUES (1, 'test ticket', 'test-sess')`)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	return &models.Session{ID: "test-sess"}
}

func TestAppend_Roundtrip(t *testing.T) {
	database := mustOpenDB(t)
	actor := mustSeedDB(t, database)

	err := ilog.Append(context.Background(), 1, "message", "hello", nil, nil, actor, database)
	if err != nil {
		t.Fatalf("Append returned unexpected error: %v", err)
	}

	entries, err := ilog.GetAll(context.Background(), "1", database)
	if err != nil {
		t.Fatalf("GetAll error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Kind != "message" {
		t.Errorf("Kind: want %q, got %q", "message", e.Kind)
	}
	if e.Body != "hello" {
		t.Errorf("Body: want %q, got %q", "hello", e.Body)
	}
	if e.SessionID != "test-sess" {
		t.Errorf("SessionID: want %q, got %q", "test-sess", e.SessionID)
	}
	if e.CreatedAt.IsZero() {
		t.Error("CreatedAt must not be zero")
	}
}

func TestAppend_EmptyBody(t *testing.T) {
	database := mustOpenDB(t)
	actor := mustSeedDB(t, database)

	err := ilog.Append(context.Background(), 1, "message", "", nil, nil, actor, database)
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}

	entries, err := ilog.GetAll(context.Background(), "1", database)
	if err != nil {
		t.Fatalf("GetAll error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestAppend_TransitionValidation(t *testing.T) {
	database := mustOpenDB(t)
	actor := mustSeedDB(t, database)

	// transition with both nil — error
	err := ilog.Append(context.Background(), 1, "transition", "note", nil, nil, actor, database)
	if err == nil {
		t.Error("expected error for transition with fromState=nil, toState=nil")
	}

	// transition with only fromState — error
	from := "TODO"
	err = ilog.Append(context.Background(), 1, "transition", "note", &from, nil, actor, database)
	if err == nil {
		t.Error("expected error for transition with toState=nil")
	}

	// message with fromState set — error
	err = ilog.Append(context.Background(), 1, "message", "note", &from, nil, actor, database)
	if err == nil {
		t.Error("expected error for message with non-nil fromState")
	}

	// valid transition — no error
	to := "PLANNING"
	err = ilog.Append(context.Background(), 1, "transition", "note", &from, &to, actor, database)
	if err != nil {
		t.Errorf("unexpected error for valid transition: %v", err)
	}
}

func TestGetAll_ChronologicalOrder(t *testing.T) {
	database := mustOpenDB(t)
	actor := mustSeedDB(t, database)

	for _, body := range []string{"body1", "body2", "body3"} {
		if err := ilog.Append(context.Background(), 1, "message", body, nil, nil, actor, database); err != nil {
			t.Fatalf("Append(%q): %v", body, err)
		}
	}

	entries, err := ilog.GetAll(context.Background(), "1", database)
	if err != nil {
		t.Fatalf("GetAll error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Body != "body1" {
		t.Errorf("entries[0].Body: want %q, got %q", "body1", entries[0].Body)
	}
	if entries[1].Body != "body2" {
		t.Errorf("entries[1].Body: want %q, got %q", "body2", entries[1].Body)
	}
	if entries[2].Body != "body3" {
		t.Errorf("entries[2].Body: want %q, got %q", "body3", entries[2].Body)
	}
}

func TestGetAll_EmptySlice(t *testing.T) {
	database := mustOpenDB(t)
	mustSeedDB(t, database)

	entries, err := ilog.GetAll(context.Background(), "1", database)
	if err != nil {
		t.Fatalf("GetAll error: %v", err)
	}
	if entries == nil {
		t.Fatal("GetAll must return non-nil slice, got nil")
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestGetAll_SoftDeleteFilter(t *testing.T) {
	database := mustOpenDB(t)
	actor := mustSeedDB(t, database)

	if err := ilog.Append(context.Background(), 1, "message", "will be deleted", nil, nil, actor, database); err != nil {
		t.Fatalf("Append: %v", err)
	}

	if _, err := database.Exec(`UPDATE ticket_log SET deleted_at = datetime('now') WHERE ticket_id = 1`); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	entries, err := ilog.GetAll(context.Background(), "1", database)
	if err != nil {
		t.Fatalf("GetAll error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after soft delete, got %d", len(entries))
	}
}

func TestLatestPlan_ReturnsMostRecent(t *testing.T) {
	database := mustOpenDB(t)
	actor := mustSeedDB(t, database)

	if err := ilog.Append(context.Background(), 1, "plan", "plan-v1", nil, nil, actor, database); err != nil {
		t.Fatalf("Append plan-v1: %v", err)
	}
	if err := ilog.Append(context.Background(), 1, "plan", "plan-v2", nil, nil, actor, database); err != nil {
		t.Fatalf("Append plan-v2: %v", err)
	}

	result, err := ilog.LatestPlan(context.Background(), "1", database)
	if err != nil {
		t.Fatalf("LatestPlan error: %v", err)
	}
	if result == nil {
		t.Fatal("LatestPlan returned nil, expected entry")
	}
	if result.Body != "plan-v2" {
		t.Errorf("Body: want %q, got %q", "plan-v2", result.Body)
	}
}

func TestLatestPlan_NilWhenAbsent(t *testing.T) {
	database := mustOpenDB(t)
	actor := mustSeedDB(t, database)

	if err := ilog.Append(context.Background(), 1, "message", "just a message", nil, nil, actor, database); err != nil {
		t.Fatalf("Append: %v", err)
	}

	result, err := ilog.LatestPlan(context.Background(), "1", database)
	if err != nil {
		t.Fatalf("LatestPlan error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestAppend_WithTransaction(t *testing.T) {
	database := mustOpenDB(t)
	actor := mustSeedDB(t, database)

	tx, err := database.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	if err := ilog.Append(context.Background(), 1, "message", "rolled back", nil, nil, actor, tx); err != nil {
		tx.Rollback()
		t.Fatalf("Append via tx: %v", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	entries, err := ilog.GetAll(context.Background(), "1", database)
	if err != nil {
		t.Fatalf("GetAll error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after rollback, got %d", len(entries))
	}
}
