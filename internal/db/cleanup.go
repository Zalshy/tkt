package db

import "database/sql"

func CleanupStaleSessions(db *sql.DB, dryRun bool) (int64, error) {
	if dryRun {
		var count int64
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sessions
             WHERE last_active < datetime('now', '-48 hours') AND expired_at IS NULL`,
		).Scan(&count)
		return count, err
	}
	result, err := db.Exec(
		`UPDATE sessions SET expired_at = datetime('now')
         WHERE last_active < datetime('now', '-48 hours') AND expired_at IS NULL`,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
