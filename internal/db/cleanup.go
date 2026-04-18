package db

import (
	"database/sql"
	"fmt"
)

func CleanupStaleSessions(db *sql.DB, dryRun bool) (int64, error) {
	if dryRun {
		var count int64
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sessions
             WHERE last_active < datetime('now', '-7 days') AND expired_at IS NULL`,
		).Scan(&count)
		if err != nil {
			return 0, fmt.Errorf("CleanupStaleSessions: scan count: %w", err)
		}
		return count, nil
	}

	// Soft-delete: expire sessions inactive for > 7 days.
	result, err := db.Exec(
		`UPDATE sessions SET expired_at = datetime('now')
         WHERE last_active < datetime('now', '-7 days') AND expired_at IS NULL`,
	)
	if err != nil {
		return 0, fmt.Errorf("CleanupStaleSessions: expire stale: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("CleanupStaleSessions: rows affected: %w", err)
	}

	// Hard-delete: purge sessions expired for > 7 days — no audit value after that.
	// Also frees ID slots in the small wordlist (~100 words).
	if _, err := db.Exec(
		`DELETE FROM sessions WHERE expired_at < datetime('now', '-7 days')`,
	); err != nil {
		return n, fmt.Errorf("CleanupStaleSessions: purge expired: %w", err)
	}

	return n, nil
}
