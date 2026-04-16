package context

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
)

const testSessionID = "impl-test-ffff"

func setupDB(t *testing.T) (root string, sqlDB *sql.DB) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir .tkt: %v", err)
	}
	sqlDB, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if _, err := sqlDB.Exec(
		`INSERT INTO sessions (id, role, name) VALUES (?, 'implementer', 'test')`,
		testSessionID,
	); err != nil {
		t.Fatalf("insert test session: %v", err)
	}
	return root, sqlDB
}

func makeActor() *models.Session {
	return &models.Session{ID: testSessionID, Role: models.RoleImplementer}
}

func TestAdd_Roundtrip(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	ctx, err := Add("my title", "my body", actor, sqlDB)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if ctx.ID <= 0 {
		t.Errorf("ID = %d, want > 0", ctx.ID)
	}
	if ctx.Title != "my title" {
		t.Errorf("Title = %q, want 'my title'", ctx.Title)
	}
	if ctx.Body != "my body" {
		t.Errorf("Body = %q, want 'my body'", ctx.Body)
	}

	all, err := ReadAll(sqlDB)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("ReadAll len = %d, want 1", len(all))
	}
	if all[0].ID != ctx.ID {
		t.Errorf("ReadAll[0].ID = %d, want %d", all[0].ID, ctx.ID)
	}
}

func TestAdd_EmptyTitleOrBody(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	if _, err := Add("", "some body", actor, sqlDB); err == nil {
		t.Error("Add with empty title: expected error, got nil")
	}

	all, err := ReadAll(sqlDB)
	if err != nil {
		t.Fatalf("ReadAll after empty title: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 entries after failed Add (empty title), got %d", len(all))
	}

	if _, err := Add("some title", "", actor, sqlDB); err == nil {
		t.Error("Add with empty body: expected error, got nil")
	}

	all, err = ReadAll(sqlDB)
	if err != nil {
		t.Fatalf("ReadAll after empty body: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 entries after failed Add (empty body), got %d", len(all))
	}
}

func TestReadAll_InsertionOrder(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	titles := []string{"alpha", "beta", "gamma"}
	var ids []int
	for _, title := range titles {
		ctx, err := Add(title, "body", actor, sqlDB)
		if err != nil {
			t.Fatalf("Add %q: %v", title, err)
		}
		ids = append(ids, ctx.ID)
	}

	all, err := ReadAll(sqlDB)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ReadAll len = %d, want 3", len(all))
	}
	for i, want := range ids {
		if all[i].ID != want {
			t.Errorf("all[%d].ID = %d, want %d (ascending order)", i, all[i].ID, want)
		}
	}
}

func TestReadAll_EmptySlice(t *testing.T) {
	_, sqlDB := setupDB(t)

	all, err := ReadAll(sqlDB)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if all == nil {
		t.Error("ReadAll returned nil, want non-nil empty slice")
	}
	if len(all) != 0 {
		t.Errorf("ReadAll len = %d, want 0", len(all))
	}
}

func TestRead_NotFound(t *testing.T) {
	_, sqlDB := setupDB(t)

	_, err := Read(9999, sqlDB)
	if err == nil {
		t.Fatal("Read non-existent ID: expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected errors.Is(err, ErrNotFound), got: %v", err)
	}
}

func TestDelete_SoftDelete(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	ctx, err := Add("to delete", "body", actor, sqlDB)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := Delete(ctx.ID, sqlDB); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	all, err := ReadAll(sqlDB)
	if err != nil {
		t.Fatalf("ReadAll after delete: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("ReadAll after delete len = %d, want 0", len(all))
	}

	// Confirm the row still exists with deleted_at set.
	var deletedAt sql.NullTime
	row := sqlDB.QueryRow(
		`SELECT deleted_at FROM project_context WHERE id = ?`, ctx.ID,
	)
	if err := row.Scan(&deletedAt); err != nil {
		t.Fatalf("direct query for deleted row: %v", err)
	}
	if !deletedAt.Valid {
		t.Error("deleted_at is NULL, expected a non-null timestamp after soft-delete")
	}
}

func TestUpdate_Roundtrip(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	ctx, err := Add("original title", "original body", actor, sqlDB)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := Update(ctx.ID, "new title", "new body", actor, sqlDB); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := Read(ctx.ID, sqlDB)
	if err != nil {
		t.Fatalf("Read after update: %v", err)
	}
	if updated.Title != "new title" {
		t.Errorf("Title = %q, want 'new title'", updated.Title)
	}
	if updated.Body != "new body" {
		t.Errorf("Body = %q, want 'new body'", updated.Body)
	}
}
