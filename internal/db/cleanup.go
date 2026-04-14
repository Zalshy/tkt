package db

import (
	"database/sql"
	"fmt"
)

func CleanupStaleSessions(db *sql.DB, dryRun bool) (int64, error) {
	// 4-hour threshold: aggressive enough to catch agents that exited without
	// tkt session --end, but note this also affects `tkt cleanup` invocations.
	if dryRun {
		var count int64
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sessions
             WHERE last_active < datetime('now', '-4 hours') AND expired_at IS NULL`,
		).Scan(&count)
		if err != nil {
			return 0, fmt.Errorf("CleanupStaleSessions: scan count: %w", err)
		}
		return count, nil
	}
	result, err := db.Exec(
		`UPDATE sessions SET expired_at = datetime('now')
         WHERE last_active < datetime('now', '-4 hours') AND expired_at IS NULL`,
	)
	if err != nil {
		return 0, fmt.Errorf("CleanupStaleSessions: exec update: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("CleanupStaleSessions: rows affected: %w", err)
	}
	return n, nil
}
