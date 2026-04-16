package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/zalshy/tkt/internal/project"
	sqlite3 "modernc.org/sqlite"        // registers the "sqlite" driver and provides typed errors
	sqlite3lib "modernc.org/sqlite/lib" // for SQLITE_LOCKED / SQLITE_BUSY constants
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

	// Enforce 0600 on the DB file. Non-fatal: some filesystems (FAT32, certain
	// network mounts) do not support permission bits; warn but do not abort.
	if err := os.Chmod(path, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not set db permissions: %v\n", err)
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

// wrapSQLiteError detects SQLite locked/busy errors via type assertion and returns a
// human-readable message. Uses modernc.org/sqlite's typed Error so the detection
// is robust to error message changes in future driver versions.
func wrapSQLiteError(err error) error {
	if err == nil {
		return nil
	}
	var sqliteErr *sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.Code() == sqlite3lib.SQLITE_LOCKED || sqliteErr.Code() == sqlite3lib.SQLITE_BUSY {
			return fmt.Errorf("database is locked by another process — wait and retry, or check for a hung tkt process: %w", err)
		}
	}
	return err
}
