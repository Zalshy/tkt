package db

import (
	"database/sql"
	"fmt"
	"os"
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
	// V5 — add tier column to tickets
	{
		`ALTER TABLE tickets ADD COLUMN tier TEXT NOT NULL DEFAULT 'standard'`,
	},
	// V6 — seed monitor built-in role (system-only, not architect or implementer).
	// The roles table has CHECK (base_role IN ('architect', 'implementer')) which
	// must be widened to allow 'monitor'. SQLite cannot drop/alter CHECK constraints
	// in-place, so we recreate the table via the standard rename-copy-drop sequence.
	{
		`CREATE TABLE roles_new (
			name       TEXT PRIMARY KEY,
			base_role  TEXT NOT NULL,
			is_builtin INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			CHECK (base_role IN ('architect', 'implementer', 'monitor'))
		)`,
		`INSERT INTO roles_new SELECT name, base_role, is_builtin, created_at FROM roles`,
		`DROP TABLE roles`,
		`ALTER TABLE roles_new RENAME TO roles`,
		`INSERT INTO roles (name, base_role, is_builtin) VALUES ('monitor', 'monitor', 1)`,
	},
	// V7 — add missing indexes for common query patterns.
	{
		createIndexTicketsStatusDeletedAt,
		createIndexTicketLogTicketIDKind,
		createIndexTicketDependsDependsOn,
		createIndexSessionsExpiredAt,
	},
	// V8 — introduce ticket_usage table and backfill from ticket_log.
	// Verification in verifyV8Backfill() asserts COUNT parity before commit.
	{
		createTableTicketUsage,
		createIndexTicketUsageTicketIDDeletedAt,
		`INSERT INTO ticket_usage (ticket_id, session_id, tokens, tools, duration_ms, agent, label, created_at)
SELECT
    ticket_id,
    session_id,
    COALESCE(CAST(json_extract(body, '$.tokens')      AS INTEGER), 0),
    COALESCE(CAST(json_extract(body, '$.tools')       AS INTEGER), 0),
    COALESCE(CAST(json_extract(body, '$.duration_ms') AS INTEGER), 0),
    COALESCE(json_extract(body, '$.agent'), ''),
    COALESCE(json_extract(body, '$.label'), ''),
    created_at
FROM ticket_log
WHERE kind = 'usage'
  AND deleted_at IS NULL`,
	},
	// V9 — delete usage rows from ticket_log now that backfill into ticket_usage
	// (V8) has been verified. The data lives in ticket_usage; ticket_log no
	// longer needs these rows.
	{
		`DELETE FROM ticket_log WHERE kind = 'usage'`,
	},
	// V10 — rebuild ticket_log as ticket_log_new with CHECK(kind) and FK(session_id).
	// No DROP or RENAME in this ticket — that is V11 (#153).
	// Verification in verifyV10Backfill asserts COUNT(*) parity before commit.
	{
		createTableTicketLogNew,
		createIndexTicketLogNewTicketID,
		createIndexTicketLogNewKind,
		createIndexTicketLogNewDeletedAt,
		createIndexTicketLogNewTicketIDKind,
		`INSERT INTO ticket_log_new
         (id, ticket_id, session_id, kind, body, from_state, to_state, created_at, deleted_at)
     SELECT
          id, ticket_id, session_id, kind, body, from_state, to_state, created_at, deleted_at
     FROM ticket_log`,
	},
	// V11 — drop old ticket_log, rename ticket_log_new to ticket_log, rebuild canonical indexes.
	// Row-count parity was already asserted by verifyV10Backfill in V10.
	// Order is critical: drop _new indexes before DROP TABLE ticket_log (different table),
	// then drop canonical indexes on ticket_log, then DROP TABLE ticket_log, then RENAME,
	// then recreate canonical indexes on the renamed table.
	{
		`DROP INDEX idx_ticket_log_new_ticket_id`,
		`DROP INDEX idx_ticket_log_new_kind`,
		`DROP INDEX idx_ticket_log_new_deleted_at`,
		`DROP INDEX idx_ticket_log_new_ticket_id_kind`,
		`DROP INDEX idx_ticket_log_ticket_id`,
		`DROP INDEX idx_ticket_log_kind`,
		`DROP INDEX idx_ticket_log_deleted_at`,
		`DROP INDEX idx_ticket_log_ticket_id_kind`,
		`DROP TABLE ticket_log`,
		`ALTER TABLE ticket_log_new RENAME TO ticket_log`,
		createIndexTicketLogTicketID,
		createIndexTicketLogKind,
		createIndexTicketLogDeletedAt,
		createIndexTicketLogTicketIDKind,
	},
	// V12 — rebuild ticket_dependencies as ticket_dependencies_new with DATETIME created_at.
	// No DROP or RENAME in this ticket — that is V13 (#155).
	// Verification in verifyV12Backfill asserts COUNT(*) parity before commit.
	{
		createTableTicketDependenciesNew,
		createIndexTicketDependenciesNewDependsOn,
		`INSERT INTO ticket_dependencies_new
         (ticket_id, depends_on, created_at)
     SELECT
          ticket_id, depends_on, created_at
     FROM ticket_dependencies`,
	},
	// V13 — drop old ticket_dependencies, rename ticket_dependencies_new to ticket_dependencies,
	// rebuild canonical index. Row-count parity was already asserted by verifyV12Backfill in V12.
	{
		`DROP INDEX idx_ticket_dependencies_new_depends_on`,
		`DROP INDEX idx_ticket_dependencies_depends_on`,
		`DROP TABLE ticket_dependencies`,
		`ALTER TABLE ticket_dependencies_new RENAME TO ticket_dependencies`,
		createIndexTicketDependsDependsOn,
	},
	// V14 — add main_type and attention_level columns to tickets.
	{
		`ALTER TABLE tickets ADD COLUMN main_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tickets ADD COLUMN attention_level INTEGER NOT NULL DEFAULT 0`,
	},
}

// verifyV8Backfill asserts that the number of rows inserted into ticket_usage
// matches the number of source rows in ticket_log with kind='usage'.
// It is package-level (not a closure) so that migration tests can call it directly.
func verifyV8Backfill(tx *sql.Tx) error {
	var sourceCount, backfilledCount int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM ticket_log WHERE kind='usage' AND deleted_at IS NULL`,
	).Scan(&sourceCount); err != nil {
		return fmt.Errorf("migrate: V8 verify: source count: %w", err)
	}
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM ticket_usage`,
	).Scan(&backfilledCount); err != nil {
		return fmt.Errorf("migrate: V8 verify: backfill count: %w", err)
	}
	if backfilledCount != sourceCount {
		return fmt.Errorf(
			"migrate: V8 backfill count mismatch: source=%d backfilled=%d",
			sourceCount, backfilledCount,
		)
	}
	return nil
}

// verifyV10Backfill asserts that the number of rows copied into ticket_log_new
// matches the total row count in ticket_log (all rows, including deleted ones).
// It is package-level (not a closure) so that migration tests can call it directly.
func verifyV10Backfill(tx *sql.Tx) error {
	var sourceCount, newCount int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM ticket_log`,
	).Scan(&sourceCount); err != nil {
		return fmt.Errorf("migrate: V10 verify: source count: %w", err)
	}
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM ticket_log_new`,
	).Scan(&newCount); err != nil {
		return fmt.Errorf("migrate: V10 verify: new count: %w", err)
	}
	if newCount != sourceCount {
		return fmt.Errorf(
			"migrate: V10 backfill count mismatch: source=%d new=%d",
			sourceCount, newCount,
		)
	}
	return nil
}

// verifyV12Backfill asserts that the number of rows copied into ticket_dependencies_new
// matches the total row count in ticket_dependencies (all rows).
// It is package-level (not a closure) so that migration tests can call it directly.
func verifyV12Backfill(tx *sql.Tx) error {
	var sourceCount, newCount int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM ticket_dependencies`,
	).Scan(&sourceCount); err != nil {
		return fmt.Errorf("migrate: V12 verify: source count: %w", err)
	}
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM ticket_dependencies_new`,
	).Scan(&newCount); err != nil {
		return fmt.Errorf("migrate: V12 verify: new count: %w", err)
	}
	if newCount != sourceCount {
		return fmt.Errorf(
			"migrate: V12 backfill count mismatch: source=%d new=%d",
			sourceCount, newCount,
		)
	}
	return nil
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

		if targetVersion == 8 {
			if err := verifyV8Backfill(tx); err != nil {
				_ = tx.Rollback()
				return err
			}
			// After verifyV8Backfill passes, check for corrupt rows.
			var zeroTokens int
			_ = tx.QueryRow(`SELECT COUNT(*) FROM ticket_usage WHERE tokens = 0`).Scan(&zeroTokens)
			if zeroTokens > 0 {
				fmt.Fprintf(os.Stderr, "tkt: warning: V8 backfill: %d ticket_usage rows have tokens=0 (malformed source data)\n", zeroTokens)
			}
		}

		if targetVersion == 10 {
			if err := verifyV10Backfill(tx); err != nil {
				_ = tx.Rollback()
				return err
			}
		}

		if targetVersion == 12 {
			if err := verifyV12Backfill(tx); err != nil {
				_ = tx.Rollback()
				return err
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
