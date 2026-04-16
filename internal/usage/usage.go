package usage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zalshy/tkt/internal/models"
)

// Execer is the minimal interface required by Append.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Append inserts a new usage entry for the given ticket.
// tokens must be > 0.
func Append(ctx context.Context, ticketID int64, sessionID string, tokens, tools, durationMs int, agent, label string, db Execer) error {
	if tokens <= 0 {
		return fmt.Errorf("usage.Append: tokens must be > 0")
	}
	const q = `INSERT INTO ticket_usage (ticket_id, session_id, tokens, tools, duration_ms, agent, label)
VALUES (?, ?, ?, ?, ?, ?, ?)`
	if _, err := db.ExecContext(ctx, q, ticketID, sessionID, tokens, tools, durationMs, agent, label); err != nil {
		return fmt.Errorf("usage.Append: %w", err)
	}
	return nil
}

// GetForTicket returns all non-deleted usage entries for the given ticket,
// ordered by created_at ascending. Returns a non-nil empty slice when no rows exist.
func GetForTicket(ctx context.Context, ticketID int64, db *sql.DB) ([]models.UsageEntry, error) {
	const q = `SELECT id, ticket_id, session_id, tokens, tools, duration_ms, agent, label, created_at, deleted_at
FROM ticket_usage
WHERE ticket_id = ? AND deleted_at IS NULL
ORDER BY created_at ASC`
	rows, err := db.QueryContext(ctx, q, ticketID)
	if err != nil {
		return nil, fmt.Errorf("usage.GetForTicket: %w", err)
	}
	defer rows.Close()
	result := []models.UsageEntry{}
	for rows.Next() {
		var u models.UsageEntry
		if err := rows.Scan(&u.ID, &u.TicketID, &u.SessionID, &u.Tokens, &u.Tools, &u.DurationMs, &u.Agent, &u.Label, &u.CreatedAt, &u.DeletedAt); err != nil {
			return nil, fmt.Errorf("usage.GetForTicket: scan: %w", err)
		}
		result = append(result, u)
	}
	return result, rows.Err()
}
