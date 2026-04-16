package ticket

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
)

const testSessionID = "impl-test-ffff"

// setupDB opens a fresh DB in a temp dir and inserts a minimal session row.
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

func insertDep(t *testing.T, db *sql.DB, ticketID, dependsOn int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO ticket_dependencies (ticket_id, depends_on) VALUES (?, ?)`,
		ticketID, dependsOn)
	if err != nil {
		t.Fatalf("insertDep(%d->%d): %v", ticketID, dependsOn, err)
	}
}

func setStatus(t *testing.T, db *sql.DB, ticketID int64, status string) {
	t.Helper()
	_, err := db.Exec(`UPDATE tickets SET status = ? WHERE id = ?`, status, ticketID)
	if err != nil {
		t.Fatalf("setStatus(%d, %q): %v", ticketID, status, err)
	}
}

func TestCreate_Roundtrip(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	tk, err := Create("My title", "My description", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tk.ID <= 0 {
		t.Errorf("ID = %d, want > 0", tk.ID)
	}
	if tk.Status != models.StatusTodo {
		t.Errorf("Status = %q, want TODO", tk.Status)
	}
	if tk.Title != "My title" {
		t.Errorf("Title = %q, want 'My title'", tk.Title)
	}
	if tk.Description != "My description" {
		t.Errorf("Description = %q, want 'My description'", tk.Description)
	}
	if tk.CreatedBy != testSessionID {
		t.Errorf("CreatedBy = %q, want %q", tk.CreatedBy, testSessionID)
	}
	if tk.DeletedAt != nil {
		t.Errorf("DeletedAt should be nil, got %v", tk.DeletedAt)
	}
}

func TestGetByID_BothFormats(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	created, err := Create("Test", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	idStr := fmt.Sprintf("%d", created.ID)

	byPlain, err := GetByID(idStr, sqlDB)
	if err != nil {
		t.Fatalf("GetByID(%q): %v", idStr, err)
	}
	byHash, err := GetByID("#"+idStr, sqlDB)
	if err != nil {
		t.Fatalf("GetByID(%q): %v", "#"+idStr, err)
	}
	if byPlain.ID != byHash.ID {
		t.Errorf("plain and hash formats returned different IDs: %d vs %d", byPlain.ID, byHash.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	_, sqlDB := setupDB(t)

	_, err := GetByID("999", sqlDB)
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected errors.Is(err, ErrNotFound), got: %v", err)
	}
}

func TestList_HasMoreFlag(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	for i := 0; i < 11; i++ {
		if _, err := Create("ticket", "", "standard", actor, sqlDB); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	result, err := List(ListOptions{}, sqlDB)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Tickets) != 10 {
		t.Errorf("want 10 tickets, got %d", len(result.Tickets))
	}
	if !result.HasMore {
		t.Error("want HasMore=true, got false")
	}
}

func TestList_SoftDeleteFilter(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	tk1, err := Create("live ticket", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create tk1: %v", err)
	}
	tk2, err := Create("deleted ticket", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create tk2: %v", err)
	}

	if _, err := sqlDB.Exec(
		`UPDATE tickets SET deleted_at = datetime('now') WHERE id = ?`, tk2.ID,
	); err != nil {
		t.Fatalf("soft-delete tk2: %v", err)
	}

	result, err := List(ListOptions{}, sqlDB)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Tickets) != 1 {
		t.Fatalf("want 1 ticket, got %d", len(result.Tickets))
	}
	if result.Tickets[0].ID != tk1.ID {
		t.Errorf("want ticket ID %d, got %d", tk1.ID, result.Tickets[0].ID)
	}
}

func TestGetDependencies_MultiLevel(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	// Create A, B, C where C depends on B and B depends on A.
	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := Create("B", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}
	c, err := Create("C", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create C: %v", err)
	}

	insertDep(t, sqlDB, b.ID, a.ID) // B depends on A
	insertDep(t, sqlDB, c.ID, b.ID) // C depends on B

	deps, err := GetDependencies(c.ID, sqlDB)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}

	ids := make(map[int64]bool)
	for _, d := range deps {
		ids[d.ID] = true
	}

	if !ids[a.ID] {
		t.Errorf("expected A (id=%d) in dependencies", a.ID)
	}
	if !ids[b.ID] {
		t.Errorf("expected B (id=%d) in dependencies", b.ID)
	}
	if ids[c.ID] {
		t.Errorf("did not expect C (id=%d) in its own dependencies", c.ID)
	}
}

func TestGetDependents_MultiLevel(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	// Create A, B, C where C depends on B and B depends on A.
	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := Create("B", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}
	c, err := Create("C", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create C: %v", err)
	}

	insertDep(t, sqlDB, b.ID, a.ID) // B depends on A
	insertDep(t, sqlDB, c.ID, b.ID) // C depends on B

	dependents, err := GetDependents(a.ID, sqlDB)
	if err != nil {
		t.Fatalf("GetDependents: %v", err)
	}

	ids := make(map[int64]bool)
	for _, d := range dependents {
		ids[d.ID] = true
	}

	if !ids[b.ID] {
		t.Errorf("expected B (id=%d) in dependents", b.ID)
	}
	if !ids[c.ID] {
		t.Errorf("expected C (id=%d) in dependents", c.ID)
	}
	if ids[a.ID] {
		t.Errorf("did not expect A (id=%d) in its own dependents", a.ID)
	}
}

func TestIsReady_BlockedAndUnblocked(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	x, err := Create("X", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create X: %v", err)
	}
	y, err := Create("Y", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create Y: %v", err)
	}

	insertDep(t, sqlDB, y.ID, x.ID) // Y depends on X

	ready, err := IsReady(y.ID, sqlDB)
	if err != nil {
		t.Fatalf("IsReady: %v", err)
	}
	if ready {
		t.Error("expected IsReady=false when X is TODO")
	}

	setStatus(t, sqlDB, x.ID, "VERIFIED")

	ready, err = IsReady(y.ID, sqlDB)
	if err != nil {
		t.Fatalf("IsReady after VERIFIED: %v", err)
	}
	if !ready {
		t.Error("expected IsReady=true after X is VERIFIED")
	}
}

func TestList_Ready(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	// P has no deps, Q depends on P (TODO), R has no deps.
	p, err := Create("P", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create P: %v", err)
	}
	q, err := Create("Q", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create Q: %v", err)
	}
	r, err := Create("R", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create R: %v", err)
	}

	insertDep(t, sqlDB, q.ID, p.ID) // Q depends on P

	result, err := List(ListOptions{Ready: true}, sqlDB)
	if err != nil {
		t.Fatalf("List Ready: %v", err)
	}

	ids := make(map[int64]bool)
	for _, tk := range result.Tickets {
		ids[tk.ID] = true
	}

	if !ids[p.ID] {
		t.Errorf("expected P (id=%d) in ready list", p.ID)
	}
	if !ids[r.ID] {
		t.Errorf("expected R (id=%d) in ready list", r.ID)
	}
	if ids[q.ID] {
		t.Errorf("did not expect Q (id=%d) in ready list (blocked)", q.ID)
	}

	// Verify P and set status: now Q should be ready, P excluded (VERIFIED filtered).
	setStatus(t, sqlDB, p.ID, "VERIFIED")

	result2, err := List(ListOptions{Ready: true}, sqlDB)
	if err != nil {
		t.Fatalf("List Ready after VERIFIED: %v", err)
	}

	ids2 := make(map[int64]bool)
	for _, tk := range result2.Tickets {
		ids2[tk.ID] = true
	}

	if !ids2[q.ID] {
		t.Errorf("expected Q (id=%d) in ready list after P is VERIFIED", q.ID)
	}
	if ids2[p.ID] {
		t.Errorf("did not expect P (id=%d) in ready list (VERIFIED filtered)", p.ID)
	}
}

func TestGetDependencies_SoftDeletedExcluded(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := Create("B", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}

	insertDep(t, sqlDB, b.ID, a.ID) // B depends on A

	// Soft-delete A.
	if _, err := sqlDB.Exec(
		`UPDATE tickets SET deleted_at = datetime('now') WHERE id = ?`, a.ID,
	); err != nil {
		t.Fatalf("soft-delete A: %v", err)
	}

	deps, err := GetDependencies(b.ID, sqlDB)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected empty dependencies when A is soft-deleted, got %d entries", len(deps))
	}
}

func TestIsReady_NoDependencies(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	tk, err := Create("no deps", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ready, err := IsReady(tk.ID, sqlDB)
	if err != nil {
		t.Fatalf("IsReady: %v", err)
	}
	if !ready {
		t.Error("expected IsReady=true for ticket with no dependencies")
	}
}

func TestAddDependencies_Single(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := Create("B", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}

	if err := AddDependencies(b.ID, []int64{a.ID}, sqlDB); err != nil {
		t.Fatalf("AddDependencies: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM ticket_dependencies WHERE ticket_id = ? AND depends_on = ?`,
		b.ID, a.ID,
	).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row in ticket_dependencies, got %d", count)
	}
}

func TestAddDependencies_Multiple(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := Create("B", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}
	c, err := Create("C", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create C: %v", err)
	}

	if err := AddDependencies(c.ID, []int64{a.ID, b.ID}, sqlDB); err != nil {
		t.Fatalf("AddDependencies: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM ticket_dependencies WHERE ticket_id = ?`, c.ID,
	).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows in ticket_dependencies, got %d", count)
	}
}

func TestAddDependencies_Idempotent(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := Create("B", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}

	if err := AddDependencies(b.ID, []int64{a.ID}, sqlDB); err != nil {
		t.Fatalf("first AddDependencies: %v", err)
	}
	if err := AddDependencies(b.ID, []int64{a.ID}, sqlDB); err != nil {
		t.Fatalf("second AddDependencies (idempotent): %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM ticket_dependencies WHERE ticket_id = ? AND depends_on = ?`,
		b.ID, a.ID,
	).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after idempotent insert, got %d", count)
	}
}

func TestAddDependencies_SelfDep(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}

	err = AddDependencies(a.ID, []int64{a.ID}, sqlDB)
	if err == nil {
		t.Fatal("expected error for self-dependency, got nil")
	}
	if !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Errorf("expected 'cannot depend on itself' in error, got: %v", err)
	}
}

func TestAddDependencies_CycleDetected(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := Create("B", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}

	// B depends on A
	if err := AddDependencies(b.ID, []int64{a.ID}, sqlDB); err != nil {
		t.Fatalf("AddDependencies B->A: %v", err)
	}

	// Now try to make A depend on B — would create a cycle
	err = AddDependencies(a.ID, []int64{b.ID}, sqlDB)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle detected") {
		t.Errorf("expected 'cycle detected' in error, got: %v", err)
	}
}

func TestAddDependencies_NotFound(t *testing.T) {
	_, sqlDB := setupDB(t)

	err := AddDependencies(99999, []int64{1}, sqlDB)
	if err == nil {
		t.Fatal("expected error for non-existent dep ticket, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestRemoveDependency_HappyPath(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := Create("B", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}

	insertDep(t, sqlDB, b.ID, a.ID) // B depends on A

	if err := RemoveDependency(b.ID, a.ID, sqlDB); err != nil {
		t.Fatalf("RemoveDependency: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM ticket_dependencies WHERE ticket_id = ? AND depends_on = ?`,
		b.ID, a.ID,
	).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after RemoveDependency, got %d", count)
	}
}

func TestRemoveDependency_EdgeNotFound(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	a, err := Create("A", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := Create("B", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}

	// No dependency row inserted — remove should return ErrNotFound.
	err = RemoveDependency(b.ID, a.ID, sqlDB)
	if err == nil {
		t.Fatal("expected ErrNotFound for non-existent edge, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}
