package usage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalshy/tkt/internal/db"
)

// openTestDB creates a temp directory, opens the DB (running all migrations),
// and registers cleanup. Returns the open *sql.DB.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// insertTicket inserts a minimal ticket and returns its ID.
func insertTicket(t *testing.T, database *sql.DB) int64 {
	t.Helper()
	res, err := database.Exec(
		`INSERT INTO tickets (title, created_by) VALUES ('test ticket', 'tester')`,
	)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

func TestUsageAppend_InsertAndRead(t *testing.T) {
	database := openTestDB(t)
	ticketID := insertTicket(t, database)

	err := Append(context.Background(), ticketID, "sess-abc", 1234, 5, 60000, "implementer", "phase1", database)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, err := GetForTicket(context.Background(), ticketID, database)
	if err != nil {
		t.Fatalf("GetForTicket: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	u := entries[0]
	if u.TicketID != ticketID {
		t.Errorf("TicketID = %d, want %d", u.TicketID, ticketID)
	}
	if u.SessionID != "sess-abc" {
		t.Errorf("SessionID = %q, want %q", u.SessionID, "sess-abc")
	}
	if u.Tokens != 1234 {
		t.Errorf("Tokens = %d, want 1234", u.Tokens)
	}
	if u.Tools != 5 {
		t.Errorf("Tools = %d, want 5", u.Tools)
	}
	if u.DurationMs != 60000 {
		t.Errorf("DurationMs = %d, want 60000", u.DurationMs)
	}
	if u.Agent != "implementer" {
		t.Errorf("Agent = %q, want %q", u.Agent, "implementer")
	}
	if u.Label != "phase1" {
		t.Errorf("Label = %q, want %q", u.Label, "phase1")
	}
	if u.DeletedAt != nil {
		t.Errorf("DeletedAt = %v, want nil", u.DeletedAt)
	}
	if u.ID == 0 {
		t.Error("ID should be non-zero after insert")
	}
}

func TestUsageAppend_TokensZeroRejected(t *testing.T) {
	database := openTestDB(t)
	ticketID := insertTicket(t, database)

	err := Append(context.Background(), ticketID, "sess-abc", 0, 0, 0, "", "", database)
	if err == nil {
		t.Fatal("expected error for tokens=0, got nil")
	}
}

func TestUsageGetForTicket_EmptySlice(t *testing.T) {
	database := openTestDB(t)
	ticketID := insertTicket(t, database)

	entries, err := GetForTicket(context.Background(), ticketID, database)
	if err != nil {
		t.Fatalf("GetForTicket: %v", err)
	}
	if entries == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
