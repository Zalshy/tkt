package ticket

import (
	"testing"

	"github.com/zalshy/tkt/internal/models"
)

func TestListActive_Empty(t *testing.T) {
	_, sqlDB := setupDB(t)

	result, err := ListActive(sqlDB)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestListActive_FiltersVerifiedAndCanceled(t *testing.T) {
	_, sqlDB := setupDB(t)
	actor := makeActor()

	todo, err := Create("todo ticket", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create todo: %v", err)
	}
	planning, err := Create("planning ticket", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create planning: %v", err)
	}
	inProgress, err := Create("in_progress ticket", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create in_progress: %v", err)
	}
	done, err := Create("done ticket", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create done: %v", err)
	}
	verified, err := Create("verified ticket", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create verified: %v", err)
	}
	canceled, err := Create("canceled ticket", "", "standard", actor, sqlDB)
	if err != nil {
		t.Fatalf("Create canceled: %v", err)
	}

	setStatus(t, sqlDB, planning.ID, "PLANNING")
	setStatus(t, sqlDB, inProgress.ID, "IN_PROGRESS")
	setStatus(t, sqlDB, done.ID, "DONE")
	setStatus(t, sqlDB, verified.ID, "VERIFIED")
	setStatus(t, sqlDB, canceled.ID, "CANCELED")

	result, err := ListActive(sqlDB)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}

	// VERIFIED and CANCELED must be excluded.
	if _, ok := result[verified.ID]; ok {
		t.Errorf("VERIFIED ticket (id=%d) should not be in active map", verified.ID)
	}
	if _, ok := result[canceled.ID]; ok {
		t.Errorf("CANCELED ticket (id=%d) should not be in active map", canceled.ID)
	}

	// TODO, PLANNING, IN_PROGRESS, DONE must be included.
	if result[todo.ID] != models.StatusTodo {
		t.Errorf("expected TODO status for id=%d, got %q", todo.ID, result[todo.ID])
	}
	if result[planning.ID] != models.StatusPlanning {
		t.Errorf("expected PLANNING status for id=%d, got %q", planning.ID, result[planning.ID])
	}
	if result[inProgress.ID] != models.StatusInProgress {
		t.Errorf("expected IN_PROGRESS status for id=%d, got %q", inProgress.ID, result[inProgress.ID])
	}
	if result[done.ID] != models.StatusDone {
		t.Errorf("expected DONE status for id=%d, got %q", done.ID, result[done.ID])
	}

	if len(result) != 4 {
		t.Errorf("expected 4 entries in active map, got %d", len(result))
	}
}

func TestListDependencyEdges_Empty(t *testing.T) {
	_, sqlDB := setupDB(t)

	edges, err := ListDependencyEdges(sqlDB)
	if err != nil {
		t.Fatalf("ListDependencyEdges: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(edges))
	}
}

func TestListDependencyEdges_Basic(t *testing.T) {
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

	setStatus(t, sqlDB, b.ID, "IN_PROGRESS")

	// B depends on A.
	insertDep(t, sqlDB, b.ID, a.ID)

	edges, err := ListDependencyEdges(sqlDB)
	if err != nil {
		t.Fatalf("ListDependencyEdges: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	e := edges[0]
	if e.TicketID != b.ID {
		t.Errorf("TicketID: want %d, got %d", b.ID, e.TicketID)
	}
	if e.DependsOn != a.ID {
		t.Errorf("DependsOn: want %d, got %d", a.ID, e.DependsOn)
	}
	if e.TicketStat != models.StatusInProgress {
		t.Errorf("TicketStat: want IN_PROGRESS, got %q", e.TicketStat)
	}
	if e.DepStat != models.StatusTodo {
		t.Errorf("DepStat: want TODO, got %q", e.DepStat)
	}
}
