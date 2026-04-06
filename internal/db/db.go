package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/zalshy/tkt/internal/project"
	_ "modernc.org/sqlite" // registers the "sqlite" driver via side-effect import
)

// Open opens the SQLite database at the path computed from projectRoot, enables WAL
// mode, and runs all pending schema migrations. Returns the ready-to-use *sql.DB.
//
// On any failure after the connection is opened, Open closes the connection before
// returning the error to avoid resource leaks.
func Open(projectRoot string) (db *sql.DB, openErr error) {
	path := project.DBPath(projectRoot)

	// sql.Open is lazy — it does not connect yet.
	// The "sqlite" driver name comes from modernc.org/sqlite's side-effect registration.
	db, openErr = sql.Open("sqlite", path)
	if openErr != nil {
		return nil, fmt.Errorf("db.Open: sql.Open: %w", openErr)
	}

	// Ensure the connection is closed if anything below fails.
	defer func() {
		if openErr != nil {
			_ = db.Close()
			db = nil
		}
	}()

	// Ping forces an actual connection and surfaces file-permission errors early.
	if openErr = db.Ping(); openErr != nil {
		openErr = fmt.Errorf("db.Open: ping: %w", wrapSQLiteError(openErr))
		return
	}

	// Enable WAL mode for concurrent reads during TUI polling.
	// PRAGMA journal_mode returns the resulting mode as a result row; Exec discards it.
	if _, openErr = db.Exec("PRAGMA journal_mode=WAL"); openErr != nil {
		openErr = fmt.Errorf("db.Open: enable WAL: %w", openErr)
		return
	}

	// Enable foreign key enforcement. SQLite disables it by default; this must be
	// set per connection before any DML so REFERENCES constraints are active.
	if _, openErr = db.Exec("PRAGMA foreign_keys = ON"); openErr != nil {
		openErr = fmt.Errorf("db.Open: enable foreign keys: %w", openErr)
		return
	}

	// Run all pending migrations.
	if openErr = migrate(db); openErr != nil {
		openErr = fmt.Errorf("db.Open: migrate: %w", openErr)
		return
	}

	return db, nil
}

// wrapSQLiteError detects SQLite "database is locked" errors and returns a
// human-readable message. String matching is used because modernc.org/sqlite
// does not export typed errors for lock conditions. If the error format changes
// in a future driver version, this detection will silently stop working.
func wrapSQLiteError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "database is locked") {
		return fmt.Errorf("database is locked by another process — wait and retry, or check for a hung tkt process: %w", err)
	}
	return err
}
