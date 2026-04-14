package session

import (
	"database/sql"
	"fmt"

	"github.com/zalshy/tkt/internal/models"
)

// CountActive returns count of non-expired sessions broken down by base role.
// Keys are models.RoleArchitect and models.RoleImplementer.
// Roles not present in the result had zero active sessions.
func CountActive(db *sql.DB) (map[models.Role]int, error) {
	rows, err := db.Query(`
		SELECT r.base_role, COUNT(*)
		FROM sessions s
		JOIN roles r ON r.name = s.role
		WHERE s.expired_at IS NULL
		GROUP BY r.base_role
	`)
	if err != nil {
		return nil, fmt.Errorf("CountActive: query: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.Role]int)
	for rows.Next() {
		var role string
		var n int
		if err := rows.Scan(&role, &n); err != nil {
			return nil, fmt.Errorf("CountActive: scan: %w", err)
		}
		counts[models.Role(role)] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("CountActive: rows: %w", err)
	}
	return counts, nil
}
