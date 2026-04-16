package db

import (
	"os"
	"path/filepath"
	"testing"
)


func TestOpen_NewFile(t *testing.T) {
	dir := t.TempDir()
	tktDir := filepath.Join(dir, ".tkt")
	if err := os.MkdirAll(tktDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .tkt: %v", err)
	}

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(%q) unexpected error: %v", dir, err)
	}
	defer db.Close()

	// Verify all 4 business tables exist (schema_version is a 5th meta-table).
	expectedTables := []string{
		"tickets",
		"ticket_log",
		"sessions",
		"project_context",
	}
	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found in sqlite_master: %v", table, err)
		}
	}

	// Verify old tables do NOT exist.
	removedTables := []string{
		"ticket_events",
		"ticket_comments",
		"ticket_log_new",
	}
	for _, table := range removedTables {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err == nil {
			t.Errorf("table %q should not exist but was found in sqlite_master", table)
		}
	}

	// Verify _new indexes do NOT exist (dropped by V10 before rename).
	removedIndexes := []string{
		"idx_ticket_log_new_ticket_id",
		"idx_ticket_log_new_kind",
		"idx_ticket_log_new_deleted_at",
		"idx_ticket_log_new_ticket_id_kind",
	}
	for _, idx := range removedIndexes {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&name)
		if err == nil {
			t.Errorf("index %q should not exist after V10 but was found in sqlite_master", idx)
		}
	}

	// Verify all named indexes exist.
	expectedIndexes := []string{
		"idx_tickets_status",
		"idx_tickets_deleted_at",
		"idx_ticket_log_ticket_id",
		"idx_ticket_log_kind",
		"idx_ticket_log_deleted_at",
		"idx_project_context_deleted_at",
	}
	for _, idx := range expectedIndexes {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found in sqlite_master: %v", idx, err)
		}
	}

	// Verify tickets table has no 'plan' column.
	rows, err := db.Query(`PRAGMA table_info(tickets)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(tickets): %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan table_info row: %v", err)
		}
		if name == "plan" {
			t.Errorf("tickets table still has 'plan' column — should have been dropped in V2")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows error: %v", err)
	}
}

func TestOpen_Idempotent(t *testing.T) {
	dir := t.TempDir()
	tktDir := filepath.Join(dir, ".tkt")
	if err := os.MkdirAll(tktDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .tkt: %v", err)
	}

	db1, err := Open(dir)
	if err != nil {
		t.Fatalf("first Open() error: %v", err)
	}

	// Count sqlite_master rows after first open.
	var count1 int
	if err := db1.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&count1); err != nil {
		t.Fatalf("count sqlite_master after first open: %v", err)
	}
	db1.Close()

	db2, err := Open(dir)
	if err != nil {
		t.Fatalf("second Open() error: %v", err)
	}
	defer db2.Close()

	// Count must be identical — no duplicates.
	var count2 int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&count2); err != nil {
		t.Fatalf("count sqlite_master after second open: %v", err)
	}

	if count1 != count2 {
		t.Errorf("sqlite_master row count changed: first=%d, second=%d", count1, count2)
	}
}

func TestOpen_WALMode(t *testing.T) {
	dir := t.TempDir()
	tktDir := filepath.Join(dir, ".tkt")
	if err := os.MkdirAll(tktDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .tkt: %v", err)
	}

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestOpen_ForeignKeysEnabled(t *testing.T) {
	dir := t.TempDir()
	tktDir := filepath.Join(dir, ".tkt")
	if err := os.MkdirAll(tktDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .tkt: %v", err)
	}

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	var fk int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestOpen_SchemaVersion(t *testing.T) {
	dir := t.TempDir()
	tktDir := filepath.Join(dir, ".tkt")
	if err := os.MkdirAll(tktDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .tkt: %v", err)
	}

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("SELECT schema_version: %v", err)
	}
	if version != 13 {
		t.Errorf("schema_version = %d, want 13", version)
	}

	// Ensure exactly one row in schema_version.
	var rowCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&rowCount); err != nil {
		t.Fatalf("COUNT schema_version: %v", err)
	}
	if rowCount != 1 {
		t.Errorf("schema_version has %d rows, want exactly 1", rowCount)
	}
}
