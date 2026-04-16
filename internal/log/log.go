package log

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zalshy/tkt/internal/models"
)

// Execer is the minimal interface required by Append, allowing both *sql.DB and *sql.Tx.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Append inserts a new log entry for the given ticket.
//
// Validation rules:
//   - kind must be "transition", "plan", or "message"
//   - body must be non-empty
//   - if kind == "transition": both fromState and toState must be non-nil
//   - if kind != "transition": both must be nil
func Append(ctx context.Context, ticketID int64, kind, body string, fromState, toState *string, actor *models.Session, db Execer) error {
	switch kind {
	case "transition", "plan", "message":
		// valid
	default:
		return fmt.Errorf("log.Append: invalid kind %q", kind)
	}

	if body == "" {
		return fmt.Errorf("log.Append: body must not be empty")
	}

	if kind == "transition" {
		if fromState == nil || toState == nil {
			return fmt.Errorf("log.Append: fromState and toState must both be non-nil for kind %q", kind)
		}
	} else {
		if fromState != nil || toState != nil {
			return fmt.Errorf("log.Append: fromState and toState must be nil for kind %q", kind)
		}
	}

	const q = `INSERT INTO ticket_log (ticket_id, session_id, kind, body, from_state, to_state)
VALUES (?, ?, ?, ?, ?, ?)`

	if _, err := db.ExecContext(ctx, q,
		ticketID, actor.ID, kind, body, fromState, toState,
	); err != nil {
		return fmt.Errorf("log.Append: insert: %w", err)
	}
	return nil
}

// GetAll returns all non-deleted log entries for ticketID in ascending chronological order.
// ticketID may have an optional "#" prefix (e.g. "#1" or "1").
// The returned slice is always non-nil — callers may rely on len == 0 for the empty case.
func GetAll(ctx context.Context, ticketID string, db *sql.DB) ([]models.LogEntry, error) {
	id, err := parseTicketID(ticketID)
	if err != nil {
		return nil, err
	}

	const q = `SELECT id, ticket_id, session_id, kind, body, from_state, to_state, created_at, deleted_at
FROM ticket_log
WHERE ticket_id = ? AND deleted_at IS NULL
ORDER BY created_at ASC`

	rows, err := db.QueryContext(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("log.GetAll: query: %w", err)
	}
	defer rows.Close()

	entries := []models.LogEntry{}
	for rows.Next() {
		entry, err := scanLogEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("log.GetAll: scan: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("log.GetAll: rows: %w", err)
	}

	return entries, nil
}

// LatestPlan returns the most recently created plan entry for ticketID, or nil if none exists.
// ticketID may have an optional "#" prefix.
func LatestPlan(ctx context.Context, ticketID string, db *sql.DB) (*models.LogEntry, error) {
	id, err := parseTicketID(ticketID)
	if err != nil {
		return nil, err
	}

	const q = `SELECT id, ticket_id, session_id, kind, body, from_state, to_state, created_at, deleted_at
FROM ticket_log
WHERE ticket_id = ? AND kind = 'plan' AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT 1`

	row := db.QueryRowContext(ctx, q, id)

	entry, err := scanLogEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("log.LatestPlan: %w", err)
	}

	return &entry, nil
}

// scanner is satisfied by both *sql.Rows and *sql.Row.
type scanner interface {
	Scan(dest ...any) error
}

// scanLogEntry scans a single row into a models.LogEntry.
// It handles NullString for from_state/to_state and NullTime for deleted_at.
func scanLogEntry(s scanner) (models.LogEntry, error) {
	var entry models.LogEntry
	var fromStr, toStr sql.NullString
	var deletedAt sql.NullTime

	if err := s.Scan(
		&entry.ID,
		&entry.TicketID,
		&entry.SessionID,
		&entry.Kind,
		&entry.Body,
		&fromStr,
		&toStr,
		&entry.CreatedAt,
		&deletedAt,
	); err != nil {
		return models.LogEntry{}, err
	}

	if fromStr.Valid {
		st := models.Status(fromStr.String)
		entry.FromState = &st
	}
	if toStr.Valid {
		st := models.Status(toStr.String)
		entry.ToState = &st
	}
	if deletedAt.Valid {
		entry.DeletedAt = &deletedAt.Time
	}
	return entry, nil
}

// parseTicketID strips an optional "#" prefix and parses the string as int64.
func parseTicketID(ticketID string) (int64, error) {
	raw := strings.TrimPrefix(ticketID, "#")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("log: invalid ticket ID %q: %w", ticketID, err)
	}
	return id, nil
}
