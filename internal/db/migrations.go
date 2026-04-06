package db

import (
	"database/sql"
	"fmt"
)

// migrations is an ordered slice of SQL migration batches.
// Each entry is a slice of statements that form a single atomic migration.
// migrations[0] = V1, migrations[1] = V2, etc.
//
// V1 is frozen inline — the constants it originally referenced have been removed
// from schema.go. Migration history must never depend on mutable constants.
var migrations = [][]string{
	// V1 — full initial schema: 5 tables + 7 indexes.
	{
		`
CREATE TABLE tickets (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    plan        TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'TODO',
    created_by  TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at  DATETIME NULL
)`,
		`
CREATE TABLE ticket_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    from_state  TEXT NOT NULL,
    to_state    TEXT NOT NULL,
    session_id  TEXT NOT NULL,
    note        TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at  DATETIME NULL
)`,
		`
CREATE TABLE ticket_comments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    session_id  TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at  DATETIME NULL
)`,
		`
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    role        TEXT NOT NULL,
    name        TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    last_active DATETIME NOT NULL DEFAULT (datetime('now')),
    expired_at  DATETIME NULL
)`,
		`
CREATE TABLE project_context (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at DATETIME NULL
)`,
		`
CREATE INDEX idx_ticket_events_ticket_id ON ticket_events(ticket_id)`,
		`
CREATE INDEX idx_ticket_comments_ticket_id ON ticket_comments(ticket_id)`,
		`
CREATE INDEX idx_tickets_status ON tickets(status)`,
		`
CREATE INDEX idx_tickets_deleted_at ON tickets(deleted_at)`,
		`
CREATE INDEX idx_ticket_events_deleted_at ON ticket_events(deleted_at)`,
		`
CREATE INDEX idx_ticket_comments_deleted_at ON ticket_comments(deleted_at)`,
		`
CREATE INDEX idx_project_context_deleted_at ON project_context(deleted_at)`,
	},
	// V2 — replace ticket_events + ticket_comments with unified ticket_log.
	// DROP COLUMN requires SQLite >= 3.35 (modernc.org/sqlite >= 1.18, satisfied by go.mod).
	{
		`ALTER TABLE tickets DROP COLUMN plan`,
		`DROP INDEX IF EXISTS idx_ticket_events_ticket_id`,
		`DROP INDEX IF EXISTS idx_ticket_events_deleted_at`,
		`DROP INDEX IF EXISTS idx_ticket_comments_ticket_id`,
		`DROP INDEX IF EXISTS idx_ticket_comments_deleted_at`,
		`DROP TABLE IF EXISTS ticket_events`,
		`DROP TABLE IF EXISTS ticket_comments`,
		createTableTicketLog,
		createIndexTicketLogTicketID,
		createIndexTicketLogKind,
		createIndexTicketLogDeletedAt,
	},
	// V3 — add ticket_dependencies table for dependency graph feature
	{
		createTableTicketDependencies,
	},
	// V4 — add roles table and seed the two built-in roles.
	{
		createTableRoles,
		`INSERT INTO roles (name, base_role, is_builtin) VALUES ('architect',   'architect',   1)`,
		`INSERT INTO roles (name, base_role, is_builtin) VALUES ('implementer', 'implementer', 1)`,
	},
}

// migrate ensures the schema_version table exists, then applies any
// migrations whose version number exceeds the current stored version.
// It is only called from Open and is not exported.
func migrate(db *sql.DB) error {
	// Bootstrap: create the schema_version table if it does not exist yet.
	// This is the only IF NOT EXISTS usage — it must tolerate running on both
	// a brand-new file and an already-migrated file.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("migrate: create schema_version: %w", err)
	}

	// Read the current version. No rows means version 0.
	var current int
	err := db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&current)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("migrate: read schema_version: %w", err)
	}
	// err == sql.ErrNoRows → current stays 0 (zero value)

	// Apply each migration whose 1-based index exceeds the current version.
	for i, stmts := range migrations {
		targetVersion := i + 1
		if targetVersion <= current {
			continue // already applied
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("migrate: begin tx for V%d: %w", targetVersion, err)
		}

		for _, stmt := range stmts {
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migrate: V%d statement failed: %w", targetVersion, err)
			}
		}

		// Keep exactly one row in schema_version using delete-then-insert.
		if _, err := tx.Exec(`DELETE FROM schema_version`); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: clear schema_version for V%d: %w", targetVersion, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_version (version) VALUES (?)`, targetVersion); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: write schema_version for V%d: %w", targetVersion, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate: commit V%d: %w", targetVersion, err)
		}

		current = targetVersion
	}

	return nil
}
